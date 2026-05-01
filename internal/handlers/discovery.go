package handlers

import (
	"net/http"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/config"
)

// DiscoveryHandler serves the OAuth 2.0 Authorization Server / OpenID
// Connect discovery document at /.well-known/openid-configuration. The
// document advertises the endpoints, grant types, and capabilities
// implemented by the mock so tooling (e.g. sf CLI, OIDC libraries) can
// auto-configure without hard-coded paths.
type DiscoveryHandler struct {
	config *config.Config
	logger zerolog.Logger
}

// NewDiscoveryHandler creates a new discovery handler.
func NewDiscoveryHandler(cfg *config.Config, logger zerolog.Logger) *DiscoveryHandler {
	return &DiscoveryHandler{
		config: cfg,
		logger: logger.With().Str("handler", "discovery").Logger(),
	}
}

// HandleDiscovery serves GET /.well-known/openid-configuration. The
// endpoint URLs are built from the live request (honouring
// X-Forwarded-Proto / X-Forwarded-Host) so the document remains valid
// regardless of the host/port the server is reached on.
func (h *DiscoveryHandler) HandleDiscovery(w http.ResponseWriter, r *http.Request) {
	issuer := requestBaseURL(r)
	doc := map[string]any{
		"issuer":                                issuer,
		"authorization_endpoint":                issuer + "/services/oauth2/authorize",
		"token_endpoint":                        issuer + "/services/oauth2/token",
		"userinfo_endpoint":                     issuer + "/services/oauth2/userinfo",
		"revocation_endpoint":                   issuer + "/services/oauth2/revoke",
		"introspection_endpoint":                issuer + "/services/oauth2/introspect",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "password", "refresh_token", "client_credentials"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post"},
		"code_challenge_methods_supported":      []string{"S256", "plain"},
		"scopes_supported":                      []string{"api", "refresh_token", "openid", "id", "profile", "email"},
		"subject_types_supported":               []string{"public"},
	}
	writeJSON(w, http.StatusOK, doc)
}

// requestBaseURL returns scheme://host derived from the live request,
// honouring X-Forwarded-Proto and X-Forwarded-Host when present.
func requestBaseURL(r *http.Request) string {
	scheme := "http"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		host = fwd
	}
	return scheme + "://" + host
}
