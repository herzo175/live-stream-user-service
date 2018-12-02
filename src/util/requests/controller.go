package requests

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/herzo175/live-stream-user-service/src/util/auth"

	"github.com/gorilla/mux"
)

type ControllerError struct {
	StatusCode int
	Error      error
}

// NOTE: find better ways to dry up outerwear
func Finish(data interface{}, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(data)

	if err != nil {
		http.Error(w, "Unable to serialize into JSON", 500)
		log.Println(err)
		return
	}

	w.Write(b)
}

type SetterFunc func(
	urlParams map[string]string,
	headers map[string][]string,
	schemaPointer interface{},
) *ControllerError

type AuthenticatedSetterFunc func(
	urlParams map[string]string,
	headers map[string][]string,
	schemaPointer interface{},
	tokenBodyPointer interface{},
) *ControllerError

func Set(setter SetterFunc, schemaType interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(schemaType)

		if err != nil {
			http.Error(w, "Unable to decode body: "+err.Error(), 500)
			log.Println(err)
			return
		}

		controllerError := setter(mux.Vars(r), r.Header, schemaType)

		if controllerError != nil {
			http.Error(w, controllerError.Error.Error(), controllerError.StatusCode)
			log.Println(controllerError.Error)
			return
		}

		Finish(schemaType, w)
	}
}

func SetAuthenticated(schemaType interface{}, tokenBody auth.TokenBody, setter AuthenticatedSetterFunc) http.HandlerFunc {
	return auth.IsAuthenticated(
		tokenBody,
		os.Getenv("JWT_SIGNING_STRING"),
		func(w http.ResponseWriter, r *http.Request, tokenBodyPointer interface{}) {
			decoder := json.NewDecoder(r.Body)
			err := decoder.Decode(schemaType)

			if err != nil {
				http.Error(w, "Unable to decode body: "+err.Error(), 500)
				log.Println(err)
				return
			}

			controllerError := setter(mux.Vars(r), r.Header, schemaType, tokenBodyPointer)

			if err != nil {
				http.Error(w, controllerError.Error.Error(), controllerError.StatusCode)
				log.Println(controllerError.Error)
				return
			}

			Finish(schemaType, w)
		},
	)
}

type GetterFunc func(
	urlParams map[string]string,
	queryParams, headers map[string][]string,
) (data interface{}, err *ControllerError)

type AuthenticatedGetterFunc func(
	urlParams map[string]string,
	queryParams,
	headers map[string][]string,
	tokenBodyPointer interface{},
) (data interface{}, err *ControllerError)

func Get(getter GetterFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// NOTE: abstract further?
		data, err := getter(mux.Vars(r), r.URL.Query(), r.Header)

		if err != nil {
			http.Error(w, err.Error.Error(), err.StatusCode)
			log.Println(err.Error)
			return
		}

		Finish(data, w)
	}
}

func GetAuthenticated(tokenBody auth.TokenBody, getter AuthenticatedGetterFunc) http.HandlerFunc {
	return auth.IsAuthenticated(
		tokenBody,
		os.Getenv("JWT_SIGNING_STRING"),
		func(w http.ResponseWriter, r *http.Request, tokenBodyPointer interface{}) {
			data, err := getter(mux.Vars(r), r.URL.Query(), r.Header, tokenBodyPointer)

			if err != nil {
				http.Error(w, err.Error.Error(), err.StatusCode)
				log.Println(err.Error)
				return
			}

			Finish(data, w)
		},
	)
}
