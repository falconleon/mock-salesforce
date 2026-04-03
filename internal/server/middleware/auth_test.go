package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/server/middleware"
	"github.com/falconleon/mock-salesforce/pkg/models"
)

func TestAuth_PublicPaths(t *testing.T) {
	logger := zerolog.Nop()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authMiddleware := middleware.Auth(logger, "test-secret")(handler)

	publicPaths := []string{
		"/services/oauth2/token",
		"/health",
		"/",
	}

	for _, path := range publicPaths {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()

		authMiddleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected public path %s to return 200, got %d", path, rec.Code)
		}
	}
}

func TestAuth_MissingToken(t *testing.T) {
	logger := zerolog.Nop()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authMiddleware := middleware.Auth(logger, "test-secret")(handler)

	req := httptest.NewRequest("GET", "/services/data/v66.0/query", nil)
	rec := httptest.NewRecorder()

	authMiddleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}

	var errors []models.APIError
	if err := json.NewDecoder(rec.Body).Decode(&errors); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if len(errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(errors))
	}
	if errors[0].ErrorCode != "INVALID_SESSION_ID" {
		t.Errorf("expected errorCode 'INVALID_SESSION_ID', got '%s'", errors[0].ErrorCode)
	}
}

func TestAuth_InvalidToken(t *testing.T) {
	logger := zerolog.Nop()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authMiddleware := middleware.Auth(logger, "test-secret")(handler)

	req := httptest.NewRequest("GET", "/services/data/v66.0/query", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()

	authMiddleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestAuth_ValidToken(t *testing.T) {
	logger := zerolog.Nop()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authMiddleware := middleware.Auth(logger, "test-secret")(handler)

	// Register a test token
	middleware.RegisterToken("test-valid-token")

	req := httptest.NewRequest("GET", "/services/data/v66.0/query", nil)
	req.Header.Set("Authorization", "Bearer test-valid-token")
	rec := httptest.NewRecorder()

	authMiddleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 for valid token, got %d", rec.Code)
	}
}

func TestAuth_InvalidHeaderFormat(t *testing.T) {
	logger := zerolog.Nop()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authMiddleware := middleware.Auth(logger, "test-secret")(handler)

	req := httptest.NewRequest("GET", "/services/data/v66.0/query", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz") // Basic auth format
	rec := httptest.NewRecorder()

	authMiddleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for invalid header format, got %d", rec.Code)
	}
}
