// Command industry runs any industry scenario using a vendor profile.
//
// Usage:
//
//	go run ./cmd/industry --profile profiles/healthcare_medtech.yaml
//	go run ./cmd/industry --profile profiles/finserv_fincore.yaml --accounts 2 --cases 5
//	go run ./cmd/industry --profile profiles/saas_cloudops.yaml --reset --export
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"

	config "github.com/falconleon/mock-salesforce/internal/config/config"
	"github.com/falconleon/mock-salesforce/internal/db"
	"github.com/falconleon/mock-salesforce/internal/exporter"
	"github.com/falconleon/mock-salesforce/internal/llm"
	"github.com/falconleon/mock-salesforce/internal/scenario"
)

func main() {
	profilePath := flag.String("profile", "", "Path to vendor profile YAML (required for generation)")
	dbPath := flag.String("db", "", "Path to SQLite database (default: data/{industry}.db)")
	reset := flag.Bool("reset", false, "Reset database before generating")
	provider := flag.String("provider", "zai", "LLM provider: zai, ollama, openai")
	model := flag.String("model", "", "Model name (default: GLM-4.7 for zai)")
	accounts := flag.Int("accounts", 0, "Limit accounts per segment (0 = use profile)")
	cases := flag.Int("cases", 0, "Limit total cases (0 = default 200)")
	export := flag.Bool("export", false, "Export to JSON after generation")
	outDir := flag.String("out", "", "Output directory for export (default: output/{industry}/)")
	promote := flag.Bool("promote", false, "Copy verified database to canonical location (mock_data/mock.db)")
	flag.Parse()

	// Load .env from repo root
	repoRoot := config.LoadEnvFromRepoRoot()

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().Timestamp().Logger()

	if *promote {
		logger.Warn().Msg("--promote flag is not supported; copy database manually")
		return
	}

	if *profilePath == "" {
		fmt.Fprintln(os.Stderr, "Error: --profile is required for generation")
		flag.Usage()
		os.Exit(1)
	}

	if repoRoot != "" {
		logger.Info().Str("repo_root", repoRoot).Msg("Loaded .env from repo root")
	}

	// Extract industry name from profile path
	baseName := filepath.Base(*profilePath)
	industryName := strings.TrimSuffix(baseName, filepath.Ext(baseName))

	// Default DB path
	dbFile := *dbPath
	if dbFile == "" {
		dbFile = fmt.Sprintf("data/%s.db", industryName)
	}

	// Default output directory
	outputDir := *outDir
	if outputDir == "" {
		outputDir = fmt.Sprintf("output/%s", industryName)
	}

	logger.Info().
		Str("profile", *profilePath).
		Str("db", dbFile).
		Str("industry", industryName).
		Bool("reset", *reset).
		Msg("Starting industry scenario")

	// Open database
	store, err := db.Open(dbFile, logger)
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

	// Initialize LLM client
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

	// Create scenario with config overrides
	cfg := scenario.IndustryConfig{
		MaxAccounts: *accounts,
		MaxCases:    *cases,
	}

	s, err := scenario.NewIndustryScenarioWithConfig(*profilePath, store.DB(), llmClient, logger, cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create scenario")
	}

	logger.Info().
		Str("company", s.CompanyName()).
		Str("industry", s.IndustryName()).
		Int("accounts_limit", *accounts).
		Int("cases_limit", *cases).
		Msg("Profile loaded")

	if err := s.GenerateAll(); err != nil {
		logger.Fatal().Err(err).Msg("Scenario failed")
	}

	printSummary(store, logger)

	// Export if requested
	if *export {
		logger.Info().Str("dir", outputDir).Msg("Exporting data")

		sfExporter := exporter.NewSalesforceExporter(store.DB(), logger)
		if err := sfExporter.Export(outputDir); err != nil {
			logger.Fatal().Err(err).Msg("Salesforce export failed")
		}

		logger.Info().Str("dir", outputDir).Msg("Export complete")
	}
}

func printSummary(store *db.Store, logger zerolog.Logger) {
	logger.Info().Msg("=== Generation Summary ===")

	var count int
	store.DB().QueryRow(`SELECT COUNT(*) FROM accounts`).Scan(&count)
	fmt.Printf("📊 Accounts: %d\n", count)

	store.DB().QueryRow(`SELECT COUNT(*) FROM contacts`).Scan(&count)
	fmt.Printf("👤 Contacts: %d\n", count)

	store.DB().QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	fmt.Printf("🎧 Support Users: %d\n", count)

	store.DB().QueryRow(`SELECT COUNT(*) FROM cases`).Scan(&count)
	fmt.Printf("📁 Cases: %d\n", count)

	store.DB().QueryRow(`SELECT COUNT(*) FROM email_messages`).Scan(&count)
	fmt.Printf("📧 Emails: %d\n", count)

	store.DB().QueryRow(`SELECT COUNT(*) FROM case_comments`).Scan(&count)
	fmt.Printf("💬 Comments: %d\n", count)

	store.DB().QueryRow(`SELECT COUNT(*) FROM feed_items`).Scan(&count)
	fmt.Printf("📢 Feed Items: %d\n", count)

	store.DB().QueryRow(`SELECT COUNT(*) FROM jira_issues`).Scan(&count)
	fmt.Printf("🔧 JIRA Issues: %d\n", count)

	fmt.Println()
}

