package generator

import (
	"database/sql"
	"testing"

	"github.com/rs/zerolog"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestDBForComments(t *testing.T) (*sql.DB, func()) {
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
		CREATE TABLE case_comments (
			id TEXT PRIMARY KEY,
			case_id TEXT,
			comment_body TEXT,
			created_by_id TEXT,
			created_at TEXT,
			is_published INTEGER
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

func TestCommentGenerator_Generate(t *testing.T) {
	db, cleanup := setupTestDBForComments(t)
	defer cleanup()

	mockResp := `[
		{"CommentBody": "Checked logs - seeing timeout errors on the API gateway.", "IsPublished": false},
		{"CommentBody": "Customer confirmed workaround is acceptable.", "IsPublished": false}
	]`

	ctx := &Context{
		DB:     db,
		LLM:    &mockLLM{response: mockResp},
		Logger: zerolog.Nop(),
	}

	gen := NewCommentGenerator(ctx)
	if err := gen.Generate(); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	// Verify comments were inserted
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM case_comments").Scan(&count); err != nil {
		t.Fatalf("count comments: %v", err)
	}
	if count == 0 {
		t.Error("Expected comments to be generated, got 0")
	}

	// Verify comment has correct case reference
	var caseID string
	if err := db.QueryRow("SELECT case_id FROM case_comments LIMIT 1").Scan(&caseID); err != nil {
		t.Fatalf("get comment case_id: %v", err)
	}
	if caseID != "case-001" {
		t.Errorf("Expected case_id 'case-001', got '%s'", caseID)
	}

	// Verify comment ID starts with "00a" (CaseComment prefix)
	var commentID string
	if err := db.QueryRow("SELECT id FROM case_comments LIMIT 1").Scan(&commentID); err != nil {
		t.Fatalf("get comment id: %v", err)
	}
	if len(commentID) != 18 || commentID[:3] != "00a" {
		t.Errorf("Expected 18-char ID starting with '00a', got '%s'", commentID)
	}
}

func TestCommentGenerator_DetermineCommentCount(t *testing.T) {
	gen := &CommentGenerator{}

	tests := []struct {
		priority string
		minCount int
		maxCount int
	}{
		{"Critical", 4, 6},
		{"High", 3, 4},
		{"Medium", 2, 3},
		{"Low", 1, 2},
	}

	for _, tt := range tests {
		t.Run(tt.priority, func(t *testing.T) {
			c := caseInfo{priority: tt.priority}
			count := gen.determineCommentCount(c)
			if count < tt.minCount || count > tt.maxCount {
				t.Errorf("Expected count in [%d, %d] for priority %s, got %d", 
					tt.minCount, tt.maxCount, tt.priority, count)
			}
		})
	}
}

