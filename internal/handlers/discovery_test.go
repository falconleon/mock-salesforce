package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/config"
	"github.com/falconleon/mock-salesforce/internal/handlers"
)

// newDiscovery builds a handler against config defaults and invokes it
// with the supplied request, returning the recorded response.
func newDiscovery(t *testing.T, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	return newDiscoveryWithConfig(t, config.Default(), req)
}

// newDiscoveryWithConfig builds a handler against the supplied config
// and invokes it with the request, returning the recorded response.
func newDiscoveryWithConfig(t *testing.T, cfg *config.Config, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	h := handlers.NewDiscoveryHandler(cfg, zerolog.Nop())
	rec := httptest.NewRecorder()
	h.HandleDiscovery(rec, req)
	return rec
}

// decodeDiscovery decodes the JSON document from the recorder.
func decodeDiscovery(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&doc); err != nil {
		t.Fatalf("decode discovery doc: %v", err)
	}
	return doc
}

func TestDiscoveryHandler_Shape(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/.well-known/openid-configuration", nil)
	rec := newDiscovery(t, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	doc := decodeDiscovery(t, rec)

	requiredKeys := []string{
		"issuer",
		"authorization_endpoint",
		"token_endpoint",
		"userinfo_endpoint",
		"revocation_endpoint",
		"introspection_endpoint",
		"response_types_supported",
		"grant_types_supported",
		"token_endpoint_auth_methods_supported",
		"code_challenge_methods_supported",
		"scopes_supported",
		"subject_types_supported",
	}
	for _, k := range requiredKeys {
		if _, ok := doc[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}

	expectArr := func(key string, want []string) {
		raw, ok := doc[key].([]any)
		if !ok {
			t.Fatalf("%s not a JSON array: %T", key, doc[key])
		}
		got := make([]string, len(raw))
		for i, v := range raw {
			s, ok := v.(string)
			if !ok {
				t.Fatalf("%s[%d] not a string: %T", key, i, v)
			}
			got[i] = s
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}

	expectArr("response_types_supported", []string{"code"})
	expectArr("grant_types_supported", []string{"authorization_code", "password", "refresh_token", "client_credentials"})
	expectArr("token_endpoint_auth_methods_supported", []string{"client_secret_basic", "client_secret_post"})
	expectArr("code_challenge_methods_supported", []string{"S256"})
	expectArr("scopes_supported", []string{"api", "refresh_token", "openid", "id", "profile", "email"})
	expectArr("subject_types_supported", []string{"public"})
}

func TestDiscoveryHandler_AbsoluteURLs(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.test:9999/.well-known/openid-configuration", nil)
	rec := newDiscovery(t, req)
	doc := decodeDiscovery(t, rec)

	wantPrefix := "http://example.test:9999"
	endpoints := []string{
		"issuer",
		"authorization_endpoint",
		"token_endpoint",
		"userinfo_endpoint",
		"revocation_endpoint",
		"introspection_endpoint",
	}
	for _, k := range endpoints {
		v, ok := doc[k].(string)
		if !ok {
			t.Fatalf("%s not a string: %T", k, doc[k])
		}
		if !strings.HasPrefix(v, wantPrefix) {
			t.Errorf("%s = %q, expected prefix %q", k, v, wantPrefix)
		}
	}
}

func TestDiscoveryHandler_NoAuthHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/.well-known/openid-configuration", nil)
	// Explicitly assert no Authorization header is set.
	if req.Header.Get("Authorization") != "" {
		t.Fatalf("test setup: Authorization header should be empty")
	}
	rec := newDiscovery(t, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 with no Authorization header, got %d", rec.Code)
	}
}

func TestDiscoveryHandler_ContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/.well-known/openid-configuration", nil)
	rec := newDiscovery(t, req)
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json prefix", ct)
	}
}

// TestDiscovery_IgnoresForwardedHostByDefault confirms that when
// cfg.PublicBaseURL is unset, X-Forwarded-Proto / X-Forwarded-Host
// cannot poison the discovery document. The base URL must derive from
// the request Host header only.
func TestDiscovery_IgnoresForwardedHostByDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://internal:8080/.well-known/openid-configuration", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "evil.example")
	rec := newDiscovery(t, req)
	doc := decodeDiscovery(t, rec)

	endpoints := []string{
		"issuer",
		"authorization_endpoint",
		"token_endpoint",
		"userinfo_endpoint",
		"revocation_endpoint",
		"introspection_endpoint",
	}
	for _, k := range endpoints {
		v, _ := doc[k].(string)
		if strings.Contains(v, "evil.example") {
			t.Errorf("%s = %q must not contain evil.example", k, v)
		}
		if !strings.HasPrefix(v, "http://internal:8080") {
			t.Errorf("%s = %q, expected prefix http://internal:8080", k, v)
		}
	}
}

// TestDiscovery_PublicBaseURLOverridesEverything confirms that a
// configured cfg.PublicBaseURL wins over both the request Host and any
// X-Forwarded-* headers an attacker might supply.
func TestDiscovery_PublicBaseURLOverridesEverything(t *testing.T) {
	cfg := config.Default()
	cfg.PublicBaseURL = "https://login.example.com"
	req := httptest.NewRequest(http.MethodGet, "http://internal:8080/.well-known/openid-configuration", nil)
	req.Header.Set("X-Forwarded-Proto", "http")
	req.Header.Set("X-Forwarded-Host", "evil.example")
	rec := newDiscoveryWithConfig(t, cfg, req)
	doc := decodeDiscovery(t, rec)

	endpoints := []string{
		"issuer",
		"authorization_endpoint",
		"token_endpoint",
		"userinfo_endpoint",
		"revocation_endpoint",
		"introspection_endpoint",
	}
	for _, k := range endpoints {
		v, _ := doc[k].(string)
		if !strings.HasPrefix(v, "https://login.example.com") {
			t.Errorf("%s = %q, expected prefix https://login.example.com", k, v)
		}
		if strings.Contains(v, "evil.example") {
			t.Errorf("%s = %q must not contain evil.example", k, v)
		}
		if strings.Contains(v, "internal:8080") {
			t.Errorf("%s = %q must not contain request Host", k, v)
		}
	}
}
