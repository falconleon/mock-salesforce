package generator

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/falconleon/mock-salesforce/internal/generator/seed"
	"github.com/rs/zerolog"
)

// AccountGenerator generates Account records.
type AccountGenerator struct {
	db     *sql.DB
	llm    LLM
	logger zerolog.Logger
}

// NewAccountGenerator creates a new account generator.
func NewAccountGenerator(ctx *Context) *AccountGenerator {
	return &AccountGenerator{
		db:     ctx.DB,
		llm:    ctx.LLM,
		logger: ctx.Logger.With().Str("generator", "accounts").Logger(),
	}
}

// Account represents a generated account record.
type Account struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Industry       string  `json:"industry"`
	Type           string  `json:"type"`
	Website        string  `json:"website"`
	Phone          string  `json:"phone"`
	BillingCity    string  `json:"billing_city"`
	BillingState   string  `json:"billing_state"`
	AnnualRevenue  float64 `json:"annual_revenue"`
	NumEmployees   int     `json:"num_employees"`
	CreatedAt      string  `json:"created_at"`
}

// Generate creates the specified number of account records.
func (g *AccountGenerator) Generate(count int) error {
	g.logger.Info().Int("count", count).Msg("Generating accounts")

	// Industries to distribute across accounts
	industries := []string{"Technology", "Healthcare", "Finance", "Retail", "Education", "Manufacturing"}
	types := []string{"Enterprise", "Enterprise", "Mid-Market", "Mid-Market", "SMB"} // 40%/40%/20% distribution

	// Start transaction
	tx, err := g.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO accounts
		(id, name, industry, type, website, phone, billing_city, billing_state, annual_revenue, num_employees, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	inserted := 0
	for i := 0; i < count; i++ {
		// Create a seed for this account
		companySeed := seed.NewCompanySeed()
		industry := industries[i%len(industries)]
		accountType := types[i%len(types)]

		// Build prompt with seed data
		prompt := fmt.Sprintf(`Generate a company name for a %s company based in %s, %s.
The company was founded in %d and has approximately %d employees.
Company suffix should be: %s

Return ONLY a JSON object with these fields:
- name: realistic B2B company name (must end with "%s")
- website: company website URL
- phone: phone number (555-XXXX format)
- annual_revenue: realistic revenue for company of this size ($5M-$500M)
- created_at: ISO 8601 timestamp in 2023

Example:
{"name":"Acme %s","website":"https://acme.example.com","phone":"555-0100","annual_revenue":50000000,"created_at":"2023-01-15T10:00:00Z"}`,
			industry, companySeed.City, companySeed.State,
			companySeed.FoundedYear, companySeed.EmployeeCount,
			companySeed.Suffix, companySeed.Suffix, companySeed.Suffix)

		resp, err := g.llm.Generate(prompt)
		if err != nil {
			g.logger.Error().Err(err).Int("index", i).Msg("Failed to generate account")
			continue
		}

		var account Account
		if err := json.Unmarshal([]byte(resp), &account); err != nil {
			g.logger.Error().Err(err).Str("response", resp).Msg("Failed to parse account response")
			continue
		}

		// Fill in seed data and generated fields
		account.ID = SalesforceID("Account")
		account.Industry = industry
		account.Type = accountType
		account.BillingCity = companySeed.City
		account.BillingState = companySeed.State
		account.NumEmployees = companySeed.EmployeeCount

		_, err = stmt.Exec(
			account.ID, account.Name, account.Industry, account.Type,
			account.Website, account.Phone, account.BillingCity, account.BillingState,
			account.AnnualRevenue, account.NumEmployees, account.CreatedAt,
		)
		if err != nil {
			g.logger.Error().Err(err).Str("name", account.Name).Msg("Insert failed")
			continue
		}
		inserted++
		g.logger.Debug().Str("name", account.Name).Str("city", companySeed.City).Msg("Generated account")
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	g.logger.Info().Int("inserted", inserted).Msg("Accounts generated")
	return nil
}
