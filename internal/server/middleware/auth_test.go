package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/server/middleware"
	"github.com/falconleon/mock-salesforce/pkg/models"
)

func TestAuth_PublicPaths(t *testing.T) {
	logger := zerolog.Nop()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authMiddleware := middleware.Auth(logger, "test-secret")(handler)

	publicPaths := []string{
		"/services/oauth2/token",
		"/health",
		"/",
	}

	for _, path := range publicPaths {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()

		authMiddleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected public path %s to return 200, got %d", path, rec.Code)
		}
	}
}

func TestAuth_MissingToken(t *testing.T) {
	logger := zerolog.Nop()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authMiddleware := middleware.Auth(logger, "test-secret")(handler)

	req := httptest.NewRequest("GET", "/services/data/v66.0/query", nil)
	rec := httptest.NewRecorder()

	authMiddleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}

	var errors []models.APIError
	if err := json.NewDecoder(rec.Body).Decode(&errors); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if len(errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(errors))
	}
	if errors[0].ErrorCode != "INVALID_SESSION_ID" {
		t.Errorf("expected errorCode 'INVALID_SESSION_ID', got '%s'", errors[0].ErrorCode)
	}
}

func TestAuth_InvalidToken(t *testing.T) {
	logger := zerolog.Nop()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authMiddleware := middleware.Auth(logger, "test-secret")(handler)

	req := httptest.NewRequest("GET", "/services/data/v66.0/query", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()

	authMiddleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestAuth_ValidToken(t *testing.T) {
	logger := zerolog.Nop()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authMiddleware := middleware.Auth(logger, "test-secret")(handler)

	// Register a test token
	middleware.RegisterToken("test-valid-token")

	req := httptest.NewRequest("GET", "/services/data/v66.0/query", nil)
	req.Header.Set("Authorization", "Bearer test-valid-token")
	rec := httptest.NewRecorder()

	authMiddleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 for valid token, got %d", rec.Code)
	}
}

func TestAuth_InvalidHeaderFormat(t *testing.T) {
	logger := zerolog.Nop()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authMiddleware := middleware.Auth(logger, "test-secret")(handler)

	req := httptest.NewRequest("GET", "/services/data/v66.0/query", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz") // Basic auth format
	rec := httptest.NewRecorder()

	authMiddleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for invalid header format, got %d", rec.Code)
	}
}


func TestAuth_RevokedTokenReturnsSFArrayError(t *testing.T) {
	logger := zerolog.Nop()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	authMW := middleware.Auth(logger, "test-secret")(handler)

	middleware.RegisterToken("token-to-be-revoked")

	// Revoke and confirm subsequent calls fail with the SF wire-format
	// 401 array body.
	if !middleware.RevokeToken("token-to-be-revoked") {
		t.Fatal("expected RevokeToken to report success")
	}

	req := httptest.NewRequest("GET", "/services/data/v66.0/query", nil)
	req.Header.Set("Authorization", "Bearer token-to-be-revoked")
	rec := httptest.NewRecorder()
	authMW.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after revoke, got %d", rec.Code)
	}
	var errs []models.APIError
	if err := json.NewDecoder(rec.Body).Decode(&errs); err != nil {
		t.Fatalf("expected SF array body: %v", err)
	}
	if len(errs) != 1 || errs[0].ErrorCode != "INVALID_SESSION_ID" {
		t.Errorf("unexpected error body: %+v", errs)
	}
}

func TestAuth_RefreshTokenRejectedAsBearer(t *testing.T) {
	logger := zerolog.Nop()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	authMW := middleware.Auth(logger, "test-secret")(handler)

	// A refresh token is registered but should NOT be usable as a Bearer.
	middleware.RegisterTokenInfo(&middleware.TokenInfo{
		Token: "refresh-token-not-bearer",
		Type:  "refresh",
	})

	req := httptest.NewRequest("GET", "/services/data/v66.0/query", nil)
	req.Header.Set("Authorization", "Bearer refresh-token-not-bearer")
	rec := httptest.NewRecorder()
	authMW.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("refresh tokens must not be accepted as bearers; got %d", rec.Code)
	}
}

func TestAuth_RevokeUnknownTokenReturnsFalse(t *testing.T) {
	if middleware.RevokeToken("never-issued") {
		t.Error("expected RevokeToken to return false for unknown token")
	}
}

func TestSessionCookie_RoundTrip(t *testing.T) {
	rec := httptest.NewRecorder()
	middleware.SetSessionCookie(rec, "demo@falcon.local", "secret")

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected sf_session cookie to be set")
	}
	var sess *http.Cookie
	for _, c := range cookies {
		if c.Name == "sf_session" {
			sess = c
			break
		}
	}
	if sess == nil {
		t.Fatal("sf_session cookie missing")
	}
	if !sess.HttpOnly {
		t.Error("session cookie must be HttpOnly")
	}
	if sess.SameSite != http.SameSiteLaxMode {
		t.Error("session cookie must be SameSite=Lax")
	}
	if sess.MaxAge < 11*3600 || sess.MaxAge > 13*3600 {
		t.Errorf("session cookie MaxAge %d not ~12h", sess.MaxAge)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(sess)
	email, ok := middleware.ValidateSession(req, "secret")
	if !ok || email != "demo@falcon.local" {
		t.Errorf("ValidateSession returned (%q,%v)", email, ok)
	}

	claims, ok := middleware.ValidateSessionClaims(req, "secret")
	if !ok || claims == nil || claims.Email != "demo@falcon.local" {
		t.Errorf("ValidateSessionClaims returned %+v ok=%v", claims, ok)
	}
}

func TestLoginHandler_Success(t *testing.T) {
	users := map[string]string{"demo@falcon.local": "pw"}
	h := middleware.LoginHandler(users, "secret", "")

	form := url.Values{}
	form.Set("email", "demo@falcon.local")
	form.Set("password", "pw")
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("want 302, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/home" {
		t.Errorf("want redirect to /home, got %q", loc)
	}
	hasSession := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == "sf_session" && c.Value != "" {
			hasSession = true
		}
	}
	if !hasSession {
		t.Error("expected sf_session cookie to be set on success")
	}
}

func TestLoginHandler_BadPasswordRedirectsWithError(t *testing.T) {
	users := map[string]string{"demo@falcon.local": "pw"}
	h := middleware.LoginHandler(users, "secret", "")

	form := url.Values{}
	form.Set("email", "demo@falcon.local")
	form.Set("password", "wrong")
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("want 302, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/login?") || !strings.Contains(loc, "error=invalid") {
		t.Errorf("want /login?error=invalid..., got %q", loc)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "sf_session" && c.Value != "" {
			t.Error("must not set sf_session on failure")
		}
	}
}

func TestLoginHandler_RedirectsToNext(t *testing.T) {
	users := map[string]string{"demo@falcon.local": "pw"}
	h := middleware.LoginHandler(users, "secret", "")

	form := url.Values{}
	form.Set("email", "demo@falcon.local")
	form.Set("password", "pw")
	form.Set("next", "/lightning/o/Case/list")
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Location"); got != "/lightning/o/Case/list" {
		t.Errorf("want next-redirect, got %q", got)
	}
}

func TestLoginHandler_NextOpenRedirectRejected(t *testing.T) {
	users := map[string]string{"demo@falcon.local": "pw"}
	h := middleware.LoginHandler(users, "secret", "")

	form := url.Values{}
	form.Set("email", "demo@falcon.local")
	form.Set("password", "pw")
	form.Set("next", "//evil.example.com/x") // protocol-relative
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Location"); got != "/home" {
		t.Errorf("want fallback to /home, got %q", got)
	}
}


func TestLogoutHandler_ClearsCookieAndRedirects(t *testing.T) {
	h := middleware.LogoutHandler("")
	req := httptest.NewRequest("GET", "/logout", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("want 302, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Errorf("want /login, got %q", loc)
	}
	cleared := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == "sf_session" && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("expected sf_session cookie to be cleared (MaxAge<0)")
	}
}

func TestAuth_HTMLUnauthRedirectsToLoginWithNext(t *testing.T) {
	logger := zerolog.Nop()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := middleware.Auth(logger, "test-secret")(handler)

	req := httptest.NewRequest("GET", "/lightning/o/Case/list?x=1", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("want 302, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/login?") {
		t.Fatalf("want /login? prefix, got %q", loc)
	}
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	if got := u.Query().Get("next"); got != "/lightning/o/Case/list?x=1" {
		t.Errorf("want next=/lightning/o/Case/list?x=1, got %q", got)
	}
}

func TestAuth_AcceptsValidJWTSessionCookie(t *testing.T) {
	logger := zerolog.Nop()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := middleware.Auth(logger, "test-secret")(handler)

	// Mint a real cookie via SetSessionCookie.
	cookieRec := httptest.NewRecorder()
	middleware.SetSessionCookie(cookieRec, "demo@falcon.local", "test-secret")
	cookies := cookieRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no cookie set")
	}

	req := httptest.NewRequest("GET", "/lightning/o/Case/list", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200 with valid JWT cookie, got %d (loc=%q)", rec.Code, rec.Header().Get("Location"))
	}
}

func TestAuth_RejectsForeignSecretJWTCookie(t *testing.T) {
	logger := zerolog.Nop()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := middleware.Auth(logger, "real-secret")(handler)

	cookieRec := httptest.NewRecorder()
	middleware.SetSessionCookie(cookieRec, "demo@falcon.local", "wrong-secret")
	cookies := cookieRec.Result().Cookies()

	req := httptest.NewRequest("GET", "/lightning/o/Case/list", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("want 302 (cookie signed by wrong secret), got %d", rec.Code)
	}
}


// Content-negotiation matrix tests for the unauth response path. The
// rule (documented on isHTMLRequest) is that 302 redirects are reserved
// for browser-style hits on UI routes; API surfaces and JSON-only
// callers always receive 401 JSON.

func TestAuth_APIPath_AcceptHTML_StillReturns401JSON(t *testing.T) {
	logger := zerolog.Nop()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := middleware.Auth(logger, "test-secret")(handler)

	req := httptest.NewRequest("GET", "/services/data/v66.0/query", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 for /services/* even with Accept: text/html, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("want JSON content-type, got %q", ct)
	}
	var errs []models.APIError
	if err := json.NewDecoder(rec.Body).Decode(&errs); err != nil {
		t.Fatalf("expected SF array body: %v", err)
	}
	if len(errs) != 1 || errs[0].ErrorCode != "INVALID_SESSION_ID" {
		t.Errorf("unexpected error body: %+v", errs)
	}
}

func TestAuth_UIPath_AcceptJSON_Returns401(t *testing.T) {
	logger := zerolog.Nop()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := middleware.Auth(logger, "test-secret")(handler)

	req := httptest.NewRequest("GET", "/home", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 for UI path with Accept: application/json, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Errorf("must not redirect when caller asked for JSON; got Location=%q", loc)
	}
}

func TestAuth_UIPath_NoAccept_Returns302(t *testing.T) {
	logger := zerolog.Nop()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := middleware.Auth(logger, "test-secret")(handler)

	req := httptest.NewRequest("GET", "/home", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("want 302 for /home with no Accept, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/login") {
		t.Errorf("want Location to start with /login, got %q", loc)
	}
}

func TestAuth_UIPath_BearerHeader_Returns401(t *testing.T) {
	logger := zerolog.Nop()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := middleware.Auth(logger, "test-secret")(handler)

	// Even though /lightning/* is a UI route, an explicit Bearer header
	// signals an API caller — they should get JSON, not a redirect.
	req := httptest.NewRequest("GET", "/lightning/o/Case/list", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 when Bearer header is present, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Errorf("must not redirect when Bearer header is present; got Location=%q", loc)
	}
}

func TestAuth_PlaygroundPath_AcceptHTML_Returns302(t *testing.T) {
	logger := zerolog.Nop()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := middleware.Auth(logger, "test-secret")(handler)

	req := httptest.NewRequest("GET", "/playground", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("want 302 for /playground with Accept: text/html, got %d", rec.Code)
	}
}

func TestAuth_SettingsPath_AcceptWildcard_Returns302(t *testing.T) {
	logger := zerolog.Nop()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := middleware.Auth(logger, "test-secret")(handler)

	req := httptest.NewRequest("GET", "/settings", nil)
	req.Header.Set("Accept", "*/*") // curl's default
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("want 302 for /settings with Accept: */*, got %d", rec.Code)
	}
}

func TestAuth_UnknownPath_AcceptHTML_Returns401(t *testing.T) {
	logger := zerolog.Nop()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := middleware.Auth(logger, "test-secret")(handler)

	// Unknown / non-UI path with browser-style Accept must still 401:
	// the negotiation rule requires both an HTML-friendly Accept AND a
	// known UI route for the redirect branch.
	req := httptest.NewRequest("GET", "/no-such-route", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 for unknown path even with Accept: text/html, got %d", rec.Code)
	}
}
