package generator

import (
	"database/sql"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/rs/zerolog"
)

// FeedItemGenerator generates FeedItem records.
type FeedItemGenerator struct {
	db     *sql.DB
	llm    LLM
	logger zerolog.Logger
}

// NewFeedItemGenerator creates a new feed item generator.
func NewFeedItemGenerator(ctx *Context) *FeedItemGenerator {
	return &FeedItemGenerator{
		db:     ctx.DB,
		llm:    ctx.LLM,
		logger: ctx.Logger.With().Str("generator", "feed_items").Logger(),
	}
}

// FeedItem represents a generated activity feed entry.
type FeedItem struct {
	ID          string `json:"id"`
	CaseID      string `json:"case_id"`
	Body        string `json:"body"`
	Type        string `json:"type"`
	CreatedByID string `json:"created_by_id"`
	CreatedAt   string `json:"created_at"`
}

// FeedItemType constants for activity types.
const (
	FeedTypeStatusChange   = "StatusChange"
	FeedTypePriorityChange = "PriorityChange"
	FeedTypeOwnerChange    = "OwnerChange"
	FeedTypeComment        = "Comment"
	FeedTypeAttachment     = "Attachment"
)

// feedItemTypeDistribution defines the distribution of feed item types.
// StatusChange: 30%, PriorityChange: 15%, OwnerChange: 20%, Comment: 25%, Attachment: 10%
var feedItemTypeDistribution = []struct {
	itemType string
	weight   int
}{
	{FeedTypeStatusChange, 30},
	{FeedTypePriorityChange, 15},
	{FeedTypeOwnerChange, 20},
	{FeedTypeComment, 25},
	{FeedTypeAttachment, 10},
}

// feedCaseInfo holds case details for feed item generation.
type feedCaseInfo struct {
	id        string
	status    string
	priority  string
	ownerID   string
	createdAt time.Time
	closedAt  *time.Time
	isClosed  bool
}

// Generate creates feed items for all cases in the database.
// Feed items are written incrementally per case to survive crashes.
func (g *FeedItemGenerator) Generate() error {
	g.logger.Info().Msg("Generating feed items")

	// 1. Fetch all cases
	cases, err := g.fetchCases()
	if err != nil {
		return fmt.Errorf("fetch cases: %w", err)
	}
	if len(cases) == 0 {
		g.logger.Warn().Msg("No cases found to generate feed items for")
		return nil
	}

	// 2. Fetch all users for random selection
	users, err := g.fetchUsers()
	if err != nil {
		return fmt.Errorf("fetch users: %w", err)
	}
	if len(users) == 0 {
		g.logger.Warn().Msg("No users found to assign as feed item authors")
		return nil
	}

	// 3. Generate and insert feed items incrementally per case
	totalFeedItems := 0
	for i, c := range cases {
		items := g.generateFeedItemsForCase(c, users)

		// Insert feed items for this case immediately
		if len(items) > 0 {
			if err := g.insertFeedItems(items); err != nil {
				g.logger.Error().Err(err).Str("case_id", c.id).Msg("Failed to insert feed items")
				continue
			}
			totalFeedItems += len(items)
		}

		// Progress logging
		g.logger.Info().
			Int("progress", i+1).
			Int("total", len(cases)).
			Str("case_id", c.id).
			Int("feed_items", len(items)).
			Msg("Feed items generated for case")
	}

	g.logger.Info().Int("count", totalFeedItems).Msg("Feed items generated")
	return nil
}

// fetchCases retrieves all cases with relevant details.
func (g *FeedItemGenerator) fetchCases() ([]feedCaseInfo, error) {
	rows, err := g.db.Query(`
		SELECT id, status, priority, owner_id, created_at, closed_at, is_closed
		FROM cases
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cases []feedCaseInfo
	for rows.Next() {
		var c feedCaseInfo
		var createdAtStr string
		var closedAtStr sql.NullString
		var isClosed int
		if err := rows.Scan(&c.id, &c.status, &c.priority, &c.ownerID, &createdAtStr, &closedAtStr, &isClosed); err != nil {
			return nil, err
		}
		c.createdAt, _ = time.Parse(time.RFC3339, createdAtStr)
		c.isClosed = isClosed == 1
		if closedAtStr.Valid {
			t, _ := time.Parse(time.RFC3339, closedAtStr.String)
			c.closedAt = &t
		}
		cases = append(cases, c)
	}
	return cases, rows.Err()
}

// fetchUsers retrieves all active user IDs.
func (g *FeedItemGenerator) fetchUsers() ([]string, error) {
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

// generateFeedItemsForCase generates feed items for a single case.
func (g *FeedItemGenerator) generateFeedItemsForCase(c feedCaseInfo, users []string) []FeedItem {
	// Determine item count based on case status
	count := g.determineFeedItemCount(c)
	if count == 0 {
		return nil
	}

	items := make([]FeedItem, 0, count)

	// Calculate time range for items
	endTime := time.Now()
	if c.closedAt != nil {
		endTime = *c.closedAt
	}
	duration := endTime.Sub(c.createdAt)

	// Always start with a case creation status change
	items = append(items, FeedItem{
		ID:          SalesforceID("FeedItem"),
		CaseID:      c.id,
		Body:        "Status changed from New to Open",
		Type:        FeedTypeStatusChange,
		CreatedByID: c.ownerID,
		CreatedAt:   c.createdAt.Add(time.Minute).Format(time.RFC3339),
	})

	// Generate remaining items based on distribution
	for i := 1; i < count; i++ {
		itemType := g.pickFeedItemType()
		progress := float64(i+1) / float64(count+1)
		itemTime := c.createdAt.Add(time.Duration(float64(duration) * progress))

		// Pick author: prefer case owner
		authorID := c.ownerID
		if rand.Float64() < 0.3 && len(users) > 1 {
			authorID = users[rand.Intn(len(users))]
		}

		body := g.generateFeedItemBody(itemType, c)

		items = append(items, FeedItem{
			ID:          SalesforceID("FeedItem"),
			CaseID:      c.id,
			Body:        body,
			Type:        itemType,
			CreatedByID: authorID,
			CreatedAt:   itemTime.Format(time.RFC3339),
		})
	}

	// If case is closed, add final status change
	if c.isClosed && c.closedAt != nil {
		items = append(items, FeedItem{
			ID:          SalesforceID("FeedItem"),
			CaseID:      c.id,
			Body:        "Status changed from Working to Closed",
			Type:        FeedTypeStatusChange,
			CreatedByID: c.ownerID,
			CreatedAt:   c.closedAt.Add(-time.Minute).Format(time.RFC3339),
		})
	}

	// Sort by timestamp
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt < items[j].CreatedAt
	})

	return items
}

// determineFeedItemCount returns the number of feed items based on case complexity.
func (g *FeedItemGenerator) determineFeedItemCount(c feedCaseInfo) int {
	// More activity for closed and high-priority cases
	base := 2
	if c.isClosed {
		base += 2
	}
	switch c.priority {
	case "Critical":
		return base + 3 + rand.Intn(3) // 5-9 items
	case "High":
		return base + 2 + rand.Intn(2) // 4-7 items
	case "Medium":
		return base + 1 + rand.Intn(2) // 3-5 items
	default:
		return base + rand.Intn(2) // 2-3 items
	}
}

// pickFeedItemType selects a feed item type based on distribution.
func (g *FeedItemGenerator) pickFeedItemType() string {
	totalWeight := 0
	for _, d := range feedItemTypeDistribution {
		totalWeight += d.weight
	}

	r := rand.Intn(totalWeight)
	cumulative := 0
	for _, d := range feedItemTypeDistribution {
		cumulative += d.weight
		if r < cumulative {
			return d.itemType
		}
	}

	return FeedTypeStatusChange
}

// generateFeedItemBody creates appropriate body text for each feed item type.
func (g *FeedItemGenerator) generateFeedItemBody(itemType string, c feedCaseInfo) string {
	switch itemType {
	case FeedTypeStatusChange:
		transitions := []string{
			"Status changed from New to Working",
			"Status changed from Working to Waiting on Customer",
			"Status changed from Waiting on Customer to Working",
			"Status changed from Working to Escalated",
			"Status changed from Escalated to Working",
		}
		return transitions[rand.Intn(len(transitions))]

	case FeedTypePriorityChange:
		changes := []string{
			"Priority changed from Medium to High",
			"Priority changed from Low to Medium",
			"Priority changed from High to Critical",
			"Priority changed from Medium to Low",
		}
		return changes[rand.Intn(len(changes))]

	case FeedTypeOwnerChange:
		names := []string{"John Smith", "Sarah Chen", "Michael Johnson", "Emily Brown", "David Wilson"}
		return fmt.Sprintf("Owner changed to %s", names[rand.Intn(len(names))])

	case FeedTypeComment:
		comments := []string{
			"Posted internal update on case progress",
			"Added technical notes from investigation",
			"Documented customer feedback",
			"Updated resolution timeline",
			"Shared workaround with team",
		}
		return comments[rand.Intn(len(comments))]

	case FeedTypeAttachment:
		files := []string{
			"Attached log file: system_logs.txt",
			"Attached screenshot: error_screenshot.png",
			"Attached config file: settings.json",
			"Attached diagnostic report: diag_report.pdf",
		}
		return files[rand.Intn(len(files))]

	default:
		return "Activity recorded"
	}
}

// insertFeedItems inserts all feed items in a batch transaction.
func (g *FeedItemGenerator) insertFeedItems(items []FeedItem) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := g.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO feed_items (id, case_id, body, type, created_by_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, f := range items {
		if _, err := stmt.Exec(f.ID, f.CaseID, f.Body, f.Type, f.CreatedByID, f.CreatedAt); err != nil {
			return fmt.Errorf("insert feed item %s: %w", f.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}
