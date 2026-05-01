// Package handlers provides HTTP handlers for the Salesforce mock API.
package handlers

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/config"
	"github.com/falconleon/mock-salesforce/internal/server/middleware"
	"github.com/falconleon/mock-salesforce/pkg/models"
)

// errConflictingClientCredentials is returned when a request supplies
// HTTP Basic and form-body client credentials with disagreeing values.
var errConflictingClientCredentials = errors.New("conflicting client credentials")

// extractClientCredentials returns the client_id/client_secret from the
// request, accepting either the HTTP Basic Authorization header
// (RFC 6749 §2.3.1, the recommended form) or the form-encoded body
// parameters (the legacy form). If both methods are present with
// disagreeing values it returns errConflictingClientCredentials so the
// caller can emit invalid_request per RFC 6749 §3.2.1.
func extractClientCredentials(r *http.Request) (id, secret string, ok bool, err error) {
	if r.Form == nil {
		if perr := r.ParseForm(); perr != nil {
			return "", "", false, perr
		}
	}
	basicID, basicSecret, hasBasic := r.BasicAuth()
	formID := r.PostFormValue("client_id")
	formSecret := r.PostFormValue("client_secret")
	hasForm := formID != "" || formSecret != ""

	switch {
	case hasBasic && hasForm:
		if basicID != formID || basicSecret != formSecret {
			return "", "", false, errConflictingClientCredentials
		}
		return basicID, basicSecret, true, nil
	case hasBasic:
		return basicID, basicSecret, true, nil
	case hasForm:
		return formID, formSecret, true, nil
	}
	return "", "", false, nil
}

// OAuthHandler handles OAuth authentication requests.
type OAuthHandler struct {
	config    *config.Config
	logger    zerolog.Logger
	codes     *AuthCodeStore
	refreshMu sync.Map // per-refresh-token mutex; keyed by token string (I-2)
}

// NewOAuthHandler creates a new OAuth handler.
func NewOAuthHandler(cfg *config.Config, logger zerolog.Logger) *OAuthHandler {
	return &OAuthHandler{
		config: cfg,
		logger: logger.With().Str("handler", "oauth").Logger(),
	}
}

// WithAuthCodes wires an AuthCodeStore so the handler can service the
// authorization_code grant on /token (RFC 6749 §4.1.3). Returns the
// receiver for fluent construction at router wiring time.
func (h *OAuthHandler) WithAuthCodes(codes *AuthCodeStore) *OAuthHandler {
	h.codes = codes
	return h
}

const (
	mockOrgID  = "00Dxx0000001gPq"
	defaultSub = "005xx000001SwiUAAS"
	scopeFull  = "api refresh_token"
)

// HandleToken processes OAuth token requests.
// POST /services/oauth2/token — supports grant_type of password,
// refresh_token, and client_credentials. Client credentials may be
// supplied via HTTP Basic (RFC 6749 §2.3.1) or form-encoded body.
func (h *OAuthHandler) HandleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.writeOAuthError(w, http.StatusBadRequest, "invalid_request", "Unable to parse request body")
		return
	}

	clientID, clientSecret, _, err := extractClientCredentials(r)
	if err != nil {
		h.writeOAuthError(w, http.StatusBadRequest, "invalid_request", "Conflicting client credentials")
		return
	}
	grantType := r.PostFormValue("grant_type")

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
	case "authorization_code":
		h.handleAuthorizationCodeGrant(w, r, clientID, clientSecret)
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

	// Per-token mutex prevents concurrent exchanges of the same refresh token
	// from both succeeding (double-issue window, I-2). Mutex entries are
	// intentionally leaked on rotation/revocation — the token population in
	// a mock server is small and bounded, so the leak is acceptable.
	stored, _ := h.refreshMu.LoadOrStore(refresh, &sync.Mutex{})
	mu := stored.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	// Re-read the token INSIDE the critical section. A concurrent exchange
	// that already rotated this token will have called MarkRevoked, so
	// LookupToken returns nil here and we fall through to invalid_grant.
	info := middleware.LookupToken(refresh)
	if info == nil || info.Type != "refresh" {
		// RFC 6749 §10.4: replay of a rotated refresh token MUST revoke
		// the entire family. Distinguish "never seen" from "previously
		// rotated" via LookupTokenRaw.
		if rawInfo := middleware.LookupTokenRaw(refresh); rawInfo != nil && rawInfo.Type == "refresh" && rawInfo.Revoked {
			n := middleware.RevokeFamily(rawInfo.Family)
			h.logger.Warn().
				Str("family", rawInfo.Family).
				Int("revoked", n).
				Msg("OAuth refresh-token reuse detected; family revoked")
		}
		h.writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "expired access/refresh token")
		return
	}
	h.issueRefreshedAccess(w, info)
}

// handleAuthorizationCodeGrant exchanges a one-time authorization code
// (RFC 6749 §4.1.3) for tokens. PKCE (RFC 7636) is required by the
// front-end /authorize handler so we always have a stored
// code_challenge to verify against.
func (h *OAuthHandler) handleAuthorizationCodeGrant(w http.ResponseWriter, r *http.Request, clientID, clientSecret string) {
	if !h.validateClient(w, clientID, clientSecret) {
		return
	}
	if h.codes == nil {
		h.writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "authorization_code grant is not configured")
		return
	}
	code := r.PostFormValue("code")
	redirectURI := r.PostFormValue("redirect_uri")
	verifier := r.PostFormValue("code_verifier")
	if code == "" {
		h.writeOAuthError(w, http.StatusBadRequest, "invalid_request", "code is required")
		return
	}
	ac := h.codes.Consume(code)
	if ac == nil {
		h.writeOAuthError(w, http.StatusBadRequest, "invalid_grant",
			"authorization code is invalid, expired, or already used")
		return
	}
	if subtle.ConstantTimeCompare([]byte(ac.ClientID), []byte(clientID)) != 1 {
		h.writeOAuthError(w, http.StatusBadRequest, "invalid_grant",
			"client_id does not match the authorization code")
		return
	}
	if ac.RedirectURI != redirectURI {
		h.writeOAuthError(w, http.StatusBadRequest, "invalid_grant",
			"redirect_uri does not match the authorization request")
		return
	}
	// Defence in depth: an allowlist edit between issue and exchange
	// must invalidate codes bound to URIs that are no longer registered.
	if !h.config.IsRedirectURIAllowed(ac.RedirectURI) {
		h.writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "redirect_uri no longer registered")
		return
	}
	if !verifyPKCE(verifier, ac.CodeChallenge, ac.CodeChallengeMethod) {
		h.writeOAuthError(w, http.StatusBadRequest, "invalid_grant",
			"PKCE code_verifier verification failed")
		return
	}
	h.issueTokens(w, ac.Username, true)
}

// verifyPKCE checks the code_verifier against the stored challenge per
// RFC 7636 §4.6. S256 is the only method accepted (Salesforce mandate
// effective 2026-05-11); any other method is rejected.
func verifyPKCE(verifier, challenge, method string) bool {
	if verifier == "" || challenge == "" {
		return false
	}
	if method != "S256" {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(computed), []byte(challenge)) == 1
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

// requireClientAuth enforces RFC 6749 §2.3.1 client authentication on
// endpoints that mandate it (e.g. /revoke per RFC 7009 §2.1). On
// failure it writes a 401 invalid_client response with the
// WWW-Authenticate: Basic challenge and returns false.
func (h *OAuthHandler) requireClientAuth(w http.ResponseWriter, r *http.Request) bool {
	clientID, clientSecret, ok, err := extractClientCredentials(r)
	if err != nil {
		h.writeOAuthError(w, http.StatusBadRequest, "invalid_request", "Conflicting client credentials")
		return false
	}
	valid := ok &&
		subtle.ConstantTimeCompare([]byte(clientID), []byte(h.config.MockClientID)) == 1 &&
		subtle.ConstantTimeCompare([]byte(clientSecret), []byte(h.config.MockClientSecret)) == 1
	if !valid {
		w.Header().Set("WWW-Authenticate", `Basic realm="salesforce"`)
		h.writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "Client authentication failed")
		return false
	}
	return true
}

// validateUser checks the supplied password against the configured
// users. Real Salesforce accepts password concatenated with a security
// token; we accept either an exact match or a value that begins with
// the configured password.
//
// Both the "unknown user" and "wrong password" paths always reach the
// constant-time comparison below (M-2: timing oracle mitigation). For
// unknown users expected is "" so both comparisons return false, but
// the comparison cost is always paid.
func (h *OAuthHandler) validateUser(username, password string) bool {
	var expected string
	userFound := false
	if len(h.config.MockUsers) > 0 {
		v, ok := h.config.MockUsers[username]
		expected = v // empty string when user is not found
		userFound = ok
	} else {
		userFound = subtle.ConstantTimeCompare([]byte(username), []byte(h.config.MockUsername)) == 1
		expected = h.config.MockPassword
	}
	// Always perform constant-time password comparison regardless of whether
	// the user was found. This prevents a timing oracle on username existence.
	passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(expected)) == 1
	// Salesforce accepts password+security_token concatenation.
	tokenMatch := len(password) > len(expected) &&
		subtle.ConstantTimeCompare([]byte(password[:len(expected)]), []byte(expected)) == 1
	return userFound && (passwordMatch || tokenMatch)
}

// issueTokens generates a fresh access token (and optional refresh
// token) for the given user and writes the token response. When a
// refresh token is issued a new family ID is allocated so subsequent
// rotations can detect replays of the original refresh.
func (h *OAuthHandler) issueTokens(w http.ResponseWriter, username string, withRefresh bool) {
	now := time.Now()
	access := h.generateToken("access", username, now)
	idURL := h.identityURL(username)
	resp := h.buildTokenResponse(access, idURL, now)

	userID := userIDFromIdentity(idURL)
	var refresh, family string
	if withRefresh {
		refresh = h.generateToken("refresh", username, now)
		family = newFamilyID()
		resp.RefreshToken = refresh
		middleware.RegisterTokenInfo(&middleware.TokenInfo{
			Token:    refresh,
			Type:     "refresh",
			Username: username,
			UserID:   userID,
			ClientID: h.config.MockClientID,
			Scope:    scopeFull,
			IssuedAt: now.Unix(),
			Family:   family,
		})
	}
	middleware.RegisterTokenInfo(&middleware.TokenInfo{
		Token:     access,
		Type:      "access",
		Username:  username,
		UserID:    userID,
		ClientID:  h.config.MockClientID,
		Scope:     scopeFull,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Unix() + 7200,
		Refresh:   refresh,
		Family:    family,
	})

	if username != "" {
		h.logger.Info().Str("username", username).Msg("OAuth token issued")
	} else {
		h.logger.Info().Msg("OAuth client_credentials token issued")
	}
	writeJSON(w, http.StatusOK, resp)
}

// issueRefreshedAccess mints a new access token in response to a
// refresh exchange. When MockRefreshRotation is enabled (default) a
// fresh refresh_token is also issued and the prior one is marked
// revoked so subsequent reuse can be detected (RFC 6749 §10.4); when
// disabled, the original refresh_token is echoed unchanged.
func (h *OAuthHandler) issueRefreshedAccess(w http.ResponseWriter, refresh *middleware.TokenInfo) {
	now := time.Now()
	access := h.generateToken("access", refresh.Username, now)
	idURL := h.identityURL(refresh.Username)
	resp := h.buildTokenResponse(access, idURL, now)

	rotate := h.config.MockRefreshRotation
	newRefresh := refresh.Token
	if rotate {
		newRefresh = h.generateToken("refresh", refresh.Username, now)
		middleware.RegisterTokenInfo(&middleware.TokenInfo{
			Token:    newRefresh,
			Type:     "refresh",
			Username: refresh.Username,
			UserID:   refresh.UserID,
			ClientID: refresh.ClientID,
			Scope:    refresh.Scope,
			IssuedAt: now.Unix(),
			Family:   refresh.Family,
		})
		// Retain the prior refresh entry so a replay is recognisable.
		middleware.MarkRevoked(refresh.Token)
	}
	resp.RefreshToken = newRefresh

	middleware.RegisterTokenInfo(&middleware.TokenInfo{
		Token:     access,
		Type:      "access",
		Username:  refresh.Username,
		UserID:    refresh.UserID,
		ClientID:  refresh.ClientID,
		Scope:     refresh.Scope,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Unix() + 7200,
		Refresh:   newRefresh,
		Family:    refresh.Family,
	})
	h.logger.Info().
		Str("username", refresh.Username).
		Bool("rotated", rotate).
		Msg("OAuth access token refreshed")
	writeJSON(w, http.StatusOK, resp)
}

// newFamilyID returns a 128-bit URL-safe random family identifier used
// to link refresh-token rotations and their derived access tokens.
func newFamilyID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
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
// param or query param. Per RFC 7009 §2.1 the request MUST be
// authenticated as a confidential client; per RFC 7009 §2.2 the
// response is 200 regardless of whether the token is known.
func (h *OAuthHandler) HandleRevoke(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.writeOAuthError(w, http.StatusBadRequest, "invalid_request", "Unable to parse request body")
		return
	}
	if !h.requireClientAuth(w, r) {
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
	// RFC 7009 §2.2: respond 200 whether or not the token is known.
	middleware.RevokeToken(token)
	h.logger.Info().Msg("OAuth token revocation processed")
	w.WriteHeader(http.StatusOK)
}

// HandleUserinfo serves the OpenID-style userinfo claims for the
// authenticated bearer.
// GET /services/oauth2/userinfo — Bearer auth required.
func (h *OAuthHandler) HandleUserinfo(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" {
		writeBearerError(w, r)
		return
	}
	info := middleware.LookupToken(token)
	if info == nil || info.Type == "refresh" {
		writeBearerError(w, r)
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
		writeBearerError(w, r)
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

// authenticateIntrospect accepts a valid Bearer token, HTTP Basic, or
// form-body client_id+client_secret per RFC 6749 §2.3.1.
func (h *OAuthHandler) authenticateIntrospect(r *http.Request) bool {
	if token := bearerToken(r); token != "" {
		if info := middleware.LookupToken(token); info != nil {
			return true
		}
	}
	clientID, clientSecret, ok, err := extractClientCredentials(r)
	if err != nil || !ok {
		return false
	}
	if subtle.ConstantTimeCompare([]byte(clientID), []byte(h.config.MockClientID)) == 1 &&
		subtle.ConstantTimeCompare([]byte(clientSecret), []byte(h.config.MockClientSecret)) == 1 {
		return true
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
// a Bearer token is missing or invalid. Includes the WWW-Authenticate:
// Bearer challenge required by RFC 6750 §3. Per RFC 6750 §3.1, the error
// parameter is only emitted when the request actually presented a bearer
// token; a missing Authorization header gets only the realm challenge.
func writeBearerError(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		w.Header().Set("WWW-Authenticate", `Bearer realm="Mock Salesforce", error="invalid_token", error_description="Session expired or invalid"`)
	} else {
		w.Header().Set("WWW-Authenticate", `Bearer realm="Mock Salesforce"`)
	}
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
