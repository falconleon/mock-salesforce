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
	// T11 added client_credentials + refresh_token support, so use a
	// grant type Salesforce really doesn't accept here.
	form.Set("grant_type", "authorization_code")
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
	if refreshed.RefreshToken != seed.RefreshToken {
		t.Errorf("refresh_token should be echoed unchanged, got %q want %q",
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

func TestOAuthHandler_Revoke_Success(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())
	tok := issueToken(t, h, cfg)

	form := url.Values{}
	form.Set("token", tok.AccessToken)
	rec := postForm(h.HandleRevoke, "/services/oauth2/revoke", form, "POST")
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if middleware.LookupToken(tok.AccessToken) != nil {
		t.Error("revoked token should no longer be in the registry")
	}
}

func TestOAuthHandler_Revoke_UnknownToken(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())

	form := url.Values{}
	form.Set("token", "no-such-token")
	rec := postForm(h.HandleRevoke, "/services/oauth2/revoke", form, "POST")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unknown token, got %d", rec.Code)
	}
}

func TestOAuthHandler_Revoke_AcceptsQueryParam(t *testing.T) {
	cfg := config.Default()
	h := handlers.NewOAuthHandler(cfg, zerolog.Nop())
	tok := issueToken(t, h, cfg)

	req := httptest.NewRequest("GET", "/services/oauth2/revoke?token="+url.QueryEscape(tok.AccessToken), nil)
	rec := httptest.NewRecorder()
	h.HandleRevoke(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
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
	rec := postForm(h.HandleRevoke, "/services/oauth2/revoke", form, "POST")
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke: expected 200, got %d", rec.Code)
	}

	if middleware.LookupToken(tok.AccessToken) != nil {
		t.Error("token should be invalid after revoke")
	}
}

