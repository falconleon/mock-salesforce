package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore is a SQLite-backed implementation of Store.
type SQLiteStore struct {
	db *sql.DB
	mu sync.RWMutex

	// indexedFields: objectType -> []fieldNames to index
	indexedFields map[string][]string
}

// NewSQLiteStore creates a new SQLite-backed store.
// If dbPath is empty, uses an in-memory database.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	dsn := dbPath
	if dbPath == "" {
		dsn = "file::memory:?cache=shared"
	} else {
		// Ensure parent directory exists
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
		dsn = dbPath + "?_foreign_keys=on&_journal_mode=WAL"
	}

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	s := &SQLiteStore{
		db: db,
		indexedFields: map[string][]string{
			"EmailMessage": {"ParentId"},
			"CaseComment":  {"ParentId"},
			"FeedItem":     {"ParentId"},
			"Contact":      {"AccountId"},
		},
	}

	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return s, nil
}

// initSchema creates the tables if they don't exist.
func (s *SQLiteStore) initSchema() error {
	// Single generic table for all SF objects using JSON storage
	schema := `
		CREATE TABLE IF NOT EXISTS sf_objects (
			object_type TEXT NOT NULL,
			id TEXT NOT NULL,
			data TEXT NOT NULL,
			PRIMARY KEY (object_type, id)
		);
		CREATE INDEX IF NOT EXISTS idx_sf_objects_type ON sf_objects(object_type);
	`
	_, err := s.db.Exec(schema)
	return err
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Get retrieves a single record by ID.
func (s *SQLiteStore) Get(objectType string, id string) (Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var data string
	err := s.db.QueryRow(
		"SELECT data FROM sf_objects WHERE object_type = ? AND id = ?",
		objectType, id,
	).Scan(&data)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	var record Record
	if err := json.Unmarshal([]byte(data), &record); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	return record, nil
}

// Query returns all records of a type that match the filter.
func (s *SQLiteStore) Query(objectType string, filter func(Record) bool) ([]Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		"SELECT data FROM sf_objects WHERE object_type = ?",
		objectType,
	)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var results []Record
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}

		var record Record
		if err := json.Unmarshal([]byte(data), &record); err != nil {
			return nil, fmt.Errorf("unmarshal: %w", err)
		}

		if filter == nil || filter(record) {
			results = append(results, record)
		}
	}

	if results == nil {
		results = []Record{}
	}

	return results, rows.Err()
}

// Create adds a new record and returns its ID.
func (s *SQLiteStore) Create(objectType string, data Record) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get or generate ID
	id, ok := data["Id"].(string)
	if !ok || id == "" {
		id = s.generateID(objectType)
		data["Id"] = id
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	_, err = s.db.Exec(
		"INSERT OR REPLACE INTO sf_objects (object_type, id, data) VALUES (?, ?, ?)",
		objectType, id, string(jsonData),
	)
	if err != nil {
		return "", fmt.Errorf("insert: %w", err)
	}

	return id, nil
}

// Update modifies an existing record.
func (s *SQLiteStore) Update(objectType string, id string, data Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// First get existing record
	var existingData string
	err := s.db.QueryRow(
		"SELECT data FROM sf_objects WHERE object_type = ? AND id = ?",
		objectType, id,
	).Scan(&existingData)

	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("query existing: %w", err)
	}

	var existing Record
	if err := json.Unmarshal([]byte(existingData), &existing); err != nil {
		return fmt.Errorf("unmarshal existing: %w", err)
	}

	// Merge data
	for k, v := range data {
		existing[k] = v
	}

	jsonData, err := json.Marshal(existing)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	_, err = s.db.Exec(
		"UPDATE sf_objects SET data = ? WHERE object_type = ? AND id = ?",
		string(jsonData), objectType, id,
	)
	return err
}

// Delete removes a record.
func (s *SQLiteStore) Delete(objectType string, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(
		"DELETE FROM sf_objects WHERE object_type = ? AND id = ?",
		objectType, id,
	)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// GetByIndex retrieves records using a secondary index.
func (s *SQLiteStore) GetByIndex(objectType string, field string, value string) ([]Record, error) {
	// For SQLite, we use JSON extraction since we store data as JSON
	return s.Query(objectType, func(r Record) bool {
		if v, ok := r[field].(string); ok {
			return v == value
		}
		return false
	})
}

// ObjectTypes returns all registered object types.
func (s *SQLiteStore) ObjectTypes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query("SELECT DISTINCT object_type FROM sf_objects")
	if err != nil {
		return nil
	}
	defer rows.Close()

	var types []string
	for rows.Next() {
		var t string
		if rows.Scan(&t) == nil {
			types = append(types, t)
		}
	}

	return types
}

// Count returns the number of records for an object type.
func (s *SQLiteStore) Count(objectType string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	s.db.QueryRow(
		"SELECT COUNT(*) FROM sf_objects WHERE object_type = ?",
		objectType,
	).Scan(&count)

	return count
}

// Clear removes all data.
func (s *SQLiteStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.db.Exec("DELETE FROM sf_objects")
}

// LoadRecords loads multiple records of a given type.
func (s *SQLiteStore) LoadRecords(objectType string, records []Record) error {
	for _, record := range records {
		if _, err := s.Create(objectType, record); err != nil {
			return err
		}
	}
	return nil
}

// Stats returns store statistics.
func (s *SQLiteStore) Stats() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]int)
	rows, err := s.db.Query(
		"SELECT object_type, COUNT(*) FROM sf_objects GROUP BY object_type",
	)
	if err != nil {
		return stats
	}
	defer rows.Close()

	for rows.Next() {
		var objType string
		var count int
		if rows.Scan(&objType, &count) == nil {
			stats[objType] = count
		}
	}

	return stats
}

// generateID generates a Salesforce-style ID.
func (s *SQLiteStore) generateID(objectType string) string {
	prefix := getIDPrefix(objectType)
	suffix := make([]byte, 7)
	rand.Read(suffix)
	return prefix + hex.EncodeToString(suffix)[:15-len(prefix)] + "AAA"
}

// Ensure SQLiteStore implements Store.
var _ Store = (*SQLiteStore)(nil)

