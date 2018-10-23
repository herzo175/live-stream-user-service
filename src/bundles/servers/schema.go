package servers

import (
	"errors"
	"encoding/json"
	"fmt"
	"github.com/herzo175/live-stream-user-service/src/bundles/servers/serverservice"
	"github.com/herzo175/live-stream-user-service/src/util/querying"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// TODO: created at/modified at
type Server struct {
	Id bson.ObjectId `json:"_id" bson:"_id,omitempty"`
	// TODO: support for user groups
	UserId      bson.ObjectId `json:"user_id" bson:"user_id,omitempty"`
	ServerName  string        `json:"server_name" bson:"server_name"`
	StreamName  string        `json:"stream_name" bson:"stream_name"`
	ChannelName string        `json:"channel_name" bson:"channel_name"`
	DropletId   int64         `json:"droplet_id,omitempty" bson:"droplet_id"`
	IpAddress   string        `json:"ip_address,omitempty" bson:"ip_address"`
	Status      Status        `json:"status,omitempty" bson:"status"`
}

type Status int

const (
	Starting Status = 1
	Ready    Status = 2
	Stopping Status = 3
	Inactive Status = 4
	Errored  Status = 5
)

// Status -> string
func (s Status) MarshalJSON() (b []byte, err error) {
	var status string

	switch s {
	case Starting:
		status = "Starting"
	case Ready:
		status = "Ready"
	case Stopping:
		status = "Stopping"
	case Inactive:
		status = "Inactive"
	case Errored:
		status = "Errored"
	default:
		return b, errors.New("Status must be between 0 and 4")
	}

	return json.Marshal(status)
}

// string -> Status
func (s *Status) UnmarshalJSON(b []byte) (err error) {
	statusString := string(b)
	statusString = statusString[1:len(statusString)-1]
	var status Status

	switch statusString {
	case "Starting":
		status = Starting
	case "Ready":
		status = Ready
	case "Stopping":
		status = Stopping
	case "Inactive":
		status = Inactive
	case "Errored":
		status = Errored
	default:
		return errors.New("Status must be either Starting, Ready, Stopping, Inactive or Errored")
	}

	*s = status
	return nil
}

type serverMutation interface{
	update(collection *mgo.Collection, query, change bson.M) error
}

type ServerMutationFields struct {}

type ServerCreate struct {
	Id bson.ObjectId          `json:"_id" bson:"_id,omitempty"`
	// TODO: support for user groups
	UserId      bson.ObjectId `json:"user_id" bson:"user_id,omitempty"`
	ServerName  string        `json:"server_name" bson:"server_name"`
	StreamName  string        `json:"stream_name" bson:"stream_name"`
	ChannelName string        `json:"channel_name" bson:"channel_name"`
}

type ServerInfoUpdate struct {
	ServerMutationFields
	ServerName string `json:"server_name" bson:"server_name,omitempty"`
	StreamName string `json:"stream_name" bson:"stream_name,omitempty"`
}

type ServerStatusUpdate struct {
	ServerMutationFields
	Status Status `json:"status" bson:"status,omitempty"`
}

type ServerDropletUpdate struct {
	ServerMutationFields
	DropletId int64 `json:"droplet_id" bson:"droplet_id,omitempty"`
}

type ServerIpAddressUpdate struct {
	ServerMutationFields
	IpAddress string `json:"ip_address" bson:"ip_address,omitempty"`
}

type ServerDB struct {
	collection *mgo.Collection
	serverService *serverservice.Service
}

func MakeSchema(dbName string, session *mgo.Session) *ServerDB {
	schema := ServerDB{}
	schema.collection = session.DB(dbName).C("Servers")
	schema.serverService = serverservice.MakeService()

	return &schema
}

func (schemaConfig *ServerDB) Get(userId string, filter map[string]interface{}, start, end int) (results *querying.PaginatedList, err error) {
	clauses := make(map[string]interface{})

	for k, v := range filter {
		clauses[k] = v
	}

	clauses["user_id"] = bson.ObjectIdHex(userId)

	return querying.GetPaginatedList(schemaConfig.collection, &[]Server{}, start, end, clauses)
}

func (schemaConfig *ServerDB) Create(userId string, server *ServerCreate) (err error) {
	// create inital server
	// TODO: all fields present validator
	server.Id = bson.NewObjectId()
	server.UserId = bson.ObjectIdHex(userId)
	server.ChannelName = fmt.Sprintf(
		"%s-channel", server.Id.Hex(),
	)

	err = schemaConfig.collection.Insert(server)

	if err != nil {
		return err
	}

	// update with droplet id
	serverInstance, err := schemaConfig.serverService.CreateServer(
		fmt.Sprintf("live-stream-%s", server.Id.Hex()),
		server.StreamName,
		server.Id.Hex(),
	)

	if err != nil {
		return err
	}

	dropletUpdate := ServerDropletUpdate{
		DropletId: serverInstance.DropletId,
	}

	return schemaConfig.Update(server.Id.Hex(), &dropletUpdate)
}

func (schemaConfig *ServerDB) GetById(id string, userId string) (server *Server, err error) {
	// TODO: get based on user group
	query := bson.M{
		"_id":     bson.ObjectIdHex(id),
		"user_id": bson.ObjectIdHex(userId),
	}

	return schemaConfig.getById(query)
}

func (schemaConfig *ServerDB) getById(query bson.M) (server *Server, err error) {
	server = &Server{}
	err = schemaConfig.collection.Find(query).One(server)
	return server, err
}

func (schemaConfig *ServerDB) Update(id string, serverUpdate serverMutation) error {
	query := bson.M{"_id": bson.ObjectIdHex(id)}
	change := bson.M{"$set": serverUpdate}

	return serverUpdate.update(schemaConfig.collection, query, change)
}

func (m *ServerMutationFields) update(collection *mgo.Collection, query, change bson.M) error {
	return collection.Update(query, change)
}

func (schemaConfig *ServerDB) Delete(id string) error {
	query := bson.M{"_id": bson.ObjectIdHex(id)}
	return schemaConfig.collection.Remove(query)
}
