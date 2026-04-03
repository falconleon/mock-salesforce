package scenario

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
)

// mockLLM implements generator.LLM for testing.
type mockLLM struct{}

func (m *mockLLM) Generate(prompt string) (string, error) {
	// Return minimal valid JSON for account generation
	return `{"name":"Test Corp","website":"https://test.example.com","phone":"555-0100","created_at":"2023-03-15T10:00:00Z"}`, nil
}

func TestIndustryScenarioLoadAllProfiles(t *testing.T) {
	profiles := []struct {
		name string
		path string
	}{
		{"healthcare", "../../profiles/healthcare_medtech.yaml"},
		{"finserv", "../../profiles/finserv_fincore.yaml"},
		{"saas", "../../profiles/saas_cloudops.yaml"},
		{"retail", "../../profiles/retail_retailedge.yaml"},
		{"manufacturing", "../../profiles/manufacturing_factoryos.yaml"},
	}

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	logger := zerolog.Nop()
	llm := &mockLLM{}

	for _, p := range profiles {
		t.Run(p.name, func(t *testing.T) {
			s, err := NewIndustryScenario(p.path, db, llm, logger)
			if err != nil {
				t.Fatalf("NewIndustryScenario(%s): %v", p.path, err)
			}

			if s.Profile() == nil {
				t.Error("Profile() returned nil")
			}

			if s.CompanyName() == "" {
				t.Error("CompanyName() returned empty")
			}

			if s.IndustryName() == "" {
				t.Error("IndustryName() returned empty")
			}

			t.Logf("✅ %s: Company=%s, Industry=%s, Customers=%d, SupportTeam=%d",
				p.name,
				s.CompanyName(),
				s.IndustryName(),
				s.Profile().TotalCustomers(),
				s.Profile().TotalSupportHeadcount(),
			)
		})
	}
}

func TestIndustryScenarioWithConfig(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	logger := zerolog.Nop()
	llm := &mockLLM{}

	cfg := IndustryConfig{
		MaxAccounts: 2,
		MaxCases:    5,
	}

	s, err := NewIndustryScenarioWithConfig("../../profiles/healthcare_medtech.yaml", db, llm, logger, cfg)
	if err != nil {
		t.Fatalf("NewIndustryScenarioWithConfig: %v", err)
	}

	if s.config.MaxAccounts != 2 {
		t.Errorf("MaxAccounts = %d, want 2", s.config.MaxAccounts)
	}

	if s.config.MaxCases != 5 {
		t.Errorf("MaxCases = %d, want 5", s.config.MaxCases)
	}
}

func TestIndustryScenarioProfile(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	logger := zerolog.Nop()
	llm := &mockLLM{}

	s, err := NewIndustryScenario("../../profiles/healthcare_medtech.yaml", db, llm, logger)
	if err != nil {
		t.Fatalf("NewIndustryScenario: %v", err)
	}

	p := s.Profile()

	// Verify profile has expected structure
	if len(p.Products) == 0 {
		t.Error("Profile has no products")
	}

	if len(p.CustomerSegments) == 0 {
		t.Error("Profile has no customer segments")
	}

	if len(p.SupportTeam.Tiers) == 0 {
		t.Error("Profile has no support tiers")
	}

	t.Logf("Products: %d, Segments: %d, Support Tiers: %d",
		len(p.Products), len(p.CustomerSegments), len(p.SupportTeam.Tiers))
}

