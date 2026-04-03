package generator

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// JiraIssueGenerator generates JIRA Issue records for escalated cases.
type JiraIssueGenerator struct {
	db     *sql.DB
	llm    LLM
	logger zerolog.Logger
}

// NewJiraIssueGenerator creates a new JIRA issue generator.
func NewJiraIssueGenerator(ctx *Context) *JiraIssueGenerator {
	return &JiraIssueGenerator{
		db:     ctx.DB,
		llm:    ctx.LLM,
		logger: ctx.Logger.With().Str("generator", "jira_issues").Logger(),
	}
}

// JiraIssue represents a generated JIRA issue.
type JiraIssue struct {
	ID             string `json:"id"`
	Key            string `json:"key"`
	ProjectKey     string `json:"project_key"`
	Summary        string `json:"summary"`
	DescriptionADF string `json:"description_adf"`
	IssueType      string `json:"issue_type"`
	Status         string `json:"status"`
	Priority       string `json:"priority"`
	AssigneeID     string `json:"assignee_id,omitempty"`
	ReporterID     string `json:"reporter_id,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
	Labels         string `json:"labels,omitempty"`
	SFCaseID       string `json:"sf_case_id,omitempty"`
}

// escalatedCase holds case info for JIRA issue generation.
type escalatedCase struct {
	id          string
	caseNumber  string
	subject     string
	description string
	priority    string
	product     string
	createdAt   string
}

// jiraUser holds JIRA user info for assignment.
type jiraUser struct {
	accountID   string
	displayName string
}

// LLMJiraIssueResponse represents LLM-generated issue content.
type LLMJiraIssueResponse struct {
	Summary     string   `json:"summary"`
	Description string   `json:"description"`
	Labels      []string `json:"labels"`
}

// JIRA issue key counter for sequential numbering.
var jiraIssueKeyCounter atomic.Int64

func init() {
	jiraIssueKeyCounter.Store(1000)
}

// NextJiraIssueKey generates the next JIRA issue key.
func NextJiraIssueKey(projectKey string) string {
	return fmt.Sprintf("%s-%d", projectKey, jiraIssueKeyCounter.Add(1))
}

// Status distribution for JIRA issues:
// 15% To Do, 35% In Progress, 20% In Review, 30% Done
var jiraStatusDistribution = map[string]float64{
	"To Do":       0.15,
	"In Progress": 0.35,
	"In Review":   0.20,
	"Done":        0.30,
}

// Generate creates JIRA issues for escalated Salesforce cases.
func (g *JiraIssueGenerator) Generate() error {
	g.logger.Info().Msg("Generating JIRA issues")

	// 1. Fetch escalated cases
	escalatedCases, err := g.fetchEscalatedCases()
	if err != nil {
		return fmt.Errorf("fetch escalated cases: %w", err)
	}
	if len(escalatedCases) == 0 {
		g.logger.Info().Msg("No escalated cases found, skipping JIRA issue generation")
		return nil
	}

	// 2. Fetch JIRA users for assignment
	jiraUsers, err := g.fetchJiraUsers()
	if err != nil {
		return fmt.Errorf("fetch jira users: %w", err)
	}
	if len(jiraUsers) == 0 {
		g.logger.Warn().Msg("No JIRA users found, skipping JIRA issue generation")
		return nil
	}

	g.logger.Debug().
		Int("escalatedCases", len(escalatedCases)).
		Int("jiraUsers", len(jiraUsers)).
		Msg("Resources loaded")

	// Pre-calculate status distribution
	statusDist := buildDistribution(jiraStatusDistribution, len(escalatedCases))

	// 3. Generate and insert JIRA issues incrementally
	var allIssues []JiraIssue
	for i, c := range escalatedCases {
		// Pick random assignee and reporter
		assignee := jiraUsers[rand.Intn(len(jiraUsers))]
		reporter := jiraUsers[rand.Intn(len(jiraUsers))]

		// Determine status from distribution
		status := pickFromDistribution(statusDist, i)

		// Generate issue content via LLM
		issueContent, err := g.generateIssueContent(c)
		if err != nil {
			g.logger.Warn().Err(err).Str("caseNumber", c.caseNumber).Msg("Failed to generate issue content, using defaults")
			issueContent = g.defaultIssueContent(c)
		}

		// Build issue
		issue := g.buildIssue(c, assignee, reporter, status, issueContent)

		// Insert this issue immediately to survive crashes
		if err := g.insertIssueAndUpdateCase(issue); err != nil {
			g.logger.Error().Err(err).Str("caseNumber", c.caseNumber).Msg("Failed to insert JIRA issue")
			continue
		}

		allIssues = append(allIssues, issue)

		// Progress logging for each issue
		g.logger.Info().
			Int("progress", i+1).
			Int("total", len(escalatedCases)).
			Str("jira_key", issue.Key).
			Str("case_number", c.caseNumber).
			Msg("JIRA issue generated")
	}

	g.logDistributionSummary(allIssues)

	g.logger.Info().Int("count", len(allIssues)).Msg("JIRA issues generated")
	return nil
}

// fetchEscalatedCases retrieves cases marked as escalated.
func (g *JiraIssueGenerator) fetchEscalatedCases() ([]escalatedCase, error) {
	rows, err := g.db.Query(`
		SELECT id, case_number, subject, description, priority, COALESCE(product, ''), created_at
		FROM cases
		WHERE is_escalated = 1
		ORDER BY created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cases []escalatedCase
	for rows.Next() {
		var c escalatedCase
		if err := rows.Scan(&c.id, &c.caseNumber, &c.subject, &c.description, &c.priority, &c.product, &c.createdAt); err != nil {
			return nil, err
		}
		cases = append(cases, c)
	}
	return cases, rows.Err()
}

// fetchJiraUsers retrieves JIRA users for assignment.
func (g *JiraIssueGenerator) fetchJiraUsers() ([]jiraUser, error) {
	rows, err := g.db.Query(`
		SELECT account_id, display_name
		FROM jira_users
		WHERE active = 1
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []jiraUser
	for rows.Next() {
		var u jiraUser
		if err := rows.Scan(&u.accountID, &u.displayName); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// generateIssueContent uses LLM to generate technical issue content.
func (g *JiraIssueGenerator) generateIssueContent(c escalatedCase) (*LLMJiraIssueResponse, error) {
	prompt := fmt.Sprintf(`Generate a JIRA engineering issue for an escalated support case.

SF Case Number: %s
SF Case Subject: %s
SF Case Description: %s
Priority: %s
Product: %s

Create a technical JIRA issue with:
- Summary: A technical title suitable for engineering (different from support case title)
- Description: Technical details including problem analysis, potential root causes, stack traces, and reproduction steps
- Labels: Array of relevant technical tags (e.g., "api", "performance", "customer-escalation", "bug")

Return ONLY a JSON object:
{"summary": "...", "description": "...", "labels": ["...", "..."]}`, c.caseNumber, c.subject, c.description, c.priority, c.product)

	resp, err := g.llm.Generate(prompt)
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	// Clean response (remove markdown code blocks if present)
	resp = cleanJSONResponse(resp)

	var result LLMJiraIssueResponse
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return nil, fmt.Errorf("parse llm response: %w", err)
	}

	return &result, nil
}

// defaultIssueContent generates fallback content without LLM.
func (g *JiraIssueGenerator) defaultIssueContent(c escalatedCase) *LLMJiraIssueResponse {
	summaries := []string{
		"Investigate customer-reported issue",
		"Debug and fix escalated production issue",
		"Root cause analysis for customer escalation",
		"Performance investigation for reported issue",
	}

	descriptions := []string{
		fmt.Sprintf("Escalated from support case %s.\n\nOriginal issue: %s\n\nRequires engineering investigation.", c.caseNumber, c.subject),
		fmt.Sprintf("Customer escalation requiring immediate attention.\n\nCase: %s\nDescription: %s", c.caseNumber, c.description),
	}

	labels := []string{"customer-escalation", "support"}
	if c.priority == "Critical" || c.priority == "High" {
		labels = append(labels, "high-priority")
	}
	if c.product != "" {
		labels = append(labels, strings.ToLower(strings.ReplaceAll(c.product, " ", "-")))
	}

	return &LLMJiraIssueResponse{
		Summary:     summaries[rand.Intn(len(summaries))],
		Description: descriptions[rand.Intn(len(descriptions))],
		Labels:      labels,
	}
}

// buildIssue constructs a JiraIssue from the given inputs.
func (g *JiraIssueGenerator) buildIssue(c escalatedCase, assignee, reporter jiraUser, status string, content *LLMJiraIssueResponse) JiraIssue {
	// Parse case created time and add offset for JIRA issue creation
	caseCreated, _ := time.Parse(time.RFC3339, c.createdAt)
	issueCreated := caseCreated.Add(time.Duration(rand.Intn(48)+1) * time.Hour) // 1-48 hours after case
	issueUpdated := issueCreated.Add(time.Duration(rand.Intn(72)) * time.Hour)  // 0-72 hours after creation

	projectKey := "ENG"
	labelsJSON, _ := json.Marshal(content.Labels)

	return JiraIssue{
		ID:             JiraAccountID(), // 24-char hex ID
		Key:            NextJiraIssueKey(projectKey),
		ProjectKey:     projectKey,
		Summary:        content.Summary,
		DescriptionADF: g.toADF(content.Description),
		IssueType:      "Bug",
		Status:         status,
		Priority:       mapCasePriorityToJira(c.priority),
		AssigneeID:     assignee.accountID,
		ReporterID:     reporter.accountID,
		CreatedAt:      issueCreated.Format(time.RFC3339),
		UpdatedAt:      issueUpdated.Format(time.RFC3339),
		Labels:         string(labelsJSON),
		SFCaseID:       c.id,
	}
}

// toADF converts plain text to Atlassian Document Format (ADF).
func (g *JiraIssueGenerator) toADF(text string) string {
	// Split into paragraphs
	paragraphs := strings.Split(text, "\n\n")
	content := make([]map[string]any, 0, len(paragraphs))

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		// Check if it's a code block
		if strings.HasPrefix(para, "```") {
			code := strings.TrimPrefix(para, "```")
			code = strings.TrimSuffix(code, "```")
			content = append(content, map[string]any{
				"type": "codeBlock",
				"content": []map[string]any{
					{"type": "text", "text": code},
				},
			})
		} else {
			content = append(content, map[string]any{
				"type": "paragraph",
				"content": []map[string]any{
					{"type": "text", "text": para},
				},
			})
		}
	}

	adf := map[string]any{
		"version": 1,
		"type":    "doc",
		"content": content,
	}

	result, _ := json.Marshal(adf)
	return string(result)
}

// mapCasePriorityToJira maps Salesforce case priority to JIRA priority.
func mapCasePriorityToJira(casePriority string) string {
	switch casePriority {
	case "Critical":
		return "Highest"
	case "High":
		return "High"
	case "Medium":
		return "Medium"
	case "Low":
		return "Low"
	default:
		return "Medium"
	}
}

// insertIssueAndUpdateCase inserts a single JIRA issue and updates the corresponding case.
// This is used for incremental writes to survive crashes.
func (g *JiraIssueGenerator) insertIssueAndUpdateCase(issue JiraIssue) error {
	tx, err := g.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert the issue
	_, err = tx.Exec(`INSERT INTO jira_issues
		(id, key, project_key, summary, description_adf, issue_type, status, priority, assignee_id, reporter_id, created_at, updated_at, labels, sf_case_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		issue.ID, issue.Key, issue.ProjectKey, issue.Summary, issue.DescriptionADF,
		issue.IssueType, issue.Status, issue.Priority, issue.AssigneeID, issue.ReporterID,
		issue.CreatedAt, issue.UpdatedAt, issue.Labels, issue.SFCaseID,
	)
	if err != nil {
		return fmt.Errorf("insert issue %s: %w", issue.Key, err)
	}

	// Update case with JIRA key for bidirectional linking
	if _, err := tx.Exec(`UPDATE cases SET jira_issue_key = ? WHERE id = ?`, issue.Key, issue.SFCaseID); err != nil {
		return fmt.Errorf("update case %s with jira key: %w", issue.SFCaseID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// logDistributionSummary logs the actual distribution of issues.
func (g *JiraIssueGenerator) logDistributionSummary(issues []JiraIssue) {
	statusCounts := make(map[string]int)
	priorityCounts := make(map[string]int)

	for _, issue := range issues {
		statusCounts[issue.Status]++
		priorityCounts[issue.Priority]++
	}

	g.logger.Info().
		Interface("statusDistribution", statusCounts).
		Interface("priorityDistribution", priorityCounts).
		Msg("JIRA issue distribution summary")
}

// cleanJSONResponse removes markdown code blocks if present.
func cleanJSONResponse(resp string) string {
	resp = strings.TrimSpace(resp)
	if strings.HasPrefix(resp, "```json") {
		resp = strings.TrimPrefix(resp, "```json")
		resp = strings.TrimSuffix(resp, "```")
	} else if strings.HasPrefix(resp, "```") {
		resp = strings.TrimPrefix(resp, "```")
		resp = strings.TrimSuffix(resp, "```")
	}
	return strings.TrimSpace(resp)
}
