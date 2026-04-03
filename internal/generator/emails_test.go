package generator

import (
	"database/sql"
	"regexp"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
)

// emailTestDB creates an in-memory SQLite database with required tables for email tests.
func emailTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	// Create all required tables
	tables := []string{
		`CREATE TABLE accounts (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, industry TEXT NOT NULL,
			type TEXT NOT NULL, website TEXT, phone TEXT, billing_city TEXT,
			billing_state TEXT, annual_revenue REAL, num_employees INTEGER, created_at TEXT NOT NULL
		)`,
		`CREATE TABLE contacts (
			id TEXT PRIMARY KEY, account_id TEXT NOT NULL REFERENCES accounts(id),
			first_name TEXT NOT NULL, last_name TEXT NOT NULL, email TEXT NOT NULL,
			phone TEXT, title TEXT, department TEXT, is_primary INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL
		)`,
		`CREATE TABLE users (
			id TEXT PRIMARY KEY, first_name TEXT NOT NULL, last_name TEXT NOT NULL,
			email TEXT NOT NULL, username TEXT NOT NULL, title TEXT, department TEXT,
			is_active INTEGER NOT NULL DEFAULT 1, manager_id TEXT REFERENCES users(id),
			user_role TEXT, created_at TEXT NOT NULL
		)`,
		`CREATE TABLE cases (
			id TEXT PRIMARY KEY, case_number TEXT NOT NULL UNIQUE, subject TEXT NOT NULL,
			description TEXT NOT NULL, status TEXT NOT NULL, priority TEXT NOT NULL,
			product TEXT, case_type TEXT, origin TEXT, reason TEXT,
			owner_id TEXT NOT NULL REFERENCES users(id), contact_id TEXT NOT NULL REFERENCES contacts(id),
			account_id TEXT NOT NULL REFERENCES accounts(id), created_at TEXT NOT NULL,
			closed_at TEXT, is_closed INTEGER NOT NULL DEFAULT 0, is_escalated INTEGER NOT NULL DEFAULT 0, jira_issue_key TEXT
		)`,
		`CREATE TABLE email_messages (
			id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES cases(id),
			subject TEXT NOT NULL, text_body TEXT NOT NULL, html_body TEXT,
			from_address TEXT NOT NULL, from_name TEXT, to_address TEXT NOT NULL,
			cc_address TEXT, bcc_address TEXT, message_date TEXT NOT NULL,
			status TEXT NOT NULL, incoming INTEGER NOT NULL, has_attachment INTEGER NOT NULL DEFAULT 0,
			headers TEXT, sequence_num INTEGER NOT NULL
		)`,
	}

	for _, tbl := range tables {
		if _, err := db.Exec(tbl); err != nil {
			db.Close()
			t.Fatalf("create table: %v", err)
		}
	}

	return db, func() { db.Close() }
}

// insertEmailTestData inserts test accounts, contacts, users, and cases.
func insertEmailTestData(t *testing.T, db *sql.DB) {
	t.Helper()

	// Insert account
	_, err := db.Exec(`INSERT INTO accounts (id, name, industry, type, created_at)
		VALUES ('001abc123456789AAV', 'TechFlow Solutions', 'Technology', 'Enterprise', '2024-01-15T10:00:00Z')`)
	if err != nil {
		t.Fatalf("insert account: %v", err)
	}

	// Insert contact
	_, err = db.Exec(`INSERT INTO contacts (id, account_id, first_name, last_name, email, title, is_primary, created_at)
		VALUES ('003abc123456789AAV', '001abc123456789AAV', 'John', 'Smith', 'john.smith@techflow.com', 'CTO', 1, '2024-03-01T09:00:00Z')`)
	if err != nil {
		t.Fatalf("insert contact: %v", err)
	}

	// Insert user (support agent)
	_, err = db.Exec(`INSERT INTO users (id, first_name, last_name, email, username, title, is_active, user_role, created_at)
		VALUES ('005abc123456789AAV', 'Sarah', 'Chen', 'sarah.chen@acme.com', 'schen', 'L2 Support Engineer', 1, 'L2 Support', '2023-01-15T09:00:00Z')`)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	// Insert case
	_, err = db.Exec(`INSERT INTO cases (id, case_number, subject, description, status, priority, owner_id, contact_id, account_id, created_at)
		VALUES ('500abc123456789AAV', '00123457', 'API Integration Failing', 'Our API integration returns 500 errors after update.', 'Working', 'High', '005abc123456789AAV', '003abc123456789AAV', '001abc123456789AAV', '2024-03-15T10:00:00Z')`)
	if err != nil {
		t.Fatalf("insert case: %v", err)
	}
}

func TestEmailGenerator_Generate(t *testing.T) {
	db, cleanup := emailTestDB(t)
	defer cleanup()

	insertEmailTestData(t, db)

	// Mock LLM response with a coherent email thread
	mockResp := `[
		{"subject":"API Integration Failing","text_body":"Hello, I'm experiencing 500 errors with our API integration after the recent update.","from_address":"john.smith@techflow.com","to_address":"sarah.chen@acme.com","message_date":"2024-03-15T10:30:00Z","incoming":true},
		{"subject":"Re: API Integration Failing","text_body":"Thank you for reaching out. I'll investigate the issue right away.","from_address":"sarah.chen@acme.com","to_address":"john.smith@techflow.com","message_date":"2024-03-15T14:00:00Z","incoming":false},
		{"subject":"Re: API Integration Failing","text_body":"I found the issue - it's related to the new authentication headers.","from_address":"sarah.chen@acme.com","to_address":"john.smith@techflow.com","message_date":"2024-03-15T16:30:00Z","incoming":false}
	]`

	ctx := &Context{
		DB:     db,
		LLM:    &mockLLM{response: mockResp},
		Logger: zerolog.Nop(),
	}

	gen := NewEmailGenerator(ctx)
	if err := gen.Generate(); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify emails were stored
	rows, err := db.Query(`SELECT id, case_id, subject, from_address, to_address, message_date, incoming, sequence_num 
		FROM email_messages ORDER BY sequence_num`)
	if err != nil {
		t.Fatalf("query emails: %v", err)
	}
	defer rows.Close()

	var emails []EmailMessage
	for rows.Next() {
		var e EmailMessage
		var incoming int
		if err := rows.Scan(&e.ID, &e.CaseID, &e.Subject, &e.FromAddress, &e.ToAddress, &e.MessageDate, &incoming, &e.SequenceNum); err != nil {
			t.Fatalf("scan email: %v", err)
		}
		e.Incoming = incoming == 1
		emails = append(emails, e)
	}

	if len(emails) != 3 {
		t.Errorf("expected 3 emails, got %d", len(emails))
	}

	// Verify Salesforce ID format: 18 chars starting with "02s"
	sfIDPattern := regexp.MustCompile(`^02s[a-f0-9]{12}AAV$`)

	var prevMsgDate time.Time
	for i, e := range emails {
		// Check ID format
		if len(e.ID) != 18 {
			t.Errorf("email %d: ID length = %d, want 18", i, len(e.ID))
		}
		if !sfIDPattern.MatchString(e.ID) {
			t.Errorf("email %d: ID %q does not match Salesforce EmailMessage format", i, e.ID)
		}

		// Check sequence_num ordering
		if e.SequenceNum != i+1 {
			t.Errorf("email %d: sequence_num = %d, want %d", i, e.SequenceNum, i+1)
		}

		// Check chronological ordering
		msgDate, err := time.Parse(time.RFC3339, e.MessageDate)
		if err != nil {
			t.Errorf("email %d: invalid message_date: %v", i, err)
			continue
		}
		if i > 0 && !msgDate.After(prevMsgDate) {
			t.Errorf("email %d: message_date %v not after previous %v", i, msgDate, prevMsgDate)
		}
		prevMsgDate = msgDate

		// Verify case reference
		if e.CaseID != "500abc123456789AAV" {
			t.Errorf("email %d: case_id = %q, want %q", i, e.CaseID, "500abc123456789AAV")
		}
	}

	// Verify first email is incoming (from contact)
	if !emails[0].Incoming {
		t.Error("first email should be incoming (from contact)")
	}

	t.Logf("Generated %d emails for case", len(emails))
	for _, e := range emails {
		t.Logf("  [%d] %s -> %s (incoming=%v)", e.SequenceNum, e.FromAddress, e.ToAddress, e.Incoming)
	}
}

func TestEmailGenerator_NoCases(t *testing.T) {
	db, cleanup := emailTestDB(t)
	defer cleanup()

	// No test data - should handle gracefully
	ctx := &Context{
		DB:     db,
		LLM:    &mockLLM{response: "[]"},
		Logger: zerolog.Nop(),
	}

	gen := NewEmailGenerator(ctx)
	err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() with no cases should not error, got: %v", err)
	}

	// Verify no emails were created
	var count int
	db.QueryRow("SELECT COUNT(*) FROM email_messages").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 emails with no cases, got %d", count)
	}
}

func TestEmailGenerator_FallbackOnLLMError(t *testing.T) {
	db, cleanup := emailTestDB(t)
	defer cleanup()

	insertEmailTestData(t, db)

	// Mock LLM that returns an error
	ctx := &Context{
		DB:     db,
		LLM:    &mockLLM{err: errLLMFailed},
		Logger: zerolog.Nop(),
	}

	gen := NewEmailGenerator(ctx)
	if err := gen.Generate(); err != nil {
		t.Fatalf("Generate() should succeed with fallback, got: %v", err)
	}

	// Verify emails were created using fallback
	var count int
	db.QueryRow("SELECT COUNT(*) FROM email_messages").Scan(&count)
	if count == 0 {
		t.Error("expected fallback emails to be created")
	}

	t.Logf("Created %d fallback emails", count)
}

func TestEmailGenerator_ThreadLengthDistribution(t *testing.T) {
	gen := &EmailGenerator{}

	// Sample thread lengths
	counts := map[string]int{"short": 0, "medium": 0, "long": 0}
	for i := 0; i < 1000; i++ {
		length := gen.pickThreadLength()
		switch {
		case length >= 2 && length <= 3:
			counts["short"]++
		case length >= 4 && length <= 6:
			counts["medium"]++
		case length >= 7 && length <= 10:
			counts["long"]++
		default:
			t.Errorf("unexpected thread length: %d", length)
		}
	}

	// Verify approximate distribution (with some tolerance)
	// 40% short, 40% medium, 20% long
	if counts["short"] < 300 || counts["short"] > 500 {
		t.Errorf("short distribution out of range: %d (expected ~400)", counts["short"])
	}
	if counts["medium"] < 300 || counts["medium"] > 500 {
		t.Errorf("medium distribution out of range: %d (expected ~400)", counts["medium"])
	}
	if counts["long"] < 100 || counts["long"] > 300 {
		t.Errorf("long distribution out of range: %d (expected ~200)", counts["long"])
	}

	t.Logf("Thread length distribution: short=%d, medium=%d, long=%d", counts["short"], counts["medium"], counts["long"])
}

func TestSalesforceID_EmailMessageFormat(t *testing.T) {
	id := SalesforceID("EmailMessage")

	if len(id) != 18 {
		t.Errorf("SalesforceID length = %d, want 18", len(id))
	}

	if id[:3] != "02s" {
		t.Errorf("SalesforceID prefix = %q, want %q", id[:3], "02s")
	}

	if id[15:] != "AAV" {
		t.Errorf("SalesforceID suffix = %q, want %q", id[15:], "AAV")
	}

	hexPattern := regexp.MustCompile(`^[a-f0-9]{12}$`)
	if !hexPattern.MatchString(id[3:15]) {
		t.Errorf("SalesforceID middle = %q, want 12 hex chars", id[3:15])
	}
}

// errLLMFailed is a test error for LLM failures.
var errLLMFailed = &llmError{msg: "LLM service unavailable"}

type llmError struct {
	msg string
}

func (e *llmError) Error() string {
	return e.msg
}

