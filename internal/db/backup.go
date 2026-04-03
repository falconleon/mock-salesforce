package db

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ProfileImagesBackup represents the JSON backup file structure.
type ProfileImagesBackup struct {
	Version   string         `json:"version"`
	UpdatedAt string         `json:"updated_at"`
	Count     int            `json:"count"`
	Images    []ProfileImage `json:"images"`
}

// SyncMetadataToJSON exports profile_images table to a JSON file.
// Uses atomic write (temp file + rename) for crash safety.
func (s *Store) SyncMetadataToJSON(jsonPath string) error {
	// Query all profile images from the database
	images, err := s.QueryAllProfileImages()
	if err != nil {
		return fmt.Errorf("query profile images: %w", err)
	}

	backup := ProfileImagesBackup{
		Version:   "1.0",
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Count:     len(images),
		Images:    images,
	}

	// If images is nil, use empty slice for cleaner JSON
	if backup.Images == nil {
		backup.Images = []ProfileImage{}
	}

	return writeJSONAtomic(jsonPath, backup)
}

// LoadMetadataFromJSON reads profile images from a JSON backup file.
// Returns the images without modifying the database.
func LoadMetadataFromJSON(jsonPath string) ([]ProfileImage, error) {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []ProfileImage{}, nil
		}
		return nil, fmt.Errorf("read backup file: %w", err)
	}

	if len(data) == 0 {
		return []ProfileImage{}, nil
	}

	var backup ProfileImagesBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		return nil, fmt.Errorf("parse backup JSON: %w", err)
	}

	return backup.Images, nil
}

// RecoverFromJSON rebuilds the profile_images table from a JSON backup.
// This drops all existing data and replaces it with the backup.
func (s *Store) RecoverFromJSON(jsonPath string) error {
	images, err := LoadMetadataFromJSON(jsonPath)
	if err != nil {
		return fmt.Errorf("load backup: %w", err)
	}

	// Clear existing data
	if err := s.DeleteAllProfileImages(); err != nil {
		return fmt.Errorf("clear profile images: %w", err)
	}

	// Insert all images from backup
	if len(images) == 0 {
		return nil
	}

	tx, err := s.BeginTx()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if err := s.InsertProfileImagesBatch(tx, images); err != nil {
		tx.Rollback()
		return fmt.Errorf("insert profile images: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// writeJSONAtomic writes data to a file using atomic write pattern.
// Writes to a temp file first, then renames to the target path.
func writeJSONAtomic(path string, data any) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Marshal with pretty printing for git diffs
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}

	// Add trailing newline for POSIX compliance
	jsonData = append(jsonData, '\n')

	// Write to temp file in same directory (for atomic rename)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, jsonData, 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // Clean up on failure
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

