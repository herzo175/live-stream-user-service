package users

import (
	"log"
	"strconv"

	"github.com/herzo175/live-stream-user-service/src/util/auth"
	"github.com/herzo175/live-stream-user-service/src/util/payments"
	"golang.org/x/crypto/bcrypt"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// TODO: update billing info
// TODO: reset password, email
// TODO: permission groups
// TODO: error handling struct instead of returning err
type User struct {
	Id               bson.ObjectId     `json:"_id" bson:"_id,omitempty"`
	StripeCustomerId string            `json:"stripe_customer_id" bson:"stripe_customer_id,omitempty"`
	Name             string            `json:"name" bson:"name,omitempty"`
	Email            string            `json:"email" bson:"email,omitempty"`
	Password         string            `json:"password" bson:"password,omitempty"`
	Roles            []auth.Permission `json:"roles" bson:"roles,omitempty"`
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

type Schema struct {
	collection *mgo.Collection
}

func MakeSchema(db *mgo.Database) *Schema {
	schema := Schema{}
	schema.collection = db.C("Users")

	// TODO: notify if duplicate key
	index := mgo.Index{
		Key:      []string{"email"},
		Unique:   true,
		DropDups: true,
	}

	err := schema.collection.EnsureIndex(index)

	if err != nil {
		log.Fatal(err)
	}

	return &schema
}

type UserCreate struct {
	Id               bson.ObjectId `bson:"_id"`
	Name             string        `json:"name" bson:"name"`
	Email            string        `json:"email" bson:"email"`
	Password         string        `json:"password" bson:"password"`
	StripeCustomerId string        `bson:"stripe_customer_id"`
	NewPaymentSource
}

type NewPaymentSource struct {
	CardNumber string `json:"card_number"`
	ExpMonth   string `json:"exp_month"`
	ExpYear    string `json:"exp_year"`
	CVC        string `json:"cvc"`
}

type PaymentSourceMeta struct {
	Brand    string `json:"brand"`
	LastFour string `json:"last_four"`
	ExpMonth string `json:"exp_month"`
	ExpYear  string `json:"exp_year"`
}

func (schema *Schema) Register(userCreate *UserCreate) (token auth.TokenResponse, err error) {
	// TODO: validate user (ex. check if password is good)
	// TODO: confirm email mechanism
	user := User{}
	hash, err := bcrypt.GenerateFromPassword([]byte(userCreate.Password), bcrypt.DefaultCost)

	if err != nil {
		return token, err
	}

	user.Password = string(hash)

	stripeCustomer, err := payments.CreateCustomer(userCreate.Email)

	if err != nil {
		return token, err
	}

	user.StripeCustomerId = stripeCustomer.ID

	_, err = payments.AddSource(
		userCreate.NewPaymentSource.CardNumber,
		userCreate.NewPaymentSource.ExpMonth,
		userCreate.NewPaymentSource.ExpYear,
		userCreate.NewPaymentSource.CVC,
		stripeCustomer.ID,
	)

	if err != nil {
		return token, err
	}

	user.Id = bson.NewObjectId()
	user.Name = userCreate.Name
	user.Email = userCreate.Email
	err = schema.collection.Insert(user)

	if err != nil {
		// TODO: remove customer
		return token, err
	}

	return auth.GenerateToken(user)
}

func (schema *Schema) Login(email, plaintext string) (token auth.TokenResponse, err error) {
	user := User{}
	err = schema.collection.Find(bson.M{"email": email}).One(&user)

	if err != nil {
		return token, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(plaintext))

	if err != nil {
		return token, err
	}

	return auth.GenerateToken(user)
}

func (schema *Schema) GetById(id string) (user *User, err error) {
	user = &User{}
	err = schema.collection.Find(bson.M{"_id": bson.ObjectIdHex(id)}).One(user)

	if err != nil {
		return user, err
	}

	return user, nil
}

func (schema *Schema) GetPaymentSources(id string) (sources []PaymentSourceMeta, err error) {
	user, err := schema.GetById(id)

	if err != nil {
		return nil, err
	}

	stripeUser, err := payments.GetCustomer(user.StripeCustomerId)

	if err != nil {
		return nil, err
	}

	sources = []PaymentSourceMeta{}

	for _, cardSource := range stripeUser.Sources.Data {
		source := PaymentSourceMeta{}

		source.LastFour = cardSource.Card.Last4
		source.Brand = string(cardSource.Card.Brand)
		source.ExpMonth = strconv.Itoa(int(cardSource.Card.ExpMonth))
		source.ExpYear = strconv.Itoa(int(cardSource.Card.ExpYear))

		sources = append(sources, source)
	}

	return sources, nil
}
