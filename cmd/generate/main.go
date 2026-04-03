// Command generate runs the mock data generation pipeline.
//
// Usage:
//
//	go run ./cmd/generate --config config.yaml
//	go run ./cmd/generate --config config.yaml --phase 1
//	go run ./cmd/generate --config config.yaml --reset
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/db"
	"github.com/falconleon/mock-salesforce/internal/llm"
	"github.com/falconleon/mock-salesforce/internal/pipeline"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to config file")
	phase := flag.Int("phase", 0, "Run specific phase (1-4), 0 for all")
	reset := flag.Bool("reset", false, "Reset database before generating")
	dbPath := flag.String("db", "", "Override database path from config")
	provider := flag.String("provider", "zai", "LLM provider: zai, ollama, openai")
	model := flag.String("model", "", "Model name (default: GLM-4.7 for zai)")
	flag.Parse()

	// Load .env file if present (silently ignore errors)
	_ = godotenv.Load()

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().Timestamp().Logger()

	logger.Info().
		Str("config", *configPath).
		Int("phase", *phase).
		Bool("reset", *reset).
		Str("provider", *provider).
		Msg("Starting mock data generator")

	// TODO: Load config from YAML file
	_ = configPath

	// Determine DB path
	path := "data/mock.db"
	if *dbPath != "" {
		path = *dbPath
	}

	// Open database
	store, err := db.Open(path, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to open database")
	}
	defer store.Close()

	if *reset {
		if err := store.Reset(); err != nil {
			logger.Fatal().Err(err).Msg("Failed to reset database")
		}
	} else {
		if err := store.Init(); err != nil {
			logger.Fatal().Err(err).Msg("Failed to initialize database")
		}
	}

	// Initialize LLM client from endpoint module
	llmCfg := llm.DefaultConfig()
	llmCfg.Provider = *provider
	if *model != "" {
		llmCfg.Model = *model
	}

	llmClient, err := llm.New(llmCfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize LLM client")
	}
	logger.Info().Str("provider", *provider).Str("model", llmCfg.Model).Msg("LLM client initialized")

	// TODO: Create and run pipeline once fully connected
	// p := pipeline.New(store, llmClient, pipelineConfig, logger)
	_ = pipeline.Config{}
	_ = llmClient

	if *phase == 0 {
		logger.Info().Msg("Would run all phases")
	} else {
		logger.Info().Int("phase", *phase).Msg("Would run single phase")
	}

	// Print current stats
	stats, err := store.Stats()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get stats")
	} else {
		for table, count := range stats {
			if count > 0 {
				fmt.Printf("  %s: %d records\n", table, count)
			}
		}
	}

	logger.Info().Msg("Done")
}