package generator

import (
	"database/sql"
	"regexp"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
)

// jiraTestDB creates an in-memory SQLite database with required tables for JIRA tests.
func jiraTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open test db: %v", err)
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
		owner_id        TEXT NOT NULL,
		contact_id      TEXT NOT NULL,
		account_id      TEXT NOT NULL,
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

	// Create jira_users table
	_, err = db.Exec(`CREATE TABLE jira_users (
		account_id      TEXT PRIMARY KEY,
		display_name    TEXT NOT NULL,
		email           TEXT,
		account_type    TEXT NOT NULL,
		active          INTEGER NOT NULL DEFAULT 1,
		sf_user_id      TEXT
	)`)
	if err != nil {
		db.Close()
		t.Fatalf("create jira_users table: %v", err)
	}

	// Create jira_issues table
	_, err = db.Exec(`CREATE TABLE jira_issues (
		id              TEXT PRIMARY KEY,
		key             TEXT NOT NULL UNIQUE,
		project_key     TEXT NOT NULL,
		summary         TEXT NOT NULL,
		description_adf TEXT NOT NULL,
		issue_type      TEXT NOT NULL,
		status          TEXT NOT NULL,
		priority        TEXT NOT NULL,
		assignee_id     TEXT,
		reporter_id     TEXT,
		created_at      TEXT NOT NULL,
		updated_at      TEXT NOT NULL,
		labels          TEXT,
		sf_case_id      TEXT
	)`)
	if err != nil {
		db.Close()
		t.Fatalf("create jira_issues table: %v", err)
	}

	// Create jira_comments table
	_, err = db.Exec(`CREATE TABLE jira_comments (
		id              TEXT PRIMARY KEY,
		issue_id        TEXT NOT NULL,
		author_id       TEXT NOT NULL,
		body_adf        TEXT NOT NULL,
		created_at      TEXT NOT NULL,
		updated_at      TEXT NOT NULL
	)`)
	if err != nil {
		db.Close()
		t.Fatalf("create jira_comments table: %v", err)
	}

	return db, func() { db.Close() }
}

// insertTestEscalatedCase inserts a test escalated case.
func insertTestEscalatedCase(t *testing.T, db *sql.DB, id, caseNum, subject, desc, priority string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO cases (id, case_number, subject, description, status, priority, product, owner_id, contact_id, account_id, created_at, is_escalated)
		VALUES (?, ?, ?, ?, 'Escalated', ?, 'TestProduct', 'owner-1', 'contact-1', 'account-1', '2024-01-15T10:00:00Z', 1)`,
		id, caseNum, subject, desc, priority)
	if err != nil {
		t.Fatalf("insert test case: %v", err)
	}
}

// insertTestJiraUser inserts a test JIRA user.
func insertTestJiraUser(t *testing.T, db *sql.DB, accountID, displayName string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO jira_users (account_id, display_name, email, account_type, active)
		VALUES (?, ?, 'test@example.com', 'atlassian', 1)`, accountID, displayName)
	if err != nil {
		t.Fatalf("insert test jira user: %v", err)
	}
}

// insertTestJiraIssue inserts a test JIRA issue.
func insertTestJiraIssue(t *testing.T, db *sql.DB, id, key, summary, status string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO jira_issues (id, key, project_key, summary, description_adf, issue_type, status, priority, created_at, updated_at)
		VALUES (?, ?, 'ENG', ?, '{}', 'Bug', ?, 'High', '2024-01-16T10:00:00Z', '2024-01-16T10:00:00Z')`,
		id, key, summary, status)
	if err != nil {
		t.Fatalf("insert test jira issue: %v", err)
	}
}

// mockJiraLLM implements the LLM interface for JIRA testing.
type mockJiraLLM struct {
	issueResponse   string
	commentResponse string
	err             error
}

func (m *mockJiraLLM) Generate(prompt string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	// Return different responses based on prompt content
	if contains(prompt, "JIRA engineering issue") {
		return m.issueResponse, nil
	}
	if contains(prompt, "JIRA comments") {
		return m.commentResponse, nil
	}
	return m.issueResponse, nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- JiraIssueGenerator Tests ---

func TestJiraIssueGenerator_Generate(t *testing.T) {
	db, cleanup := jiraTestDB(t)
	defer cleanup()

	// Setup test data
	insertTestEscalatedCase(t, db, "case-001", "00123457", "API timeout errors", "Customer experiencing timeouts", "High")
	insertTestEscalatedCase(t, db, "case-002", "00123458", "Login failures", "Users cannot log in", "Critical")
	insertTestJiraUser(t, db, "jira-user-001", "John Developer")
	insertTestJiraUser(t, db, "jira-user-002", "Jane Engineer")

	mockLLM := &mockJiraLLM{
		issueResponse: `{"summary": "Investigate API timeout in request handler", "description": "Root cause analysis required.\n\nStack trace shows connection pool exhaustion.", "labels": ["api", "performance", "customer-escalation"]}`,
	}

	ctx := &Context{
		DB:     db,
		LLM:    mockLLM,
		Logger: zerolog.Nop(),
	}

	gen := NewJiraIssueGenerator(ctx)
	err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify issues were created
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM jira_issues").Scan(&count); err != nil {
		t.Fatalf("count jira_issues: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 JIRA issues, got %d", count)
	}

	// Verify issue key format (ENG-NNNN)
	rows, err := db.Query("SELECT key, sf_case_id FROM jira_issues")
	if err != nil {
		t.Fatalf("query jira_issues: %v", err)
	}
	defer rows.Close()

	keyPattern := regexp.MustCompile(`^ENG-\d{4,}$`)
	for rows.Next() {
		var key, sfCaseID string
		if err := rows.Scan(&key, &sfCaseID); err != nil {
			t.Fatalf("scan row: %v", err)
		}
		if !keyPattern.MatchString(key) {
			t.Errorf("issue key %q does not match pattern ENG-NNNN", key)
		}
		if sfCaseID == "" {
			t.Error("sf_case_id should not be empty")
		}
	}

	// Verify bidirectional linking - cases should have jira_issue_key set
	var linkedCases int
	if err := db.QueryRow("SELECT COUNT(*) FROM cases WHERE jira_issue_key IS NOT NULL AND jira_issue_key != ''").Scan(&linkedCases); err != nil {
		t.Fatalf("count linked cases: %v", err)
	}
	if linkedCases != 2 {
		t.Errorf("expected 2 cases with jira_issue_key, got %d", linkedCases)
	}
}

func TestJiraIssueGenerator_NoEscalatedCases(t *testing.T) {
	db, cleanup := jiraTestDB(t)
	defer cleanup()

	// No escalated cases inserted
	insertTestJiraUser(t, db, "jira-user-001", "Developer")

	ctx := &Context{
		DB:     db,
		LLM:    &mockJiraLLM{},
		Logger: zerolog.Nop(),
	}

	gen := NewJiraIssueGenerator(ctx)
	err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM jira_issues").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 issues for no escalated cases, got %d", count)
	}
}

func TestJiraIssueGenerator_FallbackOnLLMError(t *testing.T) {
	db, cleanup := jiraTestDB(t)
	defer cleanup()

	insertTestEscalatedCase(t, db, "case-001", "00123457", "Test issue", "Description", "Medium")
	insertTestJiraUser(t, db, "jira-user-001", "Developer")

	mockLLM := &mockJiraLLM{
		issueResponse: "invalid json", // Will cause parse error
	}

	ctx := &Context{
		DB:     db,
		LLM:    mockLLM,
		Logger: zerolog.Nop(),
	}

	gen := NewJiraIssueGenerator(ctx)
	err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() should succeed with fallback, got error = %v", err)
	}

	// Should still create issue with default content
	var count int
	db.QueryRow("SELECT COUNT(*) FROM jira_issues").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 issue with fallback content, got %d", count)
	}
}

func TestJiraIssueKey_Format(t *testing.T) {
	key := NextJiraIssueKey("ENG")
	pattern := regexp.MustCompile(`^ENG-\d+$`)
	if !pattern.MatchString(key) {
		t.Errorf("JiraIssueKey %q does not match expected format", key)
	}
}


func TestJiraAccountID_Format(t *testing.T) {
	id := JiraAccountID()
	if len(id) != 24 {
		t.Errorf("JiraAccountID length = %d, want 24", len(id))
	}
	// Should be hex
	hexPattern := regexp.MustCompile(`^[a-f0-9]{24}$`)
	if !hexPattern.MatchString(id) {
		t.Errorf("JiraAccountID %q is not 24 hex chars", id)
	}
}

// --- JiraCommentGenerator Tests ---

func TestJiraCommentGenerator_Generate(t *testing.T) {
	db, cleanup := jiraTestDB(t)
	defer cleanup()

	// Setup test data - JIRA user and issue
	insertTestJiraUser(t, db, "jira-user-001", "Developer")
	insertTestJiraIssue(t, db, "issue-001", "ENG-1001", "API timeout issue", "In Progress")
	insertTestJiraIssue(t, db, "issue-002", "ENG-1002", "Login failure", "Done")

	mockLLM := &mockJiraLLM{
		commentResponse: `{"comments": ["Investigating the issue now.", "Found root cause.", "PR ready."]}`,
	}

	ctx := &Context{
		DB:     db,
		LLM:    mockLLM,
		Logger: zerolog.Nop(),
	}

	gen := NewJiraCommentGenerator(ctx)
	err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify comments were created
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM jira_comments").Scan(&count); err != nil {
		t.Fatalf("count jira_comments: %v", err)
	}
	if count < 2 {
		t.Errorf("expected at least 2 comments (one per issue), got %d", count)
	}

	// Verify comment format
	rows, err := db.Query("SELECT id, issue_id, body_adf FROM jira_comments")
	if err != nil {
		t.Fatalf("query jira_comments: %v", err)
	}
	defer rows.Close()

	hexPattern := regexp.MustCompile(`^[a-f0-9]{24}$`)
	for rows.Next() {
		var id, issueID, bodyADF string
		if err := rows.Scan(&id, &issueID, &bodyADF); err != nil {
			t.Fatalf("scan row: %v", err)
		}
		if !hexPattern.MatchString(id) {
			t.Errorf("comment id %q is not 24 hex chars", id)
		}
		if bodyADF == "" || bodyADF == "{}" {
			t.Error("body_adf should not be empty")
		}
	}
}

func TestJiraCommentGenerator_NoIssues(t *testing.T) {
	db, cleanup := jiraTestDB(t)
	defer cleanup()

	insertTestJiraUser(t, db, "jira-user-001", "Developer")
	// No issues

	ctx := &Context{
		DB:     db,
		LLM:    &mockJiraLLM{},
		Logger: zerolog.Nop(),
	}

	gen := NewJiraCommentGenerator(ctx)
	err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM jira_comments").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 comments for no issues, got %d", count)
	}
}

func TestJiraCommentGenerator_FallbackOnLLMError(t *testing.T) {
	db, cleanup := jiraTestDB(t)
	defer cleanup()

	insertTestJiraUser(t, db, "jira-user-001", "Developer")
	insertTestJiraIssue(t, db, "issue-001", "ENG-1001", "Test issue", "To Do")

	mockLLM := &mockJiraLLM{
		commentResponse: "not json", // Will fail parse
	}

	ctx := &Context{
		DB:     db,
		LLM:    mockLLM,
		Logger: zerolog.Nop(),
	}

	gen := NewJiraCommentGenerator(ctx)
	err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() should succeed with fallback, got error = %v", err)
	}

	// Should still create comments with default content
	var count int
	db.QueryRow("SELECT COUNT(*) FROM jira_comments").Scan(&count)
	if count < 1 {
		t.Errorf("expected at least 1 comment with fallback content, got %d", count)
	}
}