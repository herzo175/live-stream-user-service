package users

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/herzo175/live-stream-user-service/src/util/requests"

	"github.com/gorilla/mux"

	"github.com/herzo175/live-stream-user-service/src/util/auth"
)

type UserController struct {
	logic *UserLogic
}

func MakeRouter(router *mux.Router, logic *UserLogic) {
	controller := UserController{}
	subrouter := router.PathPrefix("/users").Subrouter()

	controller.logic = logic

	subrouter.HandleFunc("", controller.Register).Methods("POST")
	subrouter.HandleFunc("/login", controller.Login).Methods("POST")
	subrouter.HandleFunc("/logout", controller.Logout)
	subrouter.HandleFunc("/authenticate_channel", controller.AuthenticateChannel)
	subrouter.HandleFunc("/send_verification_email", controller.SendVerificationEmail).Methods("POST")
	subrouter.HandleFunc("/verify_email/{token}", controller.VerifyEmail).Methods("GET")

	subrouter.HandleFunc("/send_password_reset", controller.SendResetPasswordEmail).Methods("POST")
	subrouter.HandleFunc("/reset_password", controller.ResetPassword).Methods("POST")

	subrouter.HandleFunc(
		"/set_email",
		requests.SetAuthenticated(&SetEmailRequest{}, &UserTokenBody{}, controller.SetEmail),
	).Methods("POST")

	subrouter.HandleFunc(
		"/me", requests.GetAuthenticated(&UserTokenBody{}, controller.Me),
	).Methods("GET")

	subrouter.HandleFunc(
		"/payment_sources", auth.IsAuthenticated(&UserTokenBody{}, os.Getenv("JWT_SIGNING_STRING"), controller.GetPaymentSources),
	).Methods("GET")

	subrouter.HandleFunc(
		"/add_source", auth.IsAuthenticated(&UserTokenBody{}, os.Getenv("JWT_SIGNING_STRING"), controller.AddPaymentSource),
	).Methods("POST")

	// TODO: generic delete func
	subrouter.HandleFunc(
		"/remove_source/{source_id}", requests.GetAuthenticated(&UserTokenBody{}, controller.RemoveCard),
	).Methods("DELETE")

	subrouter.HandleFunc(
		"/set_default", requests.SetAuthenticated(&SetSourceRequest{}, &UserTokenBody{}, controller.SetDefaultSource),
	).Methods("POST")
}

type VerifyEmailRequest struct {
	Name                 string `json:"name"`
	EmailVerifiedChannel string `json:"email_verified_channel"`
	Email                string `json:"email"`
}

// TODO: use built-in controller helpers
func (controller *UserController) SendVerificationEmail(w http.ResponseWriter, r *http.Request) {
	// get email
	verifyEmailRequest := VerifyEmailRequest{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&verifyEmailRequest)

	if err != nil {
		http.Error(w, "Unable to read email address", 400)
		log.Println(err)
		return
	}

	controllerError := controller.logic.SendVerificationEmail(
		verifyEmailRequest.Name,
		verifyEmailRequest.Email,
		verifyEmailRequest.EmailVerifiedChannel,
		os.Getenv("EMAIL_VERIFICATION_SIGNATURE"),
	)

	if controllerError != nil {
		http.Error(w, controllerError.Error.Error(), controllerError.StatusCode)
		log.Println(controllerError.Error)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (controller *UserController) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	token, hasInitalToken := mux.Vars(r)["token"]

	if !hasInitalToken {
		http.Redirect(
			w,
			r,
			fmt.Sprintf(
				"%s/verification?success=%s&message=%s",
				os.Getenv("UI_ENDPOINT"),
				"false",
				"token missing in request query string",
			),
			302,
		)
	}

	controllerError := controller.logic.VerifyEmail(
		token,
		os.Getenv("EMAIL_VERIFICATION_SIGNATURE"),
		os.Getenv("EMAIL_VERIFICATION_FINGERPRINT_KEY"),
	)

	if controllerError != nil {
		http.Redirect(
			w,
			r,
			fmt.Sprintf(
				"%s/verification?success=%s&message=%s",
				os.Getenv("UI_ENDPOINT"),
				"false",
				controllerError.Error.Error(),
			),
			302,
		)
	} else {
		http.Redirect(
			w,
			r,
			fmt.Sprintf(
				"%s/verification?success=%s&message=%s",
				os.Getenv("UI_ENDPOINT"),
				"true",
				"Email Successfully Verified",
			),
			302,
		)
	}
}

type UserRegisterRequest struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	Fingerprint []byte `json:"fingerprint"`
	CardNumber  string `json:"card_number"`
	ExpMonth    string `json:"exp_month"`
	ExpYear     string `json:"exp_year"`
	CVC         string `json:"cvc"`
}

func (controller *UserController) Register(w http.ResponseWriter, r *http.Request) {
	// TODO: use generic setter method
	userRegisterRequest := UserRegisterRequest{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&userRegisterRequest)

	if err != nil {
		http.Error(w, "Unable to read user register request", 400)
		log.Println(err)
		return
	}

	token, controllerError := controller.logic.Register(
		userRegisterRequest.Name,
		userRegisterRequest.Email,
		userRegisterRequest.Password,
		userRegisterRequest.CardNumber,
		userRegisterRequest.ExpMonth,
		userRegisterRequest.ExpYear,
		userRegisterRequest.CVC,
		os.Getenv("EMAIL_VERIFICATION_FINGERPRINT_KEY"),
		userRegisterRequest.Fingerprint,
	)

	if controllerError != nil {
		http.Error(w, controllerError.Error.Error(), controllerError.StatusCode)
		return
	}

	requests.Finish(*token, w)
}

type UserLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Requires email and password in body
func (controller *UserController) Login(w http.ResponseWriter, r *http.Request) {
	// TODO: redirect to intended link
	loginRequest := UserLoginRequest{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&loginRequest)

	if err != nil {
		http.Error(w, "Unable to decode user", 400)
		log.Println(err)
		return
	}

	token, controllerError := controller.logic.Login(loginRequest.Email, loginRequest.Password)

	if controllerError != nil {
		http.Error(w, controllerError.Error.Error(), controllerError.StatusCode)
		return
	}

	requests.Finish(token, w)
}

func (controller *UserController) Logout(w http.ResponseWriter, r *http.Request) {
	bearerStrings := strings.Split(r.Header.Get("Authorization"), "Bearer ")

	if len(bearerStrings) != 2 {
		http.Error(w, "Must provide a bearer token", 401)
		return
	}

	// add token to logout cache
	token := bearerStrings[1]
	controllerError := controller.logic.Logout(token)

	if controllerError != nil {
		http.Error(w, controllerError.Error.Error(), controllerError.StatusCode)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (controller *UserController) AuthenticateChannel(w http.ResponseWriter, r *http.Request) {
	params, err := ioutil.ReadAll(r.Body)

	if err != nil {
		http.Error(w, "Unable to read channel authentication request", 400)
		log.Println(err)
		return
	}

	resp, controllerErr := controller.logic.AuthenticateChannel(params)

	if controllerErr != nil {
		http.Error(w, controllerErr.Error.Error(), controllerErr.StatusCode)
		return
	}

	w.Write(resp)
}

type ResetPasswordEmailRequest struct {
	Email string `json:"email"`
}

func (controller *UserController) SendResetPasswordEmail(w http.ResponseWriter, r *http.Request) {
	// lookup user information by email
	resetPasswordEmailRequest := ResetPasswordEmailRequest{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&resetPasswordEmailRequest)

	if err != nil {
		http.Error(w, "Unable to read email address", 400)
		log.Println(err)
		return
	}

	controllerError := controller.logic.SendPasswordResetEmail(
		resetPasswordEmailRequest.Email,
		os.Getenv("PASSWORD_RESET_FINGERPRINT_KEY"),
		os.Getenv("PASSWORD_RESET_SIGNATURE"),
	)

	if controllerError != nil {
		http.Error(w, controllerError.Error.Error(), controllerError.StatusCode)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type SetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (controller *UserController) ResetPassword(w http.ResponseWriter, r *http.Request) {
	// TODO: refactor to create common pattern with verify email
	setPasswordRequest := SetPasswordRequest{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&setPasswordRequest)

	if err != nil {
		http.Error(w, "Unable to read reset password request", 400)
		log.Println(err)
		return
	}

	controllerErr := controller.logic.ResetPassword(
		setPasswordRequest.Token,
		"resetPassword",
		"secretReset",
		setPasswordRequest.Password,
	)

	if controllerErr != nil {
		http.Error(w, controllerErr.Error.Error(), controllerErr.StatusCode)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type SetEmailRequest struct {
	Fingerprint []byte `json:"fingerprint"`
}

func (controller *UserController) SetEmail(
	urlParams map[string]string,
	headers map[string][]string,
	body interface{},
	tokenBodyPointer interface{},
) *requests.ControllerError {
	userId := tokenBodyPointer.(*UserTokenBody).Id
	fingerprint := body.(*SetEmailRequest).Fingerprint

	return controller.logic.SetEmail(
		userId,
		os.Getenv("EMAIL_VERIFICATION_FINGERPRINT_KEY"),
		fingerprint,
	)
}

func (controller *UserController) Me(
	urlParams map[string]string,
	queryParams, headers map[string][]string,
	tokenBodyPointer interface{},
) (interface{}, *requests.ControllerError) {
	id := tokenBodyPointer.(*UserTokenBody).Id
	return controller.logic.GetById(id)
}

func (controller *UserController) GetPaymentSources(w http.ResponseWriter, r *http.Request, tokenBodyPointer interface{}) {
	// NOTE: paginated list in future?
	id := tokenBodyPointer.(*UserTokenBody).Id
	sources, controllerError := controller.logic.GetPaymentSources(id)

	if controllerError != nil {
		http.Error(w, controllerError.Error.Error(), controllerError.StatusCode)
		return
	}

	requests.Finish(sources, w)
}

func (controller *UserController) AddPaymentSource(w http.ResponseWriter, r *http.Request, tokenBodyPointer interface{}) {
	id := tokenBodyPointer.(*UserTokenBody).Id
	newPaymentSource := NewPaymentSource{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&newPaymentSource)

	if err != nil {
		http.Error(w, "Unable to read new source", 400)
		log.Println(err)
		return
	}

	cardSource, controllerError := controller.logic.AddSource(
		id,
		newPaymentSource.CardNumber,
		newPaymentSource.ExpMonth,
		newPaymentSource.ExpYear,
		newPaymentSource.CVC,
	)

	if controllerError != nil {
		http.Error(w, controllerError.Error.Error(), controllerError.StatusCode)
		return
	}

	b, err := json.Marshal(*cardSource)

	if err != nil {
		http.Error(w, "Unable to encode new card info", 500)
		log.Println(err)
		return
	}

	w.Write(b)
}

func (controller *UserController) RemoveCard(
	urlParams map[string]string,
	queryParams, headers map[string][]string,
	tokenBodyPointer interface{},
) (interface{}, *requests.ControllerError) {
	userId := tokenBodyPointer.(*UserTokenBody).Id
	sourceId := urlParams["source_id"]

	// NOTE: return structured result?
	return controller.logic.DeleteSource(userId, sourceId)
}

type SetSourceRequest struct {
	SourceId string `json:"source_id"`
}

func (controller *UserController) SetDefaultSource(
	urlParams map[string]string,
	headers map[string][]string,
	schemaPointer interface{},
	tokenBodyPointer interface{},
) *requests.ControllerError {
	userId := tokenBodyPointer.(*UserTokenBody).Id
	sourceId := schemaPointer.(*SetSourceRequest).SourceId
	return controller.logic.SetDefaultSource(userId, sourceId)
}
