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
		Html:    fmt.Sprintf("<p>Your verification token: <strong>%s</strong></p><p>Submit this token to POST %s/api/v1/customer/auth/verify</p>", token, c.baseURL),
	}

	_, err := c.resend.Emails.Send(params)
	return err
}

func (c *Client) SendEmailChangeVerification(to, token string) error {
	params := &resend.SendEmailRequest{
		From:    c.from,
		To:      []string{to},
		Subject: "Verify your new email",
		Html:    fmt.Sprintf("<p>Your email change verification token: <strong>%s</strong></p><p>Submit this token to POST %s/api/v1/customer/profile/email/verify</p>", token, c.baseURL),
	}

	_, err := c.resend.Emails.Send(params)
	return err
}

func (c *Client) SendPasswordResetEmail(to, token string) error {
	params := &resend.SendEmailRequest{
		From:    c.from,
		To:      []string{to},
		Subject: "Reset your password",
		Html:    fmt.Sprintf("<p>Your password reset token: <strong>%s</strong></p>", token),
	}

	_, err := c.resend.Emails.Send(params)
	return err
}
