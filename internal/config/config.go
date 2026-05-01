// Package config provides configuration management for the mock server.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

// Config holds all server configuration.
type Config struct {
	Port             int
	LogLevel         string
	SeedDataPath     string
	AuthEnabled      bool
	MockClientID     string
	MockClientSecret string
	MockUsername     string
	MockPassword     string
	APIVersion       string
	InstanceURL      string
	BaseURL          string // Externally-reachable URL (overrides InstanceURL+BasePath for OAuth)
	BasePath         string
	MockUsers        map[string]string // email -> password
	SessionSecret    string
	AdminToken       string // X-Admin-Token required for /admin/users; empty disables the endpoint
	// MockRefreshRotation enables refresh-token rotation on the
	// refresh_token grant per OAuth 2.1 / RFC 6749 §10.4. When true
	// (default), each refresh exchange issues a fresh refresh_token and
	// invalidates the prior one; replaying a rotated token revokes the
	// whole token family. Set to false to retain the legacy "echo same
	// refresh_token" behaviour.
	MockRefreshRotation bool
	// MockRedirectURIs is the allowlist of redirect_uri values accepted
	// by the OAuth 2.0 authorization_code flow (RFC 6749 §3.1.2.2).
	// Default() leaves this nil so unit tests run in permissive mode;
	// the env loader (FromEnv) and main() populate it with the SF CLI
	// defaults when MOCK_REDIRECT_URIS is unset. An empty (env-set but
	// blank) value disables the check entirely and emits a startup WARN.
	MockRedirectURIs []string
}

// Default returns a Config with default values.
func Default() *Config {
	return &Config{
		Port:             8080,
		LogLevel:         "info",
		SeedDataPath:     "./testdata/seed",
		AuthEnabled:      true,
		MockClientID:     "mock-client-id",
		MockClientSecret: "mock-client-secret",
		MockUsername:     "demo@falcon.local",
		MockPassword:     "demo123",
		APIVersion:       "v66.0",
		InstanceURL:      "http://localhost:8080",
		SessionSecret:    "sf-mock-dev-secret",
		MockRefreshRotation: true,
	}
}

// ParseUsers parses a comma-separated list of email:password pairs.
// Returns an error if any entry is malformed (missing colon, empty email,
// empty password, or empty entry between separators). An empty input
// yields an empty map without error.
func ParseUsers(s string) (map[string]string, error) {
	users := make(map[string]string)
	if strings.TrimSpace(s) == "" {
		return users, nil
	}
	for i, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			return nil, fmt.Errorf("ParseUsers: entry %d is empty", i)
		}
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("ParseUsers: entry %d %q missing ':' separator", i, pair)
		}
		email := strings.TrimSpace(parts[0])
		password := strings.TrimSpace(parts[1])
		if email == "" {
			return nil, fmt.Errorf("ParseUsers: entry %d has empty email", i)
		}
		if password == "" {
			return nil, fmt.Errorf("ParseUsers: entry %d has empty password for %q", i, email)
		}
		users[email] = password
	}
	return users, nil
}

// FromEnv creates a Config from environment variables, falling back to defaults.
// Returns an error if MOCK_USERS is set but malformed.
func FromEnv() (*Config, error) {
	cfg := Default()

	if port := os.Getenv("PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Port = p
		}
	}
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		cfg.LogLevel = level
	}
	if path := os.Getenv("SEED_DATA_PATH"); path != "" {
		cfg.SeedDataPath = path
	}
	if auth := os.Getenv("AUTH_ENABLED"); auth != "" {
		cfg.AuthEnabled = auth != "false" && auth != "0"
	}
	if clientID := os.Getenv("MOCK_CLIENT_ID"); clientID != "" {
		cfg.MockClientID = clientID
	}
	if clientSecret := os.Getenv("MOCK_CLIENT_SECRET"); clientSecret != "" {
		cfg.MockClientSecret = clientSecret
	}
	if username := os.Getenv("MOCK_USERNAME"); username != "" {
		cfg.MockUsername = username
	}
	if password := os.Getenv("MOCK_PASSWORD"); password != "" {
		cfg.MockPassword = password
	}
	if version := os.Getenv("API_VERSION"); version != "" {
		cfg.APIVersion = version
	}
	if instanceURL := os.Getenv("INSTANCE_URL"); instanceURL != "" {
		cfg.InstanceURL = instanceURL
	}
	if baseURL := os.Getenv("BASE_URL"); baseURL != "" {
		cfg.BaseURL = strings.TrimRight(baseURL, "/")
	}
	if basePath := os.Getenv("BASE_PATH"); basePath != "" {
		cfg.BasePath = strings.TrimRight(basePath, "/")
	}
	if mockUsers := os.Getenv("MOCK_USERS"); mockUsers != "" {
		parsed, err := ParseUsers(mockUsers)
		if err != nil {
			return nil, fmt.Errorf("MOCK_USERS: %w", err)
		}
		cfg.MockUsers = parsed
	}
	if sessionSecret := os.Getenv("SESSION_SECRET"); sessionSecret != "" {
		cfg.SessionSecret = sessionSecret
	}
	if adminToken := os.Getenv("ADMIN_TOKEN"); adminToken != "" {
		cfg.AdminToken = adminToken
	}
	if rot := os.Getenv("MOCK_REFRESH_ROTATION"); rot != "" {
		cfg.MockRefreshRotation = rot != "false" && rot != "0"
	}
	uris, permissive := LoadRedirectURIsFromEnv()
	cfg.MockRedirectURIs = uris
	if permissive {
		log.Warn().Msg("redirect_uri allowlist disabled")
	}

	return cfg, nil
}

// DefaultRedirectURIs returns the built-in allowlist used when
// MOCK_REDIRECT_URIS is unset. The two entries cover the Salesforce CLI
// (`sf org login web`, port 1717) and the mock's own /callback debug page.
func DefaultRedirectURIs() []string {
	return []string{
		"http://localhost:1717/OauthRedirect",
		"http://localhost:8080/callback",
	}
}

// ParseRedirectURIs splits a comma-separated MOCK_REDIRECT_URIS value
// into a trimmed, non-empty slice. Returns nil if the input is empty or
// contains only whitespace/commas.
func ParseRedirectURIs(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// LoadRedirectURIsFromEnv resolves MOCK_REDIRECT_URIS into the effective
// allowlist. Returns (DefaultRedirectURIs(), false) when the env var is
// unset; (parsed, false) when set with at least one entry; and
// (nil, true) when set but empty/whitespace — the permissive fallback.
func LoadRedirectURIsFromEnv() (uris []string, permissive bool) {
	raw, ok := os.LookupEnv("MOCK_REDIRECT_URIS")
	if !ok {
		return DefaultRedirectURIs(), false
	}
	parsed := ParseRedirectURIs(raw)
	if len(parsed) == 0 {
		return nil, true
	}
	return parsed, false
}

// IsRedirectURIAllowed reports whether uri is permitted by the
// configured MockRedirectURIs allowlist. A nil/empty allowlist is the
// permissive fallback and accepts any well-formed URI.
func (c *Config) IsRedirectURIAllowed(uri string) bool {
	if len(c.MockRedirectURIs) == 0 {
		return true
	}
	for _, allowed := range c.MockRedirectURIs {
		if allowed == uri {
			return true
		}
	}
	return false
}
