package profiles_test

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// VendorProfile represents the top-level structure of a vendor profile YAML
type VendorProfile struct {
	Company          Company            `yaml:"company"`
	Products         []Product          `yaml:"products"`
	SupportTeam      SupportTeam        `yaml:"support_team"`
	CaseTypes        []CaseType         `yaml:"case_types"`
	SLAs             []SLA              `yaml:"slas"`
	CustomerSegments []CustomerSegment  `yaml:"customer_segments"`
	Communication    Communication      `yaml:"communication"`
	Jira             JiraConfig         `yaml:"jira"`
}

type Company struct {
	Name         string `yaml:"name"`
	Founded      int    `yaml:"founded"`
	Headquarters string `yaml:"headquarters"`
	Website      string `yaml:"website"`
	Employees    int    `yaml:"employees"`
}

type Product struct {
	Name         string        `yaml:"name"`
	Code         string        `yaml:"code"`
	Type         string        `yaml:"type"`
	Category     string        `yaml:"category"`
	CommonIssues []IssueGroup  `yaml:"common_issues"`
}

type IssueGroup struct {
	Category string   `yaml:"category"`
	Issues   []string `yaml:"issues"`
}

type SupportTeam struct {
	Tiers []SupportTier `yaml:"tiers"`
}

type SupportTier struct {
	Level     string `yaml:"level"`
	Headcount int    `yaml:"headcount"`
}

type CaseType struct {
	Type       string `yaml:"type"`
	Percentage int    `yaml:"percentage"`
}

type SLA struct {
	Priority   string `yaml:"priority"`
	Percentage int    `yaml:"percentage"`
}

type CustomerSegment struct {
	Segment string `yaml:"segment"`
	Count   int    `yaml:"count"`
}

type Communication struct {
	EmailTemplates   []any `yaml:"email_templates"`
	LanguagePatterns any   `yaml:"language_patterns"`
}

type JiraConfig struct {
	ProjectKey string `yaml:"project_key"`
	IssueTypes []any  `yaml:"issue_types"`
}

func TestAcmeSoftwareProfile(t *testing.T) {
	data, err := os.ReadFile("acme_software.yaml")
	if err != nil {
		t.Fatalf("Failed to read YAML file: %v", err)
	}

	var profile VendorProfile
	if err := yaml.Unmarshal(data, &profile); err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	// Section 1: Company Information
	t.Run("Section1_Company", func(t *testing.T) {
		if profile.Company.Name == "" {
			t.Error("Company name is empty")
		}
		if profile.Company.Founded < 1900 {
			t.Errorf("Invalid founded year: %d", profile.Company.Founded)
		}
		t.Logf("Company: %s (founded %d)", profile.Company.Name, profile.Company.Founded)
	})

	// Section 2: Products
	t.Run("Section2_Products", func(t *testing.T) {
		if len(profile.Products) < 3 {
			t.Errorf("Expected at least 3 products, got %d", len(profile.Products))
		}
		for _, p := range profile.Products {
			if len(p.CommonIssues) == 0 {
				t.Errorf("Product %s has no common issues defined", p.Name)
			}
		}
		t.Logf("Products: %d defined", len(profile.Products))
	})

	// Section 3: Support Team
	t.Run("Section3_SupportTeam", func(t *testing.T) {
		if len(profile.SupportTeam.Tiers) == 0 {
			t.Error("No support tiers defined")
		}
		totalHeadcount := 0
		for _, tier := range profile.SupportTeam.Tiers {
			totalHeadcount += tier.Headcount
		}
		t.Logf("Support team: %d tiers, %d total headcount", len(profile.SupportTeam.Tiers), totalHeadcount)
	})

	// Section 4: Case Types and SLAs
	t.Run("Section4_CaseTypes", func(t *testing.T) {
		totalPct := 0
		for _, ct := range profile.CaseTypes {
			totalPct += ct.Percentage
		}
		if totalPct != 100 {
			t.Errorf("Case type percentages sum to %d, expected 100", totalPct)
		}
		t.Logf("Case types: %d types summing to %d%%", len(profile.CaseTypes), totalPct)
	})

	t.Run("Section4_SLAs", func(t *testing.T) {
		totalPct := 0
		for _, sla := range profile.SLAs {
			totalPct += sla.Percentage
		}
		if totalPct != 100 {
			t.Errorf("SLA percentages sum to %d, expected 100", totalPct)
		}
		t.Logf("SLAs: %d priorities summing to %d%%", len(profile.SLAs), totalPct)
	})

	// Section 5: Customer Segments
	t.Run("Section5_CustomerSegments", func(t *testing.T) {
		totalCustomers := 0
		for _, seg := range profile.CustomerSegments {
			totalCustomers += seg.Count
		}
		if totalCustomers != 10 {
			t.Errorf("Customer segments total %d, expected 10", totalCustomers)
		}
		t.Logf("Customer segments: %d segments, %d total customers", len(profile.CustomerSegments), totalCustomers)
	})

	// Section 6: Communication
	t.Run("Section6_Communication", func(t *testing.T) {
		if len(profile.Communication.EmailTemplates) == 0 {
			t.Error("No email templates defined")
		}
		t.Logf("Communication: %d email templates", len(profile.Communication.EmailTemplates))
	})

	// Section 7: JIRA
	t.Run("Section7_Jira", func(t *testing.T) {
		if profile.Jira.ProjectKey == "" {
			t.Error("JIRA project key is empty")
		}
		if len(profile.Jira.IssueTypes) == 0 {
			t.Error("No JIRA issue types defined")
		}
		t.Logf("JIRA: project %s, %d issue types", profile.Jira.ProjectKey, len(profile.Jira.IssueTypes))
	})
}

