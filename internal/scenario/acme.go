// Package scenario provides profile-driven data generation scenarios.
// Each scenario uses a VendorProfile to drive realistic data generation
// matching the specified customer segments, support team structure, etc.
package scenario

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/falconleon/mock-salesforce/internal/generator"
	"github.com/falconleon/mock-salesforce/internal/generator/seed"
	"github.com/falconleon/mock-salesforce/internal/profile"
	"github.com/rs/zerolog"
)

// AcmeScenario generates data for Acme Software based on the vendor profile.
type AcmeScenario struct {
	profile *profile.VendorProfile
	db      *sql.DB
	llm     generator.LLM
	logger  zerolog.Logger
}

// NewAcmeScenario creates a new Acme scenario with the given profile.
func NewAcmeScenario(p *profile.VendorProfile, db *sql.DB, llm generator.LLM, logger zerolog.Logger) *AcmeScenario {
	return &AcmeScenario{
		profile: p,
		db:      db,
		llm:     llm,
		logger:  logger.With().Str("scenario", "acme").Logger(),
	}
}

// GenerateAll runs the complete Acme scenario: customers, contacts, support team.
func (s *AcmeScenario) GenerateAll() error {
	s.logger.Info().Msg("Starting Acme scenario generation")

	if err := s.GenerateCustomers(); err != nil {
		return fmt.Errorf("generate customers: %w", err)
	}

	if err := s.GenerateSupportTeam(); err != nil {
		return fmt.Errorf("generate support team: %w", err)
	}

	s.logger.Info().Msg("Acme scenario generation complete")
	return nil
}

// GenerateCustomers generates accounts and contacts based on customer_segments.
func (s *AcmeScenario) GenerateCustomers() error {
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
		s.logger.Debug().
			Str("segment", segment.Segment).
			Int("count", segment.Count).
			Msg("Generating segment")

		for i := 0; i < segment.Count; i++ {
			// Select industry for this account (round-robin from segment industries)
			industry := segment.Industries[i%len(segment.Industries)]

			// Generate employee count within range
			employeeCount := gofakeit.Number(segment.EmployeeRange[0], segment.EmployeeRange[1])

			// Generate annual revenue within range
			annualRevenue := gofakeit.Number(segment.AnnualContractValueRange[0], segment.AnnualContractValueRange[1])

			// Generate company seed
			companySeed := seed.NewCompanySeed()

			// Generate account via LLM
			account, err := s.generateAccount(industry, segment.Segment, employeeCount, annualRevenue, companySeed)
			if err != nil {
				s.logger.Error().Err(err).Str("segment", segment.Segment).Msg("Failed to generate account")
				continue
			}

			// Insert account
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

			// Generate contacts for this account
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

			s.logger.Debug().
				Str("name", account.Name).
				Str("segment", segment.Segment).
				Int("contacts", contactCount).
				Msg("Generated account with contacts")
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
func (s *AcmeScenario) generateAccount(industry, segment string, employees, revenue int, cs seed.CompanySeed) (*generator.Account, error) {
	prompt := fmt.Sprintf(`Generate a %s company in the %s industry.
The company is based in %s, %s and has approximately %d employees.
It should be a customer of a B2B SaaS vendor.
Company suffix should be: %s

Return ONLY a JSON object with these fields:
- name: realistic B2B company name (must end with "%s")
- website: company website URL
- phone: phone number (555-XXXX format)
- created_at: ISO 8601 timestamp in 2023

Example:
{"name":"Pinnacle %s","website":"https://pinnacle.example.com","phone":"555-0100","created_at":"2023-03-15T10:00:00Z"}`,
		segment, industry, cs.City, cs.State, employees, cs.Suffix, cs.Suffix, cs.Suffix)

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
func (s *AcmeScenario) generateContact(account *generator.Account, industry string, isPrimary bool) (*generator.Contact, error) {
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
func (s *AcmeScenario) GenerateSupportTeam() error {
	s.logger.Info().
		Int("total_headcount", s.profile.TotalSupportHeadcount()).
		Msg("Generating support team")

	// Build role distribution from profile
	roleCounts := make(map[string]int)
	for _, tier := range s.profile.SupportTeam.Tiers {
		roleCounts[tier.Level] = tier.Headcount
	}

	// Create generator context
	ctx := &generator.Context{
		DB:     s.db,
		LLM:    s.llm,
		Logger: s.logger,
	}

	// Configure user generation from profile
	totalCount := s.profile.TotalSupportHeadcount()
	cfg := generator.UserConfig{
		Count: totalCount,
		RoleDistribution: map[string]float64{
			"agent":    float64(roleCounts["L1"]+roleCounts["L2"]) / float64(totalCount),
			"engineer": float64(roleCounts["L3"]) / float64(totalCount),
			"manager":  float64(roleCounts["Manager"]) / float64(totalCount),
		},
	}

	s.logger.Debug().
		Int("l1", roleCounts["L1"]).
		Int("l2", roleCounts["L2"]).
		Int("l3", roleCounts["L3"]).
		Int("manager", roleCounts["Manager"]).
		Msg("Support team distribution from profile")

	userGen := generator.NewUserGenerator(ctx)
	if err := userGen.GenerateWithConfig(cfg); err != nil {
		return fmt.Errorf("generate users: %w", err)
	}

	s.logger.Info().Msg("Support team generation complete")
	return nil
}

// Helper functions

// domainFromName creates a plausible domain from a company name.
func domainFromName(name string) string {
	domain := ""
	for _, c := range name {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			if c >= 'A' && c <= 'Z' {
				c = c + 32
			}
			domain += string(c)
		}
	}
	return domain + ".example.com"
}

// firstNameToEmail converts a first name to lowercase for email.
func firstNameToEmail(name string) string {
	result := ""
	for _, c := range name {
		if c >= 'A' && c <= 'Z' {
			c = c + 32
		}
		if c >= 'a' && c <= 'z' {
			result += string(c)
		}
	}
	return result
}

// lastNameToEmail converts a last name to lowercase for email.
func lastNameToEmail(name string) string {
	return firstNameToEmail(name)
}

// extractJSON extracts JSON from a response that may contain markdown.
func extractJSON(s string) string {
	// Try object first
	startObj := -1
	for i, c := range s {
		if c == '{' {
			startObj = i
			break
		}
	}
	if startObj >= 0 {
		endObj := -1
		for i := len(s) - 1; i >= startObj; i-- {
			if s[i] == '}' {
				endObj = i
				break
			}
		}
		if endObj > startObj {
			return s[startObj : endObj+1]
		}
	}
	return s
}

// GenerateInteractions generates the full interaction history for all customers.
// This includes cases, emails, comments, feed items, and JIRA escalations.
func (s *AcmeScenario) GenerateInteractions() error {
	s.logger.Info().Msg("Starting interaction generation")

	// Create generator context
	ctx := &generator.Context{
		DB:     s.db,
		LLM:    s.llm,
		Logger: s.logger,
	}

	// 1. Generate cases distributed by segment
	if err := s.generateCases(ctx); err != nil {
		return fmt.Errorf("generate cases: %w", err)
	}

	// 2. Generate email threads for each case
	emailGen := generator.NewEmailGenerator(ctx)
	if err := emailGen.Generate(); err != nil {
		return fmt.Errorf("generate emails: %w", err)
	}

	// 3. Generate case comments
	commentGen := generator.NewCommentGenerator(ctx)
	if err := commentGen.Generate(); err != nil {
		return fmt.Errorf("generate comments: %w", err)
	}

	// 4. Generate feed items (activity log)
	feedGen := generator.NewFeedItemGenerator(ctx)
	if err := feedGen.Generate(); err != nil {
		return fmt.Errorf("generate feed items: %w", err)
	}

	// 5. Generate JIRA escalations for high/critical cases
	jiraIssueGen := generator.NewJiraIssueGenerator(ctx)
	if err := jiraIssueGen.Generate(); err != nil {
		return fmt.Errorf("generate jira issues: %w", err)
	}

	// 6. Generate JIRA comments for escalated issues
	jiraCommentGen := generator.NewJiraCommentGenerator(ctx)
	if err := jiraCommentGen.Generate(); err != nil {
		return fmt.Errorf("generate jira comments: %w", err)
	}

	s.logger.Info().Msg("Interaction generation complete")
	return nil
}

// generateCases generates support cases distributed by segment and priority.
// Target: ~200 cases over 12 months
// Distribution by segment: 40% Enterprise, 40% Mid-Market, 20% SMB
// Distribution by priority: 10% Critical, 25% High, 45% Medium, 20% Low
// Distribution by status: 70% Closed, 15% Open, 15% In Progress
func (s *AcmeScenario) generateCases(ctx *generator.Context) error {
	s.logger.Info().Msg("Generating cases from profile")

	// Fetch accounts by segment
	accountsBySegment, err := s.fetchAccountsBySegment()
	if err != nil {
		return fmt.Errorf("fetch accounts: %w", err)
	}

	// Calculate case counts per segment
	// Target: ~200 cases, 40% Enterprise, 40% Mid-Market, 20% SMB
	totalCases := 200
	segmentCaseCounts := map[string]int{
		"Enterprise": int(float64(totalCases) * 0.40), // 80 cases
		"Mid-Market": int(float64(totalCases) * 0.40), // 80 cases
		"SMB":        int(float64(totalCases) * 0.20), // 40 cases
	}

	s.logger.Debug().
		Int("enterprise_cases", segmentCaseCounts["Enterprise"]).
		Int("midmarket_cases", segmentCaseCounts["Mid-Market"]).
		Int("smb_cases", segmentCaseCounts["SMB"]).
		Msg("Case distribution by segment")

	// Fetch support users for assignment
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
			// Round-robin account selection
			account := accounts[i%len(accounts)]

			// Get a contact for this account
			contact, err := s.getContactForAccount(account.id)
			if err != nil {
				s.logger.Warn().Err(err).Str("account", account.name).Msg("No contact found")
				continue
			}

			// Pick priority and status from distribution
			priority := pickPriorityFromDist(priorityDist, i)
			status := pickStatusFromDist(statusDist, i)

			// Pick a random product from profile
			product := s.pickProduct(segment)

			// Pick a random user as owner
			owner := users[gofakeit.Number(0, len(users)-1)]

			// Generate case using LLM or defaults
			caseData, err := s.generateCase(account, contact, priority, status, product, owner)
			if err != nil {
				s.logger.Warn().Err(err).Msg("Failed to generate case")
				continue
			}

			// Determine if should be escalated (20% of high/critical)
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

// accountInfo holds basic account data for case generation.
type accountInfo struct {
	id       string
	name     string
	industry string
	segment  string
}

// contactInfo holds basic contact data for case generation.
type contactInfo struct {
	id        string
	firstName string
	lastName  string
	email     string
}

// caseData holds generated case content.
type caseData struct {
	ID          string
	CaseNumber  string
	Subject     string
	Description string
	CaseType    string
	Origin      string
	Reason      string
	CreatedAt   string
	ClosedAt    string
}

// fetchAccountsBySegment retrieves accounts grouped by segment type.
func (s *AcmeScenario) fetchAccountsBySegment() (map[string][]accountInfo, error) {
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
func (s *AcmeScenario) fetchSupportUsers() ([]string, error) {
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
func (s *AcmeScenario) getContactForAccount(accountID string) (*contactInfo, error) {
	row := s.db.QueryRow(`SELECT id, first_name, last_name, email FROM contacts WHERE account_id = ? LIMIT 1`, accountID)
	var c contactInfo
	if err := row.Scan(&c.id, &c.firstName, &c.lastName, &c.email); err != nil {
		return nil, err
	}
	return &c, nil
}

// pickProduct selects a product based on segment's typical products.
func (s *AcmeScenario) pickProduct(segment string) string {
	// Find segment config from profile
	for _, seg := range s.profile.CustomerSegments {
		if seg.Segment == segment && len(seg.ProductsUsed) > 0 {
			// Pick a product code and find the full name
			code := seg.ProductsUsed[gofakeit.Number(0, len(seg.ProductsUsed)-1)]
			for _, p := range s.profile.Products {
				if p.Code == code {
					return p.Name
				}
			}
			return code
		}
	}
	// Default to first product
	if len(s.profile.Products) > 0 {
		return s.profile.Products[0].Name
	}
	return "Acme Product"
}

// generateCase creates case content using the LLM.
func (s *AcmeScenario) generateCase(account accountInfo, contact *contactInfo, priority, status, product, ownerID string) (*caseData, error) {
	// Generate created date spread over 12 months
	daysAgo := gofakeit.Number(1, 365)
	createdAt := gofakeit.Date().AddDate(0, 0, -daysAgo)

	// Closed date if applicable
	closedAt := ""
	if status == "Closed" {
		resolutionDays := gofakeit.Number(1, 14)
		closed := createdAt.AddDate(0, 0, resolutionDays)
		closedAt = closed.Format("2006-01-02T15:04:05Z")
	}

	prompt := fmt.Sprintf(`Generate a support case for a B2B software company.

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
- reason: Brief reason category (e.g., "Configuration Error", "Missing Documentation", "Software Defect")

Make it realistic and contextual to the company's industry.

Return ONLY a JSON object:
{"subject":"...","description":"...","case_type":"...","origin":"...","reason":"..."}`,
		account.name, account.industry, account.segment,
		contact.firstName, contact.lastName,
		product, priority)

	resp, err := s.llm.Generate(prompt)
	if err != nil {
		// Use default content on LLM failure
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
func (s *AcmeScenario) defaultCaseData(account accountInfo, priority, createdAt, closedAt string) (*caseData, error) {
	subjects := []string{
		"Login issues after password reset",
		"Dashboard performance degradation",
		"Integration sync failing intermittently",
		"Report export shows incorrect data",
		"Unable to create new projects",
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

// pickPriorityFromDist selects a priority from the distribution based on index.
func pickPriorityFromDist(dist []struct {
	priority   string
	percentage float64
}, idx int) string {
	total := 0.0
	for _, d := range dist {
		total += d.percentage
	}

	cumulative := 0.0
	normalized := float64(idx%100) / 100.0
	for _, d := range dist {
		cumulative += d.percentage / total
		if normalized < cumulative {
			return d.priority
		}
	}
	return dist[len(dist)-1].priority
}

// pickStatusFromDist selects a status from the distribution based on index.
func pickStatusFromDist(dist []struct {
	status     string
	percentage float64
}, idx int) string {
	total := 0.0
	for _, d := range dist {
		total += d.percentage
	}

	cumulative := 0.0
	normalized := float64(idx%100) / 100.0
	for _, d := range dist {
		cumulative += d.percentage / total
		if normalized < cumulative {
			return d.status
		}
	}
	return dist[len(dist)-1].status
}

