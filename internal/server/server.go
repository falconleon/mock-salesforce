// Package server provides the HTTP server for the Salesforce mock API.
package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/config"
	"github.com/falconleon/mock-salesforce/internal/store"
)

// Server represents the HTTP server for the mock API.
type Server struct {
	httpServer *http.Server
	config     *config.Config
	store      store.Store
	logger     zerolog.Logger
}

// New creates a new Server with the given configuration.
func New(cfg *config.Config, dataStore store.Store, logger zerolog.Logger) *Server {
	s := &Server{
		config: cfg,
		store:  dataStore,
		logger: logger,
	}

	handler := s.setupRoutes()

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// Start begins listening for HTTP requests.
func (s *Server) Start() error {
	s.logger.Info().
		Int("port", s.config.Port).
		Str("api_version", s.config.APIVersion).
		Bool("auth_enabled", s.config.AuthEnabled).
		Msg("Starting Salesforce Mock API server")

	return s.httpServer.ListenAndServe()
}

// Stop gracefully shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info().Msg("Shutting down server...")
	return s.httpServer.Shutdown(ctx)
}

// Handler returns the HTTP handler for use with httptest.Server.
func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
}

// Store returns the data store.
func (s *Server) Store() store.Store {
	return s.store
}
