// Package photo provides profile photo generation using CogView-4 with local storage and reuse.
package photo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PhotoMetadata stores metadata about a generated profile photo.
type PhotoMetadata struct {
	ID        string    `json:"id"`         // UUID
	Filename  string    `json:"filename"`   // e.g., "abc123.png"
	Ethnicity string    `json:"ethnicity"`  // Asian, Black, Hispanic, White
	Gender    string    `json:"gender"`     // Male, Female
	AgeRange  string    `json:"age_range"`  // "20-30", "30-40", etc.
	HairColor string    `json:"hair_color"` // Brown, Black, Blonde, etc.
	HairStyle string    `json:"hair_style"` // short, medium, long, bald
	EyeColor  string    `json:"eye_color"`  // Brown, Blue, Green, etc.
	Glasses   bool      `json:"glasses"`
	Build     string    `json:"build"` // slim, average, athletic, stocky
	InUseBy   []string  `json:"in_use_by"`  // Entity IDs using this photo
	CreatedAt time.Time `json:"created_at"`
}

// MetadataStore manages photo metadata persistence using a JSON file.
type MetadataStore struct {
	path string
	mu   sync.RWMutex
}

// NewMetadataStore creates a new metadata store.
func NewMetadataStore(path string) *MetadataStore {
	return &MetadataStore{path: path}
}

// Load reads all metadata from the JSON file.
func (s *MetadataStore) Load() ([]PhotoMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []PhotoMetadata{}, nil
		}
		return nil, fmt.Errorf("read metadata file: %w", err)
	}

	if len(data) == 0 {
		return []PhotoMetadata{}, nil
	}

	var metadata []PhotoMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("parse metadata JSON: %w", err)
	}

	return metadata, nil
}

// Save writes all metadata to the JSON file.
func (s *MetadataStore) Save(metadata []PhotoMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure directory exists
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create metadata directory: %w", err)
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0644); err != nil {
		return fmt.Errorf("write metadata file: %w", err)
	}

	return nil
}

// Add appends a new photo metadata entry.
func (s *MetadataStore) Add(photo PhotoMetadata) error {
	metadata, err := s.Load()
	if err != nil {
		return err
	}
	metadata = append(metadata, photo)
	return s.Save(metadata)
}

// MarkInUse adds an entity ID to a photo's InUseBy list.
func (s *MetadataStore) MarkInUse(photoID, entityID string) error {
	metadata, err := s.Load()
	if err != nil {
		return err
	}

	for i := range metadata {
		if metadata[i].ID == photoID {
			// Check if already in use by this entity
			for _, id := range metadata[i].InUseBy {
				if id == entityID {
					return nil // Already marked
				}
			}
			metadata[i].InUseBy = append(metadata[i].InUseBy, entityID)
			return s.Save(metadata)
		}
	}

	return fmt.Errorf("photo not found: %s", photoID)
}

// ReleaseUsage removes an entity ID from a photo's InUseBy list.
func (s *MetadataStore) ReleaseUsage(photoID, entityID string) error {
	metadata, err := s.Load()
	if err != nil {
		return err
	}

	for i := range metadata {
		if metadata[i].ID == photoID {
			newInUseBy := make([]string, 0, len(metadata[i].InUseBy))
			for _, id := range metadata[i].InUseBy {
				if id != entityID {
					newInUseBy = append(newInUseBy, id)
				}
			}
			metadata[i].InUseBy = newInUseBy
			return s.Save(metadata)
		}
	}

	return fmt.Errorf("photo not found: %s", photoID)
}

// GetByID retrieves a photo by its ID.
func (s *MetadataStore) GetByID(photoID string) (*PhotoMetadata, error) {
	metadata, err := s.Load()
	if err != nil {
		return nil, err
	}

	for _, photo := range metadata {
		if photo.ID == photoID {
			return &photo, nil
		}
	}

	return nil, nil
}

