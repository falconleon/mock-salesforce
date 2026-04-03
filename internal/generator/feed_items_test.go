package generator

import (
	"database/sql"
	"testing"

	"github.com/rs/zerolog"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestDBForFeedItems(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	// Create tables
	schema := `
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			first_name TEXT,
			last_name TEXT,
			email TEXT,
			username TEXT,
			title TEXT,
			department TEXT,
			is_active INTEGER,
			manager_id TEXT,
			user_role TEXT,
			created_at TEXT
		);
		CREATE TABLE cases (
			id TEXT PRIMARY KEY,
			case_number TEXT,
			subject TEXT,
			description TEXT,
			status TEXT,
			priority TEXT,
			product TEXT,
			case_type TEXT,
			origin TEXT,
			reason TEXT,
			owner_id TEXT,
			contact_id TEXT,
			account_id TEXT,
			created_at TEXT,
			closed_at TEXT,
			is_closed INTEGER,
			is_escalated INTEGER,
			jira_issue_key TEXT
		);
		CREATE TABLE feed_items (
			id TEXT PRIMARY KEY,
			case_id TEXT,
			body TEXT,
			type TEXT,
			created_by_id TEXT,
			created_at TEXT
		);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	// Insert test data
	_, err = db.Exec(`INSERT INTO users (id, email, is_active) VALUES ('user-001', 'test@example.com', 1)`)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	_, err = db.Exec(`INSERT INTO cases (id, subject, status, priority, owner_id, created_at, is_closed) 
		VALUES ('case-001', 'API timeout issue', 'Working', 'High', 'user-001', '2024-01-15T10:00:00Z', 0)`)
	if err != nil {
		t.Fatalf("insert case: %v", err)
	}

	cleanup := func() {
		db.Close()
	}
	return db, cleanup
}

func TestFeedItemGenerator_Generate(t *testing.T) {
	db, cleanup := setupTestDBForFeedItems(t)
	defer cleanup()

	ctx := &Context{
		DB:     db,
		LLM:    &mockLLM{response: "[]"},
		Logger: zerolog.Nop(),
	}

	gen := NewFeedItemGenerator(ctx)
	if err := gen.Generate(); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	// Verify feed items were inserted
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM feed_items").Scan(&count); err != nil {
		t.Fatalf("count feed items: %v", err)
	}
	if count == 0 {
		t.Error("Expected feed items to be generated, got 0")
	}

	// Verify feed item has correct case reference
	var caseID string
	if err := db.QueryRow("SELECT case_id FROM feed_items LIMIT 1").Scan(&caseID); err != nil {
		t.Fatalf("get feed item case_id: %v", err)
	}
	if caseID != "case-001" {
		t.Errorf("Expected case_id 'case-001', got '%s'", caseID)
	}

	// Verify feed item ID starts with "0D5" (FeedItem prefix)
	var feedItemID string
	if err := db.QueryRow("SELECT id FROM feed_items LIMIT 1").Scan(&feedItemID); err != nil {
		t.Fatalf("get feed item id: %v", err)
	}
	if len(feedItemID) != 18 || feedItemID[:3] != "0D5" {
		t.Errorf("Expected 18-char ID starting with '0D5', got '%s'", feedItemID)
	}
}

func TestFeedItemGenerator_TypeDistribution(t *testing.T) {
	gen := &FeedItemGenerator{}

	typeCounts := make(map[string]int)
	iterations := 1000

	for i := 0; i < iterations; i++ {
		itemType := gen.pickFeedItemType()
		typeCounts[itemType]++
	}

	// Verify all types are represented
	expectedTypes := []string{FeedTypeStatusChange, FeedTypePriorityChange, FeedTypeOwnerChange, FeedTypeComment, FeedTypeAttachment}
	for _, et := range expectedTypes {
		if typeCounts[et] == 0 {
			t.Errorf("Expected type %s to appear at least once in %d iterations", et, iterations)
		}
	}

	// Verify rough distribution (allow 50% variance)
	expectedPcts := map[string]float64{
		FeedTypeStatusChange:   0.30,
		FeedTypePriorityChange: 0.15,
		FeedTypeOwnerChange:    0.20,
		FeedTypeComment:        0.25,
		FeedTypeAttachment:     0.10,
	}

	for itemType, expectedPct := range expectedPcts {
		actualPct := float64(typeCounts[itemType]) / float64(iterations)
		if actualPct < expectedPct*0.5 || actualPct > expectedPct*1.5 {
			t.Errorf("Type %s: expected ~%.0f%%, got %.1f%%", itemType, expectedPct*100, actualPct*100)
		}
	}
}

