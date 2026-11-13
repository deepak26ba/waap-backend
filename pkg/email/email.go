package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
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

	htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta name="color-scheme" content="light dark">
  <meta name="supported-color-schemes" content="light dark">
  <title>Your WAAP Verification OTP</title>
  <style>
    :root {
      color-scheme: light dark;
      supported-color-schemes: light dark;
    }
    @media (prefers-color-scheme: dark) {
      body, .email-bg { background-color: #0b0f19 !important; }
      .email-card { background-color: #1e293b !important; border-color: #334155 !important; }
      .email-title { color: #f8fafc !important; }
      .email-text { color: #cbd5e1 !important; }
      .email-name { color: #ffffff !important; }
      .otp-box { background-color: #0f172a !important; border-color: #334155 !important; }
      .otp-pill { background-color: #1e1b4b !important; border-color: #4338ca !important; }
      .otp-text { color: #a5b4fc !important; }
      .otp-hint { color: #94a3b8 !important; }
      .warning-card { background-color: #2c0b0e !important; border-color: #881337 !important; border-left-color: #f43f5e !important; }
      .warning-heading { color: #fda4af !important; }
      .warning-body { color: #fecdd3 !important; }
      .sub-text { color: #64748b !important; }
      .footer-card { background-color: #0f172a !important; border-top-color: #334155 !important; }
      .footer-title { color: #94a3b8 !important; }
      .footer-text { color: #64748b !important; }
    }
  </style>
</head>
<body class="email-bg" style="margin: 0; padding: 0; background-color: #f1f5f9; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; -webkit-font-smoothing: antialiased; -moz-osx-font-smoothing: grayscale;">
  <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" class="email-bg" style="background-color: #f1f5f9; padding: 48px 16px;">
    <tr>
      <td align="center">
        <!-- Main Card -->
        <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" class="email-card" style="max-width: 560px; width: 100%%; background-color: #ffffff; border-radius: 16px; overflow: hidden; box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.05), 0 8px 10px -6px rgba(0, 0, 0, 0.01); border: 1px solid #e2e8f0;">
          
          <!-- Top Gradient Accent Bar -->
          <tr>
            <td style="height: 5px; background: linear-gradient(90deg, #4f46e5 0%%, #7c3aed 50%%, #06b6d4 100%%); line-height: 5px; font-size: 1px;">&nbsp;</td>
          </tr>

          <!-- Header -->
          <tr>
            <td style="background-color: #0f172a; padding: 32px 36px; text-align: center;">
              <table role="presentation" cellspacing="0" cellpadding="0" border="0" align="center">
                <tr>
                  <td style="vertical-align: middle; padding-right: 12px;">
                    <div style="background: linear-gradient(135deg, #4f46e5 0%%, #7c3aed 100%%); width: 44px; height: 44px; border-radius: 12px; text-align: center; color: #ffffff; font-size: 22px; font-weight: 800; line-height: 44px; box-shadow: 0 4px 12px rgba(79, 70, 229, 0.35);">
                      W
                    </div>
                  </td>
                  <td style="vertical-align: middle; text-align: left;">
                    <div style="font-size: 24px; font-weight: 800; letter-spacing: 2px; color: #ffffff; line-height: 1.1;">WAAP</div>
                    <div style="font-size: 11px; font-weight: 600; letter-spacing: 1.5px; text-transform: uppercase; color: #94a3b8; margin-top: 2px;">Account Security</div>
                  </td>
                </tr>
              </table>
            </td>
          </tr>

          <!-- Body Content -->
          <tr>
            <td style="padding: 40px 36px 32px 36px;">
              <h1 class="email-title" style="margin: 0 0 16px 0; font-size: 22px; font-weight: 700; color: #0f172a; text-align: center; letter-spacing: -0.3px;">
                Verify Your Email Address
              </h1>
              <p class="email-text" style="margin: 0 0 16px 0; font-size: 15px; line-height: 1.6; color: #475569;">
                Hi <strong class="email-name" style="color: #0f172a;">%s</strong>,
              </p>
              <p class="email-text" style="margin: 0 0 28px 0; font-size: 15px; line-height: 1.6; color: #475569;">
                Welcome to WAAP! To finalize your registration and protect your account, please enter the One-Time Password (OTP) shown below:
              </p>

              <!-- OTP Code Box -->
              <div class="otp-box" style="background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 14px; padding: 28px 20px; text-align: center; margin: 0 0 28px 0;">
                <span class="otp-hint" style="display: block; font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: 2px; color: #64748b; margin-bottom: 12px;">Your One-Time Password</span>
                <div class="otp-pill" style="display: inline-block; background: #ffffff; border: 2px solid #cbd5e1; border-radius: 10px; padding: 14px 28px; box-shadow: 0 2px 4px rgba(0, 0, 0, 0.04);">
                  <span class="otp-text" style="font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, Monaco, monospace; font-size: 36px; font-weight: 800; letter-spacing: 10px; color: #4338ca; display: inline-block; line-height: 1;">%s</span>
                </div>
                <div class="otp-hint" style="margin-top: 14px; font-size: 12px; font-weight: 600; color: #64748b;">
                  ⏱️ Valid for <span class="email-name" style="color: #0f172a; font-weight: 700;">5 minutes</span> only
                </div>
              </div>

              <!-- Security Warning Callout (CRITICAL) -->
              <div class="warning-card" style="background-color: #fff1f2; border: 1px solid #fecdd3; border-left: 4px solid #e11d48; border-radius: 8px; padding: 16px 18px; margin: 0 0 28px 0;">
                <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0">
                  <tr>
                    <td style="vertical-align: top; width: 28px; font-size: 18px; line-height: 1.2;">
                      ⚠️
                    </td>
                    <td style="vertical-align: top;">
                      <div class="warning-heading" style="font-size: 13px; font-weight: 700; color: #9f1239; margin-bottom: 4px; text-transform: uppercase; letter-spacing: 0.5px;">
                        Do Not Share This OTP
                      </div>
                      <div class="warning-body" style="font-size: 13px; line-height: 1.5; color: #be123c;">
                        Never share this code with anyone under any circumstances. <strong>WAAP staff, customer support, or administrators will NEVER ask for your OTP or password.</strong> Anyone asking for this code is attempting unauthorized access.
                      </div>
                    </td>
                  </tr>
                </table>
              </div>

              <p class="sub-text" style="margin: 0; font-size: 13px; line-height: 1.6; color: #94a3b8; text-align: center;">
                If you didn't create an account with WAAP, you can safely ignore this email. No changes will occur without this OTP.
              </p>
            </td>
          </tr>

          <!-- Footer -->
          <tr>
            <td class="footer-card" style="background-color: #f8fafc; border-top: 1px solid #e2e8f0; padding: 24px 36px; text-align: center;">
              <p class="footer-title" style="margin: 0 0 6px 0; font-size: 12px; font-weight: 600; color: #64748b;">
                WAAP E-Commerce Platform
              </p>
              <p class="footer-text" style="margin: 0 0 8px 0; font-size: 11px; color: #94a3b8;">
                &copy; 2026 WAAP Inc. All rights reserved. &bull; Secure Authentication Service
              </p>
              <p class="footer-text" style="margin: 0; font-size: 11px; line-height: 1.4; color: #94a3b8;">
                This is an automated security transmission. Please do not reply directly to this email.
              </p>
            </td>
          </tr>

        </table>
      </td>
    </tr>
  </table>
</body>
</html>
      </td>
    </tr>
  </table>
</body>
</html>`, html.EscapeString(toName), otp)

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
		HTMLContent: htmlContent,
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
