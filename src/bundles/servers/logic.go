package servers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/herzo175/live-stream-user-service/src/util/requests"

	"github.com/herzo175/live-stream-user-service/src/util/channels"

	"github.com/herzo175/live-stream-user-service/src/bundles/billing"
	"github.com/herzo175/live-stream-user-service/src/util/database"
	"github.com/herzo175/live-stream-user-service/src/util/serverservice"
)

// TODO: created at/modified at
type Server struct {
	Id string `json:"_id" gorm:"column:_id"`
	// TODO: support for user groups
	UserId      string `json:"user_id" gorm:"column:user_id"`
	ServerName  string `json:"server_name" gorm:"column:server_name"`
	StreamName  string `json:"stream_name" gorm:"column:stream_name"`
	ChannelName string `json:"channel_name" gorm:"column:channel_name"`
	DropletId   int64  `json:"droplet_id,omitempty" gorm:"column:droplet_id"`
	IpAddress   string `json:"ip_address,omitempty" gorm:"column:ip_address"`
	Status      Status `json:"status,omitempty" gorm:"column:status"`
}

func (Server) TableName() string {
	return "public.live_stream_servers"
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
	statusString = statusString[1 : len(statusString)-1]
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

type ServerDropletUpdate struct {
	DropletId int64 `json:"droplet_id" bson:"droplet_id,omitempty"`
}

type ServerLogicConfig struct {
	db                 database.Database
	serverService      *serverservice.Service
	billingLogic       billing.BillingLogic
	notificationClient *channels.Client
}

type ServerLogic interface {
	Get(userId string, filter map[string]interface{}, start, end int) (*database.PaginatedList, *requests.ControllerError)
	GetById(id string, userId string) (*Server, *requests.ControllerError)
	Create(userId, serverName, streamName string) *requests.ControllerError
	UpdateServerInfo(serverId, userId, serverName, streamName string) *requests.ControllerError
	SetIpAddress(serverId, newIpAddress string) *requests.ControllerError
	SetStatus(serverId string, newStatus Status) *requests.ControllerError
	Delete(serverId, userId string) *requests.ControllerError
	RestartServer(serverId, userId string) *requests.ControllerError
}

func MakeServerLogic(
	db database.Database,
	serverService *serverservice.Service,
	billingLogic billing.BillingLogic,
	notificationClient *channels.Client,
) *ServerLogicConfig {
	config := ServerLogicConfig{}

	config.db = db
	config.serverService = serverService
	config.billingLogic = billingLogic
	config.notificationClient = notificationClient

	return &config
}

func (config *ServerLogicConfig) Get(userId string, filter map[string]interface{}, start, end int) (*database.PaginatedList, *requests.ControllerError) {
	filter["user_id"] = userId

	billingEvents := new([]Server)
	total, err := config.db.FindMany(billingEvents, start, end, filter)

	if err != nil {
		log.Printf("Error retrieving billing events for user %s: %v", userId, err)
		return nil, &requests.ControllerError{
			StatusCode: 400,
			Error:      errors.New("Could not find billing events"),
		}
	}

	return database.ToPaginatedList(*billingEvents, start, end, total), nil
}

func (config *ServerLogicConfig) Create(userId, serverName, streamName string) *requests.ControllerError {
	// create inital server
	server := Server{}

	server.Id = uuid.New().String()
	server.UserId = userId
	server.ServerName = serverName
	server.StreamName = streamName
	server.ChannelName = fmt.Sprintf("%s-channel", server.Id)
	server.Status = Starting

	err := config.db.Create(&server)

	if err != nil {
		log.Printf("Unable to create new server with name %s: %v", serverName, err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error: fmt.Errorf("Unable to create new server with name %s", serverName),
		}
	}

	controllerError := config.billingLogic.Start(
		server.UserId, os.Getenv("STRIPE_SERVER_PLAN_ID"), server.Id)

	if controllerError != nil {
		if err := config.db.Delete(&server, "_id = ?", server.Id); err != nil {
			log.Printf("Error deleting server from db after failure to start billing: %v", err)
		}

		return controllerError
	}

	// update with droplet id
	serverInstance, err := config.serverService.CreateServer(
		fmt.Sprintf("live-stream-%s", server.Id),
		server.StreamName,
		server.Id,
	)

	if err != nil {
		log.Printf("Unable to create server instance for %s: %v", serverName, err)

		if controllerError := config.billingLogic.Stop(server.Id); controllerError != nil {
			log.Printf(
				"Error stopping billing for server after failure to create instance: %v",
				controllerError.Error,
			)
		}

		if err := config.db.Delete(&server, "_id = ?", server.Id); err != nil {
			log.Printf("Error deleting server from db after failure to create instance: %v", err)
		}

		return &requests.ControllerError{
			StatusCode: 500,
			Error: fmt.Errorf("Unable to create server instance for %s", serverName),
		}
	}

	err = config.db.Update(
		&server,
		map[string]interface{}{"droplet_id": serverInstance.DropletId},
		"_id = ?", server.Id,
	)

	if err != nil {
		log.Printf("Unable to link instance for %s: %v", serverName, err)

		if controllerError := config.billingLogic.Stop(server.Id); controllerError != nil {
			log.Printf(
				"Error stopping billing for server after failure to link to instance: %v",
				controllerError.Error,
			)
		}

		if err := config.db.Delete(&server, "_id = ?", server.Id); err != nil {
			log.Printf(
				"Error deleting server from db after failure to failure to link to instance: %v",
				err,
			)
		}

		return &requests.ControllerError{
			StatusCode: 500,
			Error: fmt.Errorf("Unable link instance for %s", serverName),
		}
	}

	return nil
}

func (config *ServerLogicConfig) GetById(id string, userId string) (*Server, *requests.ControllerError) {
	// TODO: get based on user group
	server := Server{}

	err := config.db.FindOne(&server, "_id = ? AND user_id = ?", id, userId)

	if err != nil {
		return nil, &requests.ControllerError{
			StatusCode: 404,
			Error:      fmt.Errorf("Could not find server %s", id),
		}
	}

	return &server, nil
}

func (config *ServerLogicConfig) getById(id string) (*Server, error) {
	server := Server{}
	err := config.db.FindOne(&server, "_id = ?", id)

	if err != nil {
		return nil, err
	}

	return &server, err
}

func (config *ServerLogicConfig) UpdateServerInfo(serverId, userId, serverName, streamName string) *requests.ControllerError {
	server, controllerError := config.GetById(serverId, userId)

	if controllerError != nil {
		return controllerError
	}

	err := config.serverService.DeleteServer(server.DropletId)

	if err != nil {
		log.Printf("Cannot delete server %s: %v", serverId, err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error:      fmt.Errorf("Cannot delete server %s", serverId),
		}
	}

	newServer, err := config.serverService.CreateServer(serverName, streamName, server.Id)

	if err != nil {
		log.Printf("Cannot re-create server %s: %v", serverId, err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error:      fmt.Errorf("Cannot re-create server %s", serverId),
		}
	}

	err = config.db.Update(
		server,
		map[string]interface{}{"droplet_id": newServer.DropletId},
		"_id = ?", server.Id,
	)

	if err != nil {
		log.Printf("Unable to relink instance to server %s: %v", serverId, err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error:      fmt.Errorf("Unable to relink instance to server %s", serverId),
		}
	}

	return nil
}

func (config *ServerLogicConfig) SetIpAddress(id, newServerIp string) *requests.ControllerError {
	// NOTE: currently can be set be services other than the user
	server, err := config.getById(id)

	if err != nil {
		log.Printf("Server %s not found: %v", id, err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error:      fmt.Errorf("Server %s not found", id),
		}
	}

	err = config.db.Update(
		server,
		map[string]interface{}{"ip_address": newServerIp},
		"_id = ?", server.Id,
	)

	if err != nil {
		log.Printf("Unable to save new ip address for server %s: %v", id, err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error:      fmt.Errorf("Unable to save new ip address for server %s", id),
		}
	}

	err = config.notificationClient.Send(server.ChannelName, channels.UPDATE_IP_EVENT, newServerIp)

	if err != nil {
		log.Printf("Unable to notify client about ip update for server %s: %v", id, err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error:      fmt.Errorf("Unable to notify client about ip update for server %s", id),
		}
	}

	return nil
}

func (config *ServerLogicConfig) SetStatus(id string, newStatus Status) *requests.ControllerError {
	// NOTE: currently can be set be services other than the user
	server, err := config.getById(id)

	if err != nil {
		log.Printf("Server %s not found: %v", id, err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error:      fmt.Errorf("Server %s not found", id),
		}
	}

	err = config.db.Update(
		server,
		map[string]interface{}{"status": newStatus},
		"_id = ?", server.Id,
	)

	if err != nil {
		log.Printf("Unable to save new ip address for server %s: %v", id, err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error:      fmt.Errorf("Unable to save new ip address for server %s", id),
		}
	}

	err = config.notificationClient.Send(
		server.ChannelName, channels.UPDATE_STATUS_EVENT, newStatus,
	)

	if err != nil {
		log.Printf("Unable to notify client about status update for server %s: %v", id, err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error:      fmt.Errorf("Unable to notify client about status update for server %s", id),
		}
	}

	return nil
}

func (config *ServerLogicConfig) Delete(id, userId string) *requests.ControllerError {
	// delete server
	server, controllerErr := config.GetById(id, userId)

	if controllerErr != nil {
		return controllerErr
	}

	// stop billing
	controllerErr = config.billingLogic.Stop(id)

	if controllerErr != nil {
		return controllerErr
	}

	err := config.serverService.DeleteServer(server.DropletId)

	if err != nil {
		log.Printf("Could not delete instance for server %s: %v", id, err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error:      fmt.Errorf("Could not delete instance for server %s", id),
		}
	}

	err = config.db.Delete(server, "_id = ?", server.Id)

	if err != nil {
		log.Printf("Could delete server %s: %v", id, err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error:      fmt.Errorf("Could not delete server %s", id),
		}
	}

	return nil
}

func (config *ServerLogicConfig) RestartServer(id, userId string) *requests.ControllerError {
	server, controllerErr := config.GetById(id, userId)

	if controllerErr != nil {
		return controllerErr
	}

	err := config.serverService.RestartServer(server.Id, server.StreamName, server.DropletId)

	if err != nil {
		log.Printf("Unable to restart server %s: %v", id, err)
		return &requests.ControllerError{
			StatusCode: 500,
			Error:      fmt.Errorf("Unable to restart server %s", id),
		}
	}

	return nil
}
