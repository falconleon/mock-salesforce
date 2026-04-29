// Package handlers provides HTTP handlers for the Salesforce mock API.
package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/config"
	"github.com/falconleon/mock-salesforce/internal/server/middleware"
	"github.com/falconleon/mock-salesforce/pkg/models"
)

// OAuthHandler handles OAuth authentication requests.
type OAuthHandler struct {
	config *config.Config
	logger zerolog.Logger
}

// NewOAuthHandler creates a new OAuth handler.
func NewOAuthHandler(cfg *config.Config, logger zerolog.Logger) *OAuthHandler {
	return &OAuthHandler{
		config: cfg,
		logger: logger.With().Str("handler", "oauth").Logger(),
	}
}

const (
	mockOrgID  = "00Dxx0000001gPq"
	defaultSub = "005xx000001SwiUAAS"
	scopeFull  = "api refresh_token"
)

// HandleToken processes OAuth token requests.
// POST /services/oauth2/token — supports grant_type of password,
// refresh_token, and client_credentials.
func (h *OAuthHandler) HandleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.writeOAuthError(w, http.StatusBadRequest, "invalid_request", "Unable to parse request body")
		return
	}

	grantType := r.FormValue("grant_type")
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")

	h.logger.Debug().
		Str("grant_type", grantType).
		Str("client_id", clientID).
		Msg("OAuth token request received")

	switch grantType {
	case "password":
		h.handlePasswordGrant(w, r, clientID, clientSecret)
	case "refresh_token":
		h.handleRefreshGrant(w, r, clientID, clientSecret)
	case "client_credentials":
		h.handleClientCredentialsGrant(w, clientID, clientSecret)
	default:
		h.writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type",
			fmt.Sprintf("Grant type '%s' not supported.", grantType))
	}
}

func (h *OAuthHandler) handlePasswordGrant(w http.ResponseWriter, r *http.Request, clientID, clientSecret string) {
	if !h.validateClient(w, clientID, clientSecret) {
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")
	if !h.validateUser(username, password) {
		h.writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "authentication failure")
		return
	}
	h.issueTokens(w, username, true)
}

func (h *OAuthHandler) handleRefreshGrant(w http.ResponseWriter, r *http.Request, clientID, clientSecret string) {
	if !h.validateClient(w, clientID, clientSecret) {
		return
	}
	refresh := r.FormValue("refresh_token")
	info := middleware.LookupToken(refresh)
	if info == nil || info.Type != "refresh" {
		h.writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "expired access/refresh token")
		return
	}
	h.issueRefreshedAccess(w, info)
}

func (h *OAuthHandler) handleClientCredentialsGrant(w http.ResponseWriter, clientID, clientSecret string) {
	if !h.validateClient(w, clientID, clientSecret) {
		return
	}
	// Server-to-server: no user context, no refresh token.
	h.issueTokens(w, "", false)
}

// validateClient checks client_id and client_secret, writing an error
// response and returning false if either is invalid.
func (h *OAuthHandler) validateClient(w http.ResponseWriter, clientID, clientSecret string) bool {
	if subtle.ConstantTimeCompare([]byte(clientID), []byte(h.config.MockClientID)) != 1 {
		h.writeOAuthError(w, http.StatusBadRequest, "invalid_client_id", "Invalid client_id")
		return false
	}
	if subtle.ConstantTimeCompare([]byte(clientSecret), []byte(h.config.MockClientSecret)) != 1 {
		h.writeOAuthError(w, http.StatusBadRequest, "invalid_client", "Invalid client credentials")
		return false
	}
	return true
}

// validateUser checks the supplied password against the configured
// users. Real Salesforce accepts password concatenated with a security
// token; we accept either an exact match or a value that begins with
// the configured password.
func (h *OAuthHandler) validateUser(username, password string) bool {
	expected := ""
	if len(h.config.MockUsers) > 0 {
		v, ok := h.config.MockUsers[username]
		if !ok {
			return false
		}
		expected = v
	} else {
		if subtle.ConstantTimeCompare([]byte(username), []byte(h.config.MockUsername)) != 1 {
			return false
		}
		expected = h.config.MockPassword
	}
	if subtle.ConstantTimeCompare([]byte(password), []byte(expected)) == 1 {
		return true
	}
	if len(password) > len(expected) &&
		subtle.ConstantTimeCompare([]byte(password[:len(expected)]), []byte(expected)) == 1 {
		return true
	}
	return false
}


// issueTokens generates a fresh access token (and optional refresh
// token) for the given user and writes the token response.
func (h *OAuthHandler) issueTokens(w http.ResponseWriter, username string, withRefresh bool) {
	now := time.Now()
	access := h.generateToken("access", username, now)
	idURL := h.identityURL(username)
	resp := h.buildTokenResponse(access, idURL, now)

	info := &TokenInfoBuilder{
		Token:    access,
		Type:     "access",
		Username: username,
		UserID:   userIDFromIdentity(idURL),
		IssuedAt: now.Unix(),
	}
	if withRefresh {
		refresh := h.generateToken("refresh", username, now)
		resp.RefreshToken = refresh
		info.Refresh = refresh
		middleware.RegisterTokenInfo(&middleware.TokenInfo{
			Token:    refresh,
			Type:     "refresh",
			Username: username,
			UserID:   info.UserID,
			ClientID: h.config.MockClientID,
			Scope:    scopeFull,
			IssuedAt: now.Unix(),
		})
	}
	middleware.RegisterTokenInfo(&middleware.TokenInfo{
		Token:     info.Token,
		Type:      info.Type,
		Username:  info.Username,
		UserID:    info.UserID,
		ClientID:  h.config.MockClientID,
		Scope:     scopeFull,
		IssuedAt:  info.IssuedAt,
		ExpiresAt: info.IssuedAt + 7200,
		Refresh:   info.Refresh,
	})

	if username != "" {
		h.logger.Info().Str("username", username).Msg("OAuth token issued")
	} else {
		h.logger.Info().Msg("OAuth client_credentials token issued")
	}
	writeJSON(w, http.StatusOK, resp)
}

// issueRefreshedAccess mints a new access token for an existing refresh
// token and writes the response (echoing the same refresh_token).
func (h *OAuthHandler) issueRefreshedAccess(w http.ResponseWriter, refresh *middleware.TokenInfo) {
	now := time.Now()
	access := h.generateToken("access", refresh.Username, now)
	idURL := h.identityURL(refresh.Username)
	resp := h.buildTokenResponse(access, idURL, now)
	resp.RefreshToken = refresh.Token

	middleware.RegisterTokenInfo(&middleware.TokenInfo{
		Token:     access,
		Type:      "access",
		Username:  refresh.Username,
		UserID:    refresh.UserID,
		ClientID:  refresh.ClientID,
		Scope:     refresh.Scope,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Unix() + 7200,
		Refresh:   refresh.Token,
	})
	h.logger.Info().Str("username", refresh.Username).Msg("OAuth access token refreshed")
	writeJSON(w, http.StatusOK, resp)
}

// TokenInfoBuilder is an internal scratch struct used by issueTokens
// before the final middleware.TokenInfo is constructed.
type TokenInfoBuilder struct {
	Token, Type, Username, UserID, Refresh string
	IssuedAt                               int64
}

// buildTokenResponse fills the common token response fields.
func (h *OAuthHandler) buildTokenResponse(access, idURL string, now time.Time) models.OAuthResponse {
	issuedAt := fmt.Sprintf("%d", now.UnixMilli())
	return models.OAuthResponse{
		AccessToken: access,
		InstanceURL: h.instanceURL(),
		ID:          idURL,
		TokenType:   "Bearer",
		IssuedAt:    issuedAt,
		Signature:   h.signResponse(idURL, issuedAt),
		Scope:       scopeFull,
	}
}

// instanceURL resolves the OAuth response instance_url, honouring
// BASE_URL when set and falling back to INSTANCE_URL + BASE_PATH.
func (h *OAuthHandler) instanceURL() string {
	if h.config.BaseURL != "" {
		return h.config.BaseURL
	}
	url := h.config.InstanceURL
	if strings.HasPrefix(url, ":") {
		url = "http://localhost" + url
	}
	if h.config.BasePath != "" {
		url = strings.TrimRight(url, "/") + h.config.BasePath
	}
	return url
}

// identityURL returns the canonical Salesforce identity URL for a user.
func (h *OAuthHandler) identityURL(username string) string {
	user := defaultSub
	if username != "" {
		user = userIDFromUsername(username)
	}
	return fmt.Sprintf("https://login.salesforce.com/id/%s/%s", mockOrgID, user)
}

// generateToken creates a deterministic but unique-looking token of the
// requested type ("access" or "refresh").
func (h *OAuthHandler) generateToken(kind, username string, t time.Time) string {
	data := fmt.Sprintf("%s:%s:%d", kind, username, t.UnixNano())
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%s!%s", mockOrgID, base64.RawURLEncoding.EncodeToString(hash[:24]))
}

// signResponse computes the Salesforce-style token signature:
// base64(HMAC-SHA256(client_secret, id + issued_at)).
func (h *OAuthHandler) signResponse(idURL, issuedAt string) string {
	mac := hmac.New(sha256.New, []byte(h.config.MockClientSecret))
	mac.Write([]byte(idURL + issuedAt))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// userIDFromUsername derives a stable 18-char-ish user ID from a username.
func userIDFromUsername(username string) string {
	h := sha256.Sum256([]byte(username))
	return "005" + strings.ToUpper(base64.RawURLEncoding.EncodeToString(h[:11]))[:15]
}

// userIDFromIdentity extracts the trailing user ID segment of an identity URL.
func userIDFromIdentity(idURL string) string {
	idx := strings.LastIndex(idURL, "/")
	if idx < 0 || idx == len(idURL)-1 {
		return ""
	}
	return idURL[idx+1:]
}

// writeJSON writes any value as JSON with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeOAuthError writes an OAuth error response.
func (h *OAuthHandler) writeOAuthError(w http.ResponseWriter, status int, errorCode, description string) {
	h.logger.Warn().
		Str("error", errorCode).
		Str("description", description).
		Msg("OAuth error")
	writeJSON(w, status, models.OAuthError{
		Error:            errorCode,
		ErrorDescription: description,
	})
}

// HandleRevoke processes OAuth token revocation requests.
// POST or GET /services/oauth2/revoke — token may be supplied via form
// param or query param. 200 on success, 400 on unknown token.
func (h *OAuthHandler) HandleRevoke(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.writeOAuthError(w, http.StatusBadRequest, "invalid_request", "Unable to parse request body")
		return
	}
	token := r.FormValue("token")
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	if token == "" {
		h.writeOAuthError(w, http.StatusBadRequest, "invalid_request", "missing token parameter")
		return
	}
	if !middleware.RevokeToken(token) {
		h.writeOAuthError(w, http.StatusBadRequest, "unsupported_token_type", "unknown token")
		return
	}
	h.logger.Info().Msg("OAuth token revoked")
	w.WriteHeader(http.StatusOK)
}

// HandleUserinfo serves the OpenID-style userinfo claims for the
// authenticated bearer.
// GET /services/oauth2/userinfo — Bearer auth required.
func (h *OAuthHandler) HandleUserinfo(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" {
		writeBearerError(w)
		return
	}
	info := middleware.LookupToken(token)
	if info == nil || info.Type == "refresh" {
		writeBearerError(w)
		return
	}
	username := info.Username
	if username == "" {
		username = "service@" + h.config.MockClientID
	}
	resp := h.buildUserinfo(info, username)
	writeJSON(w, http.StatusOK, resp)
}

// buildUserinfo assembles the userinfo response payload.
func (h *OAuthHandler) buildUserinfo(info *middleware.TokenInfo, username string) models.UserinfoResponse {
	instance := h.instanceURL()
	apiVer := h.config.APIVersion
	idURL := fmt.Sprintf("https://login.salesforce.com/id/%s/%s", mockOrgID, info.UserID)
	given, family := splitName(username)
	urls := models.UserinfoURLs{
		Enterprise:   fmt.Sprintf("%s/services/Soap/c/%s/%s", instance, strings.TrimPrefix(apiVer, "v"), mockOrgID),
		Metadata:     fmt.Sprintf("%s/services/Soap/m/%s/%s", instance, strings.TrimPrefix(apiVer, "v"), mockOrgID),
		Partner:      fmt.Sprintf("%s/services/Soap/u/%s/%s", instance, strings.TrimPrefix(apiVer, "v"), mockOrgID),
		Rest:         fmt.Sprintf("%s/services/data/%s/", instance, apiVer),
		Sobjects:     fmt.Sprintf("%s/services/data/%s/sobjects/", instance, apiVer),
		Search:       fmt.Sprintf("%s/services/data/%s/search/", instance, apiVer),
		Query:        fmt.Sprintf("%s/services/data/%s/query/", instance, apiVer),
		Recent:       fmt.Sprintf("%s/services/data/%s/recent/", instance, apiVer),
		Profile:      fmt.Sprintf("%s/%s", instance, info.UserID),
		Feeds:        fmt.Sprintf("%s/services/data/%s/chatter/feeds", instance, apiVer),
		Groups:       fmt.Sprintf("%s/services/data/%s/chatter/groups", instance, apiVer),
		Users:        fmt.Sprintf("%s/services/data/%s/chatter/users", instance, apiVer),
		FeedItems:    fmt.Sprintf("%s/services/data/%s/chatter/feed-items", instance, apiVer),
		FeedElements: fmt.Sprintf("%s/services/data/%s/chatter/feed-elements", instance, apiVer),
	}
	picture := fmt.Sprintf("%s/profilephoto/005/F", instance)
	thumb := fmt.Sprintf("%s/profilephoto/005/T", instance)
	return models.UserinfoResponse{
		Sub:               idURL,
		UserID:            info.UserID,
		OrganizationID:    mockOrgID,
		PreferredUsername: username,
		Nickname:          given,
		Name:              username,
		Email:             username,
		EmailVerified:     true,
		GivenName:         given,
		FamilyName:        family,
		Zoneinfo:          "America/Los_Angeles",
		Photos:            models.UserinfoPhotos{Picture: picture, Thumbnail: thumb},
		Profile:           urls.Profile,
		Picture:           picture,
		Address:           map[string]any{"country": "US"},
		URLs:              urls,
		Active:            true,
		UserType:          "STANDARD",
		Language:          "en_US",
		Locale:            "en_US",
		UTCOffset:         -28800000,
		UpdatedAt:         time.Unix(info.IssuedAt, 0).UTC().Format(time.RFC3339),
	}
}


// HandleIntrospect implements the RFC 7662 token introspection endpoint.
// POST /services/oauth2/introspect — accepts Bearer or HTTP Basic
// (client_id:client_secret) authentication.
func (h *OAuthHandler) HandleIntrospect(w http.ResponseWriter, r *http.Request) {
	if !h.authenticateIntrospect(r) {
		writeBearerError(w)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.writeOAuthError(w, http.StatusBadRequest, "invalid_request", "Unable to parse request body")
		return
	}
	token := r.FormValue("token")
	if token == "" {
		writeJSON(w, http.StatusOK, models.IntrospectResponse{Active: false})
		return
	}
	info := middleware.LookupToken(token)
	if info == nil {
		writeJSON(w, http.StatusOK, models.IntrospectResponse{Active: false})
		return
	}
	resp := models.IntrospectResponse{
		Active:    true,
		Scope:     info.Scope,
		ClientID:  info.ClientID,
		Username:  info.Username,
		Iat:       info.IssuedAt,
		Nbf:       info.IssuedAt,
		Exp:       info.ExpiresAt,
		Sub:       info.UserID,
		Aud:       h.config.MockClientID,
		Iss:       h.instanceURL(),
		TokenType: info.Type,
	}
	writeJSON(w, http.StatusOK, resp)
}

// authenticateIntrospect accepts either a valid Bearer token or HTTP
// Basic with the configured client_id:client_secret.
func (h *OAuthHandler) authenticateIntrospect(r *http.Request) bool {
	if token := bearerToken(r); token != "" {
		if info := middleware.LookupToken(token); info != nil {
			return true
		}
	}
	if user, pass, ok := r.BasicAuth(); ok {
		if subtle.ConstantTimeCompare([]byte(user), []byte(h.config.MockClientID)) == 1 &&
			subtle.ConstantTimeCompare([]byte(pass), []byte(h.config.MockClientSecret)) == 1 {
			return true
		}
	}
	return false
}

// bearerToken extracts the Bearer token from the Authorization header.
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}

// writeBearerError writes the Salesforce array-shape 401 error used when
// a Bearer token is missing or invalid.
func writeBearerError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode([]models.APIError{{
		Message:   "Session expired or invalid",
		ErrorCode: "INVALID_SESSION_ID",
	}})
}

// splitName splits an email-like username into a given/family pair.
func splitName(username string) (string, string) {
	local := username
	if i := strings.Index(username, "@"); i > 0 {
		local = username[:i]
	}
	parts := strings.FieldsFunc(local, func(r rune) bool {
		return r == '.' || r == '_' || r == '-'
	})
	switch len(parts) {
	case 0:
		return "User", "Mock"
	case 1:
		return titleCase(parts[0]), "Mock"
	default:
		return titleCase(parts[0]), titleCase(parts[len(parts)-1])
	}
}

// titleCase uppercases the first rune of s; lower-cases the rest.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}
