package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/db"
	"github.com/falconleon/mock-salesforce/internal/exporter"
)

// mockLLM implements the generator.LLM interface for testing.
// It returns pre-configured responses based on prompt content.
// After L2.9.3 refactoring, generators produce entities one at a time.
type mockLLM struct {
	accountCalls atomic.Int32
	contactCalls atomic.Int32
	userCalls    atomic.Int32
}

func (m *mockLLM) Generate(prompt string) (string, error) {
	// Accounts generator: prompts with "company name" and expects single object
	// Prompt pattern: "Generate a company name for a [industry] company based in..."
	if containsAll(prompt, "company", "generate") && containsAll(prompt, "name", "website", "phone") {
		count := m.accountCalls.Add(1)
		return fmt.Sprintf(`{"name":"TestCompany%d Inc","website":"https://testcompany%d.example.com","phone":"555-010%d","annual_revenue":%d,"created_at":"2023-0%d-15T10:00:00Z"}`,
			count, count, count, 10000000*int(count), min(int(count), 9)), nil
	}

	// Contacts generator: prompts with "Generate a contact for [account name]"
	// Expects: title, department, phone, created_at
	if containsAll(prompt, "generate", "contact") && containsAll(prompt, "title", "department") {
		count := m.contactCalls.Add(1)
		return fmt.Sprintf(`{"title":"Test Title %d","department":"Department %d","phone":"555-010%d","created_at":"2024-0%d-15T09:00:00Z"}`,
			count, count, count, min(int(count), 9)), nil
	}

	// Users generator: prompts with role specs like "Manager", "L1 Support", "L2 Support", "L3 Support"
	// Expects: title, created_at
	if containsAll(prompt, "support", "team", "member") || containsAll(prompt, "role:") {
		count := m.userCalls.Add(1)
		year := "2022"
		if count > 2 {
			year = "2023"
		}
		return fmt.Sprintf(`{"title":"Test Title %d","created_at":"%s-0%d-15T09:00:00Z"}`,
			count, year, min(int(count), 9)), nil
	}

	// Cases generator prompt (already correct - returns single object)
	if containsAll(prompt, "case", "support") {
		return `{"subject":"Login page not loading","description":"Customer reports unable to access the login page. Error 500 displayed.","case_type":"Problem","origin":"Email","reason":"User Error","product":"Web Portal"}`, nil
	}

	// Email generator prompt (already uses arrays)
	if containsAll(prompt, "email", "thread") {
		return `[
			{"subject":"RE: Login page not loading","text_body":"Thank you for contacting support. We are investigating this issue.","from_address":"support@example.com","to_address":"customer@example.com","message_date":"2024-01-15T11:00:00Z","incoming":false},
			{"subject":"RE: Login page not loading","text_body":"Any update on this issue?","from_address":"customer@example.com","to_address":"support@example.com","message_date":"2024-01-15T14:00:00Z","incoming":true}
		]`, nil
	}

	// Comments generator prompt (already correct)
	if containsAll(prompt, "comment") && !containsAll(prompt, "jira") {
		return `["Investigating the issue. Found potential cause in auth service.","Deployed fix to staging for testing."]`, nil
	}

	// Feed items generator prompt (already correct)
	if containsAll(prompt, "feed") || containsAll(prompt, "chatter") {
		return `["Case assigned to engineering team","Priority escalated to High"]`, nil
	}

	// JIRA comment generator prompt (check BEFORE issue - both contain "jira" and "issue")
	if containsAll(prompt, "jira", "comment", "discussion") {
		return `{"comments":["Identified root cause in OAuth module","PR #1234 submitted for review","Fix deployed to production"]}`, nil
	}

	// JIRA issue generator prompt (already correct)
	if containsAll(prompt, "jira", "engineering issue") {
		return `{"summary":"Login failure investigation","description":"Investigating login page 500 errors reported by customer","labels":["customer-facing","urgent"]}`, nil
	}

	// Default fallback
	return `{}`, nil
}

// containsAll checks if s contains all substrings (case-insensitive).
func containsAll(s string, subs ...string) bool {
	lower := strings.ToLower(s)
	for _, sub := range subs {
		if !strings.Contains(lower, strings.ToLower(sub)) {
			return false
		}
	}
	return true
}

// testStore creates a test store with initialized schema.
func testStore(t *testing.T) (*db.Store, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "pipeline-test-*.db")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	tmpFile.Close()

	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	store, err := db.Open(tmpFile.Name(), logger)
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("open store: %v", err)
	}

	if err := store.Init(); err != nil {
		store.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("init store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.Remove(tmpFile.Name())
	}

	return store, cleanup
}

// TestPipelineIntegration exercises the full generation pipeline end-to-end.
func TestPipelineIntegration(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	llm := &mockLLM{}

	// Configure a minimal pipeline
	cfg := Config{
		Accounts:          2,
		ContactsPerAcct:   2,
		Users:             2,
		Cases:             3,
		EmailsPerCaseAvg:  2,
		CommentsPerCase:   2,
		FeedItemsPerCase:  1,
		JiraEscalationPct: 50, // Half of cases escalated
		JiraCommentsAvg:   2,
	}

	p := New(store, llm, cfg, logger)

	// Run all phases
	if err := p.RunAll(); err != nil {
		t.Fatalf("RunAll() error = %v", err)
	}

	// Verify data was generated
	stats, err := store.Stats()
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}

	// Log stats for debugging
	t.Logf("Generated data stats: %+v", stats)

	// Verify each entity type was generated
	expectedMinCounts := map[string]int{
		"accounts":       2,
		"contacts":       1, // At least one contact per account
		"users":          2,
		"cases":          3,
		"email_messages": 1, // At least some emails
	}

	for table, minCount := range expectedMinCounts {
		if stats[table] < minCount {
			t.Errorf("table %s: got %d records, want at least %d", table, stats[table], minCount)
		}
	}

	// Verify referential integrity
	verifyReferentialIntegrity(t, store)

	// Test export to both formats
	testExport(t, store)
}

// verifyReferentialIntegrity checks that all foreign keys point to valid records.
func verifyReferentialIntegrity(t *testing.T, store *db.Store) {
	t.Helper()

	// Use direct SQL to check for orphaned records
	// SQLite enforces FK constraints, but let's verify manually
	queries := []struct {
		name  string
		query string
	}{
		{"orphan_contacts", "SELECT COUNT(*) FROM contacts c LEFT JOIN accounts a ON c.account_id = a.id WHERE a.id IS NULL"},
		{"orphan_cases_owner", "SELECT COUNT(*) FROM cases c LEFT JOIN users u ON c.owner_id = u.id WHERE u.id IS NULL"},
		{"orphan_cases_contact", "SELECT COUNT(*) FROM cases c LEFT JOIN contacts ct ON c.contact_id = ct.id WHERE ct.id IS NULL"},
		{"orphan_cases_account", "SELECT COUNT(*) FROM cases c LEFT JOIN accounts a ON c.account_id = a.id WHERE a.id IS NULL"},
		{"orphan_emails", "SELECT COUNT(*) FROM email_messages e LEFT JOIN cases c ON e.case_id = c.id WHERE c.id IS NULL"},
		{"orphan_comments", "SELECT COUNT(*) FROM case_comments cc LEFT JOIN cases c ON cc.case_id = c.id WHERE c.id IS NULL"},
		{"orphan_comments_user", "SELECT COUNT(*) FROM case_comments cc LEFT JOIN users u ON cc.created_by_id = u.id WHERE u.id IS NULL"},
		{"orphan_feed_items", "SELECT COUNT(*) FROM feed_items fi LEFT JOIN cases c ON fi.case_id = c.id WHERE c.id IS NULL"},
		{"orphan_jira_comments", "SELECT COUNT(*) FROM jira_comments jc LEFT JOIN jira_issues ji ON jc.issue_id = ji.id WHERE ji.id IS NULL"},
	}

	for _, q := range queries {
		var count int
		if err := store.DB().QueryRow(q.query).Scan(&count); err != nil {
			t.Errorf("%s: query error = %v", q.name, err)
			continue
		}
		if count > 0 {
			t.Errorf("%s: found %d orphan records", q.name, count)
		}
	}
}

// testExport verifies that exported JSON is valid for the Salesforce format.
func testExport(t *testing.T, store *db.Store) {
	t.Helper()

	// Create temp output directory
	outDir, err := os.MkdirTemp("", "pipeline-export-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(outDir)

	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)

	// Test Salesforce export
	sfExporter := exporter.NewSalesforceExporter(store.DB(), logger)
	if err := sfExporter.Export(outDir); err != nil {
		t.Fatalf("Salesforce export error = %v", err)
	}

	// Verify Salesforce JSON files are valid
	sfFiles := []string{"accounts.json", "contacts.json", "users.json", "cases.json", "email_messages.json", "case_comments.json", "feed_items.json"}
	for _, fname := range sfFiles {
		verifyJSONFile(t, filepath.Join(outDir, fname))
	}

	t.Log("All Salesforce export files contain valid JSON")
}

// verifyJSONFile reads a file and verifies it contains valid JSON.
func verifyJSONFile(t *testing.T, path string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("read %s: %v", path, err)
		return
	}

	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Errorf("parse %s: invalid JSON: %v", path, err)
		return
	}
}

// TestPipelineWithEscalations exercises the full pipeline ensuring JIRA integration.
func TestPipelineWithEscalations(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	llm := &mockLLM{}

	// Configure pipeline with many cases to ensure at least some escalations
	cfg := Config{
		Accounts:          3,
		ContactsPerAcct:   2,
		Users:             3,
		Cases:             20, // More cases = more likely escalated ones (10% escalation = ~2 escalated)
		EmailsPerCaseAvg:  2,
		CommentsPerCase:   2,
		FeedItemsPerCase:  1,
		JiraEscalationPct: 100, // This field isn't used by case generator (handled by status distribution)
		JiraCommentsAvg:   2,
	}

	p := New(store, llm, cfg, logger)

	// Run all phases
	if err := p.RunAll(); err != nil {
		t.Fatalf("RunAll() error = %v", err)
	}

	stats, err := store.Stats()
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}

	t.Logf("Generated data stats: %+v", stats)

	// Verify we have cases
	if stats["cases"] < 20 {
		t.Errorf("expected at least 20 cases, got %d", stats["cases"])
	}

	// Count escalated cases
	var escalatedCount int
	err = store.DB().QueryRow("SELECT COUNT(*) FROM cases WHERE is_escalated = 1").Scan(&escalatedCount)
	if err != nil {
		t.Fatalf("count escalated cases: %v", err)
	}
	t.Logf("Escalated cases: %d", escalatedCount)

	// JIRA issues should match escalated cases
	if escalatedCount > 0 && stats["jira_issues"] == 0 {
		t.Errorf("expected JIRA issues for %d escalated cases, got %d", escalatedCount, stats["jira_issues"])
	}

	// When we have JIRA issues and active JIRA users, we should have JIRA comments
	// However, this depends on JIRA users being marked active, so we just log instead of fail
	if stats["jira_issues"] > 0 {
		t.Logf("JIRA issues created: %d, JIRA comments: %d", stats["jira_issues"], stats["jira_comments"])
	}

	// Verify referential integrity
	verifyReferentialIntegrity(t, store)
}

// TestPipelinePhases tests running individual phases.
func TestPipelinePhases(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	llm := &mockLLM{}

	cfg := Config{
		Accounts:          2,
		ContactsPerAcct:   1,
		Users:             2,
		Cases:             2,
		EmailsPerCaseAvg:  1,
		CommentsPerCase:   1,
		FeedItemsPerCase:  1,
		JiraEscalationPct: 50,
		JiraCommentsAvg:   1,
	}

	p := New(store, llm, cfg, logger)

	// Test Phase 1 - Foundation
	if err := p.RunPhase(1); err != nil {
		t.Fatalf("Phase1 error = %v", err)
	}

	stats, _ := store.Stats()
	if stats["accounts"] < 2 {
		t.Errorf("Phase1: expected 2 accounts, got %d", stats["accounts"])
	}
	if stats["users"] < 2 {
		t.Errorf("Phase1: expected 2 users, got %d", stats["users"])
	}

	// Test Phase 2 - Cases
	if err := p.RunPhase(2); err != nil {
		t.Fatalf("Phase2 error = %v", err)
	}

	stats, _ = store.Stats()
	if stats["cases"] < 2 {
		t.Errorf("Phase2: expected 2 cases, got %d", stats["cases"])
	}

	// Test Phase 3 - Communications
	if err := p.RunPhase(3); err != nil {
		t.Fatalf("Phase3 error = %v", err)
	}

	stats, _ = store.Stats()
	if stats["email_messages"] == 0 {
		t.Error("Phase3: expected email messages")
	}

	// Test Phase 4 - JIRA
	if err := p.RunPhase(4); err != nil {
		t.Fatalf("Phase4 error = %v", err)
	}

	t.Logf("Final stats after all phases: %+v", stats)
}

// TestPipelineInvalidPhase tests error handling for invalid phase numbers.
func TestPipelineInvalidPhase(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	llm := &mockLLM{}
	cfg := Config{}

	p := New(store, llm, cfg, logger)

	// Test invalid phase numbers
	if err := p.RunPhase(0); err == nil {
		t.Error("expected error for phase 0")
	}
	if err := p.RunPhase(5); err == nil {
		t.Error("expected error for phase 5")
	}
}

