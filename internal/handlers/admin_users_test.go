package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/handlers"
	"github.com/falconleon/mock-salesforce/internal/server/middleware"
	"github.com/falconleon/mock-salesforce/internal/users"
)

const testAdminToken = "test-admin-token"

// adminMux returns an http.ServeMux wired with the AdminUsersHandler
// route patterns so r.PathValue("id") / "tokenId" populate correctly.
func adminMux(t *testing.T, store users.Store, token string) http.Handler {
	t.Helper()
	h := handlers.NewAdminUsersHandler(store, token, zerolog.Nop())
	mux := http.NewServeMux()
	mux.HandleFunc("/admin/users", h.HandleUsers)
	mux.HandleFunc("/admin/users/{id}", h.HandleUser)
	mux.HandleFunc("/admin/users/{id}/tokens", h.HandleTokens)
	mux.HandleFunc("/admin/users/{id}/tokens/{tokenId}", h.HandleToken)
	return mux
}

func doJSON(t *testing.T, h http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if token != "" {
		req.Header.Set("X-Admin-Token", token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAdminUsers_RequiresAdminToken(t *testing.T) {
	store := users.NewMemoryStore()
	h := adminMux(t, store, testAdminToken)

	if rec := doJSON(t, h, "GET", "/admin/users", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("missing token: expected 401, got %d", rec.Code)
	}
	if rec := doJSON(t, h, "GET", "/admin/users", "wrong", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong token: expected 401, got %d", rec.Code)
	}
	if rec := doJSON(t, h, "GET", "/admin/users", testAdminToken, nil); rec.Code != http.StatusOK {
		t.Errorf("correct token: expected 200, got %d", rec.Code)
	}
}

func TestAdminUsers_DisabledWhenAdminTokenEmpty(t *testing.T) {
	store := users.NewMemoryStore()
	h := adminMux(t, store, "") // no admin token configured

	rec := doJSON(t, h, "GET", "/admin/users", "anything", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when ADMIN_TOKEN unset, got %d", rec.Code)
	}
}

func TestAdminUsers_CRUDFlow(t *testing.T) {
	store := users.NewMemoryStore()
	h := adminMux(t, store, testAdminToken)

	// Create
	createBody := map[string]string{
		"username": "alice@example.com",
		"name":     "Alice",
		"email":    "alice@example.com",
		"password": "alice-pw",
	}
	rec := doJSON(t, h, "POST", "/admin/users", testAdminToken, createBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var created users.User
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.ID == "" || created.Username != "alice@example.com" {
		t.Errorf("unexpected created user: %+v", created)
	}

	// Duplicate → 409
	rec = doJSON(t, h, "POST", "/admin/users", testAdminToken, createBody)
	if rec.Code != http.StatusConflict {
		t.Errorf("duplicate: expected 409, got %d", rec.Code)
	}

	// List
	rec = doJSON(t, h, "GET", "/admin/users", testAdminToken, nil)
	var listResp struct {
		Users []users.User `json:"users"`
	}
	json.NewDecoder(rec.Body).Decode(&listResp)
	if len(listResp.Users) != 1 {
		t.Errorf("list: expected 1 user, got %d", len(listResp.Users))
	}

	// Get
	rec = doJSON(t, h, "GET", "/admin/users/"+created.ID, testAdminToken, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("get: expected 200, got %d", rec.Code)
	}

	// Patch
	patchBody := map[string]string{"name": "Alice Updated"}
	rec = doJSON(t, h, "PATCH", "/admin/users/"+created.ID, testAdminToken, patchBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var patched users.User
	json.NewDecoder(rec.Body).Decode(&patched)
	if patched.Name != "Alice Updated" {
		t.Errorf("expected name 'Alice Updated', got %q", patched.Name)
	}

	// Delete
	rec = doJSON(t, h, "DELETE", "/admin/users/"+created.ID, testAdminToken, nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("delete: expected 204, got %d", rec.Code)
	}

	// Get after delete → 404
	rec = doJSON(t, h, "GET", "/admin/users/"+created.ID, testAdminToken, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("get-after-delete: expected 404, got %d", rec.Code)
	}
}

func TestAdminUsers_TokenMintUseRevoke(t *testing.T) {
	resetHandlerState(t)
	store := users.NewMemoryStore()
	h := adminMux(t, store, testAdminToken)

	// Seed a user
	u, err := store.CreateUser("bob@example.com", "Bob", "bob@example.com", "pw")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	// Mint a token via the admin endpoint
	mintBody := map[string]any{"label": "ci-runner", "ttl_seconds": 3600}
	rec := doJSON(t, h, "POST", "/admin/users/"+u.ID+"/tokens", testAdminToken, mintBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("mint: expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var minted struct {
		ID     string `json:"id"`
		UserID string `json:"user_id"`
		Token  string `json:"token"`
		Label  string `json:"label"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&minted); err != nil {
		t.Fatalf("decode mint: %v", err)
	}
	if minted.Token == "" || minted.ID == "" {
		t.Fatalf("mint response missing fields: %+v", minted)
	}
	if minted.Label != "ci-runner" {
		t.Errorf("expected label 'ci-runner', got %q", minted.Label)
	}

	// Token must validate through T11's Bearer registry
	info := middleware.LookupToken(minted.Token)
	if info == nil {
		t.Fatal("minted token should be registered with the OAuth validator")
	}
	if info.UserID != u.ID || info.Username != u.Username {
		t.Errorf("registered token info mismatch: %+v", info)
	}

	// List shows the token (without exposing plaintext)
	rec = doJSON(t, h, "GET", "/admin/users/"+u.ID+"/tokens", testAdminToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list tokens: %d", rec.Code)
	}
	var listResp struct {
		Tokens []users.Token `json:"tokens"`
	}
	json.NewDecoder(rec.Body).Decode(&listResp)
	if len(listResp.Tokens) != 1 || listResp.Tokens[0].ID != minted.ID {
		t.Errorf("list tokens unexpected: %+v", listResp.Tokens)
	}
	if listResp.Tokens[0].Token != "" {
		t.Error("plaintext token should not be exposed in list")
	}

	// Revoke
	rec = doJSON(t, h, "DELETE", "/admin/users/"+u.ID+"/tokens/"+minted.ID, testAdminToken, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("revoke: expected 204, got %d body=%s", rec.Code, rec.Body.String())
	}

	// After revoke the token must no longer validate
	if middleware.LookupToken(minted.Token) != nil {
		t.Error("revoked token should be removed from OAuth registry")
	}

	// Revoke again → 404
	rec = doJSON(t, h, "DELETE", "/admin/users/"+u.ID+"/tokens/"+minted.ID, testAdminToken, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("re-revoke: expected 404, got %d", rec.Code)
	}
}

func TestAdminUsers_DeleteUser_RemovesTokensFromStore(t *testing.T) {
	store := users.NewMemoryStore()
	h := adminMux(t, store, testAdminToken)
	u, _ := store.CreateUser("carol@example.com", "", "", "pw")
	store.CreateToken(u.ID, "x", 0)

	rec := doJSON(t, h, "DELETE", "/admin/users/"+u.ID, testAdminToken, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", rec.Code)
	}
	rec = doJSON(t, h, "GET", "/admin/users/"+u.ID+"/tokens", testAdminToken, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("list-after-delete: expected 404, got %d", rec.Code)
	}
}
