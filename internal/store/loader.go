package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
)

// LoadableStore is a store that supports loading records.
type LoadableStore interface {
	Store
	LoadRecords(objectType string, records []Record) error
	Stats() map[string]int
}

// Loader loads seed data from JSON files into a store.
type Loader struct {
	store  LoadableStore
	logger zerolog.Logger
}

// NewLoader creates a new data loader.
func NewLoader(store LoadableStore, logger zerolog.Logger) *Loader {
	return &Loader{
		store:  store,
		logger: logger.With().Str("component", "loader").Logger(),
	}
}

// objectFileMapping maps object types to their JSON file names.
var objectFileMapping = map[string]string{
	"Account":      "accounts.json",
	"User":         "users.json",
	"Contact":      "contacts.json",
	"Case":         "cases.json",
	"EmailMessage": "email_messages.json",
	"CaseComment":  "case_comments.json",
	"FeedItem":     "feed_items.json",
}

// LoadFromDirectory loads all seed data from a directory.
func (l *Loader) LoadFromDirectory(dir string) error {
	l.logger.Info().Str("dir", dir).Msg("Loading seed data")

	// Load in dependency order (accounts/users first, then contacts, etc.)
	loadOrder := []string{
		"Account",
		"User",
		"Contact",
		"Case",
		"EmailMessage",
		"CaseComment",
		"FeedItem",
	}

	for _, objType := range loadOrder {
		filename, ok := objectFileMapping[objType]
		if !ok {
			continue
		}

		path := filepath.Join(dir, filename)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			l.logger.Debug().Str("file", filename).Msg("Seed file not found, skipping")
			continue
		}

		if err := l.LoadFile(objType, path); err != nil {
			return fmt.Errorf("loading %s: %w", filename, err)
		}
	}

	// Log summary
	stats := l.store.Stats()
	l.logger.Info().
		Interface("stats", stats).
		Msg("Seed data loaded")

	return nil
}

// LoadFile loads records from a single JSON file.
func (l *Loader) LoadFile(objectType string, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	var records []Record
	if err := json.Unmarshal(data, &records); err != nil {
		return fmt.Errorf("parsing JSON: %w", err)
	}

	if err := l.store.LoadRecords(objectType, records); err != nil {
		return fmt.Errorf("loading records: %w", err)
	}

	l.logger.Debug().
		Str("object", objectType).
		Int("count", len(records)).
		Msg("Loaded records")

	return nil
}

// LoadJSON loads records from a JSON byte slice.
func (l *Loader) LoadJSON(objectType string, data []byte) error {
	var records []Record
	if err := json.Unmarshal(data, &records); err != nil {
		return fmt.Errorf("parsing JSON: %w", err)
	}

	return l.store.LoadRecords(objectType, records)
}

// LoadScenario loads scenario overlay from a JSON file and merges it with existing data.
// Scenario files follow the same structure as seed files and overlay/extend the data.
func (l *Loader) LoadScenario(path string) error {
	l.logger.Info().Str("path", path).Msg("Loading scenario overlay")

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("scenario file not found: %s", path)
	}

	// Read and parse scenario JSON
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading scenario file: %w", err)
	}

	// Parse as a map with object type keys
	var scenarioData map[string][]Record
	if err := json.Unmarshal(data, &scenarioData); err != nil {
		return fmt.Errorf("parsing scenario JSON: %w", err)
	}

	// Load records for each object type in dependency order
	loadOrder := []string{
		"Account",
		"User",
		"Contact",
		"Case",
		"EmailMessage",
		"CaseComment",
		"FeedItem",
	}

	for _, objType := range loadOrder {
		records, ok := scenarioData[objType]
		if !ok || len(records) == 0 {
			continue
		}

		if err := l.store.LoadRecords(objType, records); err != nil {
			return fmt.Errorf("loading scenario records for %s: %w", objType, err)
		}

		l.logger.Debug().
			Str("object", objType).
			Int("count", len(records)).
			Msg("Loaded scenario records")
	}

	l.logger.Info().Msg("Scenario overlay loaded successfully")
	return nil
}
