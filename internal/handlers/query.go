package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/soql"
	"github.com/falconleon/mock-salesforce/internal/store"
	"github.com/falconleon/mock-salesforce/pkg/models"
)

// QueryHandler handles SOQL query requests.
type QueryHandler struct {
	store  store.Store
	logger zerolog.Logger
}

// NewQueryHandler creates a new query handler.
func NewQueryHandler(s store.Store, logger zerolog.Logger) *QueryHandler {
	return &QueryHandler{
		store:  s,
		logger: logger.With().Str("handler", "query").Logger(),
	}
}

// HandleQuery processes SOQL query requests.
// GET /services/data/vXX.0/query?q=...
func (h *QueryHandler) HandleQuery(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		h.writeError(w, http.StatusBadRequest, "MALFORMED_QUERY", "Missing query parameter 'q'")
		return
	}

	h.logger.Debug().Str("query", query).Msg("Executing SOQL query")

	// Parse the query
	parser := soql.NewParser(query)
	stmt, err := parser.Parse()
	if err != nil {
		h.logger.Warn().Err(err).Str("query", query).Msg("Failed to parse query")
		h.writeError(w, http.StatusBadRequest, "MALFORMED_QUERY", err.Error())
		return
	}

	// Execute the query
	executor := soql.NewExecutor(h.store)
	result, err := executor.Execute(stmt)
	if err != nil {
		h.logger.Error().Err(err).Str("query", query).Msg("Failed to execute query")
		h.writeError(w, http.StatusInternalServerError, "QUERY_ERROR", err.Error())
		return
	}

	h.logger.Info().
		Str("object", stmt.Object).
		Int("results", result.TotalSize).
		Msg("Query executed")

	// Build response
	response := models.QueryResponse{
		TotalSize: result.TotalSize,
		Done:      result.Done,
		Records:   result.Records,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// writeError writes a Salesforce-style error response.
func (h *QueryHandler) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode([]models.APIError{
		{
			Message:   message,
			ErrorCode: code,
		},
	})
}
