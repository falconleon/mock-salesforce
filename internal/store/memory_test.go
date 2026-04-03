package store_test

import (
	"testing"

	"github.com/falconleon/mock-salesforce/internal/store"
)

func TestMemoryStore_CRUD(t *testing.T) {
	s := store.NewMemoryStore()

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

func TestMemoryStore_Query(t *testing.T) {
	s := store.NewMemoryStore()

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

func TestMemoryStore_Index(t *testing.T) {
	s := store.NewMemoryStore()

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

func TestMemoryStore_Count(t *testing.T) {
	s := store.NewMemoryStore()

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

func TestMemoryStore_Clear(t *testing.T) {
	s := store.NewMemoryStore()

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

func TestMemoryStore_RecordCopy(t *testing.T) {
	s := store.NewMemoryStore()

	original := store.Record{"Id": "1", "Subject": "Original"}
	s.Create("Case", original)

	// Modify the original
	original["Subject"] = "Modified"

	// Retrieved should still have original value
	retrieved, _ := s.Get("Case", "1")
	if retrieved["Subject"] != "Original" {
		t.Error("store should copy records, not store references")
	}
}
