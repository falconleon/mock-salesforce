// Package profile loads vendor profile configurations from YAML files.
// Profiles drive mock data generation by defining customer segments,
// support team structure, products, and communication patterns.
package profile

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// VendorProfile represents the top-level structure of a vendor profile YAML.
type VendorProfile struct {
	Company          Company           `yaml:"company"`
	Products         []Product         `yaml:"products"`
	SupportTeam      SupportTeam       `yaml:"support_team"`
	CaseTypes        []CaseType        `yaml:"case_types"`
	SLAs             []SLA             `yaml:"slas"`
	CustomerSegments []CustomerSegment `yaml:"customer_segments"`
	Communication    Communication     `yaml:"communication"`
	Jira             JiraConfig        `yaml:"jira"`
}

// Company holds vendor company information.
type Company struct {
	Name          string `yaml:"name"`
	Founded       int    `yaml:"founded"`
	Headquarters  string `yaml:"headquarters"`
	Website       string `yaml:"website"`
	Industry      string `yaml:"industry"`
	Employees     int    `yaml:"employees"`
	AnnualRevenue int    `yaml:"annual_revenue"`
	Tagline       string `yaml:"tagline"`
	Description   string `yaml:"description"`
}

// Product represents a vendor product with issue profiles.
type Product struct {
	Name         string       `yaml:"name"`
	Code         string       `yaml:"code"`
	Type         string       `yaml:"type"`
	Category     string       `yaml:"category"`
	Version      string       `yaml:"version"`
	Tiers        []Tier       `yaml:"tiers"`
	Features     []string     `yaml:"features"`
	CommonIssues []IssueGroup `yaml:"common_issues"`
}

// Tier represents a product pricing tier.
type Tier struct {
	Name         string `yaml:"name"`
	PriceMonthly int    `yaml:"price_monthly"`
	UserLimit    *int   `yaml:"user_limit"`
	FlowLimit    *int   `yaml:"flow_limit"`
}

// IssueGroup groups common issues by category.
type IssueGroup struct {
	Category string   `yaml:"category"`
	Issues   []string `yaml:"issues"`
}

// SupportTeam defines the support organization structure.
type SupportTeam struct {
	Tiers []SupportTier `yaml:"tiers"`
}

// SupportTier represents a support tier level.
type SupportTier struct {
	Level                 string              `yaml:"level"`
	Name                  string              `yaml:"name"`
	Headcount             int                 `yaml:"headcount"`
	ShiftCoverage         string              `yaml:"shift_coverage"`
	Responsibilities      []string            `yaml:"responsibilities"`
	EscalationThreshold   map[string]string   `yaml:"escalation_threshold"`
	KPIs                  map[string]string   `yaml:"kpis"`
	JiraProject           string              `yaml:"jira_project"`
}

// CaseType defines a type of support case with distribution.
type CaseType struct {
	Type       string         `yaml:"type"`
	Percentage int            `yaml:"percentage"`
	Subtypes   []CaseSubtype  `yaml:"subtypes"`
}

// CaseSubtype is a specific subtype within a case category.
type CaseSubtype struct {
	Name            string   `yaml:"name"`
	Percentage      int      `yaml:"percentage"`
	TypicalProducts []string `yaml:"typical_products"`
}

// SLA defines service level agreement parameters.
type SLA struct {
	Priority         string   `yaml:"priority"`
	Label            string   `yaml:"label"`
	Description      string   `yaml:"description"`
	ResponseTime     string   `yaml:"response_time"`
	ResolutionTarget string   `yaml:"resolution_target"`
	Percentage       int      `yaml:"percentage"`
	Examples         []string `yaml:"examples"`
}

// CustomerSegment defines a customer segment with generation parameters.
type CustomerSegment struct {
	Segment                   string   `yaml:"segment"`
	Count                     int      `yaml:"count"`
	EmployeeRange             [2]int   `yaml:"employee_range"`
	Industries                []string `yaml:"industries"`
	ProductsUsed              []string `yaml:"products_used"`
	ContractValue             string   `yaml:"contract_value"`
	AnnualContractValueRange  [2]int   `yaml:"annual_contract_value_range"`
	SupportTier               string   `yaml:"support_tier"`
	ContactsPerAccount        int      `yaml:"contacts_per_account"`
	TypicalCaseVolume         string   `yaml:"typical_case_volume"`
	Characteristics           []string `yaml:"characteristics"`
}

// Communication defines email templates and language patterns.
type Communication struct {
	EmailTemplates      []EmailTemplate       `yaml:"email_templates"`
	LanguagePatterns    LanguagePatterns      `yaml:"language_patterns"`
	CustomerTonePatterns map[string]TonePattern `yaml:"customer_tone_patterns"`
}

// EmailTemplate defines a communication template.
type EmailTemplate struct {
	Scenario       string   `yaml:"scenario"`
	Tone           string   `yaml:"tone"`
	KeyElements    []string `yaml:"key_elements"`
	ExampleOpening string   `yaml:"example_opening"`
}

// LanguagePatterns holds terminology patterns.
type LanguagePatterns struct {
	FormalTerms    []string            `yaml:"formal_terms"`
	TechnicalTerms []string            `yaml:"technical_terms"`
	ProductSpecific map[string][]string `yaml:"product_specific"`
}

// TonePattern describes customer communication patterns.
type TonePattern struct {
	Indicators       []string `yaml:"indicators"`
	ResponseApproach string   `yaml:"response_approach"`
}

// JiraConfig holds JIRA integration settings.
type JiraConfig struct {
	ProjectKey         string             `yaml:"project_key"`
	ProjectName        string             `yaml:"project_name"`
	IssueTypes         []JiraIssueType    `yaml:"issue_types"`
	Components         []JiraComponent    `yaml:"components"`
	Priorities         []JiraPriority     `yaml:"priorities"`
	EscalationCriteria EscalationCriteria `yaml:"escalation_criteria"`
	Workflow           JiraWorkflow       `yaml:"workflow"`
	Labels             []string           `yaml:"labels"`
}

// JiraIssueType defines a JIRA issue type.
type JiraIssueType struct {
	Name            string   `yaml:"name"`
	Percentage      int      `yaml:"percentage"`
	Description     string   `yaml:"description"`
	TypicalPriority []string `yaml:"typical_priority"`
}

// JiraComponent defines a JIRA component.
type JiraComponent struct {
	Name string `yaml:"name"`
	Code string `yaml:"code"`
	Lead string `yaml:"lead"`
}

// JiraPriority defines a JIRA priority level.
type JiraPriority struct {
	Name         string `yaml:"name"`
	Description  string `yaml:"description"`
	ResponseTime string `yaml:"response_time"`
}

// EscalationCriteria defines when issues get escalated.
type EscalationCriteria struct {
	Automatic []string `yaml:"automatic"`
	Manual    []string `yaml:"manual"`
}

// JiraWorkflow defines the JIRA workflow.
type JiraWorkflow struct {
	Statuses    []string           `yaml:"statuses"`
	Transitions []WorkflowTransition `yaml:"transitions"`
}

// WorkflowTransition defines a JIRA workflow transition.
type WorkflowTransition struct {
	From   string `yaml:"from"`
	To     string `yaml:"to"`
	Action string `yaml:"action"`
}

// Load reads a vendor profile from a YAML file.
func Load(path string) (*VendorProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read profile: %w", err)
	}

	var profile VendorProfile
	if err := yaml.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("parse profile: %w", err)
	}

	return &profile, nil
}

// TotalCustomers returns the total customer count across all segments.
func (p *VendorProfile) TotalCustomers() int {
	total := 0
	for _, seg := range p.CustomerSegments {
		total += seg.Count
	}
	return total
}

// TotalSupportHeadcount returns the total support team headcount.
func (p *VendorProfile) TotalSupportHeadcount() int {
	total := 0
	for _, tier := range p.SupportTeam.Tiers {
		total += tier.Headcount
	}
	return total
}

