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

// mockLLM implements the LLM interface for testing.
type mockLLM struct {
	response string
	err      error
}

func (m *mockLLM) Generate(prompt string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

// dynamicAccountMockLLM returns different responses for each call.
type dynamicAccountMockLLM struct {
	callCount atomic.Int32
}

func (m *dynamicAccountMockLLM) Generate(prompt string) (string, error) {
	count := m.callCount.Add(1)
	// Return a unique account for each call
	return fmt.Sprintf(`{"name":"TestCompany%d Inc","website":"https://testcompany%d.example.com","phone":"555-010%d","annual_revenue":%d,"created_at":"2023-0%d-15T10:00:00Z"}`,
		count, count, count, 10000000*int(count), min(int(count), 9)), nil
}

// testDB creates an in-memory SQLite database with the accounts table.
func testDB(t *testing.T) (*sql.DB, func()) {
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

	return db, func() { db.Close() }
}

func TestAccountGenerator_Generate(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	// Use dynamic mock LLM that returns a unique account for each call
	ctx := &Context{
		DB:     db,
		LLM:    &dynamicAccountMockLLM{},
		Logger: zerolog.Nop(),
	}

	gen := NewAccountGenerator(ctx)
	if err := gen.Generate(3); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify accounts were stored
	rows, err := db.Query("SELECT id, name, industry, type, billing_city, billing_state, annual_revenue, num_employees FROM accounts")
	if err != nil {
		t.Fatalf("query accounts: %v", err)
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.Name, &a.Industry, &a.Type, &a.BillingCity, &a.BillingState, &a.AnnualRevenue, &a.NumEmployees); err != nil {
			t.Fatalf("scan account: %v", err)
		}
		accounts = append(accounts, a)
	}

	if len(accounts) != 3 {
		t.Errorf("expected 3 accounts, got %d", len(accounts))
	}

	// Verify Salesforce ID format: 18 chars starting with "001"
	sfIDPattern := regexp.MustCompile(`^001[a-f0-9]{12}AAV$`)

	for _, acct := range accounts {
		// Check ID format
		if len(acct.ID) != 18 {
			t.Errorf("account %s: ID length = %d, want 18", acct.Name, len(acct.ID))
		}
		if !sfIDPattern.MatchString(acct.ID) {
			t.Errorf("account %s: ID %q does not match Salesforce format", acct.Name, acct.ID)
		}

		// Verify data was stored correctly
		if acct.Name == "" {
			t.Error("account name is empty")
		}
		if acct.Industry == "" {
			t.Error("account industry is empty")
		}
	}

	// Log sample output for verification
	t.Logf("Generated accounts:")
	for _, a := range accounts {
		t.Logf("  - %s (%s, %s): %d employees, $%.0f revenue",
			a.Name, a.Industry, a.Type, a.NumEmployees, a.AnnualRevenue)
	}
}

func TestSalesforceID_AccountFormat(t *testing.T) {
	// Verify SalesforceID generates correct format for Account
	id := SalesforceID("Account")

	// Must be 18 characters
	if len(id) != 18 {
		t.Errorf("SalesforceID length = %d, want 18", len(id))
	}

	// Must start with "001" (Account prefix)
	if id[:3] != "001" {
		t.Errorf("SalesforceID prefix = %q, want %q", id[:3], "001")
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

