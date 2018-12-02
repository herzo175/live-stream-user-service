package channels

import (
	"os"

	"github.com/pusher/pusher-http-go"
)

const (
	EMAIL_VERIFIED_EVENT = "email_verified"
	UPDATE_STATUS_EVENT  = "update__status"
	UPDATE_IP_EVENT      = "update__ip_address"
)

type Client struct {
	pusherClient *pusher.Client
}

func MakeClient() *Client {
	client := Client{}

	pusherClient := pusher.Client{
		AppId:   os.Getenv("PUSHER_APP_ID"),
		Key:     os.Getenv("PUSHER_APP_KEY"),
		Secret:  os.Getenv("PUSHER_APP_SECRET"),
		Cluster: os.Getenv("PUSHER_APP_CLUSTER"),
		Secure:  true,
	}

	client.pusherClient = &pusherClient
	return &client
}

func (c *Client) Authenticate(req []byte) ([]byte, error) {
	return c.pusherClient.AuthenticatePrivateChannel(req)
}

func (c *Client) Send(channel, event string, data interface{}) (err error) {
	_, err = c.pusherClient.Trigger(channel, event, data)
	return err
}
