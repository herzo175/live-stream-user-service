package users

import (
	"strconv"
	"github.com/herzo175/live-stream-user-service/src/util/payments"
	"github.com/herzo175/live-stream-user-service/src/util/auth"
	"encoding/json"
	"log"
	"net/http"

	"github.com/herzo175/live-stream-user-service/src/bundles"
)

type UserController struct {
	Controller *bundles.Controller
	schema     *Schema
}

func (controller *UserController) MakeRouter() {
	subrouter := controller.Controller.Router.PathPrefix("/users").Subrouter()
	subrouter.HandleFunc("", controller.Register).Methods("POST")
	subrouter.HandleFunc("/login", controller.Login).Methods("POST")

	subrouter.HandleFunc(
		"/me", bundles.GetAuthenticated(&UserTokenBody{}, controller.Me),
	).Methods("GET")

	subrouter.HandleFunc(
		"/payment_sources", auth.IsAuthenticated(&UserTokenBody{}, controller.GetPaymentSources),
	).Methods("GET")

	subrouter.HandleFunc(
		"/add_source", auth.IsAuthenticated(&UserTokenBody{}, controller.AddPaymentSource),
	).Methods("POST")

	controller.schema = MakeSchema(controller.Controller.DB)
}

func (controller *UserController) Register(w http.ResponseWriter, r *http.Request) {
	user := UserCreate{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&user)

	if err != nil {
		http.Error(w, "Unable to read user", 400)
		log.Println(err)
		return
	}

	token, err := controller.schema.Register(&user)

	if err != nil {
		http.Error(w, "An error occured while saving the user", 500)
		log.Println(err)
		log.Print(user)
		return
	}

	bundles.Finish(token, w)
}

// Requires email and password in body
func (controller *UserController) Login(w http.ResponseWriter, r *http.Request) {
	user := User{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&user)

	if err != nil {
		http.Error(w, "Unable to decode user", 400)
		log.Println(err)
		return
	}

	token, err := controller.schema.Login(user.Email, user.Password)

	if err != nil {
		http.Error(w, "Unable to authenticate user", 401)
		log.Println(err)
		return
	}

	bundles.Finish(token, w)
}

func (controller *UserController) Me(
	urlParams map[string]string,
	queryParams, headers map[string][]string,
	tokenBodyPointer interface{},
) (interface{}, error) {
	id := tokenBodyPointer.(*UserTokenBody).Id
	return controller.schema.GetById(id)
}

func (controller *UserController) GetPaymentSources(w http.ResponseWriter, r *http.Request, tokenBodyPointer interface{}) {
	// TODO: paginated list?
	id := tokenBodyPointer.(*UserTokenBody).Id
	sources, err := controller.schema.GetPaymentSources(id)

	if err != nil {
		http.Error(w, "An error occured while fetching sources", 500)
		log.Println(err)
		return
	}

	b, err := json.Marshal(sources)

	if err != nil {
		http.Error(w, "An error occured while fetching sources", 500)
		log.Println(err)
		return
	}

	w.Write(b)
}

func (controller *UserController) AddPaymentSource(w http.ResponseWriter, r *http.Request, tokenBodyPointer interface{}) {
	id := tokenBodyPointer.(*UserTokenBody).Id
	user, err := controller.schema.GetById(id)

	if err != nil {
		http.Error(w, "Unable to retrieve user details", 404)
		log.Println(err)
		return
	}

	newPaymentSource := NewPaymentSource{}
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&newPaymentSource)

	if err != nil {
		http.Error(w, "Unable to add read source", 400)
		log.Println(err)
		return
	}

	cardSource, err := payments.AddSource(
		newPaymentSource.CardNumber,
		newPaymentSource.ExpMonth,
		newPaymentSource.ExpYear,
		newPaymentSource.CVC,
		user.StripeCustomerId,
	)

	if err != nil {
		http.Error(w, "Unable to add new payment source", 500)
		log.Println(err)
		return
	}

	source := PaymentSourceMeta{}

	source.LastFour = cardSource.Card.Last4
	source.Brand = string(cardSource.Card.Brand)
	source.ExpMonth = strconv.Itoa(int(cardSource.Card.ExpMonth))
	source.ExpYear = strconv.Itoa(int(cardSource.Card.ExpYear))

	b, err := json.Marshal(cardSource)

	if err != nil {
		http.Error(w, "Unable to encode new card info", 500)
		log.Println(err)
		return
	}

	w.Write(b)
}
