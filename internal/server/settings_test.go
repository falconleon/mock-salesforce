package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testSettingsClientID     = "demo-client-id-xyz"
	testSettingsClientSecret = "super-secret-shh-9876"
)

// newTestSettingsHandler builds a SettingsHandler with stable test
// credentials and an empty session secret (anonymous viewer).
func newTestSettingsHandler() *SettingsHandler {
	return NewSettingsHandler(testSettingsClientID, testSettingsClientSecret, "", "")
}

// TestSettingsPageRendersClientIDAndMaskedSecret verifies the initial
// page render shows the client ID in plaintext and the secret hidden
// behind a mask, with an HTMX trigger to reveal it.
func TestSettingsPageRendersClientIDAndMaskedSecret(t *testing.T) {
	h := newTestSettingsHandler()
	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	rr := httptest.NewRecorder()

	h.HandlePage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{
		"Settings",
		"OAuth Credentials",
		testSettingsClientID,
		"••••••••••••",
		`hx-get="/settings/secret/shown"`,
		`href="/settings/users"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q\n%s", want, body)
		}
	}
	if strings.Contains(body, testSettingsClientSecret) {
		t.Errorf("page must NOT leak the secret on initial render\n%s", body)
	}
}

// TestSettingsSecretShownPartialRevealsSecret verifies the shown
// partial includes the plaintext secret and a toggle back to hidden.
func TestSettingsSecretShownPartialRevealsSecret(t *testing.T) {
	h := newTestSettingsHandler()
	req := httptest.NewRequest(http.MethodGet, "/settings/secret/shown", nil)
	rr := httptest.NewRecorder()

	h.HandleSecretShown(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{
		testSettingsClientSecret,
		`id="client-secret-cell"`,
		`hx-get="/settings/secret/hidden"`,
		`data-state="shown"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q\n%s", want, body)
		}
	}
}

// TestSettingsSecretHiddenPartialMasksSecret verifies the hidden
// partial returns the masked placeholder and toggle back to shown.
func TestSettingsSecretHiddenPartialMasksSecret(t *testing.T) {
	h := newTestSettingsHandler()
	req := httptest.NewRequest(http.MethodGet, "/settings/secret/hidden", nil)
	rr := httptest.NewRecorder()

	h.HandleSecretHidden(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"••••••••••••",
		`id="client-secret-cell"`,
		`hx-get="/settings/secret/shown"`,
		`data-state="hidden"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q\n%s", want, body)
		}
	}
	if strings.Contains(body, testSettingsClientSecret) {
		t.Errorf("hidden partial must NOT leak the secret\n%s", body)
	}
}

// TestSettingsPageRendersNavLinksAndCurrentUser verifies the layout
// nav exposes the Playground + Settings entries added for Wave 3 and
// that the current user surfaces when a session cookie is present.
func TestSettingsPageRendersNavLinksAndCurrentUser(t *testing.T) {
	const secret = "settings-test-secret"
	h := NewSettingsHandler(testSettingsClientID, testSettingsClientSecret, "", secret)

	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	rr := httptest.NewRecorder()
	middlewareSetSession(t, rr, "admin@example.com", secret)
	for _, c := range rr.Result().Cookies() {
		req.AddCookie(c)
	}
	rr2 := httptest.NewRecorder()
	h.HandlePage(rr2, req)

	if rr2.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr2.Code)
	}
	body := rr2.Body.String()
	for _, want := range []string{
		`href="/playground"`,
		`href="/settings"`,
		"admin@example.com",
		"user-menu",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q\n%s", want, body)
		}
	}
}
