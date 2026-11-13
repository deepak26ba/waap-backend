package email

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBrevoSender_Success(t *testing.T) {
	var receivedPayload brevoEmailPayload
	var receivedKey string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedKey = r.Header.Get("api-key")
		if err := json.NewDecoder(r.Body).Decode(&receivedPayload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"messageId": "<test@brevo.com>"}`))
	}))
	defer server.Close()

	sender := NewBrevoSender("test-api-key", "noreply@example.com", "TestApp")
	sender.SetBaseURL(server.URL)

	err := sender.SendVerificationEmail(context.Background(), "user@example.com", "Test User", "123456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedKey != "test-api-key" {
		t.Errorf("expected api-key test-api-key, got %s", receivedKey)
	}
	if len(receivedPayload.To) != 1 || receivedPayload.To[0].Email != "user@example.com" {
		t.Errorf("expected recipient user@example.com, got %+v", receivedPayload.To)
	}
	if receivedPayload.Sender.Email != "noreply@example.com" {
		t.Errorf("expected sender noreply@example.com, got %s", receivedPayload.Sender.Email)
	}
	if receivedPayload.Subject != "Your WAAP Verification OTP: 123456" {
		t.Errorf("expected subject 'Your WAAP Verification OTP: 123456', got %s", receivedPayload.Subject)
	}
}

func TestBrevoSender_EmptyAPIKey(t *testing.T) {
	sender := NewBrevoSender("", "noreply@example.com", "TestApp")
	err := sender.SendVerificationEmail(context.Background(), "user@example.com", "Test User", "tok")
	if err == nil {
		t.Fatal("expected error with empty api key, got nil")
	}
}

func TestBrevoSender_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message": "unauthorized IP"}`))
	}))
	defer server.Close()

	sender := NewBrevoSender("test-api-key", "noreply@example.com", "TestApp")
	sender.SetBaseURL(server.URL)

	err := sender.SendVerificationEmail(context.Background(), "user@example.com", "Test User", "tok")
	if err == nil {
		t.Fatal("expected error on 401 response, got nil")
	}
}

func TestNoopSender(t *testing.T) {
	sender := &NoopSender{}
	err := sender.SendVerificationEmail(context.Background(), "user@example.com", "Test User", "tok")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}
