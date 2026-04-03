package server

import (
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/falconleon/mock-salesforce/internal/handlers"
	"github.com/falconleon/mock-salesforce/internal/server/middleware"
	"github.com/falconleon/mock-salesforce/internal/store"
)

// setupRoutes configures all HTTP routes for the mock API.
func (s *Server) setupRoutes() http.Handler {
	mux := http.NewServeMux()

	// Create handlers with dependencies
	oauthHandler := handlers.NewOAuthHandler(s.config, s.logger)
	healthHandler := handlers.NewHealthHandler(s.logger)
	queryHandler := handlers.NewQueryHandler(s.store, s.logger)
	sobjectHandler := handlers.NewSObjectHandler(s.store, s.logger)

	// Public routes (no auth required)
	mux.HandleFunc("POST /services/oauth2/token", oauthHandler.HandleToken)
	mux.HandleFunc("GET /health", healthHandler.HandleHealth)

	basePath := s.config.BasePath
	hasMultiUser := len(s.config.MockUsers) > 0

	// Root handler: login page when multi-user, redirect when single-user
	loginTpl := template.Must(template.ParseFS(templateFS, "templates/login.html"))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if hasMultiUser {
			// Check if already authenticated via session cookie
			if _, ok := middleware.ValidateSession(r, s.config.SessionSecret); ok {
				http.Redirect(w, r, basePath+"/lightning/o/Case/list", http.StatusFound)
				return
			}
			// Show login form
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			loginTpl.ExecuteTemplate(w, "login", map[string]any{
				"BasePath": basePath,
				"Error":    r.URL.Query().Get("error") != "",
			})
			return
		}
		http.Redirect(w, r, basePath+"/lightning/o/Case/list", http.StatusFound)
	})

	// Login endpoint for UI session auth
	if hasMultiUser {
		mux.Handle("POST /login", middleware.LoginHandler(
			s.config.MockUsers, s.config.SessionSecret, basePath,
		))
	}

	// Admin endpoints (no auth required)
	mux.HandleFunc("POST /admin/reset", func(w http.ResponseWriter, r *http.Request) {
		s.store.Clear()
		if ls, ok := s.store.(store.LoadableStore); ok {
			loader := store.NewLoader(ls, s.logger)
			if err := loader.LoadFromDirectory(s.config.SeedDataPath); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(500)
				json.NewEncoder(w).Encode(map[string]any{"status": "error", "error": err.Error()})
				return
			}
			stats := ls.Stats()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"status": "reset_complete", "stats": stats})
		} else {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"status": "reset_complete", "note": "clear only, no reload"})
		}
	})

	// UI routes — Salesforce Lightning URL patterns
	ui := NewUIHandler(s.store, basePath)
	mux.HandleFunc("GET /lightning/o/Case/list", ui.CaseList)
	mux.HandleFunc("GET /lightning/r/Case/{id}/view", ui.CaseDetail)
	mux.HandleFunc("GET /lightning/o/Account/list", ui.AccountList)
	mux.HandleFunc("GET /lightning/r/Account/{id}/view", ui.AccountDetail)

	// Static assets
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFS)))

	// Query endpoint (supports multiple API versions)
	mux.HandleFunc("GET /services/data/{version}/query", queryHandler.HandleQuery)

	// SObject CRUD endpoints
	mux.HandleFunc("GET /services/data/{version}/sobjects/{type}/describe", sobjectHandler.HandleDescribe)
	mux.HandleFunc("GET /services/data/{version}/sobjects/{type}/{id}", sobjectHandler.HandleGet)
	mux.HandleFunc("POST /services/data/{version}/sobjects/{type}", sobjectHandler.HandleCreate)
	mux.HandleFunc("PATCH /services/data/{version}/sobjects/{type}/{id}", sobjectHandler.HandleUpdate)
	mux.HandleFunc("DELETE /services/data/{version}/sobjects/{type}/{id}", sobjectHandler.HandleDelete)

	// Apply middleware chain
	var handler http.Handler = mux

	// Logging middleware (always enabled)
	handler = middleware.Logging(s.logger)(handler)

	// CORS middleware
	handler = middleware.CORS()(handler)

	// Auth middleware (conditionally enabled)
	if s.config.AuthEnabled {
		handler = middleware.Auth(s.logger, s.config.SessionSecret)(handler)
	}

	return handler
}
