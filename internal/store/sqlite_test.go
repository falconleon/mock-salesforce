package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/falconleon/mock-salesforce/internal/store"
)

func TestSQLiteStore_CRUD(t *testing.T) {
	s, err := store.NewSQLiteStore("")
	if err != nil {
		t.Fatalf("Failed to create SQLite store: %v", err)
	}
	defer s.Close()

	// Create
	record := store.Record{
		"Id":      "5001234567890ABCD",
		"Subject": "Test Case",
		"Status":  "Open",
	}

	id, err := s.Create("Case", record)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if id != "5001234567890ABCD" {
		t.Errorf("expected ID '5001234567890ABCD', got '%s'", id)
	}

	// Get
	retrieved, err := s.Get("Case", id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved["Subject"] != "Test Case" {
		t.Errorf("expected Subject 'Test Case', got '%v'", retrieved["Subject"])
	}

	// Update
	err = s.Update("Case", id, store.Record{"Status": "Closed"})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	updated, _ := s.Get("Case", id)
	if updated["Status"] != "Closed" {
		t.Errorf("expected Status 'Closed', got '%v'", updated["Status"])
	}

	// Delete
	err = s.Delete("Case", id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = s.Get("Case", id)
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestSQLiteStore_Query(t *testing.T) {
	s, err := store.NewSQLiteStore("")
	if err != nil {
		t.Fatalf("Failed to create SQLite store: %v", err)
	}
	defer s.Close()

	// Add test data
	s.Create("Case", store.Record{"Id": "1", "Status": "Open", "Priority": "P1"})
	s.Create("Case", store.Record{"Id": "2", "Status": "Open", "Priority": "P2"})
	s.Create("Case", store.Record{"Id": "3", "Status": "Closed", "Priority": "P1"})

	// Query all
	all, err := s.Query("Case", nil)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 records, got %d", len(all))
	}

	// Query with filter
	open, err := s.Query("Case", func(r store.Record) bool {
		return r["Status"] == "Open"
	})
	if err != nil {
		t.Fatalf("Query with filter failed: %v", err)
	}
	if len(open) != 2 {
		t.Errorf("expected 2 open records, got %d", len(open))
	}
}

func TestSQLiteStore_Index(t *testing.T) {
	s, err := store.NewSQLiteStore("")
	if err != nil {
		t.Fatalf("Failed to create SQLite store: %v", err)
	}
	defer s.Close()

	// EmailMessage is indexed by ParentId
	s.Create("EmailMessage", store.Record{"Id": "e1", "ParentId": "case1", "Subject": "Email 1"})
	s.Create("EmailMessage", store.Record{"Id": "e2", "ParentId": "case1", "Subject": "Email 2"})
	s.Create("EmailMessage", store.Record{"Id": "e3", "ParentId": "case2", "Subject": "Email 3"})

	// Query by index
	case1Emails, err := s.GetByIndex("EmailMessage", "ParentId", "case1")
	if err != nil {
		t.Fatalf("GetByIndex failed: %v", err)
	}
	if len(case1Emails) != 2 {
		t.Errorf("expected 2 emails for case1, got %d", len(case1Emails))
	}
}

func TestSQLiteStore_Count(t *testing.T) {
	s, err := store.NewSQLiteStore("")
	if err != nil {
		t.Fatalf("Failed to create SQLite store: %v", err)
	}
	defer s.Close()

	s.Create("Case", store.Record{"Id": "1"})
	s.Create("Case", store.Record{"Id": "2"})
	s.Create("Account", store.Record{"Id": "a1"})

	if s.Count("Case") != 2 {
		t.Errorf("expected Case count 2, got %d", s.Count("Case"))
	}

	if s.Count("Account") != 1 {
		t.Errorf("expected Account count 1, got %d", s.Count("Account"))
	}

	if s.Count("Contact") != 0 {
		t.Errorf("expected Contact count 0, got %d", s.Count("Contact"))
	}
}

func TestSQLiteStore_Clear(t *testing.T) {
	s, err := store.NewSQLiteStore("")
	if err != nil {
		t.Fatalf("Failed to create SQLite store: %v", err)
	}
	defer s.Close()

	s.Create("Case", store.Record{"Id": "1"})
	s.Create("Account", store.Record{"Id": "a1"})

	s.Clear()

	if s.Count("Case") != 0 {
		t.Error("expected Case count 0 after Clear")
	}
	if s.Count("Account") != 0 {
		t.Error("expected Account count 0 after Clear")
	}
}

func TestSQLiteStore_Persistence(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create store and add data
	s1, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create SQLite store: %v", err)
	}

	s1.Create("Case", store.Record{"Id": "persist1", "Subject": "Persistent Case"})
	s1.Close()

	// Reopen and verify data persists
	s2, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to reopen SQLite store: %v", err)
	}
	defer s2.Close()

	record, err := s2.Get("Case", "persist1")
	if err != nil {
		t.Fatalf("Failed to get persisted record: %v", err)
	}

	if record["Subject"] != "Persistent Case" {
		t.Errorf("expected Subject 'Persistent Case', got '%v'", record["Subject"])
	}
}

func TestSQLiteStore_FilePath(t *testing.T) {
	// Test that db file is created
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "subdir", "test.db")

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create SQLite store: %v", err)
	}
	defer s.Close()

	// Verify file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("expected database file to be created")
	}
}

func TestSQLiteStore_LoadRecords(t *testing.T) {
	s, err := store.NewSQLiteStore("")
	if err != nil {
		t.Fatalf("Failed to create SQLite store: %v", err)
	}
	defer s.Close()

	records := []store.Record{
		{"Id": "1", "Name": "Account 1"},
		{"Id": "2", "Name": "Account 2"},
		{"Id": "3", "Name": "Account 3"},
	}

	err = s.LoadRecords("Account", records)
	if err != nil {
		t.Fatalf("LoadRecords failed: %v", err)
	}

	if s.Count("Account") != 3 {
		t.Errorf("expected 3 accounts, got %d", s.Count("Account"))
	}
}

func TestSQLiteStore_Stats(t *testing.T) {
	s, err := store.NewSQLiteStore("")
	if err != nil {
		t.Fatalf("Failed to create SQLite store: %v", err)
	}
	defer s.Close()

	s.Create("Case", store.Record{"Id": "1"})
	s.Create("Case", store.Record{"Id": "2"})
	s.Create("Account", store.Record{"Id": "a1"})

	stats := s.Stats()

	if stats["Case"] != 2 {
		t.Errorf("expected Case stat 2, got %d", stats["Case"])
	}
	if stats["Account"] != 1 {
		t.Errorf("expected Account stat 1, got %d", stats["Account"])
	}
}

