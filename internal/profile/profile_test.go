package profile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/falconleon/mock-salesforce/internal/profile"
)

func TestLoadAcmeProfile(t *testing.T) {
	// Find the profile file relative to test location
	profilePath := filepath.Join("..", "..", "profiles", "acme_software.yaml")
	
	// Check if file exists
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		t.Skipf("Profile file not found at %s - skipping test", profilePath)
	}

	p, err := profile.Load(profilePath)
	if err != nil {
		t.Fatalf("Failed to load profile: %v", err)
	}

	// Verify company
	t.Run("Company", func(t *testing.T) {
		if p.Company.Name == "" {
			t.Error("Company name is empty")
		}
		if p.Company.Name != "Acme Software Inc." {
			t.Errorf("Expected 'Acme Software Inc.', got '%s'", p.Company.Name)
		}
		t.Logf("Company: %s (founded %d)", p.Company.Name, p.Company.Founded)
	})

	// Verify products
	t.Run("Products", func(t *testing.T) {
		if len(p.Products) != 4 {
			t.Errorf("Expected 4 products, got %d", len(p.Products))
		}
		for _, prod := range p.Products {
			if prod.Code == "" {
				t.Errorf("Product %s has no code", prod.Name)
			}
		}
		t.Logf("Products: %d defined", len(p.Products))
	})

	// Verify customer segments sum to 10
	t.Run("CustomerSegments", func(t *testing.T) {
		total := p.TotalCustomers()
		if total != 10 {
			t.Errorf("Expected 10 total customers, got %d", total)
		}

		expectedSegments := map[string]int{
			"Enterprise": 2,
			"Mid-Market": 4,
			"SMB":        4,
		}
		for _, seg := range p.CustomerSegments {
			expected, ok := expectedSegments[seg.Segment]
			if !ok {
				t.Errorf("Unexpected segment: %s", seg.Segment)
				continue
			}
			if seg.Count != expected {
				t.Errorf("Segment %s: expected %d, got %d", seg.Segment, expected, seg.Count)
			}
		}
		t.Logf("Customer segments: %d segments, %d total", len(p.CustomerSegments), total)
	})

	// Verify support team
	t.Run("SupportTeam", func(t *testing.T) {
		total := p.TotalSupportHeadcount()
		if total != 15 {
			t.Errorf("Expected 15 support headcount, got %d", total)
		}

		tierCounts := make(map[string]int)
		for _, tier := range p.SupportTeam.Tiers {
			tierCounts[tier.Level] = tier.Headcount
		}

		expected := map[string]int{
			"L1":      8,
			"L2":      4,
			"L3":      2,
			"Manager": 1,
		}
		for level, count := range expected {
			if tierCounts[level] != count {
				t.Errorf("Tier %s: expected %d, got %d", level, count, tierCounts[level])
			}
		}
		t.Logf("Support team: %d tiers, %d total headcount", len(p.SupportTeam.Tiers), total)
	})

	// Verify JIRA config
	t.Run("Jira", func(t *testing.T) {
		if p.Jira.ProjectKey != "ACME" {
			t.Errorf("Expected JIRA project key 'ACME', got '%s'", p.Jira.ProjectKey)
		}
		t.Logf("JIRA: project %s", p.Jira.ProjectKey)
	})
}

