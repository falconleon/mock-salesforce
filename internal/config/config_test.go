package config

import (
	"os"
	"strings"
	"testing"
)

func TestParseUsersValid(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want map[string]string
	}{
		{"empty", "", map[string]string{}},
		{"whitespace only", "   ", map[string]string{}},
		{"single", "alice@example.com:secret", map[string]string{"alice@example.com": "secret"}},
		{"multiple", "a@x.com:p1,b@x.com:p2", map[string]string{"a@x.com": "p1", "b@x.com": "p2"}},
		{"trims whitespace", " a@x.com : p1 , b@x.com:p2 ", map[string]string{"a@x.com": "p1", "b@x.com": "p2"}},
		{"password with colon", "a@x.com:has:colon", map[string]string{"a@x.com": "has:colon"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseUsers(tt.in)
			if err != nil {
				t.Fatalf("ParseUsers(%q) returned unexpected error: %v", tt.in, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ParseUsers(%q) len=%d want %d (got=%v)", tt.in, len(got), len(tt.want), got)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("ParseUsers(%q)[%q] = %q, want %q", tt.in, k, got[k], v)
				}
			}
		})
	}
}

func TestParseUsersRejectsMalformed(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		errSubstr string
	}{
		{"missing colon", "alice@example.com", "missing ':'"},
		{"empty email", ":secret", "empty email"},
		{"empty password", "alice@example.com:", "empty password"},
		{"whitespace-only email", "  :secret", "empty email"},
		{"whitespace-only password", "alice@example.com:   ", "empty password"},
		{"empty entry between commas", "a@x.com:p,,b@x.com:p", "empty"},
		{"trailing comma", "a@x.com:p,", "empty"},
		{"leading comma", ",a@x.com:p", "empty"},
		{"second entry malformed", "a@x.com:p,bogus", "missing ':'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseUsers(tt.in)
			if err == nil {
				t.Fatalf("ParseUsers(%q) succeeded with %v; want error", tt.in, got)
			}
			if got != nil {
				t.Errorf("ParseUsers(%q) returned non-nil map %v on error", tt.in, got)
			}
			if !strings.Contains(err.Error(), tt.errSubstr) {
				t.Errorf("ParseUsers(%q) error = %q, want substring %q", tt.in, err.Error(), tt.errSubstr)
			}
		})
	}
}

func TestFromEnvRejectsMalformedMockUsers(t *testing.T) {
	t.Setenv("MOCK_USERS", "alice@example.com")
	cfg, err := FromEnv()
	if err == nil {
		t.Fatalf("FromEnv() succeeded with cfg=%+v; want error for malformed MOCK_USERS", cfg)
	}
	if cfg != nil {
		t.Errorf("FromEnv() returned non-nil cfg %+v on error", cfg)
	}
	if !strings.Contains(err.Error(), "MOCK_USERS") {
		t.Errorf("FromEnv() error = %q, want substring %q", err.Error(), "MOCK_USERS")
	}
}

func TestFromEnvAcceptsValidMockUsers(t *testing.T) {
	t.Setenv("MOCK_USERS", "alice@example.com:secret,bob@example.com:hunter2")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("FromEnv() returned nil cfg")
	}
	if cfg.MockUsers["alice@example.com"] != "secret" {
		t.Errorf("MockUsers[alice]=%q want secret", cfg.MockUsers["alice@example.com"])
	}
	if cfg.MockUsers["bob@example.com"] != "hunter2" {
		t.Errorf("MockUsers[bob]=%q want hunter2", cfg.MockUsers["bob@example.com"])
	}
}

func TestFromEnvUnsetMockUsers(t *testing.T) {
	os.Unsetenv("MOCK_USERS")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("FromEnv() returned nil cfg")
	}
	if len(cfg.MockUsers) != 0 {
		t.Errorf("MockUsers=%v want empty", cfg.MockUsers)
	}
}

// H1: Default() must NOT seed MockRedirectURIs; tests rely on permissive
// behaviour so they don't accidentally accept random URIs.
func TestDefaultLeavesRedirectURIsNil(t *testing.T) {
	cfg := Default()
	if cfg.MockRedirectURIs != nil {
		t.Errorf("Default().MockRedirectURIs = %v, want nil", cfg.MockRedirectURIs)
	}
}

func TestLoadRedirectURIsFromEnv_UnsetReturnsSFCLIDefaults(t *testing.T) {
	os.Unsetenv("MOCK_REDIRECT_URIS")
	uris, permissive := LoadRedirectURIsFromEnv()
	if permissive {
		t.Error("permissive should be false when env unset")
	}
	want := DefaultRedirectURIs()
	if len(uris) != len(want) {
		t.Fatalf("uris=%v, want %v", uris, want)
	}
	for i, u := range want {
		if uris[i] != u {
			t.Errorf("uris[%d]=%q, want %q", i, uris[i], u)
		}
	}
}

func TestLoadRedirectURIsFromEnv_SetReturnsParsed(t *testing.T) {
	t.Setenv("MOCK_REDIRECT_URIS", "https://a.example/cb, https://b.example/cb")
	uris, permissive := LoadRedirectURIsFromEnv()
	if permissive {
		t.Error("permissive should be false when env has values")
	}
	if len(uris) != 2 || uris[0] != "https://a.example/cb" || uris[1] != "https://b.example/cb" {
		t.Errorf("uris=%v", uris)
	}
}

func TestLoadRedirectURIsFromEnv_EmptyTriggersPermissive(t *testing.T) {
	t.Setenv("MOCK_REDIRECT_URIS", "")
	uris, permissive := LoadRedirectURIsFromEnv()
	if !permissive {
		t.Error("permissive should be true when env set but empty")
	}
	if uris != nil {
		t.Errorf("uris should be nil in permissive mode, got %v", uris)
	}
}

func TestFromEnv_RefreshRotationFalse(t *testing.T) {
	t.Setenv("MOCK_REFRESH_ROTATION", "false")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() unexpected error: %v", err)
	}
	if cfg.MockRefreshRotation {
		t.Errorf("MockRefreshRotation = true, want false (env MOCK_REFRESH_ROTATION=false)")
	}
}

func TestFromEnv_RefreshRotationZero(t *testing.T) {
	t.Setenv("MOCK_REFRESH_ROTATION", "0")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() unexpected error: %v", err)
	}
	if cfg.MockRefreshRotation {
		t.Errorf("MockRefreshRotation = true, want false (env MOCK_REFRESH_ROTATION=0)")
	}
}

func TestFromEnv_RefreshRotationDefaultTrue(t *testing.T) {
	os.Unsetenv("MOCK_REFRESH_ROTATION")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() unexpected error: %v", err)
	}
	if !cfg.MockRefreshRotation {
		t.Errorf("MockRefreshRotation = false, want true (default when env unset)")
	}
}

func TestFromEnv_RedirectURIs(t *testing.T) {
	t.Setenv("MOCK_REDIRECT_URIS", "https://a/cb,https://b/cb")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() unexpected error: %v", err)
	}
	if len(cfg.MockRedirectURIs) != 2 ||
		cfg.MockRedirectURIs[0] != "https://a/cb" ||
		cfg.MockRedirectURIs[1] != "https://b/cb" {
		t.Errorf("MockRedirectURIs = %v, want [https://a/cb https://b/cb]", cfg.MockRedirectURIs)
	}
}

func TestFromEnv_AllDocumentedVars(t *testing.T) {
	// Every env var listed in the README "Environment Variables" table
	// (plus MOCK_REFRESH_ROTATION) must round-trip through FromEnv.
	t.Setenv("PORT", "9091")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("AUTH_ENABLED", "false")
	t.Setenv("SEED_DATA_PATH", "/tmp/seed")
	t.Setenv("MOCK_CLIENT_ID", "cid")
	t.Setenv("MOCK_CLIENT_SECRET", "csecret")
	t.Setenv("MOCK_USERNAME", "u@x.com")
	t.Setenv("MOCK_PASSWORD", "pw")
	t.Setenv("MOCK_USERS", "alice@x.com:a,bob@x.com:b")
	t.Setenv("SESSION_SECRET", "shh")
	t.Setenv("API_VERSION", "v99.0")
	t.Setenv("INSTANCE_URL", "http://example.test")
	t.Setenv("BASE_PATH", "/mock/sf/")
	t.Setenv("BASE_URL", "http://ext.test/sf/")
	t.Setenv("ADMIN_TOKEN", "admin-secret")
	t.Setenv("MOCK_REFRESH_ROTATION", "false")
	t.Setenv("MOCK_PUBLIC_BASE_URL", "https://login.example.com/")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() unexpected error: %v", err)
	}
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"Port", cfg.Port, 9091},
		{"LogLevel", cfg.LogLevel, "debug"},
		{"AuthEnabled", cfg.AuthEnabled, false},
		{"SeedDataPath", cfg.SeedDataPath, "/tmp/seed"},
		{"MockClientID", cfg.MockClientID, "cid"},
		{"MockClientSecret", cfg.MockClientSecret, "csecret"},
		{"MockUsername", cfg.MockUsername, "u@x.com"},
		{"MockPassword", cfg.MockPassword, "pw"},
		{"SessionSecret", cfg.SessionSecret, "shh"},
		{"APIVersion", cfg.APIVersion, "v99.0"},
		{"InstanceURL", cfg.InstanceURL, "http://example.test"},
		{"BasePath", cfg.BasePath, "/mock/sf"},
		{"BaseURL", cfg.BaseURL, "http://ext.test/sf"},
		{"AdminToken", cfg.AdminToken, "admin-secret"},
		{"MockRefreshRotation", cfg.MockRefreshRotation, false},
		{"PublicBaseURL", cfg.PublicBaseURL, "https://login.example.com"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
	if cfg.MockUsers["alice@x.com"] != "a" || cfg.MockUsers["bob@x.com"] != "b" {
		t.Errorf("MockUsers = %v, want alice:a bob:b", cfg.MockUsers)
	}
}

func TestIsRedirectURIAllowed(t *testing.T) {
	cfg := &Config{MockRedirectURIs: []string{"https://a/cb", "https://b/cb"}}
	if !cfg.IsRedirectURIAllowed("https://a/cb") {
		t.Error("expected allowlisted URI to be accepted")
	}
	if cfg.IsRedirectURIAllowed("https://c/cb") {
		t.Error("expected non-allowlisted URI to be rejected")
	}
	permissive := &Config{}
	if !permissive.IsRedirectURIAllowed("https://anything/cb") {
		t.Error("permissive (empty allowlist) should accept any URI")
	}
}
