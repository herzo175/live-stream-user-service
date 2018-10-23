package servers

import (
	"gopkg.in/mgo.v2/bson"
	"github.com/herzo175/live-stream-user-service/src/util/channels"
	"log"
	"fmt"
	"net/http"
	"github.com/gorilla/mux"
	"github.com/herzo175/live-stream-user-service/src/util/auth"
	"github.com/herzo175/live-stream-user-service/src/bundles/users"
	"github.com/herzo175/live-stream-user-service/src/util/querying"

	"github.com/herzo175/live-stream-user-service/src/bundles"
)

type ServerController struct {
	Controller         *bundles.Controller
	schema             *ServerDB
	notificationClient *channels.Client
}

func (controller *ServerController) MakeRouter() {
	subrouter := controller.Controller.Router.PathPrefix("/servers").Subrouter()

	subrouter.HandleFunc(
		"", bundles.GetAuthenticated(&users.UserTokenBody{}, controller.Get),
	).Methods("GET")

	subrouter.HandleFunc(
		"", bundles.SetAuthenticated(&ServerCreate{}, &users.UserTokenBody{}, controller.Create),
	).Methods("POST")

	subrouter.HandleFunc(
		"/{id}", bundles.GetAuthenticated(&users.UserTokenBody{}, controller.GetById),
	).Methods("GET")

	subrouter.HandleFunc(
		"/{id}", bundles.SetAuthenticated(&ServerInfoUpdate{}, &users.UserTokenBody{}, controller.Update),
	).Methods("PUT")

	// TODO: set authorized
	subrouter.HandleFunc(
		"/{id}/status", bundles.SetAuthenticated(&ServerStatusUpdate{}, &users.UserTokenBody{}, controller.SetStatus),
	).Methods("PUT")

	// TODO: set authorized
	subrouter.HandleFunc(
		"/{id}/ip_address", bundles.SetAuthenticated(&ServerIpAddressUpdate{}, &users.UserTokenBody{}, controller.SetIpAddress),
	).Methods("PUT")

	subrouter.HandleFunc(
		"/{id}/restart", auth.IsAuthenticated(&users.UserTokenBody{}, controller.Restart),
	).Methods("POST")

	subrouter.HandleFunc(
		"/{id}", auth.IsAuthenticated(&users.UserTokenBody{}, controller.Delete),
	).Methods("DELETE")

	controller.schema = MakeSchema(controller.Controller.DBName, controller.Controller.DBClient)
	controller.notificationClient = channels.MakeClient()
}

func (controller *ServerController) Get(
	urlParams map[string]string,
	queryParams, headers map[string][]string,
	tokenBodyPointer interface{},
) (interface{}, error) {
	start, end, err := querying.ExtractStartEnd(queryParams)

	if err != nil {
		return nil, err
	}

	userId := tokenBodyPointer.(*users.UserTokenBody).Id
	filter, err := querying.GenerateQueryFromMultivaluedMap(queryParams, Server{})

	if err != nil {
		return nil, err
	}

	paginatedList, err := controller.schema.Get(userId, filter, int(start), int(end))
	return &paginatedList, err
}

func (controller *ServerController) Create(
	urlParams map[string]string,
	headers map[string][]string,
	server interface{},
	tokenBodyPointer interface{},
) error {
	userId := tokenBodyPointer.(*users.UserTokenBody).Id
	return controller.schema.Create(userId, server.(*ServerCreate))
}

func (controller *ServerController) GetById(
	urlParams map[string]string,
	queryParams,
	headers map[string][]string,
	tokenBodyPointer interface{},
) (interface{}, error) {
	id := urlParams["id"]
	userId := tokenBodyPointer.(*users.UserTokenBody).Id
	return controller.schema.GetById(id, userId)
}

func (controller *ServerController) Update(
	urlParams map[string]string,
	headers map[string][]string,
	update interface{},
	tokenBodyPointer interface{},
) error {
	id := urlParams["id"]
	userId := tokenBodyPointer.(*users.UserTokenBody).Id
	server, err := controller.schema.GetById(id, userId)

	if err != nil {
		return err
	}

	updatePointer := update.(*ServerInfoUpdate)

	err = controller.schema.serverService.RestartServer(
		server.Id.Hex(), updatePointer.StreamName, server.DropletId,
	)

	return controller.schema.Update(id, updatePointer)
}

func (controller *ServerController) SetStatus(
	urlParams map[string]string,
	headers map[string][]string,
	update interface{},
	tokenBodyPointer interface{},
) error {
	id := urlParams["id"]
	server, err := controller.schema.getById(bson.M{"_id": bson.ObjectIdHex(id)})
	updatePointer := update.(*ServerStatusUpdate)

	if err != nil {
		return err
	}

	err = controller.notificationClient.Send(
		server.ChannelName, "update__status", updatePointer.Status,
	)

	if err != nil {
		return err
	}

	return controller.schema.Update(id, updatePointer)
}

func (controller *ServerController) SetIpAddress(
	urlParams map[string]string,
	headers map[string][]string,
	update interface{},
	tokenBodyPointer interface{},
) error {
	id := urlParams["id"]
	server, err := controller.schema.getById(bson.M{"_id": bson.ObjectIdHex(id)})
	updatePointer := update.(*ServerIpAddressUpdate)

	if err != nil {
		return err
	}

	err = controller.notificationClient.Send(
		server.ChannelName, "update__ip_address", updatePointer.IpAddress,
	)

	if err != nil {
		return err
	}

	return controller.schema.Update(id, updatePointer)
}

func (controller *ServerController) Restart(w http.ResponseWriter, r *http.Request, tokenBodyPointer interface{}) {
	id := mux.Vars(r)["id"]
	userId := tokenBodyPointer.(*users.UserTokenBody).Id
	server, err := controller.schema.GetById(id, userId)

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get server with id %s", id), 404)
		log.Println(err)
	}

	err = controller.schema.serverService.RestartServer(
		server.Id.Hex(), server.StreamName, server.DropletId,
	)

	if err != nil {
		http.Error(w, err.Error(), 500)
		log.Println(err)
	}
}

func (controller *ServerController) Delete(w http.ResponseWriter, r *http.Request, tokenBodyPointer interface{}) {
	id := mux.Vars(r)["id"]
	userId := tokenBodyPointer.(*users.UserTokenBody).Id
	server, err := controller.schema.GetById(id, userId)

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get server with id %s", id), 404)
		log.Println(err)
	}

	err = controller.schema.serverService.DeleteServer(server.DropletId)

	if err != nil {
		http.Error(w, err.Error(), 500)
		log.Println(err)
	}

	err = controller.schema.Delete(id)

	if err != nil {
		http.Error(w, "Failed to delete server from database", 500)
		log.Println(err)
	}
}
