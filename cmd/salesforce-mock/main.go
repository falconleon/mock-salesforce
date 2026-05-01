// Salesforce Mock API Server
//
// A mock implementation of the Salesforce REST API for development,
// testing, and demos.
//
// Usage:
//
//	go run ./cmd/salesforce-mock -port 8080
//	go run ./cmd/salesforce-mock -scenario demo-01-basic
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/config"
	"github.com/falconleon/mock-salesforce/internal/server"
	"github.com/falconleon/mock-salesforce/internal/server/middleware"
	"github.com/falconleon/mock-salesforce/internal/store"
	"github.com/falconleon/mock-salesforce/internal/users"
)

func main() {
	// Parse command line flags. Flag defaults match config.Default(); env
	// vars are loaded via config.FromEnv() and CLI flags override env values
	// (precedence: defaults < env < flags).
	port := flag.Int("port", 8080, "Server port")
	scenario := flag.String("scenario", "", "Load specific demo scenario")
	seedPath := flag.String("seed", "./testdata/seed", "Path to seed data")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	authEnabled := flag.Bool("auth", true, "Enable OAuth token validation")
	dbPath := flag.String("db-path", "", "Path to SQLite database (empty for in-memory)")
	basePath := flag.String("base-path", "", "URL prefix for template links")
	baseURL := flag.String("base-url", "", "Externally-reachable URL for OAuth instance_url (e.g. http://sf-mock:8080/mock/salesforce)")
	mockUsers := flag.String("mock-users", "", "Comma-separated email:password pairs")
	sessionSecret := flag.String("session-secret", "sf-mock-dev-secret", "HMAC key for session cookies")
	adminToken := flag.String("admin-token", "", "X-Admin-Token value for /admin/users endpoints; empty disables them")
	flag.Parse()

	// Build configuration: defaults <- env vars <- CLI flags.
	cfg, err := config.FromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: invalid environment configuration: %v\n", err)
		os.Exit(1)
	}

	// Track which flags were explicitly set so they win over env values.
	flagsSet := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { flagsSet[f.Name] = true })

	if flagsSet["port"] {
		cfg.Port = *port
	}
	if flagsSet["seed"] {
		cfg.SeedDataPath = *seedPath
	}
	if flagsSet["log-level"] {
		cfg.LogLevel = *logLevel
	}
	if flagsSet["auth"] {
		cfg.AuthEnabled = *authEnabled
	}
	if flagsSet["base-path"] {
		cfg.BasePath = strings.TrimRight(*basePath, "/")
	}
	if flagsSet["base-url"] {
		cfg.BaseURL = strings.TrimRight(*baseURL, "/")
	}
	if flagsSet["mock-users"] {
		parsed, perr := config.ParseUsers(*mockUsers)
		if perr != nil {
			fmt.Fprintf(os.Stderr, "FATAL: invalid -mock-users value: %v\n", perr)
			os.Exit(1)
		}
		cfg.MockUsers = parsed
	}
	if flagsSet["session-secret"] {
		cfg.SessionSecret = *sessionSecret
	}
	if flagsSet["admin-token"] {
		cfg.AdminToken = *adminToken
	}

	// Derive InstanceURL from port unless INSTANCE_URL was set in the env
	// (FromEnv has already populated cfg.InstanceURL from the env value).
	if _, set := os.LookupEnv("INSTANCE_URL"); !set {
		cfg.InstanceURL = fmt.Sprintf("http://localhost:%d", cfg.Port)
	}

	// Configure logger using the merged log level.
	logger := setupLogger(cfg.LogLevel)

	// Log startup configuration
	logger.Info().
		Int("port", cfg.Port).
		Str("seed_path", cfg.SeedDataPath).
		Str("scenario", orDefault(*scenario, "(none)")).
		Bool("auth_enabled", cfg.AuthEnabled).
		Str("api_version", cfg.APIVersion).
		Str("db_path", orDefault(*dbPath, "(in-memory)")).
		Str("base_path", orDefault(cfg.BasePath, "(none)")).
		Str("base_url", orDefault(cfg.BaseURL, "(auto)")).
		Int("mock_users", len(cfg.MockUsers)).
		Msg("Starting Salesforce Mock API")

	// Create data store (SQLite if db-path provided, otherwise in-memory)
	var dataStore store.Store
	var loader *store.Loader

	if *dbPath != "" {
		sqliteStore, err := store.NewSQLiteStore(*dbPath)
		if err != nil {
			logger.Fatal().Err(err).Str("db_path", *dbPath).Msg("Failed to open SQLite database")
		}
		defer sqliteStore.Close()

		// Only load seed data if database is empty
		if sqliteStore.Count("Account") == 0 {
			loader = store.NewLoader(sqliteStore, logger)
			if err := loader.LoadFromDirectory(cfg.SeedDataPath); err != nil {
				logger.Warn().Err(err).Msg("Failed to load seed data (continuing without data)")
			}
		} else {
			loader = store.NewLoader(sqliteStore, logger)
			logger.Info().Interface("stats", sqliteStore.Stats()).Msg("Using existing database data")
		}
		dataStore = sqliteStore
	} else {
		memStore := store.NewMemoryStore()
		loader = store.NewLoader(memStore, logger)
		if err := loader.LoadFromDirectory(cfg.SeedDataPath); err != nil {
			logger.Warn().Err(err).Msg("Failed to load seed data (continuing without data)")
		}
		dataStore = memStore
	}

	// Phase 4: Load scenario overlay
	if *scenario != "" {
		scenarioPath := filepath.Join("./testdata/scenarios", *scenario+".json")
		if err := loader.LoadScenario(scenarioPath); err != nil {
			logger.Warn().Err(err).Str("scenario", *scenario).Msg("Failed to load scenario (continuing with seed data)")
		} else {
			logger.Info().Str("scenario", *scenario).Msg("Scenario loaded successfully")
		}
	}

	// Build the runtime user store for /admin/users CRUD + token mint.
	// Persists to SQLite at <db-path>.users.db when -db-path is set.
	userStore := buildUserStore(*dbPath, cfg.MockUsers, logger)

	// Create and start server
	srv := server.New(cfg, dataStore, userStore, logger)

	// Start server in goroutine
	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Server failed to start")
		}
	}()

	// Log available endpoints
	logger.Info().
		Str("oauth", "POST /services/oauth2/token").
		Str("query", fmt.Sprintf("GET /services/data/%s/query?q=...", cfg.APIVersion)).
		Str("health", "GET /health").
		Msg("Endpoints available")

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Stop(ctx); err != nil {
		logger.Error().Err(err).Msg("Server shutdown error")
	}

	logger.Info().Msg("Server stopped")
}

func setupLogger(level string) zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// Set log level
	switch level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// Use console writer for development
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	return zerolog.New(output).With().Timestamp().Logger()
}

func orDefault(val, def string) string {
	if val == "" {
		return def
	}
	return val
}

// buildUserStore constructs the runtime user store, seeds it from the
// MOCK_USERS env var, and re-registers any persisted bearer tokens with
// the OAuth validator so they remain valid across restarts.
func buildUserStore(dbPath string, mockUsers map[string]string, logger zerolog.Logger) users.Store {
	var store users.Store
	if dbPath != "" {
		usersDB := dbPath + ".users.db"
		s, err := users.NewSQLiteStore(usersDB)
		if err != nil {
			logger.Warn().Err(err).Msg("user store SQLite open failed; falling back to in-memory")
			store = users.NewMemoryStore()
		} else {
			logger.Info().Str("users_db", usersDB).Msg("Opened SQLite user store")
			store = s
		}
	} else {
		store = users.NewMemoryStore()
	}
	if err := users.SeedFromMap(store, mockUsers); err != nil {
		logger.Warn().Err(err).Msg("failed to seed users from MOCK_USERS")
	}
	if toks, err := store.AllTokens(); err == nil {
		for _, t := range toks {
			info := &middleware.TokenInfo{
				Token:    t.Token,
				Type:     "access",
				UserID:   t.UserID,
				IssuedAt: t.CreatedAt.Unix(),
			}
			if !t.ExpiresAt.IsZero() {
				info.ExpiresAt = t.ExpiresAt.Unix()
			}
			middleware.RegisterTokenInfo(info)
		}
	}
	return store
}
