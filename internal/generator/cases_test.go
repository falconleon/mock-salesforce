package generator

import (
	"database/sql"
	"regexp"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
)

// caseTestDB creates an in-memory SQLite database with required tables.
func caseTestDB(t *testing.T) (*sql.DB, func()) {
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

	// Create cases table
	_, err = db.Exec(`CREATE TABLE cases (
		id              TEXT PRIMARY KEY,
		case_number     TEXT NOT NULL UNIQUE,
		subject         TEXT NOT NULL,
		description     TEXT NOT NULL,
		status          TEXT NOT NULL,
		priority        TEXT NOT NULL,
		product         TEXT,
		case_type       TEXT,
		origin          TEXT,
		reason          TEXT,
		owner_id        TEXT NOT NULL REFERENCES users(id),
		contact_id      TEXT NOT NULL REFERENCES contacts(id),
		account_id      TEXT NOT NULL REFERENCES accounts(id),
		created_at      TEXT NOT NULL,
		closed_at       TEXT,
		is_closed       INTEGER NOT NULL DEFAULT 0,
		is_escalated    INTEGER NOT NULL DEFAULT 0,
		jira_issue_key  TEXT
	)`)
	if err != nil {
		db.Close()
		t.Fatalf("create cases table: %v", err)
	}

	return db, func() { db.Close() }
}

// insertCaseTestData inserts test accounts, contacts, and users.
func insertCaseTestData(t *testing.T, db *sql.DB) {
	t.Helper()

	// Insert test accounts
	_, err := db.Exec(`INSERT INTO accounts (id, name, industry, type, website, phone, billing_city, billing_state, annual_revenue, num_employees, created_at)
		VALUES 
		('001abc123456789AAV', 'TechFlow Solutions', 'Technology', 'Enterprise', 'https://techflow.example.com', '555-0100', 'San Francisco', 'CA', 50000000, 500, '2024-01-15T10:00:00Z'),
		('001def456789012AAV', 'HealthCore Analytics', 'Healthcare', 'Mid-Market', 'https://healthcore.example.com', '555-0200', 'Boston', 'MA', 25000000, 200, '2024-02-20T14:30:00Z')`)
	if err != nil {
		t.Fatalf("insert accounts: %v", err)
	}

	// Insert test contacts
	_, err = db.Exec(`INSERT INTO contacts (id, account_id, first_name, last_name, email, phone, title, department, is_primary, created_at)
		VALUES 
		('003abc123456789AAV', '001abc123456789AAV', 'John', 'Smith', 'john.smith@techflow.com', '555-0101', 'CTO', 'Technology', 1, '2024-03-01T09:00:00Z'),
		('003def456789012AAV', '001abc123456789AAV', 'Jane', 'Doe', 'jane.doe@techflow.com', '555-0102', 'IT Director', 'IT', 0, '2024-03-15T10:00:00Z'),
		('003ghi789012345AAV', '001def456789012AAV', 'Bob', 'Wilson', 'bob.wilson@healthcore.com', '555-0201', 'VP Engineering', 'Engineering', 1, '2024-04-01T11:00:00Z')`)
	if err != nil {
		t.Fatalf("insert contacts: %v", err)
	}

	// Insert test users (support agents)
	_, err = db.Exec(`INSERT INTO users (id, first_name, last_name, email, username, title, department, is_active, user_role, created_at)
		VALUES 
		('005abc123456789AAV', 'Sarah', 'Chen', 'sarah.chen@acme.com', 'schen', 'L2 Support Engineer', 'Support', 1, 'L2 Support', '2023-01-15T09:00:00Z'),
		('005def456789012AAV', 'Mike', 'Johnson', 'mike.johnson@acme.com', 'mjohnson', 'L1 Support Agent', 'Support', 1, 'L1 Support', '2023-03-01T09:00:00Z'),
		('005ghi789012345AAV', 'Emily', 'Brown', 'emily.brown@acme.com', 'ebrown', 'L3 Support Engineer', 'Support', 1, 'L3 Support', '2022-06-15T09:00:00Z')`)
	if err != nil {
		t.Fatalf("insert users: %v", err)
	}
}

func TestCaseGenerator_Generate(t *testing.T) {
	db, cleanup := caseTestDB(t)
	defer cleanup()

	insertCaseTestData(t, db)

	// Mock LLM response for case generation
	mockResp := `{"subject":"API Integration Failing After Update","description":"After updating to version 3.2, our API integration is returning 500 errors. This is affecting our production environment and is urgent.\n\nSteps to reproduce:\n1. Call the /api/v2/data endpoint\n2. Observe 500 error response\n\nExpected: 200 OK\nActual: 500 Internal Server Error","case_type":"Technical Issue","origin":"Email","reason":"Software Defect","product":"Integration Hub"}`

	ctx := &Context{
		DB:     db,
		LLM:    &mockLLM{response: mockResp},
		Logger: zerolog.Nop(),
	}

	gen := NewCaseGenerator(ctx)
	if err := gen.Generate(10); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify cases were stored
	rows, err := db.Query(`SELECT id, case_number, subject, status, priority, owner_id, contact_id, account_id, is_closed, is_escalated FROM cases`)
	if err != nil {
		t.Fatalf("query cases: %v", err)
	}
	defer rows.Close()

	var cases []Case
	for rows.Next() {
		var c Case
		var isClosed, isEscalated int
		if err := rows.Scan(&c.ID, &c.CaseNumber, &c.Subject, &c.Status, &c.Priority, &c.OwnerID, &c.ContactID, &c.AccountID, &isClosed, &isEscalated); err != nil {
			t.Fatalf("scan case: %v", err)
		}
		c.IsClosed = isClosed == 1
		c.IsEscalated = isEscalated == 1
		cases = append(cases, c)
	}

	if len(cases) != 10 {
		t.Errorf("expected 10 cases, got %d", len(cases))
	}

	// Verify Salesforce ID format: 18 chars starting with "500"
	sfIDPattern := regexp.MustCompile(`^500[a-f0-9]{12}AAV$`)
	caseNumPattern := regexp.MustCompile(`^\d{8}$`)

	statusCounts := make(map[string]int)
	priorityCounts := make(map[string]int)

	for _, c := range cases {
		// Check ID format
		if len(c.ID) != 18 {
			t.Errorf("case %s: ID length = %d, want 18", c.CaseNumber, len(c.ID))
		}
		if !sfIDPattern.MatchString(c.ID) {
			t.Errorf("case %s: ID %q does not match Salesforce format", c.CaseNumber, c.ID)
		}

		// Check case number format
		if !caseNumPattern.MatchString(c.CaseNumber) {
			t.Errorf("case %s: case_number does not match pattern", c.CaseNumber)
		}

		// Verify foreign key references
		var exists int
		if err := db.QueryRow("SELECT 1 FROM accounts WHERE id = ?", c.AccountID).Scan(&exists); err != nil {
			t.Errorf("case %s: invalid account_id %s", c.CaseNumber, c.AccountID)
		}
		if err := db.QueryRow("SELECT 1 FROM contacts WHERE id = ?", c.ContactID).Scan(&exists); err != nil {
			t.Errorf("case %s: invalid contact_id %s", c.CaseNumber, c.ContactID)
		}
		if err := db.QueryRow("SELECT 1 FROM users WHERE id = ?", c.OwnerID).Scan(&exists); err != nil {
			t.Errorf("case %s: invalid owner_id %s", c.CaseNumber, c.OwnerID)
		}

		// Verify status/closed flag consistency
		if c.Status == "Closed" && !c.IsClosed {
			t.Errorf("case %s: status=Closed but is_closed=false", c.CaseNumber)
		}
		if c.Status == "Escalated" && !c.IsEscalated {
			t.Errorf("case %s: status=Escalated but is_escalated=false", c.CaseNumber)
		}

		statusCounts[c.Status]++
		priorityCounts[c.Priority]++
	}

	// Log distribution
	t.Logf("Generated %d cases", len(cases))
	t.Logf("Status distribution: %v", statusCounts)
	t.Logf("Priority distribution: %v", priorityCounts)
}

func TestCaseGenerator_NoAccounts(t *testing.T) {
	db, cleanup := caseTestDB(t)
	defer cleanup()

	// No test data - should handle gracefully
	ctx := &Context{
		DB:     db,
		LLM:    &mockLLM{response: "{}"},
		Logger: zerolog.Nop(),
	}

	gen := NewCaseGenerator(ctx)
	err := gen.Generate(5)
	if err != nil {
		t.Fatalf("Generate() with no accounts should not error, got: %v", err)
	}

	// Verify no cases were created
	var count int
	db.QueryRow("SELECT COUNT(*) FROM cases").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 cases with no accounts, got %d", count)
	}
}

func TestCaseGenerator_NoUsers(t *testing.T) {
	db, cleanup := caseTestDB(t)
	defer cleanup()

	// Insert accounts and contacts but no users
	_, err := db.Exec(`INSERT INTO accounts (id, name, industry, type, created_at)
		VALUES ('001abc123456789AAV', 'TechFlow Solutions', 'Technology', 'Enterprise', '2024-01-15T10:00:00Z')`)
	if err != nil {
		t.Fatalf("insert account: %v", err)
	}
	_, err = db.Exec(`INSERT INTO contacts (id, account_id, first_name, last_name, email, is_primary, created_at)
		VALUES ('003abc123456789AAV', '001abc123456789AAV', 'John', 'Smith', 'john@test.com', 1, '2024-03-01T09:00:00Z')`)
	if err != nil {
		t.Fatalf("insert contact: %v", err)
	}

	ctx := &Context{
		DB:     db,
		LLM:    &mockLLM{response: "{}"},
		Logger: zerolog.Nop(),
	}

	gen := NewCaseGenerator(ctx)
	err = gen.Generate(5)
	if err != nil {
		t.Fatalf("Generate() with no users should not error, got: %v", err)
	}

	// Verify no cases were created
	var count int
	db.QueryRow("SELECT COUNT(*) FROM cases").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 cases with no users, got %d", count)
	}
}

func TestCaseGenerator_StatusDistribution(t *testing.T) {
	db, cleanup := caseTestDB(t)
	defer cleanup()

	insertCaseTestData(t, db)

	mockResp := `{"subject":"Test Case","description":"Test description","case_type":"Technical Issue","origin":"Email","reason":"Test","product":"Test Product"}`

	ctx := &Context{
		DB:     db,
		LLM:    &mockLLM{response: mockResp},
		Logger: zerolog.Nop(),
	}

	gen := NewCaseGenerator(ctx)
	// Generate 100 cases for better distribution testing
	cfg := DefaultCaseConfig(100)
	if err := gen.GenerateWithConfig(cfg); err != nil {
		t.Fatalf("GenerateWithConfig() error = %v", err)
	}

	// Query status distribution
	rows, err := db.Query("SELECT status, COUNT(*) FROM cases GROUP BY status")
	if err != nil {
		t.Fatalf("query status distribution: %v", err)
	}
	defer rows.Close()

	statusCounts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			t.Fatalf("scan: %v", err)
		}
		statusCounts[status] = count
	}

	// Verify all expected statuses are present
	expectedStatuses := []string{"New", "Working", "Escalated", "Resolved", "Closed"}
	for _, s := range expectedStatuses {
		if _, ok := statusCounts[s]; !ok {
			t.Errorf("expected status %q not found in distribution", s)
		}
	}

	t.Logf("Status distribution for 100 cases: %v", statusCounts)
}

func TestSalesforceID_CaseFormat(t *testing.T) {
	id := SalesforceID("Case")

	if len(id) != 18 {
		t.Errorf("SalesforceID length = %d, want 18", len(id))
	}

	if id[:3] != "500" {
		t.Errorf("SalesforceID prefix = %q, want %q", id[:3], "500")
	}

	if id[15:] != "AAV" {
		t.Errorf("SalesforceID suffix = %q, want %q", id[15:], "AAV")
	}

	hexPattern := regexp.MustCompile(`^[a-f0-9]{12}$`)
	if !hexPattern.MatchString(id[3:15]) {
		t.Errorf("SalesforceID middle = %q, want 12 hex chars", id[3:15])
	}
}

func TestNextCaseNumber(t *testing.T) {
	num1 := NextCaseNumber()
	num2 := NextCaseNumber()

	// Should be 8 digits
	if len(num1) != 8 {
		t.Errorf("CaseNumber length = %d, want 8", len(num1))
	}

	// Should be sequential
	pattern := regexp.MustCompile(`^\d{8}$`)
	if !pattern.MatchString(num1) {
		t.Errorf("CaseNumber %q does not match pattern", num1)
	}
	if !pattern.MatchString(num2) {
		t.Errorf("CaseNumber %q does not match pattern", num2)
	}

	// num2 should be greater than num1
	if num2 <= num1 {
		t.Errorf("CaseNumbers not sequential: %s, %s", num1, num2)
	}
}

