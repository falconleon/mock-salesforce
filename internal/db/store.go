package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
)

// Store wraps the SQLite database for mock data storage.
type Store struct {
	db     *sql.DB
	logger zerolog.Logger
}

// Open opens or creates the SQLite database at the given path.
func Open(dbPath string, logger zerolog.Logger) (*Store, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	s := &Store{
		db:     db,
		logger: logger.With().Str("component", "db").Logger(),
	}

	return s, nil
}

// Init creates all tables if they don't exist.
func (s *Store) Init() error {
	s.logger.Info().Msg("Initializing database schema")
	_, err := s.db.Exec(Schema)
	if err != nil {
		return fmt.Errorf("execute schema: %w", err)
	}
	return nil
}

// Reset drops all tables and recreates them.
func (s *Store) Reset() error {
	s.logger.Warn().Msg("Resetting database - all data will be deleted")

	tables := []string{
		"profile_images",
		"jira_comments", "jira_issues", "jira_users",
		"feed_items", "case_comments", "email_messages",
		"cases", "contacts", "users", "accounts",
	}

	for _, table := range tables {
		if _, err := s.db.Exec("DROP TABLE IF EXISTS " + table); err != nil {
			return fmt.Errorf("drop table %s: %w", table, err)
		}
	}

	return s.Init()
}

// MigrateProfileImages adds the profile_images table if it doesn't exist.
// This is used to add the table to existing databases without full reset.
func (s *Store) MigrateProfileImages() error {
	s.logger.Info().Msg("Migrating: adding profile_images table if not exists")

	schema := `
CREATE TABLE IF NOT EXISTS profile_images (
    id              TEXT PRIMARY KEY,
    persona_type    TEXT NOT NULL,
    persona_id      TEXT NOT NULL,
    image_path      TEXT NOT NULL,
    first_name      TEXT,
    last_name       TEXT,
    age             INTEGER,
    gender          TEXT,
    ethnicity       TEXT,
    hair_color      TEXT,
    hair_style      TEXT,
    eye_color       TEXT,
    glasses         INTEGER NOT NULL DEFAULT 0,
    facial_hair     TEXT,
    generated_at    TEXT,
    prompt          TEXT,
    UNIQUE(persona_type, persona_id)
);`
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("create profile_images table: %w", err)
	}
	return nil
}

// DB returns the underlying sql.DB for direct queries.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close closes the database connection.
func (s *Store) Close() error {
	s.logger.Info().Msg("Closing database")
	return s.db.Close()
}

// Stats returns row counts for all tables.
func (s *Store) Stats() (map[string]int, error) {
	tables := []string{
		"accounts", "contacts", "users", "cases",
		"email_messages", "case_comments", "feed_items",
		"jira_issues", "jira_comments", "jira_users",
		"profile_images",
	}

	stats := make(map[string]int, len(tables))
	for _, table := range tables {
		var count int
		row := s.db.QueryRow("SELECT COUNT(*) FROM " + table)
		if err := row.Scan(&count); err != nil {
			// Table may not exist yet
			stats[table] = 0
			continue
		}
		stats[table] = count
	}

	return stats, nil
}
