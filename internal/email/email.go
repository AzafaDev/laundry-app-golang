package email

import (
	"fmt"
	"net/url"

	"github.com/resend/resend-go/v2"
)

type Client struct {
	resend      *resend.Client
	from        string
	baseURL     string
	frontendURL string
}

func NewClient(apiKey, baseURL, frontendURL string) *Client {
	return &Client{
		resend:      resend.NewClient(apiKey),
		from:        "noreply@azafadev.web.id",
		baseURL:     baseURL,
		frontendURL: frontendURL,
	}
}

func (c *Client) SendVerificationEmail(to, token string) error {
	verifyLink := fmt.Sprintf("%s/verify-email?token=%s", c.frontendURL, url.QueryEscape(token))

	params := &resend.SendEmailRequest{
		From:    c.from,
		To:      []string{to},
		Subject: "Verify your email",
		Html: fmt.Sprintf(
			`<p>Click <a href="%s">here</a> to verify your email.</p><p>Or enter this code manually: <strong>%s</strong></p>`,
			verifyLink, token,
		),
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
