package server

import (
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"strings"

	"github.com/falconleon/mock-salesforce/internal/handlers"
	"github.com/falconleon/mock-salesforce/internal/server/middleware"
	"github.com/falconleon/mock-salesforce/internal/store"
)

// staticHandler serves files from the embedded static FS at /static/.
// The embedded FS preserves the "static/" directory prefix, so we
// fs.Sub it before serving to align the FS root with the URL prefix.
func staticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return http.StripPrefix("/static/", http.FileServerFS(staticFS))
	}
	return http.StripPrefix("/static/", http.FileServerFS(sub))
}

// setupRoutes configures all HTTP routes for the mock API.
func (s *Server) setupRoutes() http.Handler {
	mux := http.NewServeMux()

	// Create handlers with dependencies
	authCodeStore := handlers.NewAuthCodeStore()
	oauthHandler := handlers.NewOAuthHandler(s.config, s.logger).WithAuthCodes(authCodeStore)
	healthHandler := handlers.NewHealthHandler(s.logger)
	queryHandler := handlers.NewQueryHandler(s.store, s.logger)
	sobjectHandler := handlers.NewSObjectHandler(s.store, s.logger)

	// Public routes (no auth required)
	mux.HandleFunc("POST /services/oauth2/token", oauthHandler.HandleToken)
	mux.HandleFunc("POST /services/oauth2/revoke", oauthHandler.HandleRevoke)
	mux.HandleFunc("GET /services/oauth2/revoke", oauthHandler.HandleRevoke)
	mux.HandleFunc("POST /services/oauth2/introspect", oauthHandler.HandleIntrospect)
	mux.HandleFunc("GET /services/oauth2/userinfo", oauthHandler.HandleUserinfo)
	mux.HandleFunc("GET /health", healthHandler.HandleHealth)

	basePath := s.config.BasePath

	// OAuth authorization_code flow front-end (RFC 6749 §4.1 + PKCE).
	// Templates resolve the layout chrome, so we parse with the same
	// basePath func map used by other UI pages.
	authorizeFuncMap := template.FuncMap{"basePath": func() string { return basePath }}
	authorizeTpl := template.Must(template.New("").Funcs(authorizeFuncMap).ParseFS(templateFS, "templates/layout.html", "templates/authorize.html"))
	authorizeHandler := handlers.NewAuthorizeHandler(s.config, authCodeStore, authorizeTpl, s.logger)
	mux.HandleFunc("GET /services/oauth2/authorize", authorizeHandler.HandleGet)
	mux.HandleFunc("POST /services/oauth2/authorize", authorizeHandler.HandlePost)
	loginUsers := s.config.MockUsers
	if len(loginUsers) == 0 && s.config.MockUsername != "" {
		loginUsers = map[string]string{s.config.MockUsername: s.config.MockPassword}
	}

	// Root handler: redirect authenticated users to /home, others to /login.
	loginTpl := template.Must(template.ParseFS(templateFS, "templates/login.html"))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		falconReturn := middleware.ExtractFalconReturn(r)
		if falconReturn != "" {
			middleware.SetFalconReturnCookie(w, falconReturn)
		}
		if _, ok := middleware.ValidateSession(r, s.config.SessionSecret); ok {
			http.Redirect(w, r, basePath+"/home", http.StatusFound)
			return
		}
		http.Redirect(w, r, basePath+"/login", http.StatusFound)
	})

	// Login form
	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		falconReturn := middleware.ExtractFalconReturn(r)
		if falconReturn != "" {
			middleware.SetFalconReturnCookie(w, falconReturn)
		}
		// If already authenticated, jump straight to next or /home.
		if _, ok := middleware.ValidateSession(r, s.config.SessionSecret); ok {
			next := r.URL.Query().Get("next")
			if next != "" && strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "//") {
				http.Redirect(w, r, basePath+next, http.StatusFound)
				return
			}
			http.Redirect(w, r, basePath+"/home", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = loginTpl.ExecuteTemplate(w, "login", map[string]any{
			"BasePath":     basePath,
			"Error":        r.URL.Query().Get("error") != "",
			"FalconReturn": falconReturn,
			"Next":         r.URL.Query().Get("next"),
		})
	})

	// Login + logout endpoints for UI session auth.
	if len(loginUsers) > 0 {
		mux.Handle("POST /login", middleware.LoginHandler(
			loginUsers, s.config.SessionSecret, basePath,
		))
	}
	mux.Handle("GET /logout", middleware.LogoutHandler(basePath))

	// Admin user CRUD + per-user token issuance (gated by X-Admin-Token).
	if s.userStore != nil {
		adminUsers := handlers.NewAdminUsersHandler(s.userStore, s.config.AdminToken, s.logger)
		mux.HandleFunc("/admin/users", adminUsers.HandleUsers)
		mux.HandleFunc("/admin/users/{id}", adminUsers.HandleUser)
		mux.HandleFunc("/admin/users/{id}/tokens", adminUsers.HandleTokens)
		mux.HandleFunc("/admin/users/{id}/tokens/{tokenId}", adminUsers.HandleToken)

		// Browser-facing settings pages for the same user store. Uses the
		// session-cookie auth path (gated by the global Auth middleware)
		// so admins can manage users without juggling X-Admin-Token in a
		// browser.
		settingsUsers := NewSettingsUsersHandler(s.userStore, basePath, s.config.SessionSecret)
		mux.HandleFunc("/settings/users", settingsUsers.HandleList)
		mux.HandleFunc("/settings/users/{id}", settingsUsers.HandleDetail)
		mux.HandleFunc("/settings/users/{id}/tokens", settingsUsers.HandleTokens)
		mux.HandleFunc("/settings/users/{id}/tokens/{tokenId}/revoke", settingsUsers.HandleTokenRevoke)
	}

	// Settings / Profile page exposing the OAuth client credentials with
	// an eyeball toggle for the secret. The hidden/shown partials are
	// served as separate routes so HTMX can swap between them.
	settings := NewSettingsHandler(s.config.MockClientID, s.config.MockClientSecret, basePath, s.config.SessionSecret)
	mux.HandleFunc("GET /settings", settings.HandlePage)
	mux.HandleFunc("GET /settings/secret/shown", settings.HandleSecretShown)
	mux.HandleFunc("GET /settings/secret/hidden", settings.HandleSecretHidden)

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
	ui := NewUIHandler(s.store, basePath, s.config.SessionSecret)
	mux.HandleFunc("GET /home", ui.Home)
	mux.HandleFunc("GET /lightning/o/Case/list", ui.CaseList)
	mux.HandleFunc("GET /lightning/r/Case/{id}/view", ui.CaseDetail)
	mux.HandleFunc("GET /lightning/r/Case/{id}/related/emails", ui.CaseEmailsPartial)
	mux.HandleFunc("GET /lightning/r/Case/{id}/related/comments", ui.CaseCommentsPartial)
	mux.HandleFunc("GET /lightning/r/Case/{id}/related/feed", ui.CaseFeedPartial)
	mux.HandleFunc("GET /lightning/r/Case/{id}/related/activities", ui.CaseActivitiesPartial)
	mux.HandleFunc("GET /lightning/r/Case/{id}/related/files", ui.CaseFilesPartial)
	mux.HandleFunc("GET /lightning/o/Account/list", ui.AccountList)
	mux.HandleFunc("GET /lightning/r/Account/{id}/view", ui.AccountDetail)
	mux.HandleFunc("GET /lightning/r/Contact/{id}/view", ui.ContactDetail)
	mux.HandleFunc("GET /lightning/r/User/{id}/view", ui.UserDetail)

	// SOQL playground UI — exercises the same executor used by the REST query API.
	playground := NewPlaygroundHandler(s.store, basePath, s.config.SessionSecret)
	mux.HandleFunc("GET /playground", playground.Page)
	mux.HandleFunc("POST /playground/run", playground.Run)

	// Static assets
	mux.Handle("GET /static/", staticHandler())

	// Query endpoint (supports multiple API versions)
	mux.HandleFunc("GET /services/data/{version}/query", queryHandler.HandleQuery)

	// Tooling API SOQL endpoint — reuses the standard query handler.
	mux.HandleFunc("GET /services/data/{version}/tooling/query", queryHandler.HandleQuery)
	mux.HandleFunc("GET /services/data/{version}/tooling/query/", queryHandler.HandleQuery)

	// SObject CRUD endpoints
	mux.HandleFunc("GET /services/data/{version}/sobjects/", sobjectHandler.HandleGlobalDescribe)
	mux.HandleFunc("GET /services/data/{version}/sobjects/{type}/describe", sobjectHandler.HandleDescribe)
	mux.HandleFunc("GET /services/data/{version}/sobjects/{type}/{id}", sobjectHandler.HandleGet)
	mux.HandleFunc("POST /services/data/{version}/sobjects/{type}", sobjectHandler.HandleCreate)
	mux.HandleFunc("PATCH /services/data/{version}/sobjects/{type}/{id}", sobjectHandler.HandleUpdate)
	mux.HandleFunc("DELETE /services/data/{version}/sobjects/{type}/{id}", sobjectHandler.HandleDelete)

	// Apply middleware chain
	var handler http.Handler = mux

	// Capture falcon_return query param on any request
	handler = middleware.CaptureFalconReturn()(handler)

	// Logging middleware (always enabled)
	handler = middleware.Logging(s.logger)(handler)

	// CORS middleware
	handler = middleware.CORS()(handler)

	// Auth middleware (conditionally enabled)
	if s.config.AuthEnabled {
		handler = middleware.Auth(s.logger, s.config.SessionSecret)(handler)
	}

	// Strip BASE_PATH prefix so the mux and middleware see unprefixed paths.
	// When deployed behind a reverse proxy at e.g. /mock/salesforce, the proxy
	// forwards the full path; StripPrefix removes the prefix before routing.
	if basePath != "" {
		handler = http.StripPrefix(basePath, handler)
	}

	return handler
}
