package serverservice

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
)

type Service struct {
	endpoint string
	client   *http.Client
}

func MakeService() *Service {
	service := Service{}
	service.endpoint = os.Getenv("SERVER_SERVICE_ENDPOINT")
	service.client = &http.Client{}
	return &service
}

type CreateServerRequest struct {
	ServerName string `json:"server_name"`
	StreamName string `json:"stream_name"`
	ServerId   string `json:"server_id"`
}

type CreateServerResponse struct {
	DropletId int64 `json:"droplet_id"`
}

type RestartServerRequest struct {
	ServerId   string `json:"server_id"`
	StreamName string `json:"stream_name"`
}

func (service *Service) CreateServer(serverName, streamName, serverId string) (data *CreateServerResponse, err error) {
	// server name must match (a-z, A-Z, 0-9, . and -)
	// matched, err := regexp.MatchString("\b[a-z]+\\z", serverName)

	// fmt.Println(serverName, streamName)
	// fmt.Println(matched, err)

	// if err != nil || !matched {
	// 	return nil, errors.New("Server name can only be lowercase letters")
	// }

	body := CreateServerRequest{
		ServerName: serverName,
		StreamName: streamName,
		ServerId:   serverId,
	}

	b, err := json.Marshal(body)

	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/server", service.endpoint),
		bytes.NewBuffer(b),
	)

	req.Header.Add("Content-Type", "application/json")
	// TODO: add bearer token
	resp, err := service.client.Do(req)

	if err != nil {
		return nil, err
	}

	data = &CreateServerResponse{}
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(data)

	return data, err
}

func (service *Service) RestartServer(serverId, streamName string, dropletId int64) (err error) {
	body := RestartServerRequest{
		ServerId:   serverId,
		StreamName: streamName,
	}

	b, err := json.Marshal(body)

	if err != nil {
		return err
	}

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/server/%v/restart", service.endpoint, dropletId),
		bytes.NewBuffer(b),
	)

	req.Header.Add("Content-Type", "application/json")
	// TODO: add bearer token
	resp, err := service.client.Do(req)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New("Failed to start restart server")
	}

	return nil
}

func (service *Service) DeleteServer(dropletId int64) (err error) {
	req, err := http.NewRequest(
		"DELETE",
		fmt.Sprintf("%s/server/%v", service.endpoint, dropletId),
		nil,
	)

	// TODO: add bearer token
	resp, err := service.client.Do(req)

	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return errors.New("Failed to start delete server")
	}

	return nil
}
