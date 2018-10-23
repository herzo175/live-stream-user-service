package users

import (
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

	controller.schema = MakeSchema(controller.Controller.DBName, controller.Controller.DBClient)
}

func (controller *UserController) Register(w http.ResponseWriter, r *http.Request) {
	user := User{}
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
