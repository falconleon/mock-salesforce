package generator

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/rs/zerolog"
)

// CaseGenerator generates support Case records.
type CaseGenerator struct {
	db     *sql.DB
	llm    LLM
	logger zerolog.Logger
}

// NewCaseGenerator creates a new case generator.
func NewCaseGenerator(ctx *Context) *CaseGenerator {
	return &CaseGenerator{
		db:     ctx.DB,
		llm:    ctx.LLM,
		logger: ctx.Logger.With().Str("generator", "cases").Logger(),
	}
}

// CaseConfig configures case generation.
type CaseConfig struct {
	// Count is the total number of cases to generate.
	Count int
	// PerAccount is the number of cases per account (if > 0, overrides Count).
	PerAccount int
	// StatusDistribution specifies the percentage of each status.
	StatusDistribution map[string]float64
	// PriorityDistribution specifies the percentage of each priority.
	PriorityDistribution map[string]float64
}

// DefaultCaseConfig returns sensible defaults for case distribution.
func DefaultCaseConfig(count int) CaseConfig {
	return CaseConfig{
		Count:      count,
		PerAccount: 0,
		StatusDistribution: map[string]float64{
			"New":       0.10, // 10% just opened
			"Working":   0.30, // 30% in progress
			"Escalated": 0.10, // 10% complex issues
			"Resolved":  0.35, // 35% fixed, awaiting confirmation
			"Closed":    0.15, // 15% complete
		},
		PriorityDistribution: map[string]float64{
			"Low":      0.20, // 20%
			"Medium":   0.40, // 40%
			"High":     0.30, // 30%
			"Critical": 0.10, // 10%
		},
	}
}

// Case represents a generated support case.
type Case struct {
	ID           string `json:"id"`
	CaseNumber   string `json:"case_number"`
	Subject      string `json:"subject"`
	Description  string `json:"description"`
	Status       string `json:"status"`
	Priority     string `json:"priority"`
	Product      string `json:"product"`
	CaseType     string `json:"case_type"`
	Origin       string `json:"origin"`
	Reason       string `json:"reason"`
	OwnerID      string `json:"owner_id"`
	ContactID    string `json:"contact_id"`
	AccountID    string `json:"account_id"`
	CreatedAt    string `json:"created_at"`
	ClosedAt     string `json:"closed_at,omitempty"`
	IsClosed     bool   `json:"is_closed"`
	IsEscalated  bool   `json:"is_escalated"`
	JiraIssueKey string `json:"jira_issue_key,omitempty"`
}

// LLMCaseResponse represents the LLM-generated case content.
type LLMCaseResponse struct {
	Subject     string `json:"subject"`
	Description string `json:"description"`
	CaseType    string `json:"case_type"`
	Origin      string `json:"origin"`
	Reason      string `json:"reason"`
	Product     string `json:"product"`
}

// accountContact holds account and contact information for case generation.
type accountContact struct {
	accountID   string
	accountName string
	industry    string
	contactID   string
	contactName string
	contactTitle string
}

// Generate creates cases with the default configuration.
func (g *CaseGenerator) Generate(count int) error {
	return g.GenerateWithConfig(DefaultCaseConfig(count))
}

// GenerateWithConfig creates cases with custom configuration.
func (g *CaseGenerator) GenerateWithConfig(cfg CaseConfig) error {
	g.logger.Info().Int("count", cfg.Count).Msg("Generating cases")

	// Fetch accounts with their contacts
	accountContacts, err := g.fetchAccountContacts()
	if err != nil {
		return err
	}
	if len(accountContacts) == 0 {
		g.logger.Warn().Msg("No accounts/contacts found, skipping case generation")
		return nil
	}

	// Fetch support users (owners)
	users, err := g.fetchSupportUsers()
	if err != nil {
		return err
	}
	if len(users) == 0 {
		g.logger.Warn().Msg("No support users found, skipping case generation")
		return nil
	}

	// Determine how many cases to generate
	totalCases := cfg.Count
	if cfg.PerAccount > 0 {
		// Count unique accounts
		accountSet := make(map[string]bool)
		for _, ac := range accountContacts {
			accountSet[ac.accountID] = true
		}
		totalCases = len(accountSet) * cfg.PerAccount
	}

	g.logger.Debug().
		Int("accountContacts", len(accountContacts)).
		Int("users", len(users)).
		Int("totalCases", totalCases).
		Msg("Resources loaded")

	// Pre-calculate status and priority distributions
	statusDist := buildDistribution(cfg.StatusDistribution, totalCases)
	priorityDist := buildDistribution(cfg.PriorityDistribution, totalCases)

	// Generate cases
	cases := make([]Case, 0, totalCases)
	for i := 0; i < totalCases; i++ {
		// Pick random account+contact and owner
		ac := accountContacts[rand.Intn(len(accountContacts))]
		owner := users[rand.Intn(len(users))]

		// Determine status and priority from distribution
		status := pickFromDistribution(statusDist, i)
		priority := pickFromDistribution(priorityDist, i)

		// Generate case content via LLM
		caseContent, err := g.generateCaseContent(ac, priority)
		if err != nil {
			g.logger.Warn().Err(err).Int("case", i).Msg("Failed to generate case content, using defaults")
			caseContent = g.defaultCaseContent(ac, priority)
		}

		// Build case
		c := g.buildCase(ac, owner, status, priority, caseContent)
		cases = append(cases, c)

		if (i+1)%10 == 0 {
			g.logger.Debug().Int("progress", i+1).Int("total", totalCases).Msg("Case generation progress")
		}
	}

	// Insert into database
	if err := g.insertCases(cases); err != nil {
		return err
	}

	// Log distribution summary
	g.logDistributionSummary(cases)

	return nil
}

// fetchAccountContacts retrieves accounts with their contacts.
func (g *CaseGenerator) fetchAccountContacts() ([]accountContact, error) {
	rows, err := g.db.Query(`
		SELECT a.id, a.name, a.industry, c.id, c.first_name || ' ' || c.last_name, COALESCE(c.title, '')
		FROM accounts a
		JOIN contacts c ON c.account_id = a.id
		ORDER BY a.id, c.is_primary DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query account contacts: %w", err)
	}
	defer rows.Close()

	var result []accountContact
	for rows.Next() {
		var ac accountContact
		if err := rows.Scan(&ac.accountID, &ac.accountName, &ac.industry,
			&ac.contactID, &ac.contactName, &ac.contactTitle); err != nil {
			return nil, fmt.Errorf("scan account contact: %w", err)
		}
		result = append(result, ac)
	}
	return result, rows.Err()
}

// fetchSupportUsers retrieves support users who can own cases.
func (g *CaseGenerator) fetchSupportUsers() ([]string, error) {
	rows, err := g.db.Query(`
		SELECT id FROM users
		WHERE is_active = 1
		AND (user_role IN ('L1 Support', 'L2 Support', 'L3 Support')
		     OR user_role LIKE '%Agent%'
		     OR user_role LIKE '%Engineer%')
	`)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	var users []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, id)
	}

	// If no users match the role filter, get any active user
	if len(users) == 0 {
		rows2, err := g.db.Query(`SELECT id FROM users WHERE is_active = 1`)
		if err != nil {
			return nil, fmt.Errorf("query all users: %w", err)
		}
		defer rows2.Close()

		for rows2.Next() {
			var id string
			if err := rows2.Scan(&id); err != nil {
				return nil, fmt.Errorf("scan user: %w", err)
			}
			users = append(users, id)
		}
	}

	return users, rows.Err()
}

// generateCaseContent uses LLM to generate realistic case content.
func (g *CaseGenerator) generateCaseContent(ac accountContact, priority string) (*LLMCaseResponse, error) {
	prompt := fmt.Sprintf(`Generate a realistic support case for a B2B software company.

Context:
- Company: %s (%s industry)
- Contact: %s (%s)
- Priority: %s

Generate a JSON object with:
- subject: A concise case subject line (max 100 chars)
- description: Detailed problem description (2-4 paragraphs, include steps to reproduce if applicable)
- case_type: One of "Technical Issue", "Feature Request", "Billing Question", "How-To Question", "Bug Report"
- origin: One of "Email", "Phone", "Web", "Chat"
- reason: Brief reason category (e.g., "Configuration Error", "Missing Documentation", "Software Defect")
- product: A realistic B2B software product name relevant to their industry

Make it realistic and contextual to the company's industry. For high/critical priority, make it more urgent.

Return ONLY a valid JSON object:
{"subject":"...","description":"...","case_type":"...","origin":"...","reason":"...","product":"..."}`,
		ac.accountName, ac.industry, ac.contactName, ac.contactTitle, priority)

	resp, err := g.llm.Generate(prompt)
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	// Extract JSON from response
	jsonStr := extractJSON(resp)
	// Handle object instead of array
	if len(jsonStr) > 0 && jsonStr[0] == '[' {
		jsonStr = jsonStr[1 : len(jsonStr)-1]
	}

	var caseResp LLMCaseResponse
	if err := json.Unmarshal([]byte(jsonStr), &caseResp); err != nil {
		// Try to extract just the object
		start := indexOf(resp, '{')
		end := lastIndexOf(resp, '}')
		if start >= 0 && end > start {
			objStr := resp[start : end+1]
			if err := json.Unmarshal([]byte(objStr), &caseResp); err != nil {
				return nil, fmt.Errorf("parse case response: %w", err)
			}
		} else {
			return nil, fmt.Errorf("parse case response: %w", err)
		}
	}

	return &caseResp, nil
}

// defaultCaseContent returns fallback case content when LLM fails.
func (g *CaseGenerator) defaultCaseContent(ac accountContact, priority string) *LLMCaseResponse {
	subjects := []string{
		"Unable to access dashboard after update",
		"API integration returning errors",
		"Performance degradation in reports",
		"Feature request: Export to CSV",
		"Login issues for multiple users",
		"Data sync not working correctly",
		"Need help configuring SSO",
		"Billing discrepancy on invoice",
	}
	caseTypes := []string{"Technical Issue", "Feature Request", "Bug Report", "How-To Question", "Billing Question"}
	origins := []string{"Email", "Phone", "Web", "Chat"}
	reasons := []string{"Configuration Error", "Software Defect", "User Error", "Missing Documentation", "Integration Issue"}
	products := []string{"Enterprise Suite", "Analytics Pro", "Integration Hub", "Data Platform", "Workflow Manager"}

	return &LLMCaseResponse{
		Subject:     subjects[rand.Intn(len(subjects))],
		Description: fmt.Sprintf("User from %s (%s industry) reported an issue. Priority: %s. Please investigate and resolve.", ac.accountName, ac.industry, priority),
		CaseType:    caseTypes[rand.Intn(len(caseTypes))],
		Origin:      origins[rand.Intn(len(origins))],
		Reason:      reasons[rand.Intn(len(reasons))],
		Product:     products[rand.Intn(len(products))],
	}
}

// buildCase constructs a Case record from the given inputs.
func (g *CaseGenerator) buildCase(ac accountContact, ownerID, status, priority string, content *LLMCaseResponse) Case {
	now := time.Now()
	// Random created date in the past year
	daysAgo := rand.Intn(365)
	createdAt := now.AddDate(0, 0, -daysAgo)

	c := Case{
		ID:          SalesforceID("Case"),
		CaseNumber:  NextCaseNumber(),
		Subject:     content.Subject,
		Description: content.Description,
		Status:      status,
		Priority:    priority,
		Product:     content.Product,
		CaseType:    content.CaseType,
		Origin:      content.Origin,
		Reason:      content.Reason,
		OwnerID:     ownerID,
		ContactID:   ac.contactID,
		AccountID:   ac.accountID,
		CreatedAt:   createdAt.Format(time.RFC3339),
		IsClosed:    status == "Closed",
		IsEscalated: status == "Escalated",
	}

	// Set ClosedAt for resolved/closed cases
	if status == "Resolved" || status == "Closed" {
		// Closed 1-30 days after creation
		closedDaysLater := rand.Intn(30) + 1
		closedAt := createdAt.AddDate(0, 0, closedDaysLater)
		if closedAt.After(now) {
			closedAt = now
		}
		c.ClosedAt = closedAt.Format(time.RFC3339)
	}

	return c
}

// insertCases inserts cases into the database.
func (g *CaseGenerator) insertCases(cases []Case) error {
	tx, err := g.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO cases
		(id, case_number, subject, description, status, priority, product, case_type, origin, reason,
		 owner_id, contact_id, account_id, created_at, closed_at, is_closed, is_escalated, jira_issue_key)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, c := range cases {
		isClosed := 0
		if c.IsClosed {
			isClosed = 1
		}
		isEscalated := 0
		if c.IsEscalated {
			isEscalated = 1
		}

		_, err := stmt.Exec(
			c.ID, c.CaseNumber, c.Subject, c.Description, c.Status, c.Priority,
			c.Product, c.CaseType, c.Origin, c.Reason,
			c.OwnerID, c.ContactID, c.AccountID, c.CreatedAt, c.ClosedAt,
			isClosed, isEscalated, c.JiraIssueKey,
		)
		if err != nil {
			return fmt.Errorf("insert case %s: %w", c.CaseNumber, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	g.logger.Info().Int("inserted", len(cases)).Msg("Cases generated")
	return nil
}

// logDistributionSummary logs the actual distribution of cases.
func (g *CaseGenerator) logDistributionSummary(cases []Case) {
	statusCounts := make(map[string]int)
	priorityCounts := make(map[string]int)

	for _, c := range cases {
		statusCounts[c.Status]++
		priorityCounts[c.Priority]++
	}

	g.logger.Info().
		Interface("statusDistribution", statusCounts).
		Interface("priorityDistribution", priorityCounts).
		Msg("Case distribution summary")
}

// buildDistribution converts percentage distribution to absolute counts.
func buildDistribution(dist map[string]float64, total int) map[string]int {
	result := make(map[string]int)
	remaining := total

	keys := make([]string, 0, len(dist))
	for k := range dist {
		keys = append(keys, k)
	}

	for i, k := range keys {
		if i == len(keys)-1 {
			// Last item gets the remainder
			result[k] = remaining
		} else {
			count := int(float64(total) * dist[k])
			result[k] = count
			remaining -= count
		}
	}

	return result
}

// pickFromDistribution picks an item based on the pre-calculated distribution.
func pickFromDistribution(dist map[string]int, index int) string {
	cumulative := 0
	for k, count := range dist {
		cumulative += count
		if index < cumulative {
			return k
		}
	}
	// Fallback to first key
	for k := range dist {
		return k
	}
	return ""
}

// indexOf returns the index of the first occurrence of char in s, or -1.
func indexOf(s string, char byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == char {
			return i
		}
	}
	return -1
}

// lastIndexOf returns the index of the last occurrence of char in s, or -1.
func lastIndexOf(s string, char byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == char {
			return i
		}
	}
	return -1
}
