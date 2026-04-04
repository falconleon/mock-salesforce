// Package config provides configuration management for the mock server.
package config

import (
	"os"
	"strconv"
	"strings"
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
	}
}

// ParseUsers parses a comma-separated list of email:password pairs.
func ParseUsers(s string) map[string]string {
	users := make(map[string]string)
	if s == "" {
		return users
	}
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) == 2 {
			users[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return users
}

// FromEnv creates a Config from environment variables, falling back to defaults.
func FromEnv() *Config {
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
		cfg.MockUsers = ParseUsers(mockUsers)
	}
	if sessionSecret := os.Getenv("SESSION_SECRET"); sessionSecret != "" {
		cfg.SessionSecret = sessionSecret
	}

	return cfg
}
