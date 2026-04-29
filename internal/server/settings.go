package server

import (
	"html/template"
	"net/http"

	"github.com/falconleon/mock-salesforce/internal/server/middleware"
)

// SettingsHandler renders the /settings profile page and the HTMX
// partials that toggle the OAuth client secret between a masked
// placeholder and the live value (the "eyeball" toggle). Credentials
// are sourced from the running config so the page always reflects the
// effective MOCK_CLIENT_ID / MOCK_CLIENT_SECRET.
type SettingsHandler struct {
	clientID      string
	clientSecret  string
	basePath      string
	sessionSecret string
	pageTpl       *template.Template
	partialTpl    *template.Template
}

// NewSettingsHandler builds the handler with parsed templates. The
// page template includes the hidden-state secret partial so the
// initial render always masks the secret.
func NewSettingsHandler(clientID, clientSecret, basePath, sessionSecret string) *SettingsHandler {
	funcMap := template.FuncMap{
		"basePath": func() string { return basePath },
	}
	pageTpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS,
		"templates/layout.html",
		"templates/settings.html",
		"templates/partials/settings_secret.html",
	))
	partialTpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS,
		"templates/partials/settings_secret.html",
	))
	return &SettingsHandler{
		clientID:      clientID,
		clientSecret:  clientSecret,
		basePath:      basePath,
		sessionSecret: sessionSecret,
		pageTpl:       pageTpl,
		partialTpl:    partialTpl,
	}
}

// currentUser returns the authenticated email from the session cookie,
// or "" if no valid session is present. Mirrors UIHandler.currentUser.
func (h *SettingsHandler) currentUser(r *http.Request) string {
	if h.sessionSecret == "" {
		return ""
	}
	email, _ := middleware.ValidateSession(r, h.sessionSecret)
	return email
}

// HandlePage renders the full Settings page with the client secret
// initially hidden.
func (h *SettingsHandler) HandlePage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.pageTpl.ExecuteTemplate(w, "settings.html", map[string]any{
		"Title":        "Settings",
		"BasePath":     h.basePath,
		"CurrentUser":  h.currentUser(r),
		"ClientID":     h.clientID,
		"ClientSecret": h.clientSecret,
	})
}

// HandleSecretShown returns the partial that reveals the client secret
// in plaintext, with an eye-slash toggle that switches back to hidden.
func (h *SettingsHandler) HandleSecretShown(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.partialTpl.ExecuteTemplate(w, "settings_secret_shown", map[string]any{
		"BasePath":     h.basePath,
		"ClientSecret": h.clientSecret,
	})
}

// HandleSecretHidden returns the partial that masks the client secret,
// with an eye toggle that switches back to shown.
func (h *SettingsHandler) HandleSecretHidden(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.partialTpl.ExecuteTemplate(w, "settings_secret_hidden", map[string]any{
		"BasePath": h.basePath,
	})
}
