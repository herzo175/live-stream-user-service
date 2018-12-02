package servers

import (
	"errors"
	"github.com/herzo175/live-stream-user-service/src/util/database"
	"github.com/herzo175/live-stream-user-service/src/util/requests"
	"os"
	"net/http"
	"github.com/gorilla/mux"
	"github.com/herzo175/live-stream-user-service/src/util/auth"
	"github.com/herzo175/live-stream-user-service/src/bundles/users"
)

type ServerController struct {
	logic ServerLogic
}

func MakeRouter(router *mux.Router, logic ServerLogic) {
	controller := ServerController{}
	subrouter := router.PathPrefix("/servers").Subrouter()

	controller.logic = logic

	subrouter.HandleFunc(
		"", requests.GetAuthenticated(&users.UserTokenBody{}, controller.Get),
	).Methods("GET")

	subrouter.HandleFunc(
		"", requests.SetAuthenticated(&CreateServerRequest{}, &users.UserTokenBody{}, controller.Create),
	).Methods("POST")

	subrouter.HandleFunc(
		"/{id}", requests.GetAuthenticated(&users.UserTokenBody{}, controller.GetById),
	).Methods("GET")

	subrouter.HandleFunc(
		"/{id}", requests.SetAuthenticated(&ServerInfoUpdateRequest{}, &users.UserTokenBody{}, controller.Update),
	).Methods("PUT")

	// TODO: set authorized
	subrouter.HandleFunc(
		"/{id}/status", requests.SetAuthenticated(&ServerStatusUpdate{}, &users.UserTokenBody{}, controller.SetStatus),
	).Methods("PUT")

	// TODO: set authorized
	subrouter.HandleFunc(
		"/{id}/ip_address", requests.SetAuthenticated(&ServerIpAddressUpdate{}, &users.UserTokenBody{}, controller.SetIpAddress),
	).Methods("PUT")

	subrouter.HandleFunc(
		"/{id}/restart", auth.IsAuthenticated(&users.UserTokenBody{}, os.Getenv("JWT_SIGNING_STRING"), controller.Restart),
	).Methods("POST")

	subrouter.HandleFunc(
		"/{id}", auth.IsAuthenticated(&users.UserTokenBody{}, os.Getenv("JWT_SIGNING_STRING"), controller.Delete),
	).Methods("DELETE")
	
	controller.logic = logic
}

func (controller *ServerController) Get(
	urlParams map[string]string,
	queryParams, headers map[string][]string,
	tokenBodyPointer interface{},
) (interface{}, *requests.ControllerError) {
	start, end, err := database.ExtractStartEnd(queryParams)

	if err != nil {
		return nil, &requests.ControllerError{
			StatusCode: 400,
			Error: errors.New("Failed to extract start and end of request"),
		}
	}

	userId := tokenBodyPointer.(*users.UserTokenBody).Id
	filter, err := database.TranslateQueryMap(queryParams, Server{})

	if err != nil {
		return nil, &requests.ControllerError{
			StatusCode: 400,
			Error: errors.New("Failed to translate query params to filter expressions"),
		}
	}

	return controller.logic.Get(userId, filter, int(start), int(end))
}

type CreateServerRequest struct {
	ServerName string `json:"server_name"`
	StreamName string `json:"stream_name"`
}

func (controller *ServerController) Create(
	urlParams map[string]string,
	headers map[string][]string,
	body interface{},
	tokenBodyPointer interface{},
) *requests.ControllerError {
	userId := tokenBodyPointer.(*users.UserTokenBody).Id
	createServerRequest := body.(*CreateServerRequest)

	return controller.logic.Create(userId, createServerRequest.ServerName, createServerRequest.StreamName)
}

func (controller *ServerController) GetById(
	urlParams map[string]string,
	queryParams,
	headers map[string][]string,
	tokenBodyPointer interface{},
) (interface{}, *requests.ControllerError) {
	id := urlParams["id"]
	userId := tokenBodyPointer.(*users.UserTokenBody).Id
	return controller.logic.GetById(id, userId)
}

type ServerInfoUpdateRequest struct {
	ServerName string `json:"server_name,omitempty"`
	StreamName string `json:"stream_name,omitempty"`
}

func (controller *ServerController) Update(
	urlParams map[string]string,
	headers map[string][]string,
	body interface{},
	tokenBodyPointer interface{},
) *requests.ControllerError {
	id := urlParams["id"]
	userId := tokenBodyPointer.(*users.UserTokenBody).Id
	newInfo := body.(*ServerInfoUpdateRequest)

	return controller.logic.UpdateServerInfo(id, userId, newInfo.ServerName, newInfo.StreamName)
}

type ServerStatusUpdate struct {
	Status Status `json:"status" bson:"status,omitempty"`
}

func (controller *ServerController) SetStatus(
	urlParams map[string]string,
	headers map[string][]string,
	body interface{},
	tokenBodyPointer interface{},
) *requests.ControllerError {
	id := urlParams["id"]
	statusUpdate := body.(*ServerStatusUpdate)

	return controller.logic.SetStatus(id, statusUpdate.Status)
}

type ServerIpAddressUpdate struct {
	IpAddress string `json:"ip_address"`
}

func (controller *ServerController) SetIpAddress(
	urlParams map[string]string,
	headers map[string][]string,
	body interface{},
	tokenBodyPointer interface{},
) *requests.ControllerError {
	id := urlParams["id"]
	serverIpAddressUpdate := body.(*ServerIpAddressUpdate)

	return controller.logic.SetIpAddress(id, serverIpAddressUpdate.IpAddress)
}

func (controller *ServerController) Restart(w http.ResponseWriter, r *http.Request, tokenBodyPointer interface{}) {
	id := mux.Vars(r)["id"]
	userId := tokenBodyPointer.(*users.UserTokenBody).Id
	
	controllerErr := controller.logic.RestartServer(id, userId)

	if controllerErr != nil {
		http.Error(w, controllerErr.Error.Error(), controllerErr.StatusCode)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (controller *ServerController) Delete(w http.ResponseWriter, r *http.Request, tokenBodyPointer interface{}) {
	id := mux.Vars(r)["id"]
	userId := tokenBodyPointer.(*users.UserTokenBody).Id	

	controllerError := controller.logic.Delete(id, userId)

	if controllerError != nil {
		http.Error(w, controllerError.Error.Error(), controllerError.StatusCode)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
