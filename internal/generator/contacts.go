package generator

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/falconleon/mock-salesforce/internal/generator/seed"
	"github.com/rs/zerolog"
)

// ContactGenerator generates Contact records linked to accounts.
type ContactGenerator struct {
	db     *sql.DB
	llm    LLM
	logger zerolog.Logger
}

// NewContactGenerator creates a new contact generator.
func NewContactGenerator(ctx *Context) *ContactGenerator {
	return &ContactGenerator{
		db:     ctx.DB,
		llm:    ctx.LLM,
		logger: ctx.Logger.With().Str("generator", "contacts").Logger(),
	}
}

// Contact represents a generated contact record.
type Contact struct {
	ID         string `json:"id"`
	AccountID  string `json:"account_id"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	Title      string `json:"title"`
	Department string `json:"department"`
	IsPrimary  bool   `json:"is_primary"`
	CreatedAt  string `json:"created_at"`
}

// Generate creates contacts for all accounts in the database.
// It generates perAccount contacts per account and marks the first one as primary.
func (g *ContactGenerator) Generate(perAccount int) error {
	g.logger.Info().Int("perAccount", perAccount).Msg("Generating contacts")

	// Fetch all accounts
	rows, err := g.db.Query("SELECT id, name, industry FROM accounts")
	if err != nil {
		return fmt.Errorf("query accounts: %w", err)
	}
	defer rows.Close()

	type acct struct {
		id, name, industry string
	}
	var accounts []acct
	for rows.Next() {
		var a acct
		if err := rows.Scan(&a.id, &a.name, &a.industry); err != nil {
			return fmt.Errorf("scan account: %w", err)
		}
		accounts = append(accounts, a)
	}

	if len(accounts) == 0 {
		g.logger.Warn().Msg("No accounts found, skipping contact generation")
		return nil
	}

	stmt, err := g.db.Prepare(`INSERT INTO contacts
		(id, account_id, first_name, last_name, email, phone, title, department, is_primary, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	totalInserted := 0
	for _, a := range accounts {
		for i := 0; i < perAccount; i++ {
			// Use RoleManager for primary contact (first), RoleSupportL1 for others
			roleLevel := seed.RoleSupportL1
			if i == 0 {
				roleLevel = seed.RoleManager
			}

			personSeed := seed.NewPersonSeed(roleLevel)
			domain := domainFromName(a.name)

			prompt := fmt.Sprintf(`Generate a contact for %s in the %s industry.

Person details:
%s

Generate:
- Job title appropriate for this person at a %s company (if this is a senior contact, use Director/VP level title)
- Department matching the title
- Phone number (555-XXXX format)
- Created date in 2024 (ISO 8601 format)

The person's name is %s %s.
Email should be: %s.%s@%s

Return ONLY a JSON object with fields: title, department, phone, created_at

Example:
{"title":"IT Director","department":"Information Technology","phone":"555-0101","created_at":"2024-02-10T09:00:00Z"}`,
				a.name, a.industry,
				personSeed.ToPromptFragment(),
				a.industry,
				personSeed.FirstName, personSeed.LastName,
				firstNameToEmail(personSeed.FirstName), lastNameToEmail(personSeed.LastName), domain)

			resp, err := g.llm.Generate(prompt)
			if err != nil {
				g.logger.Error().Err(err).Str("account", a.name).Msg("Failed to generate contact")
				continue
			}

			var contactData struct {
				Title      string `json:"title"`
				Department string `json:"department"`
				Phone      string `json:"phone"`
				CreatedAt  string `json:"created_at"`
			}
			if err := json.Unmarshal([]byte(resp), &contactData); err != nil {
				g.logger.Error().Err(err).Str("response", resp).Msg("Failed to parse contact")
				continue
			}

			contact := Contact{
				ID:         SalesforceID("Contact"),
				AccountID:  a.id,
				FirstName:  personSeed.FirstName,
				LastName:   personSeed.LastName,
				Email:      fmt.Sprintf("%s.%s@%s", firstNameToEmail(personSeed.FirstName), lastNameToEmail(personSeed.LastName), domain),
				Phone:      contactData.Phone,
				Title:      contactData.Title,
				Department: contactData.Department,
				IsPrimary:  i == 0,
				CreatedAt:  contactData.CreatedAt,
			}

			isPrimaryInt := 0
			if contact.IsPrimary {
				isPrimaryInt = 1
			}

			_, err = stmt.Exec(
				contact.ID, contact.AccountID,
				contact.FirstName, contact.LastName,
				contact.Email, contact.Phone,
				contact.Title, contact.Department,
				isPrimaryInt, contact.CreatedAt,
			)
			if err != nil {
				g.logger.Error().Err(err).Str("contact", contact.Email).Msg("Insert failed")
				continue
			}
			totalInserted++
			g.logger.Debug().
				Str("name", contact.FirstName+" "+contact.LastName).
				Str("account", a.name).
				Msg("Generated contact")
		}
	}

	g.logger.Info().Int("inserted", totalInserted).Int("accounts", len(accounts)).Msg("Contacts generated")
	return nil
}

// domainFromName creates a plausible domain from a company name.
func domainFromName(name string) string {
	// Simple transform - lowercase, remove spaces, add .example.com
	domain := ""
	for _, c := range name {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			if c >= 'A' && c <= 'Z' {
				c = c + 32 // lowercase
			}
			domain += string(c)
		}
	}
	return domain + ".example.com"
}

// firstNameToEmail converts a first name to lowercase for email.
func firstNameToEmail(name string) string {
	result := ""
	for _, c := range name {
		if c >= 'A' && c <= 'Z' {
			c = c + 32
		}
		if c >= 'a' && c <= 'z' {
			result += string(c)
		}
	}
	return result
}

// lastNameToEmail converts a last name to lowercase for email.
func lastNameToEmail(name string) string {
	return firstNameToEmail(name) // Same logic
}
