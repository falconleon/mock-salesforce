package handlers_test

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/config"
	"github.com/falconleon/mock-salesforce/internal/handlers"
	"github.com/falconleon/mock-salesforce/internal/server/middleware"
)

// minimal stub of the consent template so the handler can render
// without pulling in the full layout.html chrome.
const stubConsentTmpl = `{{define "layout"}}<html><body>consent for {{.ClientID}} as {{.CurrentUser}}</body></html>{{end}}`

func newAuthorizeHarness(t *testing.T) (*handlers.AuthorizeHandler, *handlers.AuthCodeStore, *config.Config) {
	t.Helper()
	cfg := config.Default()
	tpl := template.Must(template.New("").Parse(stubConsentTmpl))
	store := handlers.NewAuthCodeStore()
	h := handlers.NewAuthorizeHandler(cfg, store, tpl, zerolog.Nop())
	return h, store, cfg
}

func authorizeURL(cfg *config.Config, overrides url.Values) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", cfg.MockClientID)
	q.Set("redirect_uri", "https://app.example/cb")
	q.Set("code_challenge", "abc")
	q.Set("code_challenge_method", "S256")
	q.Set("state", "xyz")
	for k, vs := range overrides {
		if len(vs) == 0 {
			q.Del(k)
			continue
		}
		q.Set(k, vs[0])
	}
	return "/services/oauth2/authorize?" + q.Encode()
}

func TestAuthorize_Get_Unauthenticated_RedirectsToLogin(t *testing.T) {
	h, _, cfg := newAuthorizeHarness(t)
	req := httptest.NewRequest(http.MethodGet, authorizeURL(cfg, nil), nil)
	rec := httptest.NewRecorder()
	h.HandleGet(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/login?") {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
	if !strings.Contains(loc, "next=") || !strings.Contains(loc, "%2Fservices%2Foauth2%2Fauthorize") {
		t.Errorf("expected next= back to /services/oauth2/authorize, got %q", loc)
	}
}

func TestAuthorize_Get_BadClientID_ReturnsHTML400(t *testing.T) {
	h, _, cfg := newAuthorizeHarness(t)
	req := httptest.NewRequest(http.MethodGet, authorizeURL(cfg, url.Values{"client_id": {"unknown"}}), nil)
	rec := httptest.NewRecorder()
	h.HandleGet(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 (no redirect), got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Errorf("must not redirect on client_id error, got Location=%q", loc)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("expected HTML content type, got %q", ct)
	}
}

func TestAuthorize_Get_MissingRedirectURI_ReturnsHTML400(t *testing.T) {
	h, _, cfg := newAuthorizeHarness(t)
	req := httptest.NewRequest(http.MethodGet, authorizeURL(cfg, url.Values{"redirect_uri": nil}), nil)
	rec := httptest.NewRecorder()
	h.HandleGet(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Errorf("must not redirect on redirect_uri error, got %q", loc)
	}
}

func TestAuthorize_Get_BadResponseType_RedirectsWithError(t *testing.T) {
	h, _, cfg := newAuthorizeHarness(t)
	req := httptest.NewRequest(http.MethodGet, authorizeURL(cfg, url.Values{"response_type": {"token"}}), nil)
	rec := httptest.NewRecorder()
	h.HandleGet(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect with error, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("bad Location: %v", err)
	}
	if u.Host != "app.example" || u.Path != "/cb" {
		t.Errorf("expected redirect back to registered URI, got %q", loc)
	}
	if got := u.Query().Get("error"); got != "unsupported_response_type" {
		t.Errorf("expected error=unsupported_response_type, got %q", got)
	}
	if got := u.Query().Get("state"); got != "xyz" {
		t.Errorf("expected state=xyz preserved, got %q", got)
	}
}

func TestAuthorize_Get_MissingPKCE_RedirectsWithError(t *testing.T) {
	h, _, cfg := newAuthorizeHarness(t)
	req := httptest.NewRequest(http.MethodGet, authorizeURL(cfg, url.Values{"code_challenge": nil}), nil)
	rec := httptest.NewRecorder()
	h.HandleGet(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	if errCode := errFromLocation(t, rec.Header().Get("Location")); errCode != "invalid_request" {
		t.Errorf("expected error=invalid_request, got %q", errCode)
	}
}

func TestAuthorize_Get_BadChallengeMethod_RedirectsWithError(t *testing.T) {
	h, _, cfg := newAuthorizeHarness(t)
	req := httptest.NewRequest(http.MethodGet, authorizeURL(cfg, url.Values{"code_challenge_method": {"MD5"}}), nil)
	rec := httptest.NewRecorder()
	h.HandleGet(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	if errCode := errFromLocation(t, rec.Header().Get("Location")); errCode != "invalid_request" {
		t.Errorf("expected error=invalid_request for bad PKCE method, got %q", errCode)
	}
}

func TestAuthorize_Get_Authenticated_RendersConsent(t *testing.T) {
	h, _, cfg := newAuthorizeHarness(t)
	req := httptest.NewRequest(http.MethodGet, authorizeURL(cfg, nil), nil)
	addSession(req, cfg, "demo@falcon.local")
	rec := httptest.NewRecorder()
	h.HandleGet(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 consent render, got %d (body=%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "consent for "+cfg.MockClientID) {
		t.Errorf("expected consent body to mention client id, got: %s", rec.Body.String())
	}
}

func TestAuthorize_Post_Allow_IssuesCodeAndRedirects(t *testing.T) {
	h, store, cfg := newAuthorizeHarness(t)

	form := url.Values{}
	form.Set("response_type", "code")
	form.Set("client_id", cfg.MockClientID)
	form.Set("redirect_uri", "https://app.example/cb")
	form.Set("scope", "api")
	form.Set("state", "xyz")
	form.Set("code_challenge", "abc")
	form.Set("code_challenge_method", "S256")
	form.Set("action", "allow")

	req := httptest.NewRequest(http.MethodPost, "/services/oauth2/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addSession(req, cfg, "demo@falcon.local")
	rec := httptest.NewRecorder()
	h.HandlePost(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d (body=%s)", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	u, _ := url.Parse(loc)
	code := u.Query().Get("code")
	if code == "" {
		t.Fatalf("expected code= in redirect, got %q", loc)
	}
	if got := u.Query().Get("state"); got != "xyz" {
		t.Errorf("expected state=xyz, got %q", got)
	}
	ac := store.Lookup(code)
	if ac == nil {
		t.Fatalf("issued code %q not present in store", code)
	}
	if ac.ClientID != cfg.MockClientID || ac.RedirectURI != "https://app.example/cb" ||
		ac.CodeChallenge != "abc" || ac.CodeChallengeMethod != "S256" ||
		ac.Username != "demo@falcon.local" || ac.Scope != "api" {
		t.Errorf("auth code recorded with wrong fields: %+v", ac)
	}
}

func TestAuthorize_Post_Deny_RedirectsWithAccessDenied(t *testing.T) {
	h, _, cfg := newAuthorizeHarness(t)
	form := url.Values{}
	form.Set("response_type", "code")
	form.Set("client_id", cfg.MockClientID)
	form.Set("redirect_uri", "https://app.example/cb")
	form.Set("state", "xyz")
	form.Set("code_challenge", "abc")
	form.Set("code_challenge_method", "S256")
	form.Set("action", "deny")

	req := httptest.NewRequest(http.MethodPost, "/services/oauth2/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addSession(req, cfg, "demo@falcon.local")
	rec := httptest.NewRecorder()
	h.HandlePost(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	if got := errFromLocation(t, rec.Header().Get("Location")); got != "access_denied" {
		t.Errorf("expected error=access_denied, got %q", got)
	}
}

// addSession synthesises a valid session cookie on the request using
// the same JWT minting path as the production middleware.
func addSession(r *http.Request, cfg *config.Config, email string) {
	rec := httptest.NewRecorder()
	middleware.SetSessionCookie(rec, email, cfg.SessionSecret)
	for _, c := range rec.Result().Cookies() {
		r.AddCookie(c)
	}
}

func errFromLocation(t *testing.T, loc string) string {
	t.Helper()
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("parse Location %q: %v", loc, err)
	}
	return u.Query().Get("error")
}
