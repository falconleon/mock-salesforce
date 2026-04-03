package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
)

// MemoryStore is an in-memory implementation of Store.
type MemoryStore struct {
	// data: objectType -> id -> record
	data map[string]map[string]Record

	// indexes: objectType -> fieldName -> fieldValue -> []ids
	indexes map[string]map[string]map[string][]string

	// indexedFields: objectType -> []fieldNames to index
	indexedFields map[string][]string

	mu sync.RWMutex
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		data:    make(map[string]map[string]Record),
		indexes: make(map[string]map[string]map[string][]string),
		indexedFields: map[string][]string{
			"EmailMessage": {"ParentId"},
			"CaseComment":  {"ParentId"},
			"FeedItem":     {"ParentId"},
			"Contact":      {"AccountId"},
		},
	}
}

// Get retrieves a single record by ID.
func (s *MemoryStore) Get(objectType string, id string) (Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	objects, ok := s.data[objectType]
	if !ok {
		return nil, ErrNotFound
	}

	record, ok := objects[id]
	if !ok {
		return nil, ErrNotFound
	}

	return copyRecord(record), nil
}

// Query returns all records of a type that match the filter.
func (s *MemoryStore) Query(objectType string, filter func(Record) bool) ([]Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	objects, ok := s.data[objectType]
	if !ok {
		return []Record{}, nil
	}

	var results []Record
	for _, record := range objects {
		if filter == nil || filter(record) {
			results = append(results, copyRecord(record))
		}
	}

	return results, nil
}

// Create adds a new record and returns its ID.
func (s *MemoryStore) Create(objectType string, data Record) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get or generate ID
	id, ok := data["Id"].(string)
	if !ok || id == "" {
		id = s.generateID(objectType)
		data["Id"] = id
	}

	// Ensure object type map exists
	if s.data[objectType] == nil {
		s.data[objectType] = make(map[string]Record)
	}

	// Store record
	s.data[objectType][id] = copyRecord(data)

	// Update indexes
	s.updateIndexes(objectType, id, data)

	return id, nil
}

// Update modifies an existing record.
func (s *MemoryStore) Update(objectType string, id string, data Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	objects, ok := s.data[objectType]
	if !ok {
		return ErrNotFound
	}

	existing, ok := objects[id]
	if !ok {
		return ErrNotFound
	}

	// Merge data
	for k, v := range data {
		existing[k] = v
	}

	// Update indexes
	s.updateIndexes(objectType, id, existing)

	return nil
}

// Delete removes a record.
func (s *MemoryStore) Delete(objectType string, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	objects, ok := s.data[objectType]
	if !ok {
		return ErrNotFound
	}

	if _, ok := objects[id]; !ok {
		return ErrNotFound
	}

	// Remove from indexes first
	s.removeFromIndexes(objectType, id)

	delete(objects, id)
	return nil
}

// GetByIndex retrieves records using a secondary index.
func (s *MemoryStore) GetByIndex(objectType string, field string, value string) ([]Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	objIndexes, ok := s.indexes[objectType]
	if !ok {
		return []Record{}, nil
	}

	fieldIndex, ok := objIndexes[field]
	if !ok {
		return []Record{}, nil
	}

	ids, ok := fieldIndex[value]
	if !ok {
		return []Record{}, nil
	}

	var results []Record
	objects := s.data[objectType]
	for _, id := range ids {
		if record, ok := objects[id]; ok {
			results = append(results, copyRecord(record))
		}
	}

	return results, nil
}

// ObjectTypes returns all registered object types.
func (s *MemoryStore) ObjectTypes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	types := make([]string, 0, len(s.data))
	for t := range s.data {
		types = append(types, t)
	}
	return types
}

// Count returns the number of records for an object type.
func (s *MemoryStore) Count(objectType string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.data[objectType])
}

// Clear removes all data.
func (s *MemoryStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data = make(map[string]map[string]Record)
	s.indexes = make(map[string]map[string]map[string][]string)
}

// LoadRecords loads multiple records of a given type.
func (s *MemoryStore) LoadRecords(objectType string, records []Record) error {
	for _, record := range records {
		if _, err := s.Create(objectType, record); err != nil {
			return err
		}
	}
	return nil
}

// updateIndexes updates secondary indexes for a record.
func (s *MemoryStore) updateIndexes(objectType string, id string, data Record) {
	fields, ok := s.indexedFields[objectType]
	if !ok {
		return
	}

	if s.indexes[objectType] == nil {
		s.indexes[objectType] = make(map[string]map[string][]string)
	}

	for _, field := range fields {
		if value, ok := data[field].(string); ok && value != "" {
			if s.indexes[objectType][field] == nil {
				s.indexes[objectType][field] = make(map[string][]string)
			}

			// Check if ID already in index
			ids := s.indexes[objectType][field][value]
			found := false
			for _, existingID := range ids {
				if existingID == id {
					found = true
					break
				}
			}
			if !found {
				s.indexes[objectType][field][value] = append(ids, id)
			}
		}
	}
}

// removeFromIndexes removes a record from all indexes.
func (s *MemoryStore) removeFromIndexes(objectType string, id string) {
	objIndexes, ok := s.indexes[objectType]
	if !ok {
		return
	}

	for field, fieldIndex := range objIndexes {
		for value, ids := range fieldIndex {
			newIDs := make([]string, 0, len(ids))
			for _, existingID := range ids {
				if existingID != id {
					newIDs = append(newIDs, existingID)
				}
			}
			if len(newIDs) == 0 {
				delete(fieldIndex, value)
			} else {
				s.indexes[objectType][field][value] = newIDs
			}
		}
	}
}

// generateID generates a Salesforce-style ID.
func (s *MemoryStore) generateID(objectType string) string {
	prefix := getIDPrefix(objectType)
	suffix := make([]byte, 7)
	rand.Read(suffix)
	return prefix + hex.EncodeToString(suffix)[:15-len(prefix)] + "AAA"
}

// getIDPrefix returns the Salesforce ID prefix for an object type.
func getIDPrefix(objectType string) string {
	prefixes := map[string]string{
		"Account":      "001",
		"Contact":      "003",
		"Case":         "500",
		"User":         "005",
		"CaseComment":  "00a",
		"EmailMessage": "02s",
		"FeedItem":     "0D5",
	}
	if prefix, ok := prefixes[objectType]; ok {
		return prefix
	}
	return "000"
}

// copyRecord creates a deep copy of a record.
func copyRecord(r Record) Record {
	if r == nil {
		return nil
	}
	copy := make(Record, len(r))
	for k, v := range r {
		// Handle nested maps
		if nested, ok := v.(map[string]any); ok {
			copy[k] = copyRecord(nested)
		} else {
			copy[k] = v
		}
	}
	return copy
}

// Ensure MemoryStore implements Store.
var _ Store = (*MemoryStore)(nil)

// Stats returns store statistics.
func (s *MemoryStore) Stats() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]int)
	for objType, objects := range s.data {
		stats[objType] = len(objects)
	}
	return stats
}

// String returns a debug representation of the store.
func (s *MemoryStore) String() string {
	stats := s.Stats()
	result := "MemoryStore{"
	for k, v := range stats {
		result += fmt.Sprintf("%s:%d ", k, v)
	}
	return result + "}"
}
