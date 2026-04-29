package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/falconleon/mock-salesforce/internal/server/middleware"
	"github.com/falconleon/mock-salesforce/internal/users"
)

// settingsTestMux wires a SettingsUsersHandler onto a real ServeMux so
// path values populate the same way they do at runtime.
func settingsTestMux(t *testing.T, store users.Store) (*SettingsUsersHandler, http.Handler) {
	t.Helper()
	h := NewSettingsUsersHandler(store, "", "")
	mux := http.NewServeMux()
	mux.HandleFunc("/settings/users", h.HandleList)
	mux.HandleFunc("/settings/users/{id}", h.HandleDetail)
	mux.HandleFunc("/settings/users/{id}/tokens", h.HandleTokens)
	mux.HandleFunc("/settings/users/{id}/tokens/{tokenId}/revoke", h.HandleTokenRevoke)
	return h, mux
}

func doForm(t *testing.T, h http.Handler, method, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	var body strings.Reader
	if form != nil {
		body = *strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, path, &body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestSettingsUsers_ListEmpty(t *testing.T) {
	store := users.NewMemoryStore()
	_, mux := settingsTestMux(t, store)
	rr := doForm(t, mux, http.MethodGet, "/settings/users", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{
		"Users", "Create User", "0 items",
		`action="/settings/users"`,
		"No users yet",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestSettingsUsers_CreateAndList(t *testing.T) {
	store := users.NewMemoryStore()
	_, mux := settingsTestMux(t, store)

	form := url.Values{}
	form.Set("username", "alice@example.com")
	form.Set("name", "Alice")
	form.Set("email", "alice@example.com")
	form.Set("password", "alice-pw")

	rr := doForm(t, mux, http.MethodPost, "/settings/users", form)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("create: status = %d, want 303; body=%s", rr.Code, rr.Body.String())
	}
	if loc := rr.Header().Get("Location"); loc != "/settings/users" {
		t.Errorf("redirect location = %q, want %q", loc, "/settings/users")
	}

	all, _ := store.ListUsers()
	if len(all) != 1 || all[0].Username != "alice@example.com" {
		t.Fatalf("user not persisted: %+v", all)
	}

	rr = doForm(t, mux, http.MethodGet, "/settings/users", nil)
	body := rr.Body.String()
	for _, want := range []string{
		"alice@example.com", "Alice", "1 items",
		`href="/settings/users/` + all[0].ID + `"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestSettingsUsers_CreateValidationError(t *testing.T) {
	store := users.NewMemoryStore()
	_, mux := settingsTestMux(t, store)

	// Missing password → ErrEmptyPassword surfaced inline (no redirect).
	form := url.Values{}
	form.Set("username", "bob@example.com")
	rr := doForm(t, mux, http.MethodPost, "/settings/users", form)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (form re-render); body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "password is required") {
		t.Errorf("error message missing\n%s", body)
	}
	// Username form value should be preserved on re-render.
	if !strings.Contains(body, `value="bob@example.com"`) {
		t.Errorf("submitted username not preserved\n%s", body)
	}
}

func TestSettingsUsers_DetailRendersUserAndTokens(t *testing.T) {
	store := users.NewMemoryStore()
	u, _ := store.CreateUser("carol@example.com", "Carol", "carol@example.com", "pw")
	_, _ = store.CreateToken(u.ID, "ci-runner", 0)
	_, mux := settingsTestMux(t, store)

	rr := doForm(t, mux, http.MethodGet, "/settings/users/"+u.ID, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{
		"carol@example.com", "Carol", "Edit User",
		"API Tokens (1)", "ci-runner", "Revoke", "Danger Zone",
		`action="/settings/users/` + u.ID + `/tokens"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestSettingsUsers_DetailNotFound(t *testing.T) {
	store := users.NewMemoryStore()
	_, mux := settingsTestMux(t, store)
	rr := doForm(t, mux, http.MethodGet, "/settings/users/missing", nil)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestSettingsUsers_UpdateRedirects(t *testing.T) {
	store := users.NewMemoryStore()
	u, _ := store.CreateUser("dave@example.com", "Dave", "dave@example.com", "pw")
	_, mux := settingsTestMux(t, store)

	form := url.Values{}
	form.Set("username", "dave@example.com")
	form.Set("name", "Dave Updated")
	form.Set("email", "dave@example.com")
	rr := doForm(t, mux, http.MethodPost, "/settings/users/"+u.ID, form)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body=%s", rr.Code, rr.Body.String())
	}
	got, _ := store.GetUser(u.ID)
	if got.Name != "Dave Updated" {
		t.Errorf("name not updated: %q", got.Name)
	}
}

func TestSettingsUsers_DeleteUser(t *testing.T) {
	store := users.NewMemoryStore()
	u, _ := store.CreateUser("eve@example.com", "Eve", "eve@example.com", "pw")
	tok, _ := store.CreateToken(u.ID, "old", 0)
	middleware.RegisterTokenInfo(&middleware.TokenInfo{Token: tok.Token, Type: "access", UserID: u.ID})

	_, mux := settingsTestMux(t, store)
	form := url.Values{}
	form.Set("_action", "delete")
	rr := doForm(t, mux, http.MethodPost, "/settings/users/"+u.ID, form)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body=%s", rr.Code, rr.Body.String())
	}
	if _, err := store.GetUser(u.ID); err == nil {
		t.Errorf("user should be gone")
	}
	// Token should be removed from the OAuth registry as well so any
	// holder of the plaintext can no longer authenticate.
	if middleware.LookupToken(tok.Token) != nil {
		t.Errorf("token should be revoked from OAuth registry")
	}
}

func TestSettingsUsers_MintTokenShowsPlaintextOnce(t *testing.T) {
	store := users.NewMemoryStore()
	u, _ := store.CreateUser("frank@example.com", "Frank", "frank@example.com", "pw")
	_, mux := settingsTestMux(t, store)

	form := url.Values{}
	form.Set("label", "ci-runner")
	form.Set("ttl_seconds", "3600")
	rr := doForm(t, mux, http.MethodPost, "/settings/users/"+u.ID+"/tokens", form)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Token Minted — Copy Now") {
		t.Errorf("plaintext-token banner missing\n%s", body)
	}
	toks, _ := store.ListTokens(u.ID)
	if len(toks) != 1 {
		t.Fatalf("expected 1 token, got %d", len(toks))
	}
	if !strings.Contains(body, toks[0].Token) {
		t.Errorf("plaintext token value not rendered in response")
	}
	// The minted token must be registered with the OAuth validator so
	// /services/data calls authenticate via the same path as
	// password-grant tokens.
	info := middleware.LookupToken(toks[0].Token)
	if info == nil || info.UserID != u.ID || info.Username != u.Username {
		t.Errorf("minted token not registered with OAuth validator: %+v", info)
	}

	// Subsequent GET of the detail page must NOT re-render the
	// plaintext banner — the token is shown exactly once at mint time.
	rr2 := doForm(t, mux, http.MethodGet, "/settings/users/"+u.ID, nil)
	if strings.Contains(rr2.Body.String(), "Token Minted — Copy Now") {
		t.Errorf("plaintext token banner should only appear at mint time")
	}
	if strings.Contains(rr2.Body.String(), toks[0].Token) {
		t.Errorf("plaintext token must not be rendered on later GETs")
	}
}

func TestSettingsUsers_RevokeToken(t *testing.T) {
	store := users.NewMemoryStore()
	u, _ := store.CreateUser("grace@example.com", "Grace", "grace@example.com", "pw")
	tok, _ := store.CreateToken(u.ID, "old", 0)
	middleware.RegisterTokenInfo(&middleware.TokenInfo{Token: tok.Token, Type: "access", UserID: u.ID})

	_, mux := settingsTestMux(t, store)
	rr := doForm(t, mux, http.MethodPost,
		"/settings/users/"+u.ID+"/tokens/"+tok.ID+"/revoke", url.Values{})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body=%s", rr.Code, rr.Body.String())
	}
	if loc := rr.Header().Get("Location"); loc != "/settings/users/"+u.ID {
		t.Errorf("redirect = %q, want detail page", loc)
	}
	if toks, _ := store.ListTokens(u.ID); len(toks) != 0 {
		t.Errorf("token not removed from store: %+v", toks)
	}
	if middleware.LookupToken(tok.Token) != nil {
		t.Errorf("token not removed from OAuth registry")
	}
}

func TestSettingsUsers_MethodNotAllowed(t *testing.T) {
	store := users.NewMemoryStore()
	u, _ := store.CreateUser("henry@example.com", "Henry", "henry@example.com", "pw")
	_, mux := settingsTestMux(t, store)

	for _, tc := range []struct {
		method, path string
	}{
		{http.MethodPut, "/settings/users"},
		{http.MethodDelete, "/settings/users/" + u.ID},
		{http.MethodGet, "/settings/users/" + u.ID + "/tokens"},
		{http.MethodGet, "/settings/users/" + u.ID + "/tokens/x/revoke"},
	} {
		rr := doForm(t, mux, tc.method, tc.path, nil)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s: status = %d, want 405", tc.method, tc.path, rr.Code)
		}
	}
}
