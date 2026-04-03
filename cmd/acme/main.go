// Command acme runs the Acme Software scenario using the vendor profile.
//
// Usage:
//
//	go run ./cmd/acme --reset                    # Generate entities only
//	go run ./cmd/acme --interactions             # Generate interactions only (requires entities)
//	go run ./cmd/acme --reset --full             # Generate complete dataset
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rs/zerolog"

	config "github.com/falconleon/mock-salesforce/internal/config/config"
	"github.com/falconleon/mock-salesforce/internal/db"
	"github.com/falconleon/mock-salesforce/internal/llm"
	"github.com/falconleon/mock-salesforce/internal/profile"
	"github.com/falconleon/mock-salesforce/internal/scenario"
)

func main() {
	profilePath := flag.String("profile", "profiles/acme_software.yaml", "Path to vendor profile YAML")
	dbPath := flag.String("db", "data/mock.db", "Path to SQLite database")
	reset := flag.Bool("reset", false, "Reset database before generating")
	provider := flag.String("provider", "zai", "LLM provider: zai, ollama, openai")
	model := flag.String("model", "", "Model name (default: GLM-4.7 for zai)")
	interactions := flag.Bool("interactions", false, "Generate interactions only (cases, emails, comments, JIRA)")
	full := flag.Bool("full", false, "Generate entities AND interactions (complete dataset)")
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

	if repoRoot != "" {
		logger.Info().Str("repo_root", repoRoot).Msg("Loaded .env from repo root")
	}

	logger.Info().
		Str("profile", *profilePath).
		Str("db", *dbPath).
		Bool("reset", *reset).
		Msg("Starting Acme scenario")

	// Load vendor profile
	p, err := profile.Load(*profilePath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load profile")
	}
	logger.Info().
		Str("company", p.Company.Name).
		Int("customers", p.TotalCustomers()).
		Int("support_team", p.TotalSupportHeadcount()).
		Msg("Profile loaded")

	// Open database
	store, err := db.Open(*dbPath, logger)
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

	// Create scenario
	s := scenario.NewAcmeScenario(p, store.DB(), llmClient, logger)

	// Determine what to generate
	generateEntities := !*interactions || *full
	generateInteractions := *interactions || *full

	if generateEntities {
		if err := s.GenerateAll(); err != nil {
			logger.Fatal().Err(err).Msg("Entity generation failed")
		}
	}

	if generateInteractions {
		if err := s.GenerateInteractions(); err != nil {
			logger.Fatal().Err(err).Msg("Interaction generation failed")
		}
	}

	// Print summary
	printSummary(store, logger, generateInteractions)
}

func printSummary(store *db.Store, logger zerolog.Logger, showInteractions bool) {
	logger.Info().Msg("=== Generation Summary ===")

	// Account distribution by type
	rows, err := store.DB().Query(`
		SELECT type, industry, COUNT(*) as count 
		FROM accounts 
		GROUP BY type, industry 
		ORDER BY type, industry
	`)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to query accounts")
	} else {
		defer rows.Close()
		fmt.Println("\n📊 Accounts by Segment:")
		for rows.Next() {
			var typ, industry string
			var count int
			rows.Scan(&typ, &industry, &count)
			fmt.Printf("  %s | %s: %d\n", typ, industry, count)
		}
	}

	// Contact counts per account
	rows, err = store.DB().Query(`
		SELECT a.name, a.type, COUNT(c.id) as contacts
		FROM accounts a
		LEFT JOIN contacts c ON a.id = c.account_id
		GROUP BY a.id
		ORDER BY a.type, a.name
	`)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to query contacts")
	} else {
		defer rows.Close()
		fmt.Println("\n👥 Contacts per Account:")
		for rows.Next() {
			var name, typ string
			var contacts int
			rows.Scan(&name, &typ, &contacts)
			fmt.Printf("  [%s] %s: %d contacts\n", typ, name, contacts)
		}
	}

	// User role distribution
	rows, err = store.DB().Query(`
		SELECT user_role, COUNT(*) as count 
		FROM users 
		GROUP BY user_role 
		ORDER BY user_role
	`)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to query users")
	} else {
		defer rows.Close()
		fmt.Println("\n🎧 Support Team:")
		for rows.Next() {
			var role string
			var count int
			rows.Scan(&role, &count)
			fmt.Printf("  %s: %d\n", role, count)
		}
	}

	// Interaction counts (only when interactions were generated)
	if showInteractions {
		fmt.Println("\n📝 Interactions:")

		// Cases
		var caseCount int
		if err := store.DB().QueryRow("SELECT COUNT(*) FROM cases").Scan(&caseCount); err != nil {
			logger.Error().Err(err).Msg("Failed to query cases")
		} else {
			fmt.Printf("  Cases: %d\n", caseCount)
		}

		// Email messages
		var emailCount int
		if err := store.DB().QueryRow("SELECT COUNT(*) FROM email_messages").Scan(&emailCount); err != nil {
			logger.Error().Err(err).Msg("Failed to query email_messages")
		} else {
			fmt.Printf("  Email Messages: %d\n", emailCount)
		}

		// Case comments
		var commentCount int
		if err := store.DB().QueryRow("SELECT COUNT(*) FROM case_comments").Scan(&commentCount); err != nil {
			logger.Error().Err(err).Msg("Failed to query case_comments")
		} else {
			fmt.Printf("  Case Comments: %d\n", commentCount)
		}

		// Feed items
		var feedCount int
		if err := store.DB().QueryRow("SELECT COUNT(*) FROM feed_items").Scan(&feedCount); err != nil {
			logger.Error().Err(err).Msg("Failed to query feed_items")
		} else {
			fmt.Printf("  Feed Items: %d\n", feedCount)
		}

		// JIRA issues
		var jiraIssueCount int
		if err := store.DB().QueryRow("SELECT COUNT(*) FROM jira_issues").Scan(&jiraIssueCount); err != nil {
			logger.Error().Err(err).Msg("Failed to query jira_issues")
		} else {
			fmt.Printf("  JIRA Issues: %d\n", jiraIssueCount)
		}

		// JIRA comments
		var jiraCommentCount int
		if err := store.DB().QueryRow("SELECT COUNT(*) FROM jira_comments").Scan(&jiraCommentCount); err != nil {
			logger.Error().Err(err).Msg("Failed to query jira_comments")
		} else {
			fmt.Printf("  JIRA Comments: %d\n", jiraCommentCount)
		}
	}

	fmt.Println()
}

