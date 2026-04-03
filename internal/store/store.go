// Package store provides data storage for the mock Salesforce API.
package store

import (
	"errors"
)

// Common errors.
var (
	ErrNotFound      = errors.New("record not found")
	ErrInvalidObject = errors.New("invalid object type")
)

// Store is the interface for data storage.
type Store interface {
	// Get retrieves a single record by ID.
	Get(objectType string, id string) (map[string]any, error)

	// Query returns all records of a type that match the filter.
	Query(objectType string, filter func(map[string]any) bool) ([]map[string]any, error)

	// Create adds a new record and returns its ID.
	Create(objectType string, data map[string]any) (string, error)

	// Update modifies an existing record.
	Update(objectType string, id string, data map[string]any) error

	// Delete removes a record.
	Delete(objectType string, id string) error

	// GetByIndex retrieves records using a secondary index.
	GetByIndex(objectType string, field string, value string) ([]map[string]any, error)

	// ObjectTypes returns all registered object types.
	ObjectTypes() []string

	// Count returns the number of records for an object type.
	Count(objectType string) int

	// Clear removes all data.
	Clear()
}

// Record is a convenience type alias.
type Record = map[string]any
