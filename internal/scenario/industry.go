// Package scenario provides profile-driven data generation scenarios.
package scenario

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/falconleon/mock-salesforce/internal/generator"
	"github.com/falconleon/mock-salesforce/internal/generator/seed"
	"github.com/falconleon/mock-salesforce/internal/profile"
	"github.com/rs/zerolog"
)

// IndustryConfig allows overriding profile-based counts for testing.
type IndustryConfig struct {
	// MaxAccounts limits accounts per segment (0 = use profile counts)
	MaxAccounts int
	// MaxCases limits total cases (0 = use default 200)
	MaxCases int
}

// IndustryScenario generates data for any industry based on a vendor profile.
type IndustryScenario struct {
	profile     *profile.VendorProfile
	profilePath string
	db          *sql.DB
	llm         generator.LLM
	logger      zerolog.Logger
	config      IndustryConfig
}

// NewIndustryScenario creates a new industry scenario with the given profile.
func NewIndustryScenario(profilePath string, db *sql.DB, llm generator.LLM, logger zerolog.Logger) (*IndustryScenario, error) {
	p, err := profile.Load(profilePath)
	if err != nil {
		return nil, fmt.Errorf("load profile: %w", err)
	}

	// Extract industry name from profile path for logging
	baseName := filepath.Base(profilePath)
	industryName := strings.TrimSuffix(baseName, filepath.Ext(baseName))

	return &IndustryScenario{
		profile:     p,
		profilePath: profilePath,
		db:          db,
		llm:         llm,
		logger:      logger.With().Str("scenario", industryName).Logger(),
		config:      IndustryConfig{},
	}, nil
}

// NewIndustryScenarioWithConfig creates a scenario with custom config overrides.
func NewIndustryScenarioWithConfig(profilePath string, db *sql.DB, llm generator.LLM, logger zerolog.Logger, cfg IndustryConfig) (*IndustryScenario, error) {
	s, err := NewIndustryScenario(profilePath, db, llm, logger)
	if err != nil {
		return nil, err
	}
	s.config = cfg
	return s, nil
}

// Profile returns the loaded vendor profile.
func (s *IndustryScenario) Profile() *profile.VendorProfile {
	return s.profile
}

// IndustryName returns the industry name derived from the profile.
func (s *IndustryScenario) IndustryName() string {
	return s.profile.Company.Industry
}

// CompanyName returns the vendor company name from the profile.
func (s *IndustryScenario) CompanyName() string {
	return s.profile.Company.Name
}

// GenerateAll runs the complete scenario: customers, support team, interactions.
func (s *IndustryScenario) GenerateAll() error {
	s.logger.Info().
		Str("company", s.profile.Company.Name).
		Str("industry", s.profile.Company.Industry).
		Msg("Starting industry scenario generation")

	if err := s.GenerateCustomers(); err != nil {
		return fmt.Errorf("generate customers: %w", err)
	}

	if err := s.GenerateSupportTeam(); err != nil {
		return fmt.Errorf("generate support team: %w", err)
	}

	if err := s.GenerateInteractions(); err != nil {
		return fmt.Errorf("generate interactions: %w", err)
	}

	s.logger.Info().Msg("Industry scenario generation complete")
	return nil
}

// GenerateCustomers generates accounts and contacts based on customer_segments.
func (s *IndustryScenario) GenerateCustomers() error {
	s.logger.Info().
		Int("segments", len(s.profile.CustomerSegments)).
		Int("total_customers", s.profile.TotalCustomers()).
		Msg("Generating customer accounts")

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	accountStmt, err := tx.Prepare(`INSERT INTO accounts
		(id, name, industry, type, website, phone, billing_city, billing_state, annual_revenue, num_employees, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare account statement: %w", err)
	}
	defer accountStmt.Close()

	contactStmt, err := tx.Prepare(`INSERT INTO contacts
		(id, account_id, first_name, last_name, email, phone, title, department, is_primary, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare contact statement: %w", err)
	}
	defer contactStmt.Close()

	accountsGenerated := 0
	contactsGenerated := 0

	for _, segment := range s.profile.CustomerSegments {
		count := segment.Count
		if s.config.MaxAccounts > 0 && count > s.config.MaxAccounts {
			count = s.config.MaxAccounts
		}

		s.logger.Debug().
			Str("segment", segment.Segment).
			Int("count", count).
			Msg("Generating segment")

		for i := 0; i < count; i++ {
			industry := segment.Industries[i%len(segment.Industries)]
			employeeCount := gofakeit.Number(segment.EmployeeRange[0], segment.EmployeeRange[1])
			annualRevenue := gofakeit.Number(segment.AnnualContractValueRange[0], segment.AnnualContractValueRange[1])
			companySeed := seed.NewCompanySeed()

			account, err := s.generateAccount(industry, segment.Segment, employeeCount, annualRevenue, companySeed)
			if err != nil {
				s.logger.Error().Err(err).Str("segment", segment.Segment).Msg("Failed to generate account")
				continue
			}

			_, err = accountStmt.Exec(
				account.ID, account.Name, account.Industry, account.Type,
				account.Website, account.Phone, account.BillingCity, account.BillingState,
				account.AnnualRevenue, account.NumEmployees, account.CreatedAt,
			)
			if err != nil {
				s.logger.Error().Err(err).Str("name", account.Name).Msg("Insert account failed")
				continue
			}
			accountsGenerated++

			contactCount := segment.ContactsPerAccount
			for j := 0; j < contactCount; j++ {
				contact, err := s.generateContact(account, industry, j == 0)
				if err != nil {
					s.logger.Error().Err(err).Str("account", account.Name).Msg("Failed to generate contact")
					continue
				}

				isPrimaryInt := 0
				if contact.IsPrimary {
					isPrimaryInt = 1
				}

				_, err = contactStmt.Exec(
					contact.ID, contact.AccountID,
					contact.FirstName, contact.LastName,
					contact.Email, contact.Phone,
					contact.Title, contact.Department,
					isPrimaryInt, contact.CreatedAt,
				)
				if err != nil {
					s.logger.Error().Err(err).Str("email", contact.Email).Msg("Insert contact failed")
					continue
				}
				contactsGenerated++
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	s.logger.Info().
		Int("accounts", accountsGenerated).
		Int("contacts", contactsGenerated).
		Msg("Customer generation complete")

	return nil
}

// generateAccount creates a single account using the LLM.
func (s *IndustryScenario) generateAccount(industry, segment string, employees, revenue int, cs seed.CompanySeed) (*generator.Account, error) {
	prompt := fmt.Sprintf(`Generate a %s company in the %s industry.
The company is based in %s, %s and has approximately %d employees.
It should be a customer of %s (%s).
Company suffix should be: %s

Return ONLY a JSON object with these fields:
- name: realistic B2B company name (must end with "%s")
- website: company website URL
- phone: phone number (555-XXXX format)
- created_at: ISO 8601 timestamp in 2023

Example:
{"name":"Pinnacle %s","website":"https://pinnacle.example.com","phone":"555-0100","created_at":"2023-03-15T10:00:00Z"}`,
		segment, industry, cs.City, cs.State, employees,
		s.profile.Company.Name, s.profile.Company.Industry,
		cs.Suffix, cs.Suffix, cs.Suffix)

	resp, err := s.llm.Generate(prompt)
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	var data struct {
		Name      string `json:"name"`
		Website   string `json:"website"`
		Phone     string `json:"phone"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.Unmarshal([]byte(extractJSON(resp)), &data); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &generator.Account{
		ID:            generator.SalesforceID("Account"),
		Name:          data.Name,
		Industry:      industry,
		Type:          segment,
		Website:       data.Website,
		Phone:         data.Phone,
		BillingCity:   cs.City,
		BillingState:  cs.State,
		AnnualRevenue: float64(revenue),
		NumEmployees:  employees,
		CreatedAt:     data.CreatedAt,
	}, nil
}

// generateContact creates a single contact for an account using the LLM.
func (s *IndustryScenario) generateContact(account *generator.Account, industry string, isPrimary bool) (*generator.Contact, error) {
	roleLevel := seed.RoleSupportL1
	if isPrimary {
		roleLevel = seed.RoleManager
	}
	personSeed := seed.NewPersonSeed(roleLevel)
	domain := domainFromName(account.Name)

	prompt := fmt.Sprintf(`Generate a contact for %s in the %s industry.

Person details:
%s

Generate:
- Job title appropriate for this person at a %s company (if primary contact, use Director/VP level title)
- Department matching the title
- Phone number (555-XXXX format)
- Created date in 2024 (ISO 8601 format)

The person's name is %s %s.
Email should be: %s.%s@%s

Return ONLY a JSON object with fields: title, department, phone, created_at

Example:
{"title":"IT Director","department":"Information Technology","phone":"555-0101","created_at":"2024-02-10T09:00:00Z"}`,
		account.Name, industry,
		personSeed.ToPromptFragment(),
		industry,
		personSeed.FirstName, personSeed.LastName,
		firstNameToEmail(personSeed.FirstName), lastNameToEmail(personSeed.LastName), domain)

	resp, err := s.llm.Generate(prompt)
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	var data struct {
		Title      string `json:"title"`
		Department string `json:"department"`
		Phone      string `json:"phone"`
		CreatedAt  string `json:"created_at"`
	}
	if err := json.Unmarshal([]byte(extractJSON(resp)), &data); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &generator.Contact{
		ID:         generator.SalesforceID("Contact"),
		AccountID:  account.ID,
		FirstName:  personSeed.FirstName,
		LastName:   personSeed.LastName,
		Email:      fmt.Sprintf("%s.%s@%s", firstNameToEmail(personSeed.FirstName), lastNameToEmail(personSeed.LastName), domain),
		Phone:      data.Phone,
		Title:      data.Title,
		Department: data.Department,
		IsPrimary:  isPrimary,
		CreatedAt:  data.CreatedAt,
	}, nil
}

// GenerateSupportTeam generates support users based on support_team configuration.
func (s *IndustryScenario) GenerateSupportTeam() error {
	s.logger.Info().
		Int("total_headcount", s.profile.TotalSupportHeadcount()).
		Msg("Generating support team")

	roleCounts := make(map[string]int)
	for _, tier := range s.profile.SupportTeam.Tiers {
		roleCounts[tier.Level] = tier.Headcount
	}

	ctx := &generator.Context{
		DB:     s.db,
		LLM:    s.llm,
		Logger: s.logger,
	}

	totalCount := s.profile.TotalSupportHeadcount()
	cfg := generator.UserConfig{
		Count: totalCount,
		RoleDistribution: map[string]float64{
			"agent":    float64(roleCounts["L1"]+roleCounts["L2"]) / float64(totalCount),
			"engineer": float64(roleCounts["L3"]) / float64(totalCount),
			"manager":  float64(roleCounts["Manager"]) / float64(totalCount),
		},
	}

	userGen := generator.NewUserGenerator(ctx)
	if err := userGen.GenerateWithConfig(cfg); err != nil {
		return fmt.Errorf("generate users: %w", err)
	}

	s.logger.Info().Msg("Support team generation complete")
	return nil
}

// GenerateInteractions generates cases, emails, comments, feed items, and JIRA escalations.
func (s *IndustryScenario) GenerateInteractions() error {
	s.logger.Info().Msg("Starting interaction generation")

	ctx := &generator.Context{
		DB:     s.db,
		LLM:    s.llm,
		Logger: s.logger,
	}

	if err := s.generateCases(ctx); err != nil {
		return fmt.Errorf("generate cases: %w", err)
	}

	emailGen := generator.NewEmailGenerator(ctx)
	if err := emailGen.Generate(); err != nil {
		return fmt.Errorf("generate emails: %w", err)
	}

	commentGen := generator.NewCommentGenerator(ctx)
	if err := commentGen.Generate(); err != nil {
		return fmt.Errorf("generate comments: %w", err)
	}

	feedGen := generator.NewFeedItemGenerator(ctx)
	if err := feedGen.Generate(); err != nil {
		return fmt.Errorf("generate feed items: %w", err)
	}

	jiraIssueGen := generator.NewJiraIssueGenerator(ctx)
	if err := jiraIssueGen.Generate(); err != nil {
		return fmt.Errorf("generate jira issues: %w", err)
	}

	jiraCommentGen := generator.NewJiraCommentGenerator(ctx)
	if err := jiraCommentGen.Generate(); err != nil {
		return fmt.Errorf("generate jira comments: %w", err)
	}

	s.logger.Info().Msg("Interaction generation complete")
	return nil
}

// generateCases generates support cases distributed by segment and priority.
func (s *IndustryScenario) generateCases(ctx *generator.Context) error {
	s.logger.Info().Msg("Generating cases from profile")

	accountsBySegment, err := s.fetchAccountsBySegment()
	if err != nil {
		return fmt.Errorf("fetch accounts: %w", err)
	}

	totalCases := 200
	if s.config.MaxCases > 0 {
		totalCases = s.config.MaxCases
	}

	segmentCaseCounts := map[string]int{
		"Enterprise": int(float64(totalCases) * 0.40),
		"Mid-Market": int(float64(totalCases) * 0.40),
		"SMB":        int(float64(totalCases) * 0.20),
	}

	users, err := s.fetchSupportUsers()
	if err != nil {
		return fmt.Errorf("fetch users: %w", err)
	}
	if len(users) == 0 {
		return fmt.Errorf("no support users found")
	}

	totalGenerated := 0
	priorityDist := []struct {
		priority   string
		percentage float64
	}{
		{"Critical", 0.10},
		{"High", 0.25},
		{"Medium", 0.45},
		{"Low", 0.20},
	}

	statusDist := []struct {
		status     string
		percentage float64
	}{
		{"Closed", 0.70},
		{"Open", 0.15},
		{"In Progress", 0.15},
	}

	// Generate cases for each segment - write incrementally to survive crashes
	for segment, caseCount := range segmentCaseCounts {
		accounts, ok := accountsBySegment[segment]
		if !ok || len(accounts) == 0 {
			s.logger.Warn().Str("segment", segment).Msg("No accounts found for segment")
			continue
		}

		for i := 0; i < caseCount; i++ {
			account := accounts[i%len(accounts)]

			contact, err := s.getContactForAccount(account.id)
			if err != nil {
				s.logger.Warn().Err(err).Str("account", account.name).Msg("No contact found")
				continue
			}

			priority := pickPriorityFromDist(priorityDist, i)
			status := pickStatusFromDist(statusDist, i)
			product := s.pickProduct(segment)
			owner := users[gofakeit.Number(0, len(users)-1)]

			caseData, err := s.generateCase(account, contact, priority, status, product, owner)
			if err != nil {
				s.logger.Warn().Err(err).Msg("Failed to generate case")
				continue
			}

			isEscalated := 0
			if (priority == "Critical" || priority == "High") && gofakeit.Float64() < 0.20 {
				isEscalated = 1
			}

			isClosed := 0
			if status == "Closed" {
				isClosed = 1
			}

			// Insert each case individually and commit immediately to survive crashes
			_, err = s.db.Exec(`INSERT INTO cases
				(id, case_number, subject, description, status, priority, product, case_type, origin, reason, owner_id, contact_id, account_id, created_at, closed_at, is_closed, is_escalated, jira_issue_key)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				caseData.ID, caseData.CaseNumber, caseData.Subject, caseData.Description,
				status, priority, product, caseData.CaseType, caseData.Origin, caseData.Reason,
				owner, contact.id, account.id, caseData.CreatedAt, caseData.ClosedAt,
				isClosed, isEscalated, "",
			)
			if err != nil {
				s.logger.Error().Err(err).Str("case", caseData.CaseNumber).Msg("Insert case failed")
				continue
			}
			totalGenerated++

			// Progress logging for each case
			s.logger.Info().
				Int("progress", totalGenerated).
				Int("total", totalCases).
				Str("case_number", caseData.CaseNumber).
				Str("segment", segment).
				Msg("Case generated")
		}
	}

	s.logger.Info().Int("cases", totalGenerated).Msg("Case generation complete")
	return nil
}

// fetchAccountsBySegment retrieves accounts grouped by segment type.
func (s *IndustryScenario) fetchAccountsBySegment() (map[string][]accountInfo, error) {
	rows, err := s.db.Query(`SELECT id, name, industry, type FROM accounts`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]accountInfo)
	for rows.Next() {
		var a accountInfo
		if err := rows.Scan(&a.id, &a.name, &a.industry, &a.segment); err != nil {
			return nil, err
		}
		result[a.segment] = append(result[a.segment], a)
	}
	return result, rows.Err()
}

// fetchSupportUsers retrieves support user IDs.
func (s *IndustryScenario) fetchSupportUsers() ([]string, error) {
	rows, err := s.db.Query(`SELECT id FROM users WHERE is_active = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		users = append(users, id)
	}
	return users, rows.Err()
}

// getContactForAccount retrieves a contact for the given account.
func (s *IndustryScenario) getContactForAccount(accountID string) (*contactInfo, error) {
	row := s.db.QueryRow(`SELECT id, first_name, last_name, email FROM contacts WHERE account_id = ? LIMIT 1`, accountID)
	var c contactInfo
	if err := row.Scan(&c.id, &c.firstName, &c.lastName, &c.email); err != nil {
		return nil, err
	}
	return &c, nil
}

// pickProduct selects a product based on segment's typical products.
func (s *IndustryScenario) pickProduct(segment string) string {
	for _, seg := range s.profile.CustomerSegments {
		if seg.Segment == segment && len(seg.ProductsUsed) > 0 {
			code := seg.ProductsUsed[gofakeit.Number(0, len(seg.ProductsUsed)-1)]
			for _, p := range s.profile.Products {
				if p.Code == code {
					return p.Name
				}
			}
			return code
		}
	}
	if len(s.profile.Products) > 0 {
		return s.profile.Products[0].Name
	}
	return "Product"
}

// generateCase creates case content using the LLM.
func (s *IndustryScenario) generateCase(account accountInfo, contact *contactInfo, priority, status, product, ownerID string) (*caseData, error) {
	daysAgo := gofakeit.Number(1, 365)
	createdAt := gofakeit.Date().AddDate(0, 0, -daysAgo)

	closedAt := ""
	if status == "Closed" {
		resolutionDays := gofakeit.Number(1, 14)
		closed := createdAt.AddDate(0, 0, resolutionDays)
		closedAt = closed.Format("2006-01-02T15:04:05Z")
	}

	prompt := fmt.Sprintf(`Generate a support case for %s (%s vendor).

Context:
- Company: %s (%s industry, %s segment)
- Contact: %s %s
- Product: %s
- Priority: %s

Generate a JSON object with:
- subject: A concise case subject line (max 100 chars)
- description: Detailed problem description (2-4 paragraphs)
- case_type: One of "Technical Issue", "Feature Request", "Billing Question", "How-To Question", "Bug Report"
- origin: One of "Email", "Phone", "Web", "Chat"
- reason: Brief reason category

Return ONLY a JSON object:
{"subject":"...","description":"...","case_type":"...","origin":"...","reason":"..."}`,
		s.profile.Company.Name, s.profile.Company.Industry,
		account.name, account.industry, account.segment,
		contact.firstName, contact.lastName,
		product, priority)

	resp, err := s.llm.Generate(prompt)
	if err != nil {
		return s.defaultCaseData(account, priority, createdAt.Format("2006-01-02T15:04:05Z"), closedAt)
	}

	var data struct {
		Subject     string `json:"subject"`
		Description string `json:"description"`
		CaseType    string `json:"case_type"`
		Origin      string `json:"origin"`
		Reason      string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(extractJSON(resp)), &data); err != nil {
		return s.defaultCaseData(account, priority, createdAt.Format("2006-01-02T15:04:05Z"), closedAt)
	}

	return &caseData{
		ID:          generator.SalesforceID("Case"),
		CaseNumber:  generator.NextCaseNumber(),
		Subject:     data.Subject,
		Description: data.Description,
		CaseType:    data.CaseType,
		Origin:      data.Origin,
		Reason:      data.Reason,
		CreatedAt:   createdAt.Format("2006-01-02T15:04:05Z"),
		ClosedAt:    closedAt,
	}, nil
}

// defaultCaseData generates fallback case content.
func (s *IndustryScenario) defaultCaseData(account accountInfo, priority, createdAt, closedAt string) (*caseData, error) {
	subjects := []string{
		"Login issues after password reset",
		"Dashboard performance degradation",
		"Integration sync failing intermittently",
		"Report export shows incorrect data",
		"Unable to access feature",
	}

	descriptions := []string{
		"User is experiencing issues with the system. Please investigate and resolve.",
		"Multiple team members have reported this problem. It started recently and is affecting productivity.",
		"This has been an ongoing issue. We have tried basic troubleshooting but the problem persists.",
	}

	caseTypes := []string{"Technical Issue", "Bug Report", "How-To Question", "Feature Request"}
	origins := []string{"Email", "Phone", "Web", "Chat"}
	reasons := []string{"Configuration Error", "Software Defect", "User Error", "Performance Issue"}

	return &caseData{
		ID:          generator.SalesforceID("Case"),
		CaseNumber:  generator.NextCaseNumber(),
		Subject:     subjects[gofakeit.Number(0, len(subjects)-1)],
		Description: descriptions[gofakeit.Number(0, len(descriptions)-1)],
		CaseType:    caseTypes[gofakeit.Number(0, len(caseTypes)-1)],
		Origin:      origins[gofakeit.Number(0, len(origins)-1)],
		Reason:      reasons[gofakeit.Number(0, len(reasons)-1)],
		CreatedAt:   createdAt,
		ClosedAt:    closedAt,
	}, nil
}

