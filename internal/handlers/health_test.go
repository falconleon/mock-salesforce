package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/handlers"
)

func TestHealthHandler_HandleHealth(t *testing.T) {
	logger := zerolog.Nop()
	handler := handlers.NewHealthHandler(logger)

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	handler.HandleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp handlers.HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("expected status 'healthy', got '%s'", resp.Status)
	}
	if resp.Service != "salesforce-mock-api" {
		t.Errorf("expected service 'salesforce-mock-api', got '%s'", resp.Service)
	}
	if resp.Version != "v66.0" {
		t.Errorf("expected version 'v66.0', got '%s'", resp.Version)
	}
}

func TestHealthHandler_HandleRoot(t *testing.T) {
	logger := zerolog.Nop()
	handler := handlers.NewHealthHandler(logger)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.HandleRoot(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp handlers.RootResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Service != "Salesforce Mock API" {
		t.Errorf("expected service 'Salesforce Mock API', got '%s'", resp.Service)
	}
	if len(resp.Endpoints) == 0 {
		t.Error("expected endpoints to be listed")
	}
}
