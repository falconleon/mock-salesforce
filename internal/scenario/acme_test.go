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

// mockLLM implements generator.LLM for testing.
type mockLLM struct{}

func (m *mockLLM) Generate(prompt string) (string, error) {
	// Return deterministic mock responses based on prompt content
	return `{"name":"Test Corp Inc","website":"https://test.example.com","phone":"555-0123","annual_revenue":100000,"created_at":"2023-05-15T10:00:00Z","title":"IT Director","department":"Information Technology"}`, nil
}

func TestAcmeScenario_GenerateCustomers(t *testing.T) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "acme-test-*.db")
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
	s := scenario.NewAcmeScenario(p, store.DB(), &mockLLM{}, logger)

	// Generate customers
	if err := s.GenerateCustomers(); err != nil {
		t.Fatalf("generate customers: %v", err)
	}

	// Verify account count
	var accountCount int
	if err := store.DB().QueryRow("SELECT COUNT(*) FROM accounts").Scan(&accountCount); err != nil {
		t.Fatalf("count accounts: %v", err)
	}
	if accountCount != 10 {
		t.Errorf("Expected 10 accounts, got %d", accountCount)
	}

	// Verify account type distribution
	rows, err := store.DB().Query("SELECT type, COUNT(*) FROM accounts GROUP BY type")
	if err != nil {
		t.Fatalf("query account types: %v", err)
	}
	defer rows.Close()

	typeCounts := make(map[string]int)
	for rows.Next() {
		var accountType string
		var count int
		if err := rows.Scan(&accountType, &count); err != nil {
			t.Fatalf("scan type count: %v", err)
		}
		typeCounts[accountType] = count
	}

	expectedTypes := map[string]int{
		"Enterprise": 2,
		"Mid-Market": 4,
		"SMB":        4,
	}
	for typ, expected := range expectedTypes {
		if typeCounts[typ] != expected {
			t.Errorf("Type %s: expected %d, got %d", typ, expected, typeCounts[typ])
		}
	}

	// Verify contacts were created
	var contactCount int
	if err := store.DB().QueryRow("SELECT COUNT(*) FROM contacts").Scan(&contactCount); err != nil {
		t.Fatalf("count contacts: %v", err)
	}
	// Expected: 2*5 + 4*3 + 4*2 = 10 + 12 + 8 = 30
	if contactCount != 30 {
		t.Errorf("Expected 30 contacts, got %d", contactCount)
	}

	t.Logf("Generated %d accounts and %d contacts", accountCount, contactCount)
}

func TestAcmeScenario_GenerateSupportTeam(t *testing.T) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "acme-support-test-*.db")
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
	s := scenario.NewAcmeScenario(p, store.DB(), &mockLLM{}, logger)

	// Generate support team
	if err := s.GenerateSupportTeam(); err != nil {
		t.Fatalf("generate support team: %v", err)
	}

	// Verify user count
	verifyUserCount(t, store.DB())
}

func verifyUserCount(t *testing.T, db *sql.DB) {
	t.Helper()

	var userCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if userCount != 15 {
		t.Errorf("Expected 15 users, got %d", userCount)
	}
	t.Logf("Generated %d support users", userCount)
}

