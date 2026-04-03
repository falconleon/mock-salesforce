package db

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSyncMetadataToJSON(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Insert test profile images
	images := []ProfileImage{
		{
			ID:          "img-001",
			PersonaType: "contact",
			PersonaID:   "contact-001",
			ImagePath:   "assets/profile_images/img-001.png",
			FirstName:   "John",
			LastName:    "Doe",
			Age:         35,
			Gender:      "Male",
			Ethnicity:   "White",
			HairColor:   "Brown",
			HairStyle:   "short",
			EyeColor:    "Blue",
			Glasses:     true,
			GeneratedAt: "2024-01-15T10:30:00Z",
		},
		{
			ID:          "img-002",
			PersonaType: "user",
			PersonaID:   "user-001",
			ImagePath:   "assets/profile_images/img-002.png",
			FirstName:   "Jane",
			LastName:    "Smith",
			Age:         28,
			Gender:      "Female",
			Ethnicity:   "Asian",
			HairColor:   "Black",
			HairStyle:   "long",
			EyeColor:    "Brown",
			Glasses:     false,
			GeneratedAt: "2024-01-15T11:00:00Z",
		},
	}

	tx, err := store.BeginTx()
	if err != nil {
		t.Fatalf("BeginTx() error = %v", err)
	}
	if err := store.InsertProfileImagesBatch(tx, images); err != nil {
		t.Fatalf("InsertProfileImagesBatch() error = %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	// Sync to JSON
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "profile_images.json")

	if err := store.SyncMetadataToJSON(jsonPath); err != nil {
		t.Fatalf("SyncMetadataToJSON() error = %v", err)
	}

	// Verify file exists and is valid JSON
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var backup ProfileImagesBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if backup.Count != 2 {
		t.Errorf("Count = %d, want 2", backup.Count)
	}
	if backup.Version != "1.0" {
		t.Errorf("Version = %q, want %q", backup.Version, "1.0")
	}
	if len(backup.Images) != 2 {
		t.Errorf("len(Images) = %d, want 2", len(backup.Images))
	}
}

func TestSyncMetadataToJSON_EmptyTable(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "profile_images.json")

	if err := store.SyncMetadataToJSON(jsonPath); err != nil {
		t.Fatalf("SyncMetadataToJSON() error = %v", err)
	}

	var backup ProfileImagesBackup
	data, _ := os.ReadFile(jsonPath)
	json.Unmarshal(data, &backup)

	if backup.Count != 0 {
		t.Errorf("Count = %d, want 0", backup.Count)
	}
	if len(backup.Images) != 0 {
		t.Errorf("len(Images) = %d, want 0", len(backup.Images))
	}
}

func TestLoadMetadataFromJSON(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "profile_images.json")

	backup := ProfileImagesBackup{
		Version:   "1.0",
		UpdatedAt: "2024-01-15T10:00:00Z",
		Count:     1,
		Images: []ProfileImage{
			{
				ID:          "img-test",
				PersonaType: "contact",
				PersonaID:   "contact-test",
				ImagePath:   "assets/profile_images/img-test.png",
				Gender:      "Male",
			},
		},
	}

	data, _ := json.MarshalIndent(backup, "", "  ")
	os.WriteFile(jsonPath, data, 0644)

	images, err := LoadMetadataFromJSON(jsonPath)
	if err != nil {
		t.Fatalf("LoadMetadataFromJSON() error = %v", err)
	}

	if len(images) != 1 {
		t.Fatalf("len(images) = %d, want 1", len(images))
	}
	if images[0].ID != "img-test" {
		t.Errorf("ID = %q, want %q", images[0].ID, "img-test")
	}
}

func TestLoadMetadataFromJSON_NonExistent(t *testing.T) {
	images, err := LoadMetadataFromJSON("/nonexistent/path.json")
	if err != nil {
		t.Fatalf("LoadMetadataFromJSON() error = %v, want nil", err)
	}
	if len(images) != 0 {
		t.Errorf("len(images) = %d, want 0", len(images))
	}
}

func TestRecoverFromJSON(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Create backup JSON file
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "profile_images.json")

	backup := ProfileImagesBackup{
		Version:   "1.0",
		UpdatedAt: "2024-01-15T10:00:00Z",
		Count:     2,
		Images: []ProfileImage{
			{
				ID:          "recovered-001",
				PersonaType: "contact",
				PersonaID:   "contact-001",
				ImagePath:   "assets/profile_images/recovered-001.png",
				FirstName:   "Alice",
				Gender:      "Female",
				Glasses:     true,
			},
			{
				ID:          "recovered-002",
				PersonaType: "user",
				PersonaID:   "user-001",
				ImagePath:   "assets/profile_images/recovered-002.png",
				FirstName:   "Bob",
				Gender:      "Male",
				Glasses:     false,
			},
		},
	}

	data, _ := json.MarshalIndent(backup, "", "  ")
	os.WriteFile(jsonPath, data, 0644)

	// Recover from JSON
	if err := store.RecoverFromJSON(jsonPath); err != nil {
		t.Fatalf("RecoverFromJSON() error = %v", err)
	}

	// Verify data was inserted
	images, err := store.QueryAllProfileImages()
	if err != nil {
		t.Fatalf("QueryAllProfileImages() error = %v", err)
	}

	if len(images) != 2 {
		t.Fatalf("len(images) = %d, want 2", len(images))
	}

	// Check first image
	foundAlice := false
	foundBob := false
	for _, img := range images {
		if img.ID == "recovered-001" {
			foundAlice = true
			if img.FirstName != "Alice" {
				t.Errorf("FirstName = %q, want Alice", img.FirstName)
			}
			if !img.Glasses {
				t.Errorf("Glasses = false, want true")
			}
		}
		if img.ID == "recovered-002" {
			foundBob = true
			if img.FirstName != "Bob" {
				t.Errorf("FirstName = %q, want Bob", img.FirstName)
			}
		}
	}

	if !foundAlice {
		t.Error("Alice not found in recovered images")
	}
	if !foundBob {
		t.Error("Bob not found in recovered images")
	}
}

func TestRecoverFromJSON_ReplacesExisting(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Insert existing data
	existing := ProfileImage{
		ID:          "old-image",
		PersonaType: "contact",
		PersonaID:   "old-contact",
		ImagePath:   "old/path.png",
	}
	if err := store.InsertProfileImage(&existing); err != nil {
		t.Fatalf("InsertProfileImage() error = %v", err)
	}

	// Create backup with new data
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "profile_images.json")

	backup := ProfileImagesBackup{
		Version: "1.0",
		Count:   1,
		Images: []ProfileImage{
			{
				ID:          "new-image",
				PersonaType: "user",
				PersonaID:   "new-user",
				ImagePath:   "new/path.png",
			},
		},
	}

	data, _ := json.MarshalIndent(backup, "", "  ")
	os.WriteFile(jsonPath, data, 0644)

	// Recover - should replace old data
	if err := store.RecoverFromJSON(jsonPath); err != nil {
		t.Fatalf("RecoverFromJSON() error = %v", err)
	}

	images, _ := store.QueryAllProfileImages()

	if len(images) != 1 {
		t.Fatalf("len(images) = %d, want 1", len(images))
	}
	if images[0].ID != "new-image" {
		t.Errorf("ID = %q, want new-image", images[0].ID)
	}
}

func TestWriteJSONAtomic_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	deepPath := filepath.Join(tmpDir, "nested", "dir", "backup.json")

	err := writeJSONAtomic(deepPath, map[string]string{"test": "value"})
	if err != nil {
		t.Fatalf("writeJSONAtomic() error = %v", err)
	}

	if _, err := os.Stat(deepPath); os.IsNotExist(err) {
		t.Error("File was not created")
	}
}

func TestWriteJSONAtomic_PrettyPrinted(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "pretty.json")

	data := map[string]string{"key1": "value1", "key2": "value2"}
	if err := writeJSONAtomic(jsonPath, data); err != nil {
		t.Fatalf("writeJSONAtomic() error = %v", err)
	}

	content, _ := os.ReadFile(jsonPath)

	// Check for indentation (pretty printed)
	if string(content) == `{"key1":"value1","key2":"value2"}` {
		t.Error("JSON is not pretty-printed")
	}

	// Check for newlines
	if content[len(content)-1] != '\n' {
		t.Error("JSON does not end with newline")
	}
}

func TestSyncAndRecoverRoundTrip(t *testing.T) {
	store1, cleanup1 := testStore(t)
	defer cleanup1()

	// Insert data
	original := []ProfileImage{
		{
			ID:          "roundtrip-001",
			PersonaType: "contact",
			PersonaID:   "contact-rt",
			ImagePath:   "assets/rt.png",
			FirstName:   "Round",
			LastName:    "Trip",
			Age:         42,
			Gender:      "Male",
			Ethnicity:   "Hispanic",
			HairColor:   "Black",
			HairStyle:   "medium",
			EyeColor:    "Brown",
			Glasses:     true,
			FacialHair:  "beard",
			GeneratedAt: "2024-01-20T15:30:00Z",
			Prompt:      "Test prompt for roundtrip",
		},
	}

	tx, _ := store1.BeginTx()
	store1.InsertProfileImagesBatch(tx, original)
	tx.Commit()

	// Sync to JSON
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "roundtrip.json")

	if err := store1.SyncMetadataToJSON(jsonPath); err != nil {
		t.Fatalf("SyncMetadataToJSON() error = %v", err)
	}

	// Create new store and recover
	store2, cleanup2 := testStore(t)
	defer cleanup2()

	if err := store2.RecoverFromJSON(jsonPath); err != nil {
		t.Fatalf("RecoverFromJSON() error = %v", err)
	}

	// Verify all fields match
	recovered, _ := store2.QueryAllProfileImages()

	if len(recovered) != 1 {
		t.Fatalf("len(recovered) = %d, want 1", len(recovered))
	}

	r := recovered[0]
	o := original[0]

	if r.ID != o.ID {
		t.Errorf("ID mismatch: %q vs %q", r.ID, o.ID)
	}
	if r.PersonaType != o.PersonaType {
		t.Errorf("PersonaType mismatch")
	}
	if r.PersonaID != o.PersonaID {
		t.Errorf("PersonaID mismatch")
	}
	if r.ImagePath != o.ImagePath {
		t.Errorf("ImagePath mismatch")
	}
	if r.FirstName != o.FirstName {
		t.Errorf("FirstName mismatch: %q vs %q", r.FirstName, o.FirstName)
	}
	if r.LastName != o.LastName {
		t.Errorf("LastName mismatch")
	}
	if r.Age != o.Age {
		t.Errorf("Age mismatch: %d vs %d", r.Age, o.Age)
	}
	if r.Gender != o.Gender {
		t.Errorf("Gender mismatch")
	}
	if r.Ethnicity != o.Ethnicity {
		t.Errorf("Ethnicity mismatch")
	}
	if r.HairColor != o.HairColor {
		t.Errorf("HairColor mismatch")
	}
	if r.HairStyle != o.HairStyle {
		t.Errorf("HairStyle mismatch")
	}
	if r.EyeColor != o.EyeColor {
		t.Errorf("EyeColor mismatch")
	}
	if r.Glasses != o.Glasses {
		t.Errorf("Glasses mismatch")
	}
	if r.FacialHair != o.FacialHair {
		t.Errorf("FacialHair mismatch")
	}
	if r.GeneratedAt != o.GeneratedAt {
		t.Errorf("GeneratedAt mismatch")
	}
	if r.Prompt != o.Prompt {
		t.Errorf("Prompt mismatch")
	}
}

