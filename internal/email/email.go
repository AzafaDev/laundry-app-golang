package email

import (
	"fmt"

	"github.com/resend/resend-go/v2"
)

type Client struct {
	resend  *resend.Client
	from    string
	baseURL string
}

func NewClient(apiKey string, baseURL string) *Client {
	return &Client{
		resend:  resend.NewClient(apiKey),
		from:    "noreply@azafadev.web.id",
		baseURL: baseURL,
	}
}

func (c *Client) SendVerificationEmail(to, token string) error {
	params := &resend.SendEmailRequest{
		From:    c.from,
		To:      []string{to},
		Subject: "Verify your email",
		Html:    fmt.Sprintf("<p>Click <a href=\"%s/api/v1/customer/auth/verify?token=%s\">here</a> to verify your email.</p>", c.baseURL, token),
	}

	_, err := c.resend.Emails.Send(params)
	return err
}
