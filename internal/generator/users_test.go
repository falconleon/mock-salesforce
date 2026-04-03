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

// dynamicUserMockLLM returns different responses for each call.
type dynamicUserMockLLM struct {
	callCount atomic.Int32
}

func (m *dynamicUserMockLLM) Generate(prompt string) (string, error) {
	count := m.callCount.Add(1)
	// Return a unique user data for each call (name comes from seed, not LLM)
	year := "2022"
	if count > 2 {
		year = "2023"
	}
	return fmt.Sprintf(`{"title":"Test Title %d","created_at":"%s-0%d-15T09:00:00Z"}`,
		count, year, min(int(count), 9)), nil
}

// testUserDB creates an in-memory SQLite database with users and jira_users tables.
func testUserDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	// Create users table
	_, err = db.Exec(`CREATE TABLE users (
		id              TEXT PRIMARY KEY,
		first_name      TEXT NOT NULL,
		last_name       TEXT NOT NULL,
		email           TEXT NOT NULL,
		username        TEXT NOT NULL,
		title           TEXT,
		department      TEXT,
		is_active       INTEGER NOT NULL DEFAULT 1,
		manager_id      TEXT REFERENCES users(id),
		user_role       TEXT,
		created_at      TEXT NOT NULL
	)`)
	if err != nil {
		db.Close()
		t.Fatalf("create users table: %v", err)
	}

	// Create jira_users table
	_, err = db.Exec(`CREATE TABLE jira_users (
		account_id      TEXT PRIMARY KEY,
		display_name    TEXT NOT NULL,
		email           TEXT,
		account_type    TEXT NOT NULL DEFAULT 'atlassian',
		active          INTEGER NOT NULL DEFAULT 1,
		sf_user_id      TEXT REFERENCES users(id)
	)`)
	if err != nil {
		db.Close()
		t.Fatalf("create jira_users table: %v", err)
	}

	return db, func() { db.Close() }
}

func TestUserGenerator_Generate(t *testing.T) {
	db, cleanup := testUserDB(t)
	defer cleanup()

	// Use dynamic mock LLM that returns unique user data for each call
	ctx := &Context{
		DB:     db,
		LLM:    &dynamicUserMockLLM{},
		Logger: zerolog.Nop(),
	}

	gen := NewUserGenerator(ctx)
	if err := gen.Generate(5); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify users were stored
	var userCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if userCount != 5 {
		t.Errorf("expected 5 users, got %d", userCount)
	}

	// Verify jira_users were created
	var jiraCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM jira_users").Scan(&jiraCount); err != nil {
		t.Fatalf("count jira_users: %v", err)
	}
	if jiraCount != 5 {
		t.Errorf("expected 5 jira_users, got %d", jiraCount)
	}

	// Verify role distribution
	rows, err := db.Query("SELECT user_role, COUNT(*) FROM users GROUP BY user_role")
	if err != nil {
		t.Fatalf("query role distribution: %v", err)
	}
	defer rows.Close()

	t.Log("Role distribution:")
	for rows.Next() {
		var role string
		var count int
		if err := rows.Scan(&role, &count); err != nil {
			t.Fatalf("scan role: %v", err)
		}
		t.Logf("  %s: %d", role, count)
	}
}

func TestUserGenerator_SalesforceIDFormat(t *testing.T) {
	// Verify SalesforceID generates correct format for User
	id := SalesforceID("User")

	if len(id) != 18 {
		t.Errorf("SalesforceID length = %d, want 18", len(id))
	}

	// Must start with "005" (User prefix)
	if id[:3] != "005" {
		t.Errorf("SalesforceID prefix = %q, want %q", id[:3], "005")
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

func TestUserGenerator_ManagerHierarchy(t *testing.T) {
	db, cleanup := testUserDB(t)
	defer cleanup()

	// Use dynamic mock LLM
	ctx := &Context{
		DB:     db,
		LLM:    &dynamicUserMockLLM{},
		Logger: zerolog.Nop(),
	}

	gen := NewUserGenerator(ctx)
	if err := gen.Generate(3); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify manager has no manager_id (NULL)
	var managerManagerID sql.NullString
	err := db.QueryRow("SELECT manager_id FROM users WHERE user_role = 'Manager'").Scan(&managerManagerID)
	if err != nil {
		t.Fatalf("query manager: %v", err)
	}
	if managerManagerID.Valid {
		t.Error("Manager should not have a manager_id")
	}

	// Verify agents have manager_id set
	rows, err := db.Query("SELECT first_name, manager_id FROM users WHERE user_role != 'Manager'")
	if err != nil {
		t.Fatalf("query agents: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var firstName string
		var managerID sql.NullString
		if err := rows.Scan(&firstName, &managerID); err != nil {
			t.Fatalf("scan agent: %v", err)
		}
		if !managerID.Valid || managerID.String == "" {
			t.Errorf("Agent %s should have a manager_id", firstName)
		}
	}
}

func TestUserGenerator_JiraUserLinking(t *testing.T) {
	db, cleanup := testUserDB(t)
	defer cleanup()

	// Use dynamic mock LLM
	ctx := &Context{
		DB:     db,
		LLM:    &dynamicUserMockLLM{},
		Logger: zerolog.Nop(),
	}

	gen := NewUserGenerator(ctx)
	if err := gen.Generate(1); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Get the Salesforce user ID and name
	var sfUserID, firstName, lastName, userEmail string
	if err := db.QueryRow("SELECT id, first_name, last_name, email FROM users").Scan(&sfUserID, &firstName, &lastName, &userEmail); err != nil {
		t.Fatalf("query sf user: %v", err)
	}

	// Verify Jira user is linked to SF user
	var jiraAccountID, linkedSFUserID, displayName, email string
	err := db.QueryRow("SELECT account_id, sf_user_id, display_name, email FROM jira_users").Scan(
		&jiraAccountID, &linkedSFUserID, &displayName, &email)
	if err != nil {
		t.Fatalf("query jira user: %v", err)
	}

	if linkedSFUserID != sfUserID {
		t.Errorf("Jira user sf_user_id = %q, want %q", linkedSFUserID, sfUserID)
	}
	expectedDisplayName := firstName + " " + lastName
	if displayName != expectedDisplayName {
		t.Errorf("Jira user display_name = %q, want %q", displayName, expectedDisplayName)
	}
	if email != userEmail {
		t.Errorf("Jira user email = %q, want %q", email, userEmail)
	}

	// Verify Jira account ID format (24-char hex)
	hexPattern := regexp.MustCompile(`^[a-f0-9]{24}$`)
	if !hexPattern.MatchString(jiraAccountID) {
		t.Errorf("Jira account_id %q does not match 24-char hex format", jiraAccountID)
	}

	t.Logf("SF User ID: %s", sfUserID)
	t.Logf("Jira Account ID: %s -> linked to SF: %s", jiraAccountID, linkedSFUserID)
}

func TestDefaultUserConfig(t *testing.T) {
	cfg := DefaultUserConfig(10)

	if cfg.Count != 10 {
		t.Errorf("Count = %d, want 10", cfg.Count)
	}

	// Verify role distribution adds up to ~100%
	total := cfg.RoleDistribution["agent"] + cfg.RoleDistribution["engineer"] + cfg.RoleDistribution["manager"]
	if total < 0.99 || total > 1.01 {
		t.Errorf("Role distribution total = %.2f, want ~1.0", total)
	}

	// Verify default percentages
	if cfg.RoleDistribution["agent"] != 0.60 {
		t.Errorf("agent distribution = %.2f, want 0.60", cfg.RoleDistribution["agent"])
	}
	if cfg.RoleDistribution["engineer"] != 0.25 {
		t.Errorf("engineer distribution = %.2f, want 0.25", cfg.RoleDistribution["engineer"])
	}
	if cfg.RoleDistribution["manager"] != 0.15 {
		t.Errorf("manager distribution = %.2f, want 0.15", cfg.RoleDistribution["manager"])
	}
}
