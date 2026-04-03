package generator

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// JiraCommentGenerator generates JIRA Comment records.
type JiraCommentGenerator struct {
	db     *sql.DB
	llm    LLM
	logger zerolog.Logger
}

// NewJiraCommentGenerator creates a new JIRA comment generator.
func NewJiraCommentGenerator(ctx *Context) *JiraCommentGenerator {
	return &JiraCommentGenerator{
		db:     ctx.DB,
		llm:    ctx.LLM,
		logger: ctx.Logger.With().Str("generator", "jira_comments").Logger(),
	}
}

// JiraComment represents a generated JIRA comment.
type JiraComment struct {
	ID        string `json:"id"`
	IssueID   string `json:"issue_id"`
	AuthorID  string `json:"author_id"`
	BodyADF   string `json:"body_adf"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// jiraIssueInfo holds JIRA issue info for comment generation.
type jiraIssueInfo struct {
	id        string
	key       string
	summary   string
	status    string
	priority  string
	createdAt string
}

// LLMJiraCommentResponse represents LLM-generated comment content.
type LLMJiraCommentResponse struct {
	Comments []string `json:"comments"`
}

// Comment templates for different issue statuses.
var commentTemplatesByStatus = map[string][]string{
	"To Do": {
		"Taking a look at this. Initial analysis suggests %s.",
		"Added to sprint backlog. Will start investigation soon.",
		"Reviewed the customer ticket. Need to reproduce this issue first.",
	},
	"In Progress": {
		"Found the root cause. The issue is in %s - working on a fix now.",
		"Debugging in progress. Stack trace points to connection handling logic.",
		"Identified the issue. It's a race condition in the request handler. PR coming soon.",
		"Made some progress. The timeout is caused by an unoptimized query. Profiling now.",
	},
	"In Review": {
		"PR #%d submitted for review. Key changes: optimized query and added retry logic.",
		"Fix is ready. Please review PR #%d - added comprehensive test coverage.",
		"Awaiting code review. The fix includes performance improvements and edge case handling.",
	},
	"Done": {
		"Fix deployed to production. Monitoring for any regressions.",
		"Verified the fix with the customer. Issue is resolved.",
		"Merged and deployed. Customer confirmed the issue is fixed.",
	},
}

// Generate creates comments for all JIRA issues in the database.
func (g *JiraCommentGenerator) Generate() error {
	g.logger.Info().Msg("Generating JIRA comments")

	// 1. Fetch all JIRA issues
	issues, err := g.fetchJiraIssues()
	if err != nil {
		return fmt.Errorf("fetch jira issues: %w", err)
	}
	if len(issues) == 0 {
		g.logger.Info().Msg("No JIRA issues found, skipping comment generation")
		return nil
	}

	// 2. Fetch JIRA users for authoring comments
	jiraUsers, err := g.fetchJiraUsers()
	if err != nil {
		return fmt.Errorf("fetch jira users: %w", err)
	}
	if len(jiraUsers) == 0 {
		g.logger.Warn().Msg("No JIRA users found, skipping comment generation")
		return nil
	}

	g.logger.Debug().
		Int("issues", len(issues)).
		Int("jiraUsers", len(jiraUsers)).
		Msg("Resources loaded")

	// 3. Generate comments for each issue
	allComments := make([]JiraComment, 0)
	for i, issue := range issues {
		// Determine comment count based on status (1-5 comments, more for in-progress issues)
		commentCount := g.determineCommentCount(issue.status)

		// Generate comments via LLM or defaults
		comments, err := g.generateCommentsForIssue(issue, jiraUsers, commentCount)
		if err != nil {
			g.logger.Warn().Err(err).Str("issue", issue.key).Msg("Failed to generate comments, using defaults")
			comments = g.defaultComments(issue, jiraUsers, commentCount)
		}

		allComments = append(allComments, comments...)

		if (i+1)%10 == 0 {
			g.logger.Debug().Int("progress", i+1).Int("total", len(issues)).Msg("Comment generation progress")
		}
	}

	// 4. Insert into database
	if err := g.insertComments(allComments); err != nil {
		return err
	}

	g.logger.Info().
		Int("issues", len(issues)).
		Int("comments", len(allComments)).
		Msg("JIRA comments generated")

	return nil
}

// fetchJiraIssues retrieves all JIRA issues.
func (g *JiraCommentGenerator) fetchJiraIssues() ([]jiraIssueInfo, error) {
	rows, err := g.db.Query(`
		SELECT id, key, summary, status, priority, created_at
		FROM jira_issues
		ORDER BY created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []jiraIssueInfo
	for rows.Next() {
		var i jiraIssueInfo
		if err := rows.Scan(&i.id, &i.key, &i.summary, &i.status, &i.priority, &i.createdAt); err != nil {
			return nil, err
		}
		issues = append(issues, i)
	}
	return issues, rows.Err()
}

// fetchJiraUsers retrieves active JIRA users.
func (g *JiraCommentGenerator) fetchJiraUsers() ([]jiraUser, error) {
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

// determineCommentCount returns the number of comments based on issue status.
func (g *JiraCommentGenerator) determineCommentCount(status string) int {
	switch status {
	case "To Do":
		return 1 + rand.Intn(2) // 1-2 comments
	case "In Progress":
		return 2 + rand.Intn(4) // 2-5 comments
	case "In Review":
		return 2 + rand.Intn(3) // 2-4 comments
	case "Done":
		return 3 + rand.Intn(3) // 3-5 comments
	default:
		return 2 + rand.Intn(2) // 2-3 comments
	}
}

// generateCommentsForIssue uses LLM to generate comments.
func (g *JiraCommentGenerator) generateCommentsForIssue(issue jiraIssueInfo, users []jiraUser, count int) ([]JiraComment, error) {
	prompt := fmt.Sprintf(`Generate %d JIRA comments for a technical engineering discussion.

JIRA Issue: %s
Summary: %s
Status: %s
Priority: %s

Generate a realistic progression of engineering comments including:
- Initial investigation notes
- Debugging findings with code references
- Technical discussion about root cause
- Fix implementation details
- Resolution notes (if status is Done or In Review)

Return ONLY a JSON object:
{"comments": ["First comment text...", "Second comment text..."]}

Each comment should be 1-3 sentences, technical, and include specific details like:
- File paths (e.g., "src/api/handlers.go")
- Function names
- Error messages or stack traces
- PR numbers`, count, issue.key, issue.summary, issue.status, issue.priority)

	resp, err := g.llm.Generate(prompt)
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	// Clean response
	resp = cleanJSONResponse(resp)

	var result LLMJiraCommentResponse
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return nil, fmt.Errorf("parse llm response: %w", err)
	}

	// Build JiraComment structs
	return g.buildComments(issue, users, result.Comments)
}

// defaultComments generates fallback comments without LLM.
func (g *JiraCommentGenerator) defaultComments(issue jiraIssueInfo, users []jiraUser, count int) []JiraComment {
	commentTexts := make([]string, 0, count)

	// Get templates for this status
	templates := commentTemplatesByStatus[issue.status]
	if len(templates) == 0 {
		templates = commentTemplatesByStatus["In Progress"]
	}

	// Generate comment progression
	for i := 0; i < count; i++ {
		template := templates[i%len(templates)]
		var text string

		// Fill in placeholders
		if strings.Contains(template, "%d") {
			text = fmt.Sprintf(template, 4500+rand.Intn(1000))
		} else if strings.Contains(template, "%s") {
			components := []string{"the connection pool", "the auth middleware", "the rate limiter", "the database layer"}
			text = fmt.Sprintf(template, components[rand.Intn(len(components))])
		} else {
			text = template
		}

		commentTexts = append(commentTexts, text)
	}

	comments, _ := g.buildComments(issue, users, commentTexts)
	return comments
}

// buildComments creates JiraComment structs from text.
func (g *JiraCommentGenerator) buildComments(issue jiraIssueInfo, users []jiraUser, texts []string) ([]JiraComment, error) {
	comments := make([]JiraComment, 0, len(texts))

	// Parse issue creation time
	issueCreated, _ := time.Parse(time.RFC3339, issue.createdAt)
	currentTime := issueCreated

	for _, text := range texts {
		if strings.TrimSpace(text) == "" {
			continue
		}

		// Comments are spaced hours to days apart
		hoursOffset := rand.Intn(48) + 1
		currentTime = currentTime.Add(time.Duration(hoursOffset) * time.Hour)

		author := users[rand.Intn(len(users))]

		comment := JiraComment{
			ID:        JiraAccountID(), // 24-char hex ID
			IssueID:   issue.id,
			AuthorID:  author.accountID,
			BodyADF:   g.textToADF(text),
			CreatedAt: currentTime.Format(time.RFC3339),
			UpdatedAt: currentTime.Format(time.RFC3339),
		}

		comments = append(comments, comment)
	}

	return comments, nil
}

// textToADF converts plain text to ADF format.
func (g *JiraCommentGenerator) textToADF(text string) string {
	// Handle potential code blocks in the text
	paragraphs := strings.Split(text, "\n\n")
	content := make([]map[string]any, 0, len(paragraphs))

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		// Check for inline code
		if strings.Contains(para, "`") {
			// Parse inline code
			content = append(content, g.parseInlineCode(para))
		} else {
			content = append(content, map[string]any{
				"type": "paragraph",
				"content": []map[string]any{
					{"type": "text", "text": para},
				},
			})
		}
	}

	if len(content) == 0 {
		content = append(content, map[string]any{
			"type": "paragraph",
			"content": []map[string]any{
				{"type": "text", "text": text},
			},
		})
	}

	adf := map[string]any{
		"version": 1,
		"type":    "doc",
		"content": content,
	}

	result, _ := json.Marshal(adf)
	return string(result)
}

// parseInlineCode parses text with inline code markers.
func (g *JiraCommentGenerator) parseInlineCode(text string) map[string]any {
	parts := strings.Split(text, "`")
	content := make([]map[string]any, 0)

	for i, part := range parts {
		if part == "" {
			continue
		}
		if i%2 == 1 {
			// Code segment
			content = append(content, map[string]any{
				"type": "text",
				"text": part,
				"marks": []map[string]any{
					{"type": "code"},
				},
			})
		} else {
			// Normal text
			content = append(content, map[string]any{
				"type": "text",
				"text": part,
			})
		}
	}

	return map[string]any{
		"type":    "paragraph",
		"content": content,
	}
}

// insertComments inserts all comments in a transaction.
func (g *JiraCommentGenerator) insertComments(comments []JiraComment) error {
	tx, err := g.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO jira_comments
		(id, issue_id, author_id, body_adf, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, c := range comments {
		if _, err := stmt.Exec(c.ID, c.IssueID, c.AuthorID, c.BodyADF, c.CreatedAt, c.UpdatedAt); err != nil {
			return fmt.Errorf("insert comment %s: %w", c.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
