package handlers_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/config"
	"github.com/falconleon/mock-salesforce/internal/handlers"
	"github.com/falconleon/mock-salesforce/internal/server/middleware"
	"github.com/falconleon/mock-salesforce/pkg/models"
)

// postForm posts a form-encoded body to the given OAuth handler endpoint
// and returns the recorder. method picks between POST and GET.
func postForm(handler http.HandlerFunc, path string, form url.Values, method string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func TestOAuthHandler_HandleToken_Success(t *testing.T) {
	cfg := config.Default()
	logger := zerolog.Nop()
	handler := handlers.NewOAuthHandler(cfg, logger)

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)
	form.Set("username", cfg.MockUsername)
	form.Set("password", cfg.MockPassword)

	req := httptest.NewRequest("POST", "/services/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.HandleToken(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp models.OAuthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.AccessToken == "" {
		t.Error("expected access_token to be set")
	}
	if resp.TokenType != "Bearer" {
		t.Errorf("expected token_type 'Bearer', got '%s'", resp.TokenType)
	}
	if resp.Scope != "api refresh_token" {
		t.Errorf("expected scope 'api refresh_token', got '%s'", resp.Scope)
	}
	if resp.IssuedAt == "" {
		t.Error("expected issued_at to be set")
	}
	if resp.Signature == "" {
		t.Error("expected signature to be set")
	}
}

func TestOAuthHandler_HandleToken_InvalidGrant(t *testing.T) {
	cfg := config.Default()
	logger := zerolog.Nop()
	handler := handlers.NewOAuthHandler(cfg, logger)

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)
	form.Set("username", cfg.MockUsername)
	form.Set("password", "wrong-password")

	req := httptest.NewRequest("POST", "/services/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.HandleToken(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}

	var resp models.OAuthError
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if resp.Error != "invalid_grant" {
		t.Errorf("expected error 'invalid_grant', got '%s'", resp.Error)
	}
}

func TestOAuthHandler_HandleToken_UnsupportedGrantType(t *testing.T) {
	cfg := config.Default()
	logger := zerolog.Nop()
	handler := handlers.NewOAuthHandler(cfg, logger)

	form := url.Values{}
	// All standard Salesforce grant types are now wired (password,
	// refresh_token, client_credentials, authorization_code), so use a
	// genuinely unsupported value.
	form.Set("grant_type", "device_code")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)

	req := httptest.NewRequest("POST", "/services/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.HandleToken(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}

	var resp models.OAuthError
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if resp.Error != "unsupported_grant_type" {
		t.Errorf("expected error 'unsupported_grant_type', got '%s'", resp.Error)
	}
}

func TestOAuthHandler_HandleToken_InvalidClientID(t *testing.T) {
	cfg := config.Default()
	logger := zerolog.Nop()
	handler := handlers.NewOAuthHandler(cfg, logger)

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", "wrong-client-id")
	form.Set("client_secret", cfg.MockClientSecret)
	form.Set("username", cfg.MockUsername)
	form.Set("password", cfg.MockPassword)

	req := httptest.NewRequest("POST", "/services/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.HandleToken(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}

	var resp models.OAuthError
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if resp.Error != "invalid_client_id" {
		t.Errorf("expected error 'invalid_client_id', got '%s'", resp.Error)
	}
}


func TestOAuthHandler_HandleToken_InvalidClientSecret(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", "wrong-secret")
	form.Set("username", cfg.MockUsername)
	form.Set("password", cfg.MockPassword)

	rec := postForm(h.HandleToken, "/services/oauth2/token", form, "POST")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var resp models.OAuthError
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Error != "invalid_client" {
		t.Errorf("expected error 'invalid_client', got %q", resp.Error)
	}
}

// TestOAuthHandler_SignatureMatchesSpec verifies the response signature is
// HMAC-SHA256(client_secret, id + issued_at) base64-encoded — which is what
// real Salesforce SDKs check when validating the token response.
func TestOAuthHandler_SignatureMatchesSpec(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)
	form.Set("username", cfg.MockUsername)
	form.Set("password", cfg.MockPassword)

	rec := postForm(h.HandleToken, "/services/oauth2/token", form, "POST")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp models.OAuthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.HasPrefix(resp.ID, "https://login.salesforce.com/id/") {
		t.Errorf("id should look like an SF identity URL, got %q", resp.ID)
	}
	mac := hmac.New(sha256.New, []byte(cfg.MockClientSecret))
	mac.Write([]byte(resp.ID + resp.IssuedAt))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if resp.Signature != expected {
		t.Errorf("signature mismatch:\n  got      %s\n  expected %s", resp.Signature, expected)
	}
	if resp.RefreshToken == "" {
		t.Error("password grant should issue a refresh_token")
	}
}

func TestOAuthHandler_PasswordGrant_AcceptsSecurityTokenSuffix(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)
	form.Set("username", cfg.MockUsername)
	form.Set("password", cfg.MockPassword+"SECTOKEN1234")

	rec := postForm(h.HandleToken, "/services/oauth2/token", form, "POST")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (security_token concat), got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestOAuthHandler_ClientCredentialsGrant(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)

	rec := postForm(h.HandleToken, "/services/oauth2/token", form, "POST")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for client_credentials, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp models.OAuthResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.AccessToken == "" {
		t.Error("expected access_token")
	}
	if resp.RefreshToken != "" {
		t.Error("client_credentials should not issue a refresh_token")
	}
}

func TestOAuthHandler_RefreshTokenGrant(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())

	// First, mint a password-grant token (which yields a refresh_token).
	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)
	form.Set("username", cfg.MockUsername)
	form.Set("password", cfg.MockPassword)
	rec := postForm(h.HandleToken, "/services/oauth2/token", form, "POST")
	if rec.Code != http.StatusOK {
		t.Fatalf("seed token: expected 200, got %d", rec.Code)
	}
	var seed models.OAuthResponse
	json.NewDecoder(rec.Body).Decode(&seed)
	if seed.RefreshToken == "" {
		t.Fatal("seed token did not include refresh_token")
	}

	// Now exchange the refresh_token for a new access_token.
	form2 := url.Values{}
	form2.Set("grant_type", "refresh_token")
	form2.Set("client_id", cfg.MockClientID)
	form2.Set("client_secret", cfg.MockClientSecret)
	form2.Set("refresh_token", seed.RefreshToken)
	rec2 := postForm(h.HandleToken, "/services/oauth2/token", form2, "POST")
	if rec2.Code != http.StatusOK {
		t.Fatalf("refresh: expected 200, got %d body=%s", rec2.Code, rec2.Body.String())
	}
	var refreshed models.OAuthResponse
	json.NewDecoder(rec2.Body).Decode(&refreshed)
	if refreshed.AccessToken == "" || refreshed.AccessToken == seed.AccessToken {
		t.Errorf("expected a new access_token, got %q (was %q)", refreshed.AccessToken, seed.AccessToken)
	}
	// Default config rotates the refresh_token (RFC 6749 §10.4); the
	// echo-unchanged behaviour is gated on MockRefreshRotation=false.
	if refreshed.RefreshToken == "" || refreshed.RefreshToken == seed.RefreshToken {
		t.Errorf("expected a new (rotated) refresh_token, got %q (was %q)",
			refreshed.RefreshToken, seed.RefreshToken)
	}
}

func TestOAuthHandler_RefreshTokenGrant_InvalidToken(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)
	form.Set("refresh_token", "totally-bogus")

	rec := postForm(h.HandleToken, "/services/oauth2/token", form, "POST")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var resp models.OAuthError
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Error != "invalid_grant" {
		t.Errorf("expected invalid_grant, got %q", resp.Error)
	}
}


// issueToken is a helper that runs a successful password-grant exchange
// and returns the parsed response.
func issueToken(t *testing.T, h *handlers.OAuthHandler, cfg *config.Config) models.OAuthResponse {
	t.Helper()
	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)
	form.Set("username", cfg.MockUsername)
	form.Set("password", cfg.MockPassword)
	rec := postForm(h.HandleToken, "/services/oauth2/token", form, "POST")
	if rec.Code != http.StatusOK {
		t.Fatalf("issueToken: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var r models.OAuthResponse
	if err := json.NewDecoder(rec.Body).Decode(&r); err != nil {
		t.Fatalf("issueToken decode: %v", err)
	}
	return r
}

// revokeRequest builds a /revoke request with HTTP Basic client auth
// applied (RFC 7009 §2.1 requires confidential clients to authenticate).
func revokeRequest(t *testing.T, cfg *config.Config, method, target string, form url.Values) *http.Request {
	t.Helper()
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, target, body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(cfg.MockClientID, cfg.MockClientSecret)
	return req
}

func TestOAuthHandler_Revoke_Success(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())
	tok := issueToken(t, h, cfg)

	form := url.Values{}
	form.Set("token", tok.AccessToken)
	rec := httptest.NewRecorder()
	h.HandleRevoke(rec, revokeRequest(t, cfg, "POST", "/services/oauth2/revoke", form))
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if middleware.LookupToken(tok.AccessToken) != nil {
		t.Error("revoked token should no longer be in the registry")
	}
}

// RFC 7009 §2.2: the revocation endpoint MUST respond 200 even when the
// supplied token is unknown to the server.
func TestOAuthHandler_Revoke_UnknownTokenReturns200(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())

	form := url.Values{}
	form.Set("token", "no-such-token")
	rec := httptest.NewRecorder()
	h.HandleRevoke(rec, revokeRequest(t, cfg, "POST", "/services/oauth2/revoke", form))
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for unknown token (RFC 7009 §2.2), got %d", rec.Code)
	}
}

func TestOAuthHandler_Revoke_AcceptsQueryParam(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())
	tok := issueToken(t, h, cfg)

	req := revokeRequest(t, cfg, "GET", "/services/oauth2/revoke?token="+url.QueryEscape(tok.AccessToken), nil)
	rec := httptest.NewRecorder()
	h.HandleRevoke(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// RFC 7009 §2.1: the revocation endpoint MUST authenticate the client.
// Requests without client credentials are rejected with 401.
func TestOAuthHandler_Revoke_NoAuth_Returns401(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())

	form := url.Values{}
	form.Set("token", "anything")
	rec := postForm(h.HandleRevoke, "/services/oauth2/revoke", form, "POST")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without client auth, got %d", rec.Code)
	}
	if got := rec.Header().Get("WWW-Authenticate"); !strings.HasPrefix(got, "Basic") {
		t.Errorf("expected WWW-Authenticate: Basic challenge, got %q", got)
	}
	var resp models.OAuthError
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Error != "invalid_client" {
		t.Errorf("expected error=invalid_client, got %q", resp.Error)
	}
}

// RFC 7009 §2.1: invalid client credentials are also rejected with 401.
func TestOAuthHandler_Revoke_BadAuth_Returns401(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())

	form := url.Values{}
	form.Set("token", "anything")
	req := httptest.NewRequest("POST", "/services/oauth2/revoke", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(cfg.MockClientID, "wrong-secret")
	rec := httptest.NewRecorder()
	h.HandleRevoke(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong client_secret, got %d", rec.Code)
	}
}

// /revoke also accepts form-body client credentials per RFC 6749 §2.3.1.
func TestOAuthHandler_Revoke_FormBodyCredsAllowed(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())

	form := url.Values{}
	form.Set("token", "anything")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)
	rec := postForm(h.HandleRevoke, "/services/oauth2/revoke", form, "POST")
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 with form-body creds, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestOAuthHandler_Userinfo_Success(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())
	tok := issueToken(t, h, cfg)

	req := httptest.NewRequest("GET", "/services/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	rec := httptest.NewRecorder()
	h.HandleUserinfo(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp models.UserinfoResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Email != cfg.MockUsername {
		t.Errorf("expected email %q, got %q", cfg.MockUsername, resp.Email)
	}
	if resp.OrganizationID == "" || resp.UserID == "" {
		t.Error("expected organization_id and user_id to be set")
	}
	if resp.URLs.Query == "" || resp.URLs.Sobjects == "" {
		t.Error("expected service URLs to be populated")
	}
	if !resp.Active {
		t.Error("expected active=true")
	}
}

func TestOAuthHandler_Userinfo_NoBearer_Returns401Array(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())

	req := httptest.NewRequest("GET", "/services/oauth2/userinfo", nil)
	rec := httptest.NewRecorder()
	h.HandleUserinfo(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var errs []models.APIError
	if err := json.NewDecoder(rec.Body).Decode(&errs); err != nil {
		t.Fatalf("expected SF-array error body, decode failed: %v body=%s", err, rec.Body.String())
	}
	if len(errs) != 1 || errs[0].ErrorCode != "INVALID_SESSION_ID" {
		t.Errorf("unexpected error body: %+v", errs)
	}
}

func TestOAuthHandler_Introspect_ActiveTrue(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())
	tok := issueToken(t, h, cfg)

	form := url.Values{}
	form.Set("token", tok.AccessToken)
	req := httptest.NewRequest("POST", "/services/oauth2/introspect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(cfg.MockClientID, cfg.MockClientSecret)
	rec := httptest.NewRecorder()
	h.HandleIntrospect(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp models.IntrospectResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if !resp.Active {
		t.Error("expected active=true for valid token")
	}
	if resp.ClientID != cfg.MockClientID {
		t.Errorf("expected client_id=%q, got %q", cfg.MockClientID, resp.ClientID)
	}
	if resp.TokenType != "access" {
		t.Errorf("expected token_type=access, got %q", resp.TokenType)
	}
}

func TestOAuthHandler_Introspect_ActiveFalseForUnknown(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())

	form := url.Values{}
	form.Set("token", "definitely-not-a-real-token")
	req := httptest.NewRequest("POST", "/services/oauth2/introspect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(cfg.MockClientID, cfg.MockClientSecret)
	rec := httptest.NewRecorder()
	h.HandleIntrospect(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp models.IntrospectResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Active {
		t.Error("expected active=false for unknown token")
	}
}

func TestOAuthHandler_Introspect_NoAuthRejected(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())
	tok := issueToken(t, h, cfg)

	form := url.Values{}
	form.Set("token", tok.AccessToken)
	req := httptest.NewRequest("POST", "/services/oauth2/introspect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.HandleIntrospect(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without Bearer/Basic auth, got %d", rec.Code)
	}
}

func TestOAuthHandler_Introspect_BearerAuthAllowed(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())
	tok := issueToken(t, h, cfg)

	form := url.Values{}
	form.Set("token", tok.AccessToken)
	req := httptest.NewRequest("POST", "/services/oauth2/introspect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	rec := httptest.NewRecorder()
	h.HandleIntrospect(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Bearer-auth introspect: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestOAuthHandler_RoundTrip_RevokeInvalidatesBearer is the end-to-end
// scenario from the task spec: token → query → revoke → query=401.
func TestOAuthHandler_RoundTrip_RevokeInvalidatesBearer(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())
	tok := issueToken(t, h, cfg)

	if middleware.LookupToken(tok.AccessToken) == nil {
		t.Fatal("issued token should be valid before revoke")
	}

	form := url.Values{}
	form.Set("token", tok.AccessToken)
	rec := httptest.NewRecorder()
	h.HandleRevoke(rec, revokeRequest(t, cfg, "POST", "/services/oauth2/revoke", form))
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke: expected 200, got %d", rec.Code)
	}

	if middleware.LookupToken(tok.AccessToken) != nil {
		t.Error("token should be invalid after revoke")
	}
}


// RFC 6749 §2.3.1: confidential clients SHOULD authenticate via HTTP
// Basic; the /token endpoint MUST accept that scheme.
func TestOAuthHandler_HandleToken_BasicAuth(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("username", cfg.MockUsername)
	form.Set("password", cfg.MockPassword)

	req := httptest.NewRequest("POST", "/services/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(cfg.MockClientID, cfg.MockClientSecret)
	rec := httptest.NewRecorder()
	h.HandleToken(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with HTTP Basic client auth, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp models.OAuthResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.AccessToken == "" {
		t.Error("expected access_token to be issued")
	}
}

// Matching credentials in both Basic and form body are permitted.
func TestOAuthHandler_HandleToken_BasicMatchesForm(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)
	form.Set("username", cfg.MockUsername)
	form.Set("password", cfg.MockPassword)

	req := httptest.NewRequest("POST", "/services/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(cfg.MockClientID, cfg.MockClientSecret)
	rec := httptest.NewRecorder()
	h.HandleToken(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when Basic and form creds agree, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// Conflicting Basic and form credentials must be rejected per RFC 6749
// §2.3.1 (the server MUST NOT accept ambiguous client identification).
func TestOAuthHandler_HandleToken_BasicConflictsWithForm(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", "different-client")
	form.Set("client_secret", cfg.MockClientSecret)
	form.Set("username", cfg.MockUsername)
	form.Set("password", cfg.MockPassword)

	req := httptest.NewRequest("POST", "/services/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(cfg.MockClientID, cfg.MockClientSecret)
	rec := httptest.NewRecorder()
	h.HandleToken(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for conflicting client creds, got %d", rec.Code)
	}
	var resp models.OAuthError
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Error != "invalid_request" {
		t.Errorf("expected error=invalid_request, got %q", resp.Error)
	}
}

// RFC 6750 §3: 401 responses to bearer-protected resources MUST carry a
// WWW-Authenticate: Bearer challenge. RFC 6750 §3.1: when the request did
// NOT include an access token, the challenge MUST NOT carry an error param.
func TestOAuthHandler_Userinfo_NoBearer_HasWWWAuthenticateBearer(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())

	req := httptest.NewRequest("GET", "/services/oauth2/userinfo", nil)
	rec := httptest.NewRecorder()
	h.HandleUserinfo(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	got := rec.Header().Get("WWW-Authenticate")
	want := `Bearer realm="Mock Salesforce"`
	if got != want {
		t.Errorf("WWW-Authenticate mismatch:\n got: %q\nwant: %q", got, want)
	}
	if strings.Contains(got, "error=") {
		t.Errorf("WWW-Authenticate must not include error= when no token was sent, got %q", got)
	}
}

// RFC 6750 §3.1: when the request DID include a bearer token that was
// rejected, the challenge MUST include error="invalid_token".
func TestOAuthHandler_Userinfo_BadBearer_HasWWWAuthenticateInvalidToken(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())

	req := httptest.NewRequest("GET", "/services/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer fake")
	rec := httptest.NewRecorder()
	h.HandleUserinfo(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	got := rec.Header().Get("WWW-Authenticate")
	if !strings.HasPrefix(got, "Bearer ") {
		t.Errorf("expected WWW-Authenticate to start with 'Bearer ', got %q", got)
	}
	if !strings.Contains(got, `realm="Mock Salesforce"`) {
		t.Errorf(`expected WWW-Authenticate to include realm="Mock Salesforce", got %q`, got)
	}
	if !strings.Contains(got, `error="invalid_token"`) {
		t.Errorf(`expected WWW-Authenticate to include error="invalid_token", got %q`, got)
	}
}

// RFC 6749 §2.3.1: /introspect must also accept form-body client
// credentials, not just Bearer or HTTP Basic.
func TestOAuthHandler_Introspect_FormBodyCreds(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())
	tok := issueToken(t, h, cfg)

	form := url.Values{}
	form.Set("token", tok.AccessToken)
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)
	rec := postForm(h.HandleIntrospect, "/services/oauth2/introspect", form, "POST")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with form-body creds, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp models.IntrospectResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if !resp.Active {
		t.Error("expected active=true when authenticated via form-body creds")
	}
}

// Wrong form-body creds are rejected with 401 (no fall-through to anon).
func TestOAuthHandler_Introspect_WrongFormBodyCreds_Rejected(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())

	form := url.Values{}
	form.Set("token", "anything")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", "wrong")
	rec := postForm(h.HandleIntrospect, "/services/oauth2/introspect", form, "POST")

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with wrong form-body creds, got %d", rec.Code)
	}
}


// pkcePair returns a (verifier, S256-challenge) pair that satisfies
// RFC 7636 §4.2.
func pkcePair(t *testing.T, verifier string) (string, string) {
	t.Helper()
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

// authCodeHarness builds an OAuthHandler wired with a fresh AuthCodeStore
// and pre-issues a single authorization code, returning the handler,
// store, the issued code record, and the matching PKCE verifier.
func authCodeHarness(t *testing.T, method string) (*handlers.OAuthHandler, *handlers.AuthCodeStore, *handlers.AuthCode, string) {
	t.Helper()
	cfg := config.Default()
	cfg.MockRedirectURIs = []string{"https://app.example/cb"}
	store := handlers.NewAuthCodeStore()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop()).WithAuthCodes(store)
	verifier := "abc123-very-long-code-verifier-that-is-enough-bytes"
	var challenge string
	if method == "S256" {
		_, challenge = pkcePair(t, verifier)
	} else {
		challenge = verifier
	}
	ac := store.Issue(cfg.MockClientID, "https://app.example/cb", "api", challenge, method, cfg.MockUsername)
	return h, store, ac, verifier
}

func TestOAuthHandler_AuthorizationCodeGrant_S256_Success(t *testing.T) {
	h, _, ac, verifier := authCodeHarness(t, "S256")
	cfg := config.Default()

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)
	form.Set("code", ac.Code)
	form.Set("redirect_uri", ac.RedirectURI)
	form.Set("code_verifier", verifier)

	rec := postForm(h.HandleToken, "/services/oauth2/token", form, "POST")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp models.OAuthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Errorf("expected access+refresh tokens, got %+v", resp)
	}
}

// Defence in depth: even if a code somehow has a non-S256 method
// stored (the /authorize handler rejects it up front per the Salesforce
// 2026-05-11 mandate), the token endpoint MUST refuse the exchange.
func TestOAuthHandler_AuthorizationCodeGrant_Plain_Rejected(t *testing.T) {
	h, _, ac, verifier := authCodeHarness(t, "plain")
	cfg := config.Default()

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)
	form.Set("code", ac.Code)
	form.Set("redirect_uri", ac.RedirectURI)
	form.Set("code_verifier", verifier)

	rec := postForm(h.HandleToken, "/services/oauth2/token", form, "POST")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("plain PKCE: expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp models.OAuthError
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Error != "invalid_grant" {
		t.Errorf("expected error=invalid_grant, got %q", resp.Error)
	}
}

func TestOAuthHandler_AuthorizationCodeGrant_Reuse_Fails(t *testing.T) {
	h, _, ac, verifier := authCodeHarness(t, "S256")
	cfg := config.Default()

	mkForm := func() url.Values {
		f := url.Values{}
		f.Set("grant_type", "authorization_code")
		f.Set("client_id", cfg.MockClientID)
		f.Set("client_secret", cfg.MockClientSecret)
		f.Set("code", ac.Code)
		f.Set("redirect_uri", ac.RedirectURI)
		f.Set("code_verifier", verifier)
		return f
	}
	if rec := postForm(h.HandleToken, "/services/oauth2/token", mkForm(), "POST"); rec.Code != http.StatusOK {
		t.Fatalf("first exchange should succeed, got %d", rec.Code)
	}
	rec := postForm(h.HandleToken, "/services/oauth2/token", mkForm(), "POST")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("reuse: expected 400, got %d", rec.Code)
	}
	var resp models.OAuthError
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Error != "invalid_grant" {
		t.Errorf("expected invalid_grant on reuse, got %q", resp.Error)
	}
}

func TestOAuthHandler_AuthorizationCodeGrant_BadVerifier_Fails(t *testing.T) {
	h, _, ac, _ := authCodeHarness(t, "S256")
	cfg := config.Default()

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)
	form.Set("code", ac.Code)
	form.Set("redirect_uri", ac.RedirectURI)
	form.Set("code_verifier", "not-the-right-verifier-at-all-padding-padding")

	rec := postForm(h.HandleToken, "/services/oauth2/token", form, "POST")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var resp models.OAuthError
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Error != "invalid_grant" {
		t.Errorf("expected invalid_grant, got %q", resp.Error)
	}
}

func TestOAuthHandler_AuthorizationCodeGrant_MissingVerifier_Fails(t *testing.T) {
	h, _, ac, _ := authCodeHarness(t, "S256")
	cfg := config.Default()

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)
	form.Set("code", ac.Code)
	form.Set("redirect_uri", ac.RedirectURI)

	rec := postForm(h.HandleToken, "/services/oauth2/token", form, "POST")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 with missing verifier, got %d", rec.Code)
	}
}

func TestOAuthHandler_AuthorizationCodeGrant_RedirectURIMismatch_Fails(t *testing.T) {
	h, _, ac, verifier := authCodeHarness(t, "S256")
	cfg := config.Default()

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)
	form.Set("code", ac.Code)
	form.Set("redirect_uri", "https://attacker.example/cb")
	form.Set("code_verifier", verifier)

	rec := postForm(h.HandleToken, "/services/oauth2/token", form, "POST")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on redirect_uri mismatch, got %d", rec.Code)
	}
	var resp models.OAuthError
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Error != "invalid_grant" {
		t.Errorf("expected invalid_grant, got %q", resp.Error)
	}
}

func TestOAuthHandler_AuthorizationCodeGrant_UnknownCode_Fails(t *testing.T) {
	cfg := config.Default()
	store := handlers.NewAuthCodeStore()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop()).WithAuthCodes(store)

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)
	form.Set("code", "no-such-code")
	form.Set("redirect_uri", "https://app.example/cb")
	form.Set("code_verifier", "anything")

	rec := postForm(h.HandleToken, "/services/oauth2/token", form, "POST")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown code, got %d", rec.Code)
	}
}

// G3: refresh-token rotation reuse-detection — replaying a rotated
// refresh token must revoke the entire token family so all access
// tokens derived from it stop working (RFC 6749 §10.4).
func TestOAuthHandler_RefreshTokenGrant_ReuseRevokesFamily(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())
	seed := issueToken(t, h, cfg)

	// First refresh: rotates the refresh token, mints new access.
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)
	form.Set("refresh_token", seed.RefreshToken)
	rec := postForm(h.HandleToken, "/services/oauth2/token", form, "POST")
	if rec.Code != http.StatusOK {
		t.Fatalf("first refresh: expected 200, got %d", rec.Code)
	}
	var rotated models.OAuthResponse
	json.NewDecoder(rec.Body).Decode(&rotated)
	if rotated.RefreshToken == seed.RefreshToken {
		t.Fatal("expected refresh_token to rotate")
	}

	// Replay the now-rotated refresh token: must fail and trigger family
	// revocation.
	rec2 := postForm(h.HandleToken, "/services/oauth2/token", form, "POST")
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("reuse: expected 400, got %d", rec2.Code)
	}

	// The freshly-rotated refresh AND its derived access token must now
	// be invalid (whole family revoked).
	if middleware.LookupToken(rotated.RefreshToken) != nil {
		t.Error("post-reuse: rotated refresh_token should be revoked")
	}
	if middleware.LookupToken(rotated.AccessToken) != nil {
		t.Error("post-reuse: derived access_token should be revoked")
	}
	if middleware.LookupToken(seed.AccessToken) != nil {
		t.Error("post-reuse: original access_token should be revoked")
	}

	// And the rotated refresh can no longer be exchanged.
	form3 := url.Values{}
	form3.Set("grant_type", "refresh_token")
	form3.Set("client_id", cfg.MockClientID)
	form3.Set("client_secret", cfg.MockClientSecret)
	form3.Set("refresh_token", rotated.RefreshToken)
	rec3 := postForm(h.HandleToken, "/services/oauth2/token", form3, "POST")
	if rec3.Code != http.StatusBadRequest {
		t.Fatalf("post-reuse refresh: expected 400, got %d", rec3.Code)
	}
}

// G3: when MockRefreshRotation is disabled the refresh_token must be
// echoed unchanged (legacy behaviour).
func TestOAuthHandler_RefreshTokenGrant_RotationDisabled_EchoesRefresh(t *testing.T) {
	cfg := config.Default()
	cfg.MockRefreshRotation = false
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())
	seed := issueToken(t, h, cfg)

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)
	form.Set("refresh_token", seed.RefreshToken)
	rec := postForm(h.HandleToken, "/services/oauth2/token", form, "POST")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp models.OAuthResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.RefreshToken != seed.RefreshToken {
		t.Errorf("rotation disabled: expected echoed refresh_token, got %q want %q",
			resp.RefreshToken, seed.RefreshToken)
	}
	// And the original refresh_token must remain usable.
	if middleware.LookupToken(seed.RefreshToken) == nil {
		t.Error("rotation disabled: original refresh should remain valid")
	}
}

// H1: /token authorization_code re-checks the stored redirect_uri
// against the allowlist (defence in depth). If an operator removes a
// URI between issue and exchange, the outstanding code must be rejected
// with invalid_grant even though the form-supplied redirect_uri still
// matches the stored value.
func TestOAuthHandler_AuthorizationCodeGrant_RedirectURIRemovedFromAllowlist_Fails(t *testing.T) {
	cfg := config.Default()
	cfg.MockRedirectURIs = []string{"https://app.example/cb"}
	store := handlers.NewAuthCodeStore()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop()).WithAuthCodes(store)
	verifier := "abc123-very-long-code-verifier-that-is-enough-bytes"
	_, challenge := pkcePair(t, verifier)
	ac := store.Issue(cfg.MockClientID, "https://app.example/cb", "api", challenge, "S256", cfg.MockUsername)

	// Operator tightens the allowlist after issue but before exchange.
	cfg.MockRedirectURIs = []string{"https://other.example/cb"}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", cfg.MockClientID)
	form.Set("client_secret", cfg.MockClientSecret)
	form.Set("code", ac.Code)
	form.Set("redirect_uri", ac.RedirectURI)
	form.Set("code_verifier", verifier)

	rec := postForm(h.HandleToken, "/services/oauth2/token", form, "POST")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 after allowlist tightened, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp models.OAuthError
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Error != "invalid_grant" {
		t.Errorf("expected invalid_grant, got %q", resp.Error)
	}
	if !strings.Contains(resp.ErrorDescription, "no longer registered") {
		t.Errorf("expected description to mention no longer registered, got %q", resp.ErrorDescription)
	}
}
