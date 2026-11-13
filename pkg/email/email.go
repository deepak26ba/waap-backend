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
	SendVerificationEmail(ctx context.Context, toEmail, toName, otp string) error
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

// SendVerificationEmail delivers a registration email verification OTP via Brevo.
func (s *BrevoSender) SendVerificationEmail(ctx context.Context, toEmail, toName, otp string) error {
	if s.apiKey == "" {
		return fmt.Errorf("brevo API key is not configured")
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Your WAAP Verification OTP</title>
</head>
<body style="margin: 0; padding: 0; background-color: #f8fafc; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;">
  <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="background-color: #f8fafc; padding: 40px 16px;">
    <tr>
      <td align="center">
        <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="max-width: 580px; width: 100%%; background-color: #ffffff; border-radius: 12px; box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.05), 0 2px 4px -2px rgba(0, 0, 0, 0.05); overflow: hidden; border: 1px solid #e2e8f0;">
          <!-- Header with Logo -->
          <tr>
            <td align="center" style="background: linear-gradient(135deg, #1e293b 0%%, #0f172a 100%%); padding: 32px 20px;">
              <table role="presentation" cellspacing="0" cellpadding="0" border="0">
                <tr>
                  <td style="vertical-align: middle; padding-right: 12px;">
                    <div style="background: linear-gradient(135deg, #4f46e5 0%%, #7c3aed 100%%); width: 44px; height: 44px; border-radius: 10px; text-align: center; color: #ffffff; font-size: 22px; font-weight: bold; line-height: 44px;">
                      W
                    </div>
                  </td>
                  <td style="vertical-align: middle;">
                    <span style="font-size: 26px; font-weight: 800; letter-spacing: 2px; color: #ffffff;">WAAP</span>
                  </td>
                </tr>
              </table>
            </td>
          </tr>
          <!-- Main Body -->
          <tr>
            <td style="padding: 36px 32px; color: #334155;">
              <h1 style="margin: 0 0 16px 0; font-size: 22px; font-weight: 700; color: #0f172a; text-align: center;">Verify Your Email Address</h1>
              <p style="margin: 0 0 16px 0; font-size: 15px; line-height: 1.6; color: #475569;">
                Hi <strong>%s</strong>,
              </p>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 1.6; color: #475569;">
                Thank you for joining WAAP! Please use the 6-digit One-Time Password (OTP) below to verify your email address and activate your account.
              </p>
              
              <!-- OTP Display Box -->
              <div style="background-color: #f1f5f9; border: 2px dashed #cbd5e1; border-radius: 8px; padding: 24px 20px; text-align: center; margin: 28px 0;">
                <span style="display: block; font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: 2px; color: #64748b; margin-bottom: 12px;">Your Verification OTP</span>
                <span style="font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, Courier, monospace; font-size: 36px; font-weight: 800; letter-spacing: 8px; color: #4f46e5; display: inline-block;">%s</span>
              </div>

              <!-- Expiry Alert -->
              <div style="background-color: #fffbeb; border-left: 4px solid #f59e0b; padding: 12px 16px; border-radius: 4px; margin-bottom: 24px;">
                <p style="margin: 0; font-size: 13px; line-height: 1.5; color: #92400e;">
                  <strong>Important:</strong> This verification OTP will expire in <strong>5 minutes</strong>. Do not share this code with anyone.
                </p>
              </div>

              <p style="margin: 0 0 12px 0; font-size: 14px; line-height: 1.6; color: #64748b;">
                If you did not register for an account on WAAP, please ignore this email.
              </p>
            </td>
          </tr>
          <!-- Footer -->
          <tr>
            <td style="background-color: #f8fafc; border-top: 1px solid #e2e8f0; padding: 24px 32px; text-align: center;">
              <p style="margin: 0 0 8px 0; font-size: 12px; color: #94a3b8;">
                &copy; 2026 WAAP E-Commerce Platform. All rights reserved.
              </p>
              <p style="margin: 0; font-size: 12px; color: #94a3b8;">
                This is an automated security notification. Please do not reply directly.
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, toName, otp)

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
		Subject:     fmt.Sprintf("Your WAAP Verification OTP: %s", otp),
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
func (n *NoopSender) SendVerificationEmail(ctx context.Context, toEmail, toName, otp string) error {
	return nil
}
