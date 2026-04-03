package generator

import (
	"database/sql"
	"fmt"
	"regexp"
	"sync/atomic"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
)

// dynamicContactMockLLM returns different responses for each call.
type dynamicContactMockLLM struct {
	callCount atomic.Int32
}

func (m *dynamicContactMockLLM) Generate(prompt string) (string, error) {
	count := m.callCount.Add(1)
	// Return a unique contact data for each call (name comes from seed, not LLM)
	return fmt.Sprintf(`{"title":"Test Title %d","department":"Department %d","phone":"555-010%d","created_at":"2024-0%d-15T09:00:00Z"}`,
		count, count, count, min(int(count), 9)), nil
}

// contactTestDB creates an in-memory SQLite database with accounts and contacts tables.
func contactTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	// Create accounts table
	_, err = db.Exec(`CREATE TABLE accounts (
		id              TEXT PRIMARY KEY,
		name            TEXT NOT NULL,
		industry        TEXT NOT NULL,
		type            TEXT NOT NULL,
		website         TEXT,
		phone           TEXT,
		billing_city    TEXT,
		billing_state   TEXT,
		annual_revenue  REAL,
		num_employees   INTEGER,
		created_at      TEXT NOT NULL
	)`)
	if err != nil {
		db.Close()
		t.Fatalf("create accounts table: %v", err)
	}

	// Create contacts table
	_, err = db.Exec(`CREATE TABLE contacts (
		id              TEXT PRIMARY KEY,
		account_id      TEXT NOT NULL REFERENCES accounts(id),
		first_name      TEXT NOT NULL,
		last_name       TEXT NOT NULL,
		email           TEXT NOT NULL,
		phone           TEXT,
		title           TEXT,
		department      TEXT,
		is_primary      INTEGER NOT NULL DEFAULT 0,
		created_at      TEXT NOT NULL
	)`)
	if err != nil {
		db.Close()
		t.Fatalf("create contacts table: %v", err)
	}

	return db, func() { db.Close() }
}

// insertTestAccounts inserts test accounts and returns their IDs.
func insertTestAccounts(t *testing.T, db *sql.DB, accounts []Account) {
	t.Helper()
	stmt, err := db.Prepare(`INSERT INTO accounts
		(id, name, industry, type, website, phone, billing_city, billing_state, annual_revenue, num_employees, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		t.Fatalf("prepare statement: %v", err)
	}
	defer stmt.Close()

	for _, a := range accounts {
		_, err := stmt.Exec(a.ID, a.Name, a.Industry, a.Type, a.Website, a.Phone,
			a.BillingCity, a.BillingState, a.AnnualRevenue, a.NumEmployees, a.CreatedAt)
		if err != nil {
			t.Fatalf("insert account: %v", err)
		}
	}
}

func TestContactGenerator_Generate(t *testing.T) {
	db, cleanup := contactTestDB(t)
	defer cleanup()

	// Insert test accounts first
	testAccounts := []Account{
		{ID: "001abc123456789AAV", Name: "TechFlow Solutions", Industry: "Technology", Type: "Enterprise",
			Website: "https://techflow.example.com", Phone: "555-0100", BillingCity: "San Francisco",
			BillingState: "CA", AnnualRevenue: 50000000, NumEmployees: 500, CreatedAt: "2024-01-15T10:00:00Z"},
		{ID: "001def456789012AAV", Name: "HealthCore Analytics", Industry: "Healthcare", Type: "Mid-Market",
			Website: "https://healthcore.example.com", Phone: "555-0200", BillingCity: "Boston",
			BillingState: "MA", AnnualRevenue: 25000000, NumEmployees: 200, CreatedAt: "2024-02-20T14:30:00Z"},
	}
	insertTestAccounts(t, db, testAccounts)

	// Use dynamic mock LLM that returns unique contact data for each call
	ctx := &Context{
		DB:     db,
		LLM:    &dynamicContactMockLLM{},
		Logger: zerolog.Nop(),
	}

	gen := NewContactGenerator(ctx)
	if err := gen.Generate(3); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify contacts were stored
	rows, err := db.Query("SELECT id, account_id, first_name, last_name, email, title, is_primary FROM contacts ORDER BY account_id, is_primary DESC")
	if err != nil {
		t.Fatalf("query contacts: %v", err)
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var c Contact
		var isPrimary int
		if err := rows.Scan(&c.ID, &c.AccountID, &c.FirstName, &c.LastName, &c.Email, &c.Title, &isPrimary); err != nil {
			t.Fatalf("scan contact: %v", err)
		}
		c.IsPrimary = isPrimary == 1
		contacts = append(contacts, c)
	}

	// 2 accounts × 3 contacts each = 6 contacts
	if len(contacts) != 6 {
		t.Errorf("expected 6 contacts, got %d", len(contacts))
	}

	// Verify Salesforce ID format: 18 chars starting with "003"
	sfIDPattern := regexp.MustCompile(`^003[a-f0-9]{12}AAV$`)

	// Track primary contacts per account
	primaryByAccount := make(map[string]int)

	for _, c := range contacts {
		// Check ID format
		if len(c.ID) != 18 {
			t.Errorf("contact %s: ID length = %d, want 18", c.Email, len(c.ID))
		}
		if !sfIDPattern.MatchString(c.ID) {
			t.Errorf("contact %s: ID %q does not match Salesforce format", c.Email, c.ID)
		}

		// Count primary contacts per account
		if c.IsPrimary {
			primaryByAccount[c.AccountID]++
		}

		// Verify contact has valid account reference
		var exists int
		err := db.QueryRow("SELECT 1 FROM accounts WHERE id = ?", c.AccountID).Scan(&exists)
		if err != nil {
			t.Errorf("contact %s has invalid account_id %s: %v", c.Email, c.AccountID, err)
		}
	}

	// Verify exactly one primary contact per account
	for _, acct := range testAccounts {
		if primaryByAccount[acct.ID] != 1 {
			t.Errorf("account %s: expected 1 primary contact, got %d", acct.Name, primaryByAccount[acct.ID])
		}
	}

	t.Logf("Generated %d contacts across %d accounts", len(contacts), len(testAccounts))
	for _, c := range contacts {
		primary := ""
		if c.IsPrimary {
			primary = " (PRIMARY)"
		}
		t.Logf("  - %s %s (%s)%s", c.FirstName, c.LastName, c.Title, primary)
	}
}

func TestSalesforceID_ContactFormat(t *testing.T) {
	// Verify SalesforceID generates correct format for Contact
	id := SalesforceID("Contact")

	// Must be 18 characters
	if len(id) != 18 {
		t.Errorf("SalesforceID length = %d, want 18", len(id))
	}

	// Must start with "003" (Contact prefix)
	if id[:3] != "003" {
		t.Errorf("SalesforceID prefix = %q, want %q", id[:3], "003")
	}

	// Must end with "AAV"
	if id[15:] != "AAV" {
		t.Errorf("SalesforceID suffix = %q, want %q", id[15:], "AAV")
	}

	// Middle 12 chars must be hex
	hexPattern := regexp.MustCompile(`^[a-f0-9]{12}$`)
	if !hexPattern.MatchString(id[3:15]) {
		t.Errorf("SalesforceID middle = %q, want 12 hex chars", id[3:15])
	}
}

func TestContactGenerator_NoAccounts(t *testing.T) {
	db, cleanup := contactTestDB(t)
	defer cleanup()

	// Don't insert any accounts - test that generator handles empty case
	ctx := &Context{
		DB:     db,
		LLM:    &mockLLM{response: "[]"},
		Logger: zerolog.Nop(),
	}

	gen := NewContactGenerator(ctx)
	err := gen.Generate(3)
	if err != nil {
		t.Fatalf("Generate() with no accounts should not error, got: %v", err)
	}

	// Verify no contacts were created
	var count int
	db.QueryRow("SELECT COUNT(*) FROM contacts").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 contacts with no accounts, got %d", count)
	}
}

func TestDomainFromName(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"TechFlow Solutions", "techflowsolutions.example.com"},
		{"Health-Core Analytics", "healthcoreanalytics.example.com"},
		{"ACME Corp", "acmecorp.example.com"},
		{"ABC 123 Company", "abccompany.example.com"},
	}

	for _, tt := range tests {
		got := domainFromName(tt.name)
		if got != tt.expected {
			t.Errorf("domainFromName(%q) = %q, want %q", tt.name, got, tt.expected)
		}
	}
}

