package users

import (
	"github.com/herzo175/live-stream-user-service/src/util/auth"
	"github.com/gorilla/mux"
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
	subrouter.HandleFunc("/{id}", auth.IsAuthenticated(controller.GetById)).Methods("GET")

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

	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(token)

	if err != nil {
		http.Error(w, "Unable to format token", 500)
		log.Println(err)
		return
	}

	w.Write(data)
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

	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(token)

	if err != nil {
		http.Error(w, "Unable to format token", 500)
		log.Println(err)
		return
	}

	w.Write(data)
}

func (controller *UserController) GetById(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	user, err := controller.schema.GetById(id)

	if err != nil {
		http.Error(w, "Unable to find user", 404)
		log.Println(err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(user)

	if err != nil {
		http.Error(w, "Unable to format found user", 500)
		log.Println(err)
		log.Println(user)
		return
	}

	w.Write(data)
}
