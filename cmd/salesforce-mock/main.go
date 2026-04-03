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
	"github.com/falconleon/mock-salesforce/internal/store"
)

func main() {
	// Parse command line flags
	port := flag.Int("port", 8080, "Server port")
	scenario := flag.String("scenario", "", "Load specific demo scenario")
	seedPath := flag.String("seed", "./testdata/seed", "Path to seed data")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	authEnabled := flag.Bool("auth", true, "Enable OAuth token validation")
	dbPath := flag.String("db-path", "", "Path to SQLite database (empty for in-memory)")
	basePath := flag.String("base-path", envDefault("BASE_PATH", ""), "URL prefix for template links")
	mockUsers := flag.String("mock-users", envDefault("MOCK_USERS", ""), "Comma-separated email:password pairs")
	sessionSecret := flag.String("session-secret", envDefault("SESSION_SECRET", "sf-mock-dev-secret"), "HMAC key for session cookies")
	flag.Parse()

	// Configure logger
	logger := setupLogger(*logLevel)

	// Build configuration
	cfg := config.Default()
	cfg.Port = *port
	cfg.SeedDataPath = *seedPath
	cfg.LogLevel = *logLevel
	cfg.AuthEnabled = *authEnabled
	cfg.InstanceURL = fmt.Sprintf("http://localhost:%d", *port)
	cfg.BasePath = strings.TrimRight(*basePath, "/")
	cfg.MockUsers = config.ParseUsers(*mockUsers)
	cfg.SessionSecret = *sessionSecret

	// Log startup configuration
	logger.Info().
		Int("port", cfg.Port).
		Str("seed_path", cfg.SeedDataPath).
		Str("scenario", orDefault(*scenario, "(none)")).
		Bool("auth_enabled", cfg.AuthEnabled).
		Str("api_version", cfg.APIVersion).
		Str("db_path", orDefault(*dbPath, "(in-memory)")).
		Str("base_path", orDefault(cfg.BasePath, "(none)")).
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

	// Create and start server
	srv := server.New(cfg, dataStore, logger)

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

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
