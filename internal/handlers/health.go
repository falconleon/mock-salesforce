package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// HealthHandler handles health check requests.
type HealthHandler struct {
	logger    zerolog.Logger
	startTime time.Time
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(logger zerolog.Logger) *HealthHandler {
	return &HealthHandler{
		logger:    logger.With().Str("handler", "health").Logger(),
		startTime: time.Now(),
	}
}

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Version   string `json:"version"`
	Uptime    string `json:"uptime"`
	Timestamp string `json:"timestamp"`
}

// HandleHealth responds with server health status.
// GET /health
func (h *HealthHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(h.startTime).Round(time.Second)

	response := HealthResponse{
		Status:    "healthy",
		Service:   "salesforce-mock-api",
		Version:   "v66.0",
		Uptime:    uptime.String(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// RootResponse represents the root endpoint response.
type RootResponse struct {
	Service     string   `json:"service"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Endpoints   []string `json:"endpoints"`
}

// HandleRoot provides API information at the root path.
// GET /
func (h *HealthHandler) HandleRoot(w http.ResponseWriter, r *http.Request) {
	response := RootResponse{
		Service:     "Salesforce Mock API",
		Description: "Mock implementation of Salesforce REST API for development and testing",
		Version:     "v66.0",
		Endpoints: []string{
			"POST /services/oauth2/token - OAuth authentication",
			"GET  /services/data/v66.0/query - SOQL queries (coming soon)",
			"GET  /services/data/v66.0/sobjects/{type}/{id} - Get record (coming soon)",
			"GET  /health - Health check",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
