// Package pipeline orchestrates the phased data generation process.
package pipeline

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/db"
	"github.com/falconleon/mock-salesforce/internal/generator"
)

// Config holds pipeline configuration.
type Config struct {
	Accounts          int `yaml:"accounts"`
	ContactsPerAcct   int `yaml:"contacts_per_account"`
	Users             int `yaml:"users"`
	Cases             int `yaml:"cases"`
	EmailsPerCaseAvg  int `yaml:"emails_per_case_avg"`
	CommentsPerCase   int `yaml:"comments_per_case_avg"`
	FeedItemsPerCase  int `yaml:"feed_items_per_case_avg"`
	JiraEscalationPct int `yaml:"jira_escalation_pct"`
	JiraCommentsAvg   int `yaml:"jira_comments_per_issue_avg"`
}

// Pipeline runs generation in dependency order.
type Pipeline struct {
	store  *db.Store
	llm    generator.LLM
	config Config
	logger zerolog.Logger
}

// New creates a new generation pipeline.
func New(store *db.Store, llm generator.LLM, config Config, logger zerolog.Logger) *Pipeline {
	return &Pipeline{
		store:  store,
		llm:    llm,
		config: config,
		logger: logger.With().Str("component", "pipeline").Logger(),
	}
}

// RunAll executes all generation phases in order.
func (p *Pipeline) RunAll() error {
	phases := []struct {
		name string
		fn   func() error
	}{
		{"Phase 1: Foundation", p.Phase1Foundation},
		{"Phase 2: Cases", p.Phase2Cases},
		{"Phase 3: Communications", p.Phase3Communications},
		{"Phase 4: JIRA Escalations", p.Phase4JiraEscalations},
	}

	for _, phase := range phases {
		p.logger.Info().Str("phase", phase.name).Msg("Starting phase")
		if err := phase.fn(); err != nil {
			return fmt.Errorf("%s: %w", phase.name, err)
		}
		p.logger.Info().Str("phase", phase.name).Msg("Phase complete")
	}

	return nil
}

// RunPhase executes a single phase by number (1-4).
func (p *Pipeline) RunPhase(phase int) error {
	switch phase {
	case 1:
		return p.Phase1Foundation()
	case 2:
		return p.Phase2Cases()
	case 3:
		return p.Phase3Communications()
	case 4:
		return p.Phase4JiraEscalations()
	default:
		return fmt.Errorf("invalid phase: %d (valid: 1-4)", phase)
	}
}

// Phase1Foundation generates accounts, contacts, users.
func (p *Pipeline) Phase1Foundation() error {
	ctx := &generator.Context{
		DB:     p.store.DB(),
		LLM:    p.llm,
		Logger: p.logger,
	}

	// 1. Accounts
	if err := generator.NewAccountGenerator(ctx).Generate(p.config.Accounts); err != nil {
		return fmt.Errorf("accounts: %w", err)
	}

	// 2. Contacts (depends on accounts)
	if err := generator.NewContactGenerator(ctx).Generate(p.config.ContactsPerAcct); err != nil {
		return fmt.Errorf("contacts: %w", err)
	}

	// 3. Users
	if err := generator.NewUserGenerator(ctx).Generate(p.config.Users); err != nil {
		return fmt.Errorf("users: %w", err)
	}

	return nil
}

// Phase2Cases generates support cases.
func (p *Pipeline) Phase2Cases() error {
	ctx := &generator.Context{
		DB:     p.store.DB(),
		LLM:    p.llm,
		Logger: p.logger,
	}

	return generator.NewCaseGenerator(ctx).Generate(p.config.Cases)
}

// Phase3Communications generates emails, comments, feed items.
func (p *Pipeline) Phase3Communications() error {
	ctx := &generator.Context{
		DB:     p.store.DB(),
		LLM:    p.llm,
		Logger: p.logger,
	}

	if err := generator.NewEmailGenerator(ctx).Generate(); err != nil {
		return fmt.Errorf("emails: %w", err)
	}

	if err := generator.NewCommentGenerator(ctx).Generate(); err != nil {
		return fmt.Errorf("comments: %w", err)
	}

	if err := generator.NewFeedItemGenerator(ctx).Generate(); err != nil {
		return fmt.Errorf("feed items: %w", err)
	}

	return nil
}

// Phase4JiraEscalations generates JIRA issues and comments.
func (p *Pipeline) Phase4JiraEscalations() error {
	ctx := &generator.Context{
		DB:     p.store.DB(),
		LLM:    p.llm,
		Logger: p.logger,
	}

	if err := generator.NewJiraIssueGenerator(ctx).Generate(); err != nil {
		return fmt.Errorf("jira issues: %w", err)
	}

	if err := generator.NewJiraCommentGenerator(ctx).Generate(); err != nil {
		return fmt.Errorf("jira comments: %w", err)
	}

	return nil
}
