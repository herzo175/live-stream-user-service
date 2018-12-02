package users

import (
	"time"
	"github.com/herzo175/live-stream-user-service/src/util/cache"
	"github.com/herzo175/live-stream-user-service/src/util/channels"
	"github.com/herzo175/live-stream-user-service/src/util/database"
	"github.com/herzo175/live-stream-user-service/src/util/requests"
	"github.com/herzo175/live-stream-user-service/src/util/emails"
	"fmt"
	"errors"
	"log"
	"os"
	"strconv"

	"github.com/herzo175/live-stream-user-service/src/util/auth"
	"github.com/herzo175/live-stream-user-service/src/util/payments"
	"golang.org/x/crypto/bcrypt"
	"github.com/google/uuid"
)

type UserLogic struct {
	db database.Database
	emailClient *emails.EmailClient
	notificationClient *channels.Client
	tokenCache cache.Cache
}

// TODO: split this file into multiple files within package
// TODO: permission groups
// NOTE: consider moving user db logic to another file if this gets too large
// TODO: created at/modified at
type User struct {
	Id               string     `json:"_id" gorm:"column:_id"`
	StripeCustomerId string     `json:"stripe_customer_id" gorm:"column:stripe_customer_id"`
	Name             string     `json:"name" gorm:"column:name"`
	Email            string     `json:"email" gorm:"column:email"`
	Password         string     `json:"password" gorm:"column:password;not null"`
}

func (User) TableName() string {
	return "public.live_stream_users"
}

func MakeUserLogic(
	db database.Database,
	emailClient *emails.EmailClient,
	notificationClient *channels.Client,
	tokenCache cache.Cache,
) *UserLogic {
	return &UserLogic{
		db: db,
		emailClient: emailClient,
		notificationClient: notificationClient,
		tokenCache: tokenCache,
	}
}

type NewPaymentSource struct {
	CardNumber string `json:"card_number"`
	ExpMonth   string `json:"exp_month"`
	ExpYear    string `json:"exp_year"`
	CVC        string `json:"cvc"`
}

type UserTokenBody struct {
	Id    string            `json:"_id"`
	Roles []auth.Permission `json:"roles"`
}

func (t UserTokenBody) HasPermission(service, role string) bool {
	for _, r := range t.Roles {
		if r.Service == service && r.Role == role {
			return true
		}
	}

	return false
}

func (config *UserLogic) Register(
	name,
	email,
	password,
	cardNumber,
	expMonth, 
	expYear,
	cvc,
	fingerprintValidationString string,
	fingerprint []byte,
) (*auth.TokenResponse, *requests.ControllerError) {
	// TODO: validate user (ex. check if password is good)
	var err error

	fingerprintText, err := auth.Decrypt(
		fingerprint,
		[]byte(fingerprintValidationString),
	)

	if err != nil {
		log.Printf("Unable to read email verification fingerprint: %v", err)
		return nil, &requests.ControllerError{
			StatusCode: 500,
			Error: errors.New("Unable to read email verification fingerprint"),
		}
	}

	if string(fingerprintText) != email {
		return nil, &requests.ControllerError{
			StatusCode: 403,
			Error: errors.New("Email verification fingerprint and email provided do not match"),
		}
	}

	user := User{}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	if err != nil {
		log.Printf("Unable to create password hash: %v", err)
		return nil, &requests.ControllerError{
			StatusCode: 500,
			Error: fmt.Errorf("Failed to save password for user with email %s", email),
		}
	}

	user.Password = string(hash)

	stripeCustomer, err := payments.CreateCustomer(email)

	if err != nil {
		log.Printf("Error creating new stripe customer: %v", err)
		return nil, &requests.ControllerError{
			StatusCode: 500,
			Error: fmt.Errorf("Failed to add create customer for user with email %s", email),
		}
	}

	user.StripeCustomerId = stripeCustomer.ID

	_, err = payments.AddSource(
		cardNumber,
		expMonth,
		expYear,
		cvc,
		stripeCustomer.ID,
	)

	if err != nil {
		log.Printf("Error adding source for customer: %v", err)
		return nil, &requests.ControllerError{
			StatusCode: 500,
			Error: fmt.Errorf("Failed to add source for customer %s", email),
		}
	}

	user.Id = uuid.New().String()
	user.Name = name
	user.Email = email

	err = config.db.Create(user)

	if err != nil {
		log.Printf("Failed to create user for %s", email)

		if err := payments.DeleteCustomer(stripeCustomer.ID); err != nil {
			log.Printf("Error deleting customer: %v", err)
			return nil, &requests.ControllerError{
				StatusCode: 500,
				Error: fmt.Errorf("Failed to produce remove customer with email %s", email),
			}
		}

		return nil, &requests.ControllerError{
			StatusCode: 500,
			Error: fmt.Errorf("Failed to produce save new user with email %s", email),
		}
	}

	token, err := auth.GenerateToken(user, os.Getenv("JWT_SIGNING_STRING"), 480)

	if err != nil {
		log.Printf("Error creating new user login token: %v", err)
		return nil, &requests.ControllerError{
			StatusCode: 500,
			Error: errors.New("Failed to produce login token"),
		}
	}

	return &token, nil
}

func (config *UserLogic) Login(email, plaintext string) (*auth.TokenResponse, *requests.ControllerError) {
	var err error

	user := User{}
	// err = schema.collection.Find(bson.M{"email": email}).One(&user)
	err = config.db.FindOne(&user, "EMAIL = ?", email)

	if err != nil {
		return nil, &requests.ControllerError{
			StatusCode: 404,
			Error: fmt.Errorf("Could not find user with email %s", email),
		}
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(plaintext))

	if err != nil {
		return nil, &requests.ControllerError{
			StatusCode: 403,
			Error: errors.New("Password did not match one provided"),
		}
	}

	token, err := auth.GenerateToken(user, os.Getenv("JWT_SIGNING_STRING"), 480)

	if err != nil {
		log.Printf("Could not generate login token for %s", email)
		return nil, &requests.ControllerError{
			StatusCode: 500,
			Error: errors.New("Could not generate login token"),
		}
	}

	return &token, nil
}

func (config *UserLogic) Logout(token string) *requests.ControllerError {
	if err := config.tokenCache.SetWithExpiration(token, true, time.Hour*24); err != nil {
		log.Println(err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error: errors.New("Unable to logout user"),
		}
	}

	return nil
}

func (config *UserLogic) AuthenticateChannel(req []byte) ([]byte, *requests.ControllerError) {
	resp, err := config.notificationClient.Authenticate(req)

	if err != nil {
		log.Printf("Unable to authenticate to notification channel: %v", err)
		return nil, &requests.ControllerError{
			StatusCode: 403,
			Error: errors.New("Unable to authenticate to notification channel"),
		}
	}

	return resp, nil
}

func (config *UserLogic) GetById(id string) (*User, *requests.ControllerError) {
	user := User{}
	err := config.db.FindOne(&user, "_ID = ?", id)

	if err != nil {
		log.Printf("User %s not found: %v", id, err)
		return nil, &requests.ControllerError{
			StatusCode: 404,
			Error: fmt.Errorf("User %s not found", id),
		}
	}

	return &user, nil
}

func (config *UserLogic) GetByEmail(email string) (*User, *requests.ControllerError) {
	user := User{}
	err := config.db.FindOne(&user, "EMAIL = ?", email)

	if err != nil {
		log.Printf("User with email %s not found: %v", email, err)
		return nil, &requests.ControllerError{
			StatusCode: 404,
			Error: fmt.Errorf("User with email %s not found", email),
		}
	}

	return &user, nil
}

type PaymentSourceMeta struct {
	Id       string `json:"id"`
	Brand    string `json:"brand"`
	LastFour string `json:"last_four"`
	ExpMonth string `json:"exp_month"`
	ExpYear  string `json:"exp_year"`
}

func (config *UserLogic) GetPaymentSources(id string) ([]PaymentSourceMeta, *requests.ControllerError) {
	var err error

	user := User{}
	err = config.db.FindOne(&user, "_ID = ?", id)

	if err != nil {
		log.Printf("Could not find user %s to get payment sources", id)
		return nil, &requests.ControllerError{
			StatusCode: 404,
			Error: fmt.Errorf("Could not find user with id %s", id),
		}
	}

	stripeUser, err := payments.GetCustomer(user.StripeCustomerId)

	if err != nil {
		log.Printf("Could not find customer %s to get payment sources", user.StripeCustomerId)
		return nil, &requests.ControllerError{
			StatusCode: 500,
			Error: fmt.Errorf("Could not find customer for user with id %s", id),
		}
	}

	sources := []PaymentSourceMeta{}

	for _, cardSource := range stripeUser.Sources.Data {
		if cardSource.Deleted {
			continue
		}

		source := PaymentSourceMeta{}

		source.LastFour = cardSource.Card.Last4
		source.Brand = string(cardSource.Card.Brand)
		source.ExpMonth = strconv.Itoa(int(cardSource.Card.ExpMonth))
		source.ExpYear = strconv.Itoa(int(cardSource.Card.ExpYear))
		source.Id = cardSource.ID

		sources = append(sources, source)
	}

	return sources, nil
}

type UpdateEmail struct {
	Email string `json:"email"`
}

type UpdatePassword struct {
	Password string `json:"password"`
}

func (config *UserLogic) DeleteSource(userId, sourceId string) (string, *requests.ControllerError) {
	user, controllerError := config.GetById(userId)

	if controllerError != nil {
		return "", controllerError
	}

	stripeUser, err := payments.GetCustomer(user.StripeCustomerId)

	if err != nil {
		log.Printf("Could not find customer associated with user %s: %v", userId, err)
		return "", &requests.ControllerError{
			StatusCode: 500,
			Error: errors.New("Could not find customer associated with current user"),
		}
	}

	var deleteSourceId, newDefaultId string

	for _, cardSource := range stripeUser.Sources.Data {
		if cardSource.Deleted {
			continue
		} else if cardSource.ID == sourceId {
			deleteSourceId = cardSource.ID
		} else {
			newDefaultId = cardSource.ID
		}

		if newDefaultId != "" && deleteSourceId != "" {
			break
		}
	}

	if deleteSourceId == "" {
		return "", &requests.ControllerError{
			StatusCode: 400,
			Error: errors.New("No payment source matches the source id provided"),
		}
	}

	if newDefaultId == "" {
		return "", &requests.ControllerError{
			StatusCode: 400,
			Error: errors.New("No payment source to set to default"),
		}
	}

	// set default to new default source if deleting their default source
	if stripeUser.DefaultSource.ID != deleteSourceId {
		err = payments.SetDefaultSource(user.StripeCustomerId, newDefaultId)

		if err != nil {
			log.Printf("Error setting new default source: %v", err)
			return "", &requests.ControllerError{
				StatusCode: 500,
				Error: errors.New("Unable to set new default source"),
			}
		}
	}

	// delete source
	err = payments.DetatchSource(user.StripeCustomerId, deleteSourceId)

	if err != nil {
		log.Printf("Error removing source: %v", err)
		return "", &requests.ControllerError{
			StatusCode: 500,
			Error: errors.New("Unable to remove source"),
		}
	}

	return newDefaultId, nil
}

func (config *UserLogic) AddSource(userId, cardNumber, expMonth, expYear, cvc string) (*PaymentSourceMeta, *requests.ControllerError) {
	user, controllerError := config.GetById(userId)

	if controllerError != nil {
		return nil, controllerError
	}

	cardSource, err := payments.AddSource(
		cardNumber,
		expMonth,
		expYear,
		cvc,
		user.StripeCustomerId,
	)

	if err != nil {
		log.Printf("An error occured while saving a new payment source for user %s: %v", userId, err)
		return nil, &requests.ControllerError{
			StatusCode: 500,
			Error: errors.New("Unable to save new payment source"),
		}
	}

	source := PaymentSourceMeta{}
	source.LastFour = cardSource.Card.Last4
	source.Brand = string(cardSource.Card.Brand)
	source.ExpMonth = strconv.Itoa(int(cardSource.Card.ExpMonth))
	source.ExpYear = strconv.Itoa(int(cardSource.Card.ExpYear))

	return &source, nil
}

func (config *UserLogic) SetDefaultSource(userId, sourceId string) *requests.ControllerError {
	user, controllerError := config.GetById(userId)

	if controllerError != nil {
		return controllerError
	}

	err := payments.SetDefaultSource(user.StripeCustomerId, sourceId)

	if err != nil {
		log.Printf("Error setting default source: %v", err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error: errors.New("Unable to set new default source"),
		}
	}

	return nil
}

type VerifyEmailTokenBody struct {
	Email                string `json:"email"`
	EmailVerifiedChannel string `json:"email_verified_channel"`
}

func (config *UserLogic) SendVerificationEmail(
	name,
	emailAddress,
	emailVerifiedChannel,
	verificationSignature string,
) *requests.ControllerError {
	var err error

	token, err := auth.GenerateToken(
		VerifyEmailTokenBody{
			Email: emailAddress,
			EmailVerifiedChannel: emailVerifiedChannel,
		},
		verificationSignature,
		60,
	)

	if err != nil {
		log.Printf("Error generating verification token: %v", err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error: errors.New("Unable to generate verification token"),
		}
	}

	// load verification template and send
	// TODO: make HTML template
	// TODO: move to config
	email := emails.Email{
		To:          name,
		ToAddress:   emailAddress,
		From:        "Jeremy",
		FromAddress: "jeremyaherzog@gmail.com",
		Subject:     "Email Verification Required",
		PlainText:   fmt.Sprintf("Click the link to verify: http://localhost:3001/users/verify_email/%s", token.Token),
		HtmlText:    fmt.Sprintf("Click the link to verify: http://localhost:3001/users/verify_email/%s", token.Token),
	}

	err = config.emailClient.Send(email)

	if err != nil {
		log.Printf("Error sending verification email: %v", err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error: fmt.Errorf("Failed to send verification email to %s", emailAddress),
		}
	}

	return nil
}

type FingerprintTokenBody struct {
	Fingerprint []byte `json:"fingerprint"`
}

func (config *UserLogic) SendPasswordResetEmail(
	emailAddress,
	resetFingerprintKey,
	resetTokenKey string,
) *requests.ControllerError {
	user, controllerError := config.GetByEmail(emailAddress)

	if controllerError != nil {
		log.Printf(
			"Unable to get user by email to send password reset: %v",
			controllerError.Error.Error(),
		)
		return controllerError
	}

	// create fingerprint of email to ensure identity
	// TODO: store one time encrpytion key in redis
	fingerprintText, err := auth.Encrpyt(
		[]byte(user.Id),
		[]byte(resetFingerprintKey),
	)

	if err != nil {
		log.Printf("Error encrpyting user id fingerprint: %v", err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error: errors.New("Could not generate user id fingerprint"),
		}
	}

	fingerprint := FingerprintTokenBody{
		Fingerprint: fingerprintText,
	}

	// put fingerprint in email with expiring token
	token, err := auth.GenerateToken(fingerprint, resetTokenKey, 60)

	if err != nil {
		log.Printf("Error generating password reset token: %v", err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error: errors.New("Could not generate password reset token"),
		}
	}

	// load verification template and send
	// TODO: move to config
	email := emails.Email{
		To:          user.Name,
		ToAddress:   emailAddress,
		From:        "Jeremy",
		FromAddress: "jeremyaherzog@gmail.com",
		Subject:     "Reset Password Request",
		PlainText:   fmt.Sprintf("Click the link to verify: %s/reset_password/%s", os.Getenv("UI_ENDPOINT"), token.Token),
		HtmlText:    fmt.Sprintf("Click the link to verify: %s/reset_password/%s", os.Getenv("UI_ENDPOINT"), token.Token),
	}

	err = config.emailClient.Send(email)

	if err != nil {
		log.Printf("Unable to send password reset email: %v", err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error: errors.New("Could not send password reset email"),
		}
	}

	return nil
}

func (config *UserLogic) VerifyEmail(
	tokenString, 
	verificationSignature,
	fingerprintSignature string,
) *requests.ControllerError {
	var err error

	// check if token has already been used
	alreadyUsed, err := config.tokenCache.Exists(tokenString)

	if err != nil {
		return &requests.ControllerError{StatusCode: 500, Error: err}
	}

	if alreadyUsed {
		return &requests.ControllerError{StatusCode: 400, Error: errors.New("Token has already been used")}
	}

	// set token
	if err := config.tokenCache.SetWithExpiration(tokenString, true, time.Hour*24); err != nil {
		log.Printf("Error expiring expiration token: %v", err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error: errors.New("Could not invalidate email verification token"),
		}
	}

	// check token in email
	verifyEmailTokenBody := VerifyEmailTokenBody{}
	err = auth.ValidateToken(tokenString, &verifyEmailTokenBody, verificationSignature)

	if err != nil {
		return &requests.ControllerError{
			StatusCode: 403,
			Error: errors.New("Could not verify email verification token"),
		}
	}

	// create fingerprint token to send to client
	fingerprintText, err := auth.Encrpyt(
		[]byte(verifyEmailTokenBody.Email),
		[]byte(fingerprintSignature),
	)

	if err != nil {
		log.Printf("Error encrpyting email fingerprint: %v", err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error: errors.New("Could not generate email fingerprint"),
		}
	}

	fingerprint := FingerprintTokenBody{
		Fingerprint: fingerprintText,
	}

	// send over client channel
	err = config.notificationClient.Send(
		verifyEmailTokenBody.EmailVerifiedChannel,
		channels.EMAIL_VERIFIED_EVENT,
		fingerprint,
	)

	if err != nil {
		log.Printf("Error sending email fingerprint to client: %v", err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error: errors.New("Could not send email fingerprint to client"),
		}
	}

	return nil
}

func (config *UserLogic) ResetPassword(
	token,
	tokenVerificationString,
	userIdFingerprintKey,
	password string,
) *requests.ControllerError {
	// check if token is valid and hasn't been used yet
	alreadyUsed, err := config.tokenCache.Exists(token)

	if err != nil {
		log.Printf("Error checking reset password token in cache: %v", err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error: errors.New("Failed to check reset password token"),
		}
	}

	if alreadyUsed {
		return &requests.ControllerError{
			StatusCode: 401,
			Error: errors.New("Reset password token has been used already"),
		}
	} else {
		if err := config.tokenCache.SetWithExpiration(token, true, time.Hour*24); err != nil {
			log.Printf("Error invalidating reset password token in cache: %v", err)
			return &requests.ControllerError{
				StatusCode: 500,
				Error: errors.New("Could not invalid reset password token"),
			}
		}
	}

	// TODO: move secret to config
	fingerprintTokenBody := FingerprintTokenBody{}
	err = auth.ValidateToken(token, &fingerprintTokenBody, tokenVerificationString)

	if err != nil {
		return &requests.ControllerError{
			StatusCode: 403,
			Error: errors.New("Reset password token is invalid"),
		}
	}

	// decrypt fingerprint to ensure user id is the same
	// TODO: store one time encrpytion key in redis
	fingerprintText, err := auth.Decrypt(
		[]byte(fingerprintTokenBody.Fingerprint),
		[]byte(userIdFingerprintKey),
	)

	if err != nil {
		log.Printf("Unable to decrypt reset password fingerprint: %v", err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error: errors.New("Unable to ensure reset password request is authentic"),
		}
	}

	userId := string(fingerprintText)

	// hash new password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	if err != nil {
		log.Printf("Unable to hash new password: %v", err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error: errors.New("Unable to secure new password"),
		}
	}

	user, controllerError := config.GetById(userId)

	if controllerError != nil {
		return controllerError
	}

	err = config.db.Update(
		user,
		map[string]interface{}{"password": string(hash)},
		"_id = ?", user.Id,
	)

	if err != nil {
		log.Printf("Unable to save new password: %v", err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error: errors.New("Unable to save new password"),
		}
	}

	return nil
}

func (config *UserLogic) SetEmail(userId, fingerprintValidationString string, emailFingerprint []byte) *requests.ControllerError {
	// decrpyt token fingerprint
	// TODO: store one time encrpytion key in redis
	fingerprintText, err := auth.Decrypt(
		emailFingerprint,
		[]byte(fingerprintValidationString),
	)

	if err != nil {
		log.Printf("Unable to read email verification fingerprint: %v", err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error: errors.New("Unable to read email verification fingerprint"),
		}
	}

	email := string(fingerprintText)

	// set email to result of fingerprint
	user, controllerError := config.GetById(userId)

	if controllerError != nil {
		return controllerError
	}

	err = config.db.Update(user, map[string]interface{}{"email": email}, "_id = ?", user.Id)

	if err != nil {
		log.Printf("Failed to save new user email to %s: %v", email, err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error: errors.New("Failed to save new user email"),
		}
	}

	return nil
}
