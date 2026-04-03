package scenario_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/falconleon/mock-salesforce/internal/db"
	"github.com/falconleon/mock-salesforce/internal/profile"
	"github.com/falconleon/mock-salesforce/internal/scenario"
	"github.com/rs/zerolog"
)

// ValidationReport captures validation results for reporting.
type ValidationReport struct {
	TotalEntities  int
	EntityCounts   map[string]int
	IDFormatErrors []string
	FKViolations   []string
	DiversityIssues []string
	TemporalIssues []string
	StartTime      time.Time
	Duration       time.Duration
}

// TestAcmeScenario_FullValidation runs comprehensive end-to-end validation.
func TestAcmeScenario_FullValidation(t *testing.T) {
	report := &ValidationReport{
		EntityCounts:    make(map[string]int),
		StartTime:       time.Now(),
	}

	// Setup
	tmpFile, err := os.CreateTemp("", "acme-validation-*.db")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	logger := zerolog.New(os.Stderr).Level(zerolog.InfoLevel)
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
	s := scenario.NewAcmeScenario(p, store.DB(), &mockValidationLLM{}, logger)

	// Generate all data
	if err := s.GenerateAll(); err != nil {
		t.Fatalf("generate all: %v", err)
	}
	if err := s.GenerateInteractions(); err != nil {
		t.Fatalf("generate interactions: %v", err)
	}

	// Run validations
	t.Run("EntityCounts", func(t *testing.T) {
		validateEntityCounts(t, store.DB(), report)
	})
	t.Run("IDFormats", func(t *testing.T) {
		validateIDFormats(t, store.DB(), report)
	})
	t.Run("ReferentialIntegrity", func(t *testing.T) {
		validateReferentialIntegrity(t, store.DB(), report)
	})
	t.Run("DataDiversity", func(t *testing.T) {
		validateDataDiversity(t, store.DB(), report)
	})
	t.Run("TemporalConsistency", func(t *testing.T) {
		validateTemporalConsistency(t, store.DB(), report)
	})

	// Calculate totals
	for _, count := range report.EntityCounts {
		report.TotalEntities += count
	}
	report.Duration = time.Since(report.StartTime)

	// Print summary report
	t.Log("\n========== VALIDATION SUMMARY REPORT ==========")
	t.Logf("Total Entities Generated: %d", report.TotalEntities)
	t.Logf("Time to Generate: %v", report.Duration)
	t.Log("\nEntity Counts:")
	for entity, count := range report.EntityCounts {
		t.Logf("  - %s: %d", entity, count)
	}

	if len(report.IDFormatErrors) > 0 {
		t.Logf("\nID Format Errors: %d", len(report.IDFormatErrors))
		for _, err := range report.IDFormatErrors[:min(5, len(report.IDFormatErrors))] {
			t.Logf("  - %s", err)
		}
	}
	if len(report.FKViolations) > 0 {
		t.Logf("\nFK Violations: %d", len(report.FKViolations))
		for _, v := range report.FKViolations[:min(5, len(report.FKViolations))] {
			t.Logf("  - %s", v)
		}
	}
	if len(report.DiversityIssues) > 0 {
		t.Logf("\nDiversity Issues: %d", len(report.DiversityIssues))
		for _, issue := range report.DiversityIssues {
			t.Logf("  - %s", issue)
		}
	}
	t.Log("================================================")
}

// mockValidationLLM implements generator.LLM for validation testing.
type mockValidationLLM struct{}

func (m *mockValidationLLM) Generate(prompt string) (string, error) {
	return `{"title":"Support Agent","created_at":"2023-01-15T09:00:00Z","subject":"Test case","description":"Test description","case_type":"Technical Issue","origin":"Email","reason":"Software Defect","name":"Test Corp Inc","website":"https://test.example.com","phone":"555-0123","department":"IT"}`, nil
}

// validateEntityCounts checks that entity counts are within expected ranges.
func validateEntityCounts(t *testing.T, database *sql.DB, report *ValidationReport) {
	t.Helper()

	counts := []struct {
		name     string
		query    string
		minCount int
		maxCount int
	}{
		{"accounts", "SELECT COUNT(*) FROM accounts", 10, 10},
		{"contacts", "SELECT COUNT(*) FROM contacts", 28, 32},
		{"users", "SELECT COUNT(*) FROM users", 14, 16},
		{"cases", "SELECT COUNT(*) FROM cases", 180, 220},
		{"emails", "SELECT COUNT(*) FROM email_messages", 400, 1200},
		{"comments", "SELECT COUNT(*) FROM case_comments", 100, 800},
		{"feed_items", "SELECT COUNT(*) FROM feed_items", 1000, 1800},
		{"jira_issues", "SELECT COUNT(*) FROM jira_issues", 10, 50},
		{"jira_users", "SELECT COUNT(*) FROM jira_users", 14, 16},
	}

	for _, c := range counts {
		var count int
		if err := database.QueryRow(c.query).Scan(&count); err != nil {
			t.Errorf("query %s count: %v", c.name, err)
			continue
		}
		report.EntityCounts[c.name] = count

		if count < c.minCount || count > c.maxCount {
			t.Errorf("%s count %d outside range [%d, %d]", c.name, count, c.minCount, c.maxCount)
		} else {
			t.Logf("%s: %d (expected %d-%d) ✓", c.name, count, c.minCount, c.maxCount)
		}
	}
}

// validateIDFormats checks that IDs follow expected patterns.
func validateIDFormats(t *testing.T, database *sql.DB, report *ValidationReport) {
	t.Helper()

	// Salesforce IDs: 18 chars, starts with 3-char object prefix, 12 hex chars, ends with AAV
	// Prefix can contain alphanumeric chars (e.g., 001, 003, 005, 500, 02s, 00a, 0D5)
	sfPattern := regexp.MustCompile(`^[0-9a-zA-Z]{3}[0-9a-fA-F]{12}AAV$`)

	sfTables := []struct {
		table  string
		column string
		prefix string
	}{
		{"accounts", "id", "001"},
		{"contacts", "id", "003"},
		{"users", "id", "005"},
		{"cases", "id", "500"},
		{"email_messages", "id", "02s"},
		{"case_comments", "id", "00a"},
		{"feed_items", "id", "0D5"},
	}

	for _, tbl := range sfTables {
		query := "SELECT " + tbl.column + " FROM " + tbl.table + " LIMIT 10"
		rows, err := database.Query(query)
		if err != nil {
			t.Errorf("query %s: %v", tbl.table, err)
			continue
		}
		defer rows.Close()

		checked := 0
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				continue
			}
			checked++

			if len(id) != 18 {
				report.IDFormatErrors = append(report.IDFormatErrors,
					tbl.table+": ID length "+string(rune(len(id)))+" != 18: "+id)
				t.Errorf("%s ID wrong length: %s", tbl.table, id)
			}
			if !strings.HasPrefix(id, tbl.prefix) {
				report.IDFormatErrors = append(report.IDFormatErrors,
					tbl.table+": ID prefix mismatch: "+id)
				t.Errorf("%s ID wrong prefix: %s (expected %s)", tbl.table, id, tbl.prefix)
			}
			if !sfPattern.MatchString(id) {
				report.IDFormatErrors = append(report.IDFormatErrors,
					tbl.table+": ID format invalid: "+id)
				t.Errorf("%s ID format invalid: %s", tbl.table, id)
			}
		}
		if checked > 0 {
			t.Logf("%s IDs: %d checked, format OK ✓", tbl.table, checked)
		}
	}

	// JIRA keys: PROJECT-NUMBER format
	jiraKeyPattern := regexp.MustCompile(`^[A-Z]+-\d+$`)
	rows, err := database.Query("SELECT key FROM jira_issues LIMIT 10")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var key string
			if err := rows.Scan(&key); err != nil {
				continue
			}
			if !jiraKeyPattern.MatchString(key) {
				report.IDFormatErrors = append(report.IDFormatErrors,
					"jira_issues: key format invalid: "+key)
				t.Errorf("JIRA key format invalid: %s", key)
			}
		}
		t.Log("JIRA keys: format OK ✓")
	}
}


// validateReferentialIntegrity checks that all foreign keys resolve.
func validateReferentialIntegrity(t *testing.T, database *sql.DB, report *ValidationReport) {
	t.Helper()

	fkChecks := []struct {
		name     string
		query    string
	}{
		{"contacts->accounts", `SELECT COUNT(*) FROM contacts c LEFT JOIN accounts a ON c.account_id = a.id WHERE a.id IS NULL`},
		{"cases->accounts", `SELECT COUNT(*) FROM cases c LEFT JOIN accounts a ON c.account_id = a.id WHERE a.id IS NULL`},
		{"cases->contacts", `SELECT COUNT(*) FROM cases c LEFT JOIN contacts ct ON c.contact_id = ct.id WHERE ct.id IS NULL`},
		{"cases->users", `SELECT COUNT(*) FROM cases c LEFT JOIN users u ON c.owner_id = u.id WHERE u.id IS NULL`},
		{"email_messages->cases", `SELECT COUNT(*) FROM email_messages e LEFT JOIN cases c ON e.case_id = c.id WHERE c.id IS NULL`},
		{"case_comments->cases", `SELECT COUNT(*) FROM case_comments cc LEFT JOIN cases c ON cc.case_id = c.id WHERE c.id IS NULL`},
		{"case_comments->users", `SELECT COUNT(*) FROM case_comments cc LEFT JOIN users u ON cc.created_by_id = u.id WHERE u.id IS NULL`},
		{"feed_items->cases", `SELECT COUNT(*) FROM feed_items f LEFT JOIN cases c ON f.case_id = c.id WHERE c.id IS NULL`},
		{"feed_items->users", `SELECT COUNT(*) FROM feed_items f LEFT JOIN users u ON f.created_by_id = u.id WHERE u.id IS NULL`},
		{"jira_issues->cases", `SELECT COUNT(*) FROM jira_issues ji LEFT JOIN cases c ON ji.sf_case_id = c.id WHERE ji.sf_case_id IS NOT NULL AND c.id IS NULL`},
		{"jira_comments->issues", `SELECT COUNT(*) FROM jira_comments jc LEFT JOIN jira_issues ji ON jc.issue_id = ji.id WHERE ji.id IS NULL`},
	}

	allPassed := true
	for _, check := range fkChecks {
		var orphanCount int
		if err := database.QueryRow(check.query).Scan(&orphanCount); err != nil {
			t.Errorf("FK check %s failed: %v", check.name, err)
			continue
		}

		if orphanCount > 0 {
			allPassed = false
			report.FKViolations = append(report.FKViolations,
				check.name+": "+string(rune(orphanCount))+" orphan records")
			t.Errorf("%s: %d orphan records found", check.name, orphanCount)
		}
	}

	if allPassed {
		t.Log("Referential integrity: All FK checks passed ✓")
	}
}

// validateDataDiversity checks that generated data has variety.
func validateDataDiversity(t *testing.T, database *sql.DB, report *ValidationReport) {
	t.Helper()

	// Note: With mock LLM, fallback values reduce diversity.
	// These thresholds are set for mock LLM testing.
	// Real LLM should produce much higher diversity.
	diversityChecks := []struct {
		name      string
		query     string
		minUnique int
	}{
		{"account_names", "SELECT COUNT(DISTINCT name) FROM accounts", 1},           // Mock LLM uses fallback name
		{"account_industries", "SELECT COUNT(DISTINCT industry) FROM accounts", 3},  // Generated from profile
		{"account_types", "SELECT COUNT(DISTINCT type) FROM accounts", 3},           // Enterprise, Mid-Market, SMB
		{"user_first_names", "SELECT COUNT(DISTINCT first_name) FROM users", 10},    // Faker generates unique
		{"user_last_names", "SELECT COUNT(DISTINCT last_name) FROM users", 10},      // Faker generates unique
		{"case_subjects", "SELECT COUNT(DISTINCT subject) FROM cases", 1},           // Mock LLM uses fallback
		{"case_priorities", "SELECT COUNT(DISTINCT priority) FROM cases", 3},        // Generated from profile
		{"case_statuses", "SELECT COUNT(DISTINCT status) FROM cases", 2},            // New and Closed
		{"contact_titles", "SELECT COUNT(DISTINCT title) FROM contacts WHERE title IS NOT NULL", 1}, // Mock fallback
	}

	for _, check := range diversityChecks {
		var uniqueCount int
		if err := database.QueryRow(check.query).Scan(&uniqueCount); err != nil {
			t.Errorf("diversity check %s failed: %v", check.name, err)
			continue
		}

		if uniqueCount < check.minUnique {
			report.DiversityIssues = append(report.DiversityIssues,
				check.name+": only "+string(rune(uniqueCount))+" unique values (expected "+string(rune(check.minUnique))+")")
			t.Errorf("%s: only %d unique values (expected >= %d)", check.name, uniqueCount, check.minUnique)
		} else {
			t.Logf("%s: %d unique values ✓", check.name, uniqueCount)
		}
	}
}

// validateTemporalConsistency checks that dates are logical.
func validateTemporalConsistency(t *testing.T, database *sql.DB, report *ValidationReport) {
	t.Helper()

	// Check that contacts are created after their accounts
	var contactBeforeAccount int
	err := database.QueryRow(`
		SELECT COUNT(*) FROM contacts c
		JOIN accounts a ON c.account_id = a.id
		WHERE c.created_at < a.created_at
	`).Scan(&contactBeforeAccount)
	if err != nil {
		t.Errorf("temporal check failed: %v", err)
	} else if contactBeforeAccount > 0 {
		report.TemporalIssues = append(report.TemporalIssues,
			"contacts created before their accounts")
		t.Errorf("Found %d contacts created before their accounts", contactBeforeAccount)
	} else {
		t.Log("Contact dates: all after account creation ✓")
	}

	// Check that case closed_at is after created_at for closed cases
	var closedBeforeCreated int
	err = database.QueryRow(`
		SELECT COUNT(*) FROM cases
		WHERE is_closed = 1 AND closed_at IS NOT NULL AND closed_at < created_at
	`).Scan(&closedBeforeCreated)
	if err != nil {
		t.Errorf("temporal check failed: %v", err)
	} else if closedBeforeCreated > 0 {
		report.TemporalIssues = append(report.TemporalIssues,
			"cases closed before they were created")
		t.Errorf("Found %d cases closed before creation", closedBeforeCreated)
	} else {
		t.Log("Case dates: closed_at after created_at ✓")
	}

	// Check that emails are within reasonable time of case creation
	var emailsOutOfRange int
	err = database.QueryRow(`
		SELECT COUNT(*) FROM email_messages e
		JOIN cases c ON e.case_id = c.id
		WHERE e.message_date < c.created_at
	`).Scan(&emailsOutOfRange)
	if err != nil {
		t.Errorf("temporal check failed: %v", err)
	} else if emailsOutOfRange > 0 {
		t.Logf("Warning: %d emails dated before their case creation", emailsOutOfRange)
	} else {
		t.Log("Email dates: all after case creation ✓")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

