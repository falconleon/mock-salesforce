package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/config"
	"github.com/falconleon/mock-salesforce/internal/server/middleware"
)

// AuthorizeHandler implements the front half of the OAuth 2.0
// authorization_code flow (RFC 6749 §4.1) with mandatory PKCE
// (RFC 7636). GET renders a consent screen; POST records the user's
// allow/deny decision and redirects back to the registered redirect_uri
// with a one-time authorization code or an error.
type AuthorizeHandler struct {
	config     *config.Config
	logger     zerolog.Logger
	codes      *AuthCodeStore
	consentTpl *template.Template
}

// NewAuthorizeHandler constructs an AuthorizeHandler. consentTpl must
// resolve a "layout" template that wraps a "content" block; the handler
// renders it via ExecuteTemplate(w, "layout", data).
func NewAuthorizeHandler(cfg *config.Config, codes *AuthCodeStore, consentTpl *template.Template, logger zerolog.Logger) *AuthorizeHandler {
	return &AuthorizeHandler{
		config:     cfg,
		logger:     logger.With().Str("handler", "authorize").Logger(),
		codes:      codes,
		consentTpl: consentTpl,
	}
}

// authorizeParams collects the OAuth authorization request parameters
// from either query string (GET) or form body (POST).
type authorizeParams struct {
	responseType        string
	clientID            string
	redirectURI         string
	scope               string
	state               string
	codeChallenge       string
	codeChallengeMethod string
}

func parseAuthorizeParams(r *http.Request) authorizeParams {
	get := func(name string) string {
		if r.Method == http.MethodPost {
			if v := r.PostFormValue(name); v != "" {
				return v
			}
		}
		return r.URL.Query().Get(name)
	}
	return authorizeParams{
		responseType:        get("response_type"),
		clientID:            get("client_id"),
		redirectURI:         get("redirect_uri"),
		scope:               get("scope"),
		state:               get("state"),
		codeChallenge:       get("code_challenge"),
		codeChallengeMethod: get("code_challenge_method"),
	}
}

// HandleGet serves GET /services/oauth2/authorize. Validates the
// request, prompts for login if no session is present, and otherwise
// renders the consent screen.
func (h *AuthorizeHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	params := parseAuthorizeParams(r)
	if !h.validateClientAndRedirect(w, params) {
		return
	}
	if errCode, errDesc, ok := h.validateRest(params); !ok {
		h.redirectError(w, r, params, errCode, errDesc)
		return
	}
	user, ok := middleware.ValidateSession(r, h.config.SessionSecret)
	if !ok {
		h.redirectToLogin(w, r)
		return
	}
	h.renderConsent(w, params, user)
}

// HandlePost serves POST /services/oauth2/authorize. Re-validates the
// request, then either issues an authorization code (action=allow) or
// returns access_denied (anything else).
func (h *AuthorizeHandler) HandlePost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.errorHTML(w, http.StatusBadRequest, "invalid_request", "unable to parse form")
		return
	}
	params := parseAuthorizeParams(r)
	if !h.validateClientAndRedirect(w, params) {
		return
	}
	if errCode, errDesc, ok := h.validateRest(params); !ok {
		h.redirectError(w, r, params, errCode, errDesc)
		return
	}
	user, ok := middleware.ValidateSession(r, h.config.SessionSecret)
	if !ok {
		h.redirectToLogin(w, r)
		return
	}
	if r.PostFormValue("action") != "allow" {
		h.redirectError(w, r, params, "access_denied", "user denied request")
		return
	}
	ac := h.codes.Issue(
		params.clientID,
		params.redirectURI,
		params.scope,
		params.codeChallenge,
		params.codeChallengeMethod,
		user,
	)
	h.logger.Info().
		Str("client_id", params.clientID).
		Str("username", user).
		Msg("OAuth authorization code issued")
	h.redirectSuccess(w, r, params, ac.Code)
}

// validateClientAndRedirect enforces RFC 6749 §4.1.2.1: errors that
// concern the client_id or redirect_uri MUST NOT be reported via
// redirect (since the redirect target itself is untrusted) — return
// 400 HTML directly to the user agent instead.
func (h *AuthorizeHandler) validateClientAndRedirect(w http.ResponseWriter, p authorizeParams) bool {
	if p.clientID == "" {
		h.errorHTML(w, http.StatusBadRequest, "invalid_request", "missing client_id")
		return false
	}
	if p.clientID != h.config.MockClientID {
		h.errorHTML(w, http.StatusBadRequest, "invalid_client", "unknown client_id")
		return false
	}
	if p.redirectURI == "" {
		h.errorHTML(w, http.StatusBadRequest, "invalid_request", "missing redirect_uri")
		return false
	}
	// Reject malformed values; production Salesforce restricts to the
	// connected app's registered URIs. The MockRedirectURIs allowlist
	// (RFC 6749 §3.1.2.2) provides the same protection here when set.
	u, err := url.Parse(p.redirectURI)
	if err != nil || !u.IsAbs() {
		h.errorHTML(w, http.StatusBadRequest, "invalid_request", "malformed redirect_uri")
		return false
	}
	if !h.config.IsRedirectURIAllowed(p.redirectURI) {
		h.errorHTML(w, http.StatusBadRequest, "invalid_request", "redirect_uri not registered for this client_id")
		return false
	}
	return true
}

// validateRest checks parameters that are safe to surface to the
// client via the redirect_uri (response_type, PKCE).
func (h *AuthorizeHandler) validateRest(p authorizeParams) (string, string, bool) {
	if p.responseType == "" {
		return "invalid_request", "response_type is required", false
	}
	if p.responseType != "code" {
		return "unsupported_response_type", "only response_type=code is supported", false
	}
	if p.codeChallenge == "" {
		return "invalid_request", "code_challenge is required (PKCE)", false
	}
	if p.codeChallengeMethod == "" {
		return "invalid_request", "code_challenge_method is required", false
	}
	if p.codeChallengeMethod != "S256" && p.codeChallengeMethod != "plain" {
		return "invalid_request", "code_challenge_method must be S256 or plain", false
	}
	return "", "", true
}

func (h *AuthorizeHandler) redirectError(w http.ResponseWriter, r *http.Request, p authorizeParams, errCode, errDesc string) {
	u, _ := url.Parse(p.redirectURI)
	q := u.Query()
	q.Set("error", errCode)
	if errDesc != "" {
		q.Set("error_description", errDesc)
	}
	if p.state != "" {
		q.Set("state", p.state)
	}
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (h *AuthorizeHandler) redirectSuccess(w http.ResponseWriter, r *http.Request, p authorizeParams, code string) {
	u, _ := url.Parse(p.redirectURI)
	q := u.Query()
	q.Set("code", code)
	if p.state != "" {
		q.Set("state", p.state)
	}
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

// redirectToLogin sends the user agent to /login with a same-origin
// next= pointing back at the original /services/oauth2/authorize
// request so the consent screen reappears post-login.
func (h *AuthorizeHandler) redirectToLogin(w http.ResponseWriter, r *http.Request) {
	next := r.URL.Path
	if r.URL.RawQuery != "" {
		next += "?" + r.URL.RawQuery
	}
	params := url.Values{}
	params.Set("next", next)
	http.Redirect(w, r, h.config.BasePath+"/login?"+params.Encode(), http.StatusFound)
}

func (h *AuthorizeHandler) renderConsent(w http.ResponseWriter, p authorizeParams, user string) {
	data := map[string]any{
		"Title":               "Authorize",
		"BasePath":            h.config.BasePath,
		"CurrentUser":         user,
		"ClientID":            p.clientID,
		"RedirectURI":         p.redirectURI,
		"Scope":               p.scope,
		"State":               p.state,
		"ResponseType":        p.responseType,
		"CodeChallenge":       p.codeChallenge,
		"CodeChallengeMethod": p.codeChallengeMethod,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.consentTpl.ExecuteTemplate(w, "layout", data); err != nil {
		h.logger.Error().Err(err).Msg("render consent template")
	}
}

// errorHTML writes a minimal 400-style error page for client_id /
// redirect_uri failures that must not redirect.
func (h *AuthorizeHandler) errorHTML(w http.ResponseWriter, status int, errCode, errDesc string) {
	h.logger.Warn().Str("error", errCode).Str("description", errDesc).Msg("authorize request rejected")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en"><head><meta charset="UTF-8"><title>Authorization Error</title></head>
<body style="font-family:sans-serif;max-width:600px;margin:4rem auto;padding:2rem;">
<h1>Authorization error</h1>
<p><strong>%s</strong>: %s</p>
</body></html>`,
		template.HTMLEscapeString(errCode),
		template.HTMLEscapeString(errDesc),
	)
}
