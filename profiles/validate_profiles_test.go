package profiles_test

import (
	"path/filepath"
	"testing"

	"github.com/falconleon/mock-salesforce/internal/profile"
)

// TestAllIndustryProfiles validates all 5 industry profile templates
func TestAllIndustryProfiles(t *testing.T) {
	profiles := []struct {
		file     string
		company  string
		industry string
	}{
		{"healthcare_medtech.yaml", "MedTech Solutions", "Healthcare IT"},
		{"finserv_fincore.yaml", "FinCore Systems", "Financial Services"},
		{"saas_cloudops.yaml", "CloudOps Platform", "SaaS / B2B Technology"},
		{"retail_retailedge.yaml", "RetailEdge", "Retail Technology"},
		{"manufacturing_factoryos.yaml", "FactoryOS", "Industrial Technology"},
	}

	for _, tc := range profiles {
		t.Run(tc.file, func(t *testing.T) {
			path := filepath.Join(".", tc.file)
			p, err := profile.Load(path)
			if err != nil {
				t.Fatalf("Failed to load %s: %v", tc.file, err)
			}

			// Section 1: Company
			t.Run("Company", func(t *testing.T) {
				if p.Company.Name == "" {
					t.Error("Company name is empty")
				}
				if p.Company.Founded <= 1900 {
					t.Error("Founded year invalid")
				}
				t.Logf("Company: %s (founded %d)", p.Company.Name, p.Company.Founded)
			})

			// Section 2: Products (need 4)
			t.Run("Products", func(t *testing.T) {
				if len(p.Products) < 4 {
					t.Errorf("Expected 4 products, got %d", len(p.Products))
				}
				for _, prod := range p.Products {
					if len(prod.CommonIssues) == 0 {
						t.Errorf("Product %s has no common_issues", prod.Name)
					}
				}
				t.Logf("Products: %d defined", len(p.Products))
			})

			// Section 3: Support Team (need 15 total)
			t.Run("SupportTeam", func(t *testing.T) {
				if p.TotalSupportHeadcount() != 15 {
					t.Errorf("Expected 15 headcount, got %d", p.TotalSupportHeadcount())
				}
				t.Logf("Support team: %d tiers, %d total headcount",
					len(p.SupportTeam.Tiers), p.TotalSupportHeadcount())
			})

			// Section 4: Case Types (must sum to 100%)
			t.Run("CaseTypes", func(t *testing.T) {
				sum := 0
				for _, ct := range p.CaseTypes {
					sum += ct.Percentage
				}
				if sum != 100 {
					t.Errorf("Case types sum to %d%%, expected 100%%", sum)
				}
				t.Logf("Case types: %d types summing to %d%%", len(p.CaseTypes), sum)
			})

			// Section 4: SLAs (must sum to 100%)
			t.Run("SLAs", func(t *testing.T) {
				sum := 0
				for _, sla := range p.SLAs {
					sum += sla.Percentage
				}
				if sum != 100 {
					t.Errorf("SLAs sum to %d%%, expected 100%%", sum)
				}
				t.Logf("SLAs: %d priorities summing to %d%%", len(p.SLAs), sum)
			})

			// Section 5: Customer Segments (need 10 total)
			t.Run("CustomerSegments", func(t *testing.T) {
				if p.TotalCustomers() != 10 {
					t.Errorf("Expected 10 customers, got %d", p.TotalCustomers())
				}
				t.Logf("Customer segments: %d segments, %d total customers",
					len(p.CustomerSegments), p.TotalCustomers())
			})

			// Section 6: Communication
			t.Run("Communication", func(t *testing.T) {
				if len(p.Communication.EmailTemplates) == 0 {
					t.Error("No email templates defined")
				}
				t.Logf("Communication: %d email templates", len(p.Communication.EmailTemplates))
			})

			// Section 7: JIRA
			t.Run("Jira", func(t *testing.T) {
				if p.Jira.ProjectKey == "" {
					t.Error("JIRA project key is empty")
				}
				if len(p.Jira.IssueTypes) == 0 {
					t.Error("No JIRA issue types defined")
				}
				t.Logf("JIRA: project %s, %d issue types",
					p.Jira.ProjectKey, len(p.Jira.IssueTypes))
			})
		})
	}
}

