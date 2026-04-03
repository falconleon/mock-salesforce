package generator

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// CommentGenerator generates CaseComment records.
type CommentGenerator struct {
	db     *sql.DB
	llm    LLM
	logger zerolog.Logger
}

// NewCommentGenerator creates a new comment generator.
func NewCommentGenerator(ctx *Context) *CommentGenerator {
	return &CommentGenerator{
		db:     ctx.DB,
		llm:    ctx.LLM,
		logger: ctx.Logger.With().Str("generator", "comments").Logger(),
	}
}

// CaseComment represents a generated internal comment.
type CaseComment struct {
	ID          string `json:"id"`
	CaseID      string `json:"case_id"`
	CommentBody string `json:"comment_body"`
	CreatedByID string `json:"created_by_id"`
	CreatedAt   string `json:"created_at"`
	IsPublished bool   `json:"is_published"`
}

// LLMCommentResponse represents a comment from the LLM response.
type LLMCommentResponse struct {
	CommentBody string `json:"CommentBody"`
	IsPublished bool   `json:"IsPublished"`
}

// caseInfo holds case details for comment generation.
type caseInfo struct {
	id        string
	subject   string
	status    string
	priority  string
	ownerID   string
	createdAt time.Time
	closedAt  *time.Time
}

// Generate creates comments for all cases in the database.
// Comments are written incrementally per case to survive crashes.
func (g *CommentGenerator) Generate() error {
	g.logger.Info().Msg("Generating case comments")

	// 1. Fetch all cases with their owners
	cases, err := g.fetchCases()
	if err != nil {
		return fmt.Errorf("fetch cases: %w", err)
	}
	if len(cases) == 0 {
		g.logger.Warn().Msg("No cases found to generate comments for")
		return nil
	}

	// 2. Fetch all users (for random author selection)
	users, err := g.fetchUsers()
	if err != nil {
		return fmt.Errorf("fetch users: %w", err)
	}
	if len(users) == 0 {
		g.logger.Warn().Msg("No users found to assign as comment authors")
		return nil
	}

	// 3. Generate and insert comments incrementally per case
	totalComments := 0
	for i, c := range cases {
		comments, err := g.generateCommentsForCase(c, users)
		if err != nil {
			g.logger.Warn().Err(err).Str("case_id", c.id).Msg("Failed to generate comments, using defaults")
			comments = g.defaultComments(c, users)
		}

		// Insert comments for this case immediately
		if len(comments) > 0 {
			if err := g.insertComments(comments); err != nil {
				g.logger.Error().Err(err).Str("case_id", c.id).Msg("Failed to insert comments")
				continue
			}
			totalComments += len(comments)
		}

		// Progress logging
		g.logger.Info().
			Int("progress", i+1).
			Int("total", len(cases)).
			Str("case_id", c.id).
			Int("comments", len(comments)).
			Msg("Comments generated for case")
	}

	g.logger.Info().Int("count", totalComments).Msg("Case comments generated")
	return nil
}

// fetchCases retrieves all cases with relevant details.
func (g *CommentGenerator) fetchCases() ([]caseInfo, error) {
	rows, err := g.db.Query(`
		SELECT id, subject, status, priority, owner_id, created_at, closed_at
		FROM cases
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cases []caseInfo
	for rows.Next() {
		var c caseInfo
		var createdAtStr string
		var closedAtStr sql.NullString
		if err := rows.Scan(&c.id, &c.subject, &c.status, &c.priority, &c.ownerID, &createdAtStr, &closedAtStr); err != nil {
			return nil, err
		}
		c.createdAt, _ = time.Parse(time.RFC3339, createdAtStr)
		if closedAtStr.Valid {
			t, _ := time.Parse(time.RFC3339, closedAtStr.String)
			c.closedAt = &t
		}
		cases = append(cases, c)
	}
	return cases, rows.Err()
}

// fetchUsers retrieves all active user IDs.
func (g *CommentGenerator) fetchUsers() ([]string, error) {
	rows, err := g.db.Query(`SELECT id FROM users WHERE is_active = 1`)
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

// generateCommentsForCase generates comments for a single case using LLM.
func (g *CommentGenerator) generateCommentsForCase(c caseInfo, users []string) ([]CaseComment, error) {
	// Determine comment count based on case complexity
	count := g.determineCommentCount(c)
	if count == 0 {
		return nil, nil
	}

	// Use LLM to generate comment content
	prompt := fmt.Sprintf(`Generate %d internal support comments for a case with:
Subject: %s
Status: %s
Priority: %s

Comments should be internal notes by support staff, including:
- Troubleshooting steps taken
- Customer context and observations
- Handoff notes between agents
- Technical findings and diagnosis
- Next steps and action items

Return ONLY a JSON array with objects containing:
- CommentBody: the internal note text (50-200 chars)
- IsPublished: always false for internal comments

Example:
[{"CommentBody":"Checked logs - seeing timeout errors on their API gateway. Requested network team access.","IsPublished":false}]`, count, c.subject, c.status, c.priority)

	resp, err := g.llm.Generate(prompt)
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	// Parse LLM response
	jsonStr := extractJSON(resp)
	var llmComments []LLMCommentResponse
	if err := json.Unmarshal([]byte(jsonStr), &llmComments); err != nil {
		return nil, fmt.Errorf("parse llm response: %w", err)
	}

	// Build CaseComment objects with timestamps
	return g.buildComments(c, llmComments, users), nil
}

// determineCommentCount returns the number of comments to generate based on case complexity.
func (g *CommentGenerator) determineCommentCount(c caseInfo) int {
	// High priority or escalated cases get more comments
	switch c.priority {
	case "Critical":
		return 4 + rand.Intn(3) // 4-6 comments
	case "High":
		return 3 + rand.Intn(2) // 3-4 comments
	case "Medium":
		return 2 + rand.Intn(2) // 2-3 comments
	default:
		return 1 + rand.Intn(2) // 1-2 comments
	}
}

// buildComments creates CaseComment objects from LLM responses with proper timestamps.
func (g *CommentGenerator) buildComments(c caseInfo, llmComments []LLMCommentResponse, users []string) []CaseComment {
	comments := make([]CaseComment, 0, len(llmComments))

	// Calculate time range for comments
	endTime := time.Now()
	if c.closedAt != nil {
		endTime = *c.closedAt
	}
	duration := endTime.Sub(c.createdAt)

	for i, lc := range llmComments {
		// Distribute comments evenly across case timeline
		progress := float64(i+1) / float64(len(llmComments)+1)
		commentTime := c.createdAt.Add(time.Duration(float64(duration) * progress))

		// Pick author: prefer case owner, occasionally other users
		authorID := c.ownerID
		if rand.Float64() < 0.3 && len(users) > 1 {
			authorID = users[rand.Intn(len(users))]
		}

		comments = append(comments, CaseComment{
			ID:          SalesforceID("CaseComment"),
			CaseID:      c.id,
			CommentBody: lc.CommentBody,
			CreatedByID: authorID,
			CreatedAt:   commentTime.Format(time.RFC3339),
			IsPublished: lc.IsPublished,
		})
	}

	// Sort by timestamp
	sort.Slice(comments, func(i, j int) bool {
		return comments[i].CreatedAt < comments[j].CreatedAt
	})

	return comments
}

// defaultComments generates fallback comments when LLM fails.
func (g *CommentGenerator) defaultComments(c caseInfo, users []string) []CaseComment {
	defaultBodies := []string{
		"Initial assessment complete. Investigating root cause.",
		"Checked customer environment - no obvious configuration issues.",
		"Escalating to engineering for further analysis.",
		"Customer confirmed workaround is acceptable for now.",
		"Logs collected and attached to case.",
		"Followed up with customer via phone. Additional details captured.",
	}

	count := g.determineCommentCount(c)
	if count > len(defaultBodies) {
		count = len(defaultBodies)
	}

	// Use buildComments with default content
	llmComments := make([]LLMCommentResponse, count)
	for i := 0; i < count; i++ {
		llmComments[i] = LLMCommentResponse{
			CommentBody: defaultBodies[rand.Intn(len(defaultBodies))],
			IsPublished: false,
		}
	}

	return g.buildComments(c, llmComments, users)
}

// insertComments inserts all comments in a batch transaction.
func (g *CommentGenerator) insertComments(comments []CaseComment) error {
	if len(comments) == 0 {
		return nil
	}

	tx, err := g.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO case_comments (id, case_id, comment_body, created_by_id, created_at, is_published)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, c := range comments {
		isPublished := 0
		if c.IsPublished {
			isPublished = 1
		}
		if _, err := stmt.Exec(c.ID, c.CaseID, c.CommentBody, c.CreatedByID, c.CreatedAt, isPublished); err != nil {
			return fmt.Errorf("insert comment %s: %w", c.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

// extractJSON is defined in users.go, redeclare if needed
func init() {
	// Ensure extractJSON is available - it's defined in users.go
	_ = strings.Index
}
