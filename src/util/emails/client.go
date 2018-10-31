package emails

import (
	"os"

	"github.com/sendgrid/sendgrid-go/helpers/mail"

	"github.com/sendgrid/sendgrid-go"
)

type EmailClient struct {
	sendgridClient *sendgrid.Client
}

type Email struct {
	To          string
	ToAddress   string
	From        string
	FromAddress string
	Subject     string
	PlainText   string
	HtmlText    string
}

func MakeClient() *EmailClient {
	client := EmailClient{}
	client.sendgridClient = sendgrid.NewSendClient(os.Getenv("SENDGRID_API_KEY"))
	return &client
}

func (client *EmailClient) Send(email Email) (err error) {
	to := mail.NewEmail(email.To, email.ToAddress)
	from := mail.NewEmail(email.From, email.FromAddress)
	message := mail.NewSingleEmail(from, email.Subject, to, email.PlainText, email.HtmlText)

	_, err = client.sendgridClient.Send(message)
	return err
}
