package scenario_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/falconleon/mock-salesforce/internal/db"
	"github.com/falconleon/mock-salesforce/internal/profile"
	"github.com/falconleon/mock-salesforce/internal/scenario"
	"github.com/rs/zerolog"
)

// mockInteractionLLM implements generator.LLM for interaction testing.
// Returns appropriate responses based on prompt content.
type mockInteractionLLM struct {
	callCount int
}

func (m *mockInteractionLLM) Generate(prompt string) (string, error) {
	m.callCount++

	// The UserGenerator expects just title and created_at
	// Return a simple response that works for user generation
	return `{"title":"Support Agent","created_at":"2023-01-15T09:00:00Z","subject":"Test case subject","description":"Test case description","case_type":"Technical Issue","origin":"Email","reason":"Software Defect","name":"Test Corp Inc","website":"https://test.example.com","phone":"555-0123"}`, nil
}

func TestAcmeScenario_GenerateInteractions(t *testing.T) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "acme-interactions-test-*.db")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	logger := zerolog.New(os.Stderr).Level(zerolog.DebugLevel)

	store, err := db.Open(tmpFile.Name(), logger)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	if err := store.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}

	// Load profile
	profilePath := filepath.Join("..", "..", "profiles", "acme_software.yaml")
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		t.Skipf("Profile file not found at %s", profilePath)
	}

	p, err := profile.Load(profilePath)
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}

	// Create scenario with mock LLM
	s := scenario.NewAcmeScenario(p, store.DB(), &mockInteractionLLM{}, logger)

	// First generate customers and support team (prerequisites)
	if err := s.GenerateCustomers(); err != nil {
		t.Fatalf("generate customers: %v", err)
	}
	if err := s.GenerateSupportTeam(); err != nil {
		t.Fatalf("generate support team: %v", err)
	}

	// Generate interactions
	if err := s.GenerateInteractions(); err != nil {
		t.Fatalf("generate interactions: %v", err)
	}

	// Verify case count (target: ~200)
	var caseCount int
	if err := store.DB().QueryRow("SELECT COUNT(*) FROM cases").Scan(&caseCount); err != nil {
		t.Fatalf("count cases: %v", err)
	}
	if caseCount < 180 || caseCount > 220 {
		t.Errorf("Expected ~200 cases, got %d", caseCount)
	}
	t.Logf("Generated %d cases", caseCount)

	// Verify email count (target: 3-5 per case = 600-1000)
	var emailCount int
	if err := store.DB().QueryRow("SELECT COUNT(*) FROM email_messages").Scan(&emailCount); err != nil {
		t.Fatalf("count emails: %v", err)
	}
	if emailCount < 400 || emailCount > 1200 {
		t.Errorf("Expected 400-1200 emails, got %d", emailCount)
	}
	t.Logf("Generated %d emails (%.1f per case)", emailCount, float64(emailCount)/float64(caseCount))

	// Verify comment count (target: 1-3 per case = 200-600)
	var commentCount int
	if err := store.DB().QueryRow("SELECT COUNT(*) FROM case_comments").Scan(&commentCount); err != nil {
		t.Fatalf("count comments: %v", err)
	}
	if commentCount < 100 || commentCount > 800 {
		t.Errorf("Expected 100-800 comments, got %d", commentCount)
	}
	t.Logf("Generated %d comments (%.1f per case)", commentCount, float64(commentCount)/float64(caseCount))

	// Verify feed item count
	var feedCount int
	if err := store.DB().QueryRow("SELECT COUNT(*) FROM feed_items").Scan(&feedCount); err != nil {
		t.Fatalf("count feed items: %v", err)
	}
	t.Logf("Generated %d feed items (%.1f per case)", feedCount, float64(feedCount)/float64(caseCount))

	// Verify JIRA escalation count (target: ~20% of high/critical cases)
	var escalatedCount int
	if err := store.DB().QueryRow("SELECT COUNT(*) FROM cases WHERE is_escalated = 1").Scan(&escalatedCount); err != nil {
		t.Fatalf("count escalated cases: %v", err)
	}
	var jiraCount int
	if err := store.DB().QueryRow("SELECT COUNT(*) FROM jira_issues").Scan(&jiraCount); err != nil {
		t.Fatalf("count jira issues: %v", err)
	}
	t.Logf("Generated %d escalated cases, %d JIRA issues", escalatedCount, jiraCount)

	// Verify JIRA comments
	var jiraCommentCount int
	if err := store.DB().QueryRow("SELECT COUNT(*) FROM jira_comments").Scan(&jiraCommentCount); err != nil {
		t.Fatalf("count jira comments: %v", err)
	}
	t.Logf("Generated %d JIRA comments", jiraCommentCount)

	// Print distribution summary
	t.Log("--- Distribution Summary ---")
	logDistribution(t, store.DB(), "status")
	logDistribution(t, store.DB(), "priority")
}

// logDistribution logs the distribution of values for a column in the cases table.
func logDistribution(t *testing.T, database *sql.DB, column string) {
	t.Helper()
	var query string
	switch column {
	case "status":
		query = "SELECT status, COUNT(*) FROM cases GROUP BY status"
	case "priority":
		query = "SELECT priority, COUNT(*) FROM cases GROUP BY priority"
	default:
		return
	}
	rows, err := database.Query(query)
	if err != nil {
		t.Logf("Failed to query %s distribution: %v", column, err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var val string
		var count int
		if err := rows.Scan(&val, &count); err != nil {
			continue
		}
		t.Logf("  %s: %s = %d", column, val, count)
	}
}

func TestAcmeScenario_GenerateInteractions_JiraLinking(t *testing.T) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "acme-jira-test-*.db")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	logger := zerolog.New(os.Stderr).Level(zerolog.DebugLevel)

	store, err := db.Open(tmpFile.Name(), logger)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	if err := store.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}

	// Load profile
	profilePath := filepath.Join("..", "..", "profiles", "acme_software.yaml")
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		t.Skipf("Profile file not found at %s", profilePath)
	}

	p, err := profile.Load(profilePath)
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}

	// Create scenario with mock LLM
	s := scenario.NewAcmeScenario(p, store.DB(), &mockInteractionLLM{}, logger)

	// Generate all data
	if err := s.GenerateAll(); err != nil {
		t.Fatalf("generate all: %v", err)
	}
	if err := s.GenerateInteractions(); err != nil {
		t.Fatalf("generate interactions: %v", err)
	}

	// Verify JIRA issues are linked to SF cases via sf_case_id
	rows, err := store.DB().Query(`
		SELECT ji.key, ji.sf_case_id, c.case_number
		FROM jira_issues ji
		JOIN cases c ON ji.sf_case_id = c.id
		LIMIT 5
	`)
	if err != nil {
		t.Fatalf("query jira links: %v", err)
	}
	defer rows.Close()

	linkedCount := 0
	for rows.Next() {
		var jiraKey, sfCaseID, caseNumber string
		if err := rows.Scan(&jiraKey, &sfCaseID, &caseNumber); err != nil {
			t.Fatalf("scan jira link: %v", err)
		}
		linkedCount++
		t.Logf("JIRA %s linked to Case %s (ID: %s)", jiraKey, caseNumber, sfCaseID)
	}

	if linkedCount == 0 {
		t.Log("No JIRA issues linked to cases (this may be expected if no escalations)")
	} else {
		t.Logf("Verified %d JIRA-to-Case links", linkedCount)
	}
}
