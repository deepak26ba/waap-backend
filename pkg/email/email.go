package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Sender abstracts transactional email delivery.
type Sender interface {
	SendVerificationEmail(ctx context.Context, toEmail, toName, token string) error
}

// BrevoSender delivers transactional emails using the Brevo (Sendinblue) v3 API.
type BrevoSender struct {
	apiKey     string
	fromEmail  string
	fromName   string
	httpClient *http.Client
	apiURL     string
}

// NewBrevoSender constructs a Brevo email sender.
func NewBrevoSender(apiKey, fromEmail, fromName string) *BrevoSender {
	if fromEmail == "" {
		fromEmail = "olivergot26@gmail.com"
	}
	if fromName == "" {
		fromName = "WAAP"
	}
	return &BrevoSender{
		apiKey:     apiKey,
		fromEmail:  fromEmail,
		fromName:   fromName,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		apiURL:     "https://api.brevo.com/v3/smtp/email",
	}
}

// SetBaseURL allows overriding the API endpoint URL (useful in unit tests with httptest.Server).
func (s *BrevoSender) SetBaseURL(url string) {
	s.apiURL = url
}

// SetHTTPClient allows overriding the HTTP client for testing.
func (s *BrevoSender) SetHTTPClient(client *http.Client) {
	s.httpClient = client
}

type brevoRecipient struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type brevoSenderInfo struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type brevoEmailPayload struct {
	Sender      brevoSenderInfo  `json:"sender"`
	To          []brevoRecipient `json:"to"`
	Subject     string           `json:"subject"`
	HTMLContent string           `json:"htmlContent"`
}

// SendVerificationEmail delivers a registration email verification link/token via Brevo.
func (s *BrevoSender) SendVerificationEmail(ctx context.Context, toEmail, toName, token string) error {
	if s.apiKey == "" {
		return fmt.Errorf("brevo API key is not configured")
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
  <h2 style="color: #2c3e50;">Verify Your Email Address</h2>
  <p>Hello %s,</p>
  <p>Thank you for registering with WAAP. Please use the following verification token to verify your account:</p>
  <div style="background-color: #f4f6f8; border: 1px solid #e1e4e8; border-radius: 6px; padding: 16px; margin: 20px 0; font-family: monospace; font-size: 16px; font-weight: bold; text-align: center; letter-spacing: 1px; word-break: break-all;">
    %s
  </div>
  <p>Submit this token to the verification endpoint to complete your registration. This token will expire in 24 hours.</p>
  <p>If you did not request this email, please ignore it.</p>
  <hr style="border: none; border-top: 1px solid #eee; margin: 20px 0;">
  <p style="font-size: 12px; color: #777;">&copy; WAAP E-Commerce Platform. All rights reserved.</p>
</body>
</html>`, toName, token)

	payload := brevoEmailPayload{
		Sender: brevoSenderInfo{
			Email: s.fromEmail,
			Name:  s.fromName,
		},
		To: []brevoRecipient{
			{
				Email: toEmail,
				Name:  toName,
			},
		},
		Subject:     "Verify your email address",
		HTMLContent: html,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal email payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("api-key", s.apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to Brevo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("brevo API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// NoopSender is a mock sender that performs no action.
type NoopSender struct{}

// SendVerificationEmail always returns nil.
func (n *NoopSender) SendVerificationEmail(ctx context.Context, toEmail, toName, token string) error {
	return nil
}

