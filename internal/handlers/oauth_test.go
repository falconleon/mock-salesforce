package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/config"
	"github.com/falconleon/mock-salesforce/internal/handlers"
	"github.com/falconleon/mock-salesforce/pkg/models"
)

func TestOAuthHandler_HandleToken_Success(t *testing.T) {
	cfg := config.Default()
	logger := zerolog.Nop()
	handler := handlers.NewOAuthHandler(cfg, logger)

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)
	form.Set("username", cfg.MockUsername)
	form.Set("password", cfg.MockPassword)

	req := httptest.NewRequest("POST", "/services/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.HandleToken(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp models.OAuthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.AccessToken == "" {
		t.Error("expected access_token to be set")
	}
	if resp.TokenType != "Bearer" {
		t.Errorf("expected token_type 'Bearer', got '%s'", resp.TokenType)
	}
	if resp.Scope != "api refresh_token" {
		t.Errorf("expected scope 'api refresh_token', got '%s'", resp.Scope)
	}
	if resp.IssuedAt == "" {
		t.Error("expected issued_at to be set")
	}
	if resp.Signature == "" {
		t.Error("expected signature to be set")
	}
}

func TestOAuthHandler_HandleToken_InvalidGrant(t *testing.T) {
	cfg := config.Default()
	logger := zerolog.Nop()
	handler := handlers.NewOAuthHandler(cfg, logger)

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)
	form.Set("username", cfg.MockUsername)
	form.Set("password", "wrong-password")

	req := httptest.NewRequest("POST", "/services/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.HandleToken(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}

	var resp models.OAuthError
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if resp.Error != "invalid_grant" {
		t.Errorf("expected error 'invalid_grant', got '%s'", resp.Error)
	}
}

func TestOAuthHandler_HandleToken_UnsupportedGrantType(t *testing.T) {
	cfg := config.Default()
	logger := zerolog.Nop()
	handler := handlers.NewOAuthHandler(cfg, logger)

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)

	req := httptest.NewRequest("POST", "/services/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.HandleToken(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}

	var resp models.OAuthError
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if resp.Error != "unsupported_grant_type" {
		t.Errorf("expected error 'unsupported_grant_type', got '%s'", resp.Error)
	}
}

func TestOAuthHandler_HandleToken_InvalidClientID(t *testing.T) {
	cfg := config.Default()
	logger := zerolog.Nop()
	handler := handlers.NewOAuthHandler(cfg, logger)

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", "wrong-client-id")
	form.Set("client_secret", cfg.MockClientSecret)
	form.Set("username", cfg.MockUsername)
	form.Set("password", cfg.MockPassword)

	req := httptest.NewRequest("POST", "/services/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.HandleToken(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}

	var resp models.OAuthError
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if resp.Error != "invalid_client_id" {
		t.Errorf("expected error 'invalid_client_id', got '%s'", resp.Error)
	}
}
