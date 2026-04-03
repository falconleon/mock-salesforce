package generator_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/falconleon/mock-salesforce/internal/generator/photo"
	"github.com/falconleon/mock-salesforce/internal/generator/seed"
)

// TestPhotoMatcherFindsMatch verifies the matcher finds photos with similar traits.
func TestPhotoMatcherFindsMatch(t *testing.T) {
	tmpDir := t.TempDir()
	metadataPath := filepath.Join(tmpDir, "photo_metadata.json")
	store := photo.NewMetadataStore(metadataPath)

	// Create existing photo metadata
	existingPhotos := []photo.PhotoMetadata{
		{
			ID:        "photo-1",
			Filename:  "photo1.png",
			Gender:    "Male",
			Ethnicity: "White",
			AgeRange:  "30-40",
			HairColor: "Brown",
			HairStyle: "short",
			EyeColor:  "Blue",
			Glasses:   false,
			Build:     "average",
			InUseBy:   []string{},
			CreatedAt: time.Now().UTC(),
		},
		{
			ID:        "photo-2",
			Filename:  "photo2.png",
			Gender:    "Female",
			Ethnicity: "Asian",
			AgeRange:  "20-30",
			HairColor: "Black",
			HairStyle: "long",
			EyeColor:  "Brown",
			Glasses:   true,
			Build:     "slim",
			InUseBy:   []string{},
			CreatedAt: time.Now().UTC(),
		},
	}
	if err := store.Save(existingPhotos); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Generate a person seed that should match photo-1
	personSeed := seed.PersonSeed{
		Gender:    "Male",
		Ethnicity: "White",
		Age:       35, // Maps to "30-40"
		HairColor: "Brown",
		HairStyle: "short",
		EyeColor:  "Blue",
		Glasses:   false,
		Build:     "average",
	}

	match := store.FindMatch(personSeed)
	if match == nil {
		t.Fatal("Expected to find a match, got nil")
	}
	if match.ID != "photo-1" {
		t.Errorf("Expected match ID 'photo-1', got %q", match.ID)
	}
}

// TestPhotoMatcherRejectsIncompatible verifies no match for incompatible traits.
func TestPhotoMatcherRejectsIncompatible(t *testing.T) {
	tmpDir := t.TempDir()
	metadataPath := filepath.Join(tmpDir, "photo_metadata.json")
	store := photo.NewMetadataStore(metadataPath)

	// Create existing photo with specific traits
	existingPhotos := []photo.PhotoMetadata{
		{
			ID:        "photo-female",
			Filename:  "photo_female.png",
			Gender:    "Female",
			Ethnicity: "Asian",
			AgeRange:  "20-30",
			InUseBy:   []string{},
			CreatedAt: time.Now().UTC(),
		},
	}
	if err := store.Save(existingPhotos); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Try to match a male - should not find match (gender must match)
	maleSeed := seed.PersonSeed{
		Gender:    "Male",
		Ethnicity: "Asian",
		Age:       25,
	}

	match := store.FindMatch(maleSeed)
	if match != nil {
		t.Errorf("Expected no match for different gender, got %q", match.ID)
	}

	// Try to match different ethnicity - should not find match
	whiteSeed := seed.PersonSeed{
		Gender:    "Female",
		Ethnicity: "White",
		Age:       25,
	}

	match = store.FindMatch(whiteSeed)
	if match != nil {
		t.Errorf("Expected no match for different ethnicity, got %q", match.ID)
	}
}

// TestPhotoMatcherPrefersFewUsers verifies preference for less-used photos.
func TestPhotoMatcherPrefersFewUsers(t *testing.T) {
	tmpDir := t.TempDir()
	metadataPath := filepath.Join(tmpDir, "photo_metadata.json")
	store := photo.NewMetadataStore(metadataPath)

	// Two identical photos, one heavily used
	existingPhotos := []photo.PhotoMetadata{
		{
			ID:        "photo-busy",
			Filename:  "busy.png",
			Gender:    "Male",
			Ethnicity: "White",
			AgeRange:  "30-40",
			InUseBy:   []string{"user1", "user2", "user3", "user4", "user5"},
			CreatedAt: time.Now().UTC(),
		},
		{
			ID:        "photo-free",
			Filename:  "free.png",
			Gender:    "Male",
			Ethnicity: "White",
			AgeRange:  "30-40",
			InUseBy:   []string{},
			CreatedAt: time.Now().UTC(),
		},
	}
	if err := store.Save(existingPhotos); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	personSeed := seed.PersonSeed{
		Gender:    "Male",
		Ethnicity: "White",
		Age:       35,
	}

	match := store.FindMatch(personSeed)
	if match == nil {
		t.Fatal("Expected to find a match")
	}
	if match.ID != "photo-free" {
		t.Errorf("Expected 'photo-free' (fewer users), got %q", match.ID)
	}
}

// TestPhotoMetadataStoreMarkInUse verifies usage tracking works.
func TestPhotoMetadataStoreMarkInUse(t *testing.T) {
	tmpDir := t.TempDir()
	metadataPath := filepath.Join(tmpDir, "photo_metadata.json")
	store := photo.NewMetadataStore(metadataPath)

	// Create initial metadata
	initial := []photo.PhotoMetadata{
		{ID: "photo-1", Filename: "test.png", InUseBy: []string{}},
	}
	if err := store.Save(initial); err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// Mark in use
	if err := store.MarkInUse("photo-1", "entity-abc"); err != nil {
		t.Fatalf("MarkInUse failed: %v", err)
	}

	// Verify
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded[0].InUseBy) != 1 || loaded[0].InUseBy[0] != "entity-abc" {
		t.Errorf("InUseBy = %v, want [entity-abc]", loaded[0].InUseBy)
	}

	// Mark same entity again (should not duplicate)
	if err := store.MarkInUse("photo-1", "entity-abc"); err != nil {
		t.Fatalf("Second MarkInUse failed: %v", err)
	}
	loaded, _ = store.Load()
	if len(loaded[0].InUseBy) != 1 {
		t.Errorf("Duplicate InUseBy entries: %v", loaded[0].InUseBy)
	}
}

// TestPhotoMetadataStoreReleaseUsage verifies release tracking works.
func TestPhotoMetadataStoreReleaseUsage(t *testing.T) {
	tmpDir := t.TempDir()
	metadataPath := filepath.Join(tmpDir, "photo_metadata.json")
	store := photo.NewMetadataStore(metadataPath)

	// Create metadata with usage
	initial := []photo.PhotoMetadata{
		{ID: "photo-1", Filename: "test.png", InUseBy: []string{"entity-a", "entity-b"}},
	}
	if err := store.Save(initial); err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// Release one usage
	if err := store.ReleaseUsage("photo-1", "entity-a"); err != nil {
		t.Fatalf("ReleaseUsage failed: %v", err)
	}

	loaded, _ := store.Load()
	if len(loaded[0].InUseBy) != 1 || loaded[0].InUseBy[0] != "entity-b" {
		t.Errorf("InUseBy = %v, want [entity-b]", loaded[0].InUseBy)
	}
}

// TestMetadataStoreEmptyFile verifies handling of non-existent file.
func TestMetadataStoreEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	metadataPath := filepath.Join(tmpDir, "nonexistent.json")
	store := photo.NewMetadataStore(metadataPath)

	metadata, err := store.Load()
	if err != nil {
		t.Fatalf("Load from nonexistent file should not error: %v", err)
	}
	if len(metadata) != 0 {
		t.Errorf("Expected empty metadata, got %d items", len(metadata))
	}
}

// TestMetadataPersistence verifies data persists across store instances.
func TestMetadataPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	metadataPath := filepath.Join(tmpDir, "persistent.json")

	// Write with one store
	store1 := photo.NewMetadataStore(metadataPath)
	testData := []photo.PhotoMetadata{
		{ID: "persist-1", Filename: "test.png", Gender: "Male", Ethnicity: "White"},
	}
	if err := store1.Save(testData); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Read with another store instance
	store2 := photo.NewMetadataStore(metadataPath)
	loaded, err := store2.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded) != 1 || loaded[0].ID != "persist-1" {
		t.Errorf("Data not persisted correctly: %v", loaded)
	}
}

// TestPhotoMatcherWithGeneratedSeeds tests matcher with real faker-generated seeds.
func TestPhotoMatcherWithGeneratedSeeds(t *testing.T) {
	tmpDir := t.TempDir()
	metadataPath := filepath.Join(tmpDir, "generated.json")
	store := photo.NewMetadataStore(metadataPath)

	// Generate some person seeds and create matching photos
	var photos []photo.PhotoMetadata
	for i := 0; i < 5; i++ {
		s := seed.NewPersonSeed(seed.RoleSupportL1)
		photos = append(photos, photo.PhotoMetadata{
			ID:        "generated-" + string(rune('a'+i)),
			Filename:  "gen" + string(rune('a'+i)) + ".png",
			Gender:    s.Gender,
			Ethnicity: s.Ethnicity,
			AgeRange:  ageToRange(s.Age),
			HairColor: s.HairColor,
			HairStyle: s.HairStyle,
			EyeColor:  s.EyeColor,
			Glasses:   s.Glasses,
			Build:     s.Build,
			InUseBy:   []string{},
			CreatedAt: time.Now().UTC(),
		})
	}
	if err := store.Save(photos); err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// Now generate new seeds and try to find matches
	matchCount := 0
	for i := 0; i < 10; i++ {
		newSeed := seed.NewPersonSeed(seed.RoleSupportL1)
		match := store.FindMatch(newSeed)
		if match != nil {
			matchCount++
		}
	}

	t.Logf("Found %d matches out of 10 generated seeds against 5 photos", matchCount)
	// Some matches are expected, but not all (diversity in seeds)
}

// ageToRange converts age to age range string (matching photo package logic).
func ageToRange(age int) string {
	switch {
	case age < 20:
		return "under-20"
	case age < 30:
		return "20-30"
	case age < 40:
		return "30-40"
	case age < 50:
		return "40-50"
	case age < 60:
		return "50-60"
	default:
		return "60+"
	}
}

// TestDirectoryCreation verifies the store creates directories as needed.
func TestDirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "nested", "dir", "metadata.json")
	store := photo.NewMetadataStore(nestedPath)

	testData := []photo.PhotoMetadata{
		{ID: "nested-1", Filename: "test.png"},
	}
	if err := store.Save(testData); err != nil {
		t.Fatalf("Save to nested path failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(nestedPath); os.IsNotExist(err) {
		t.Error("Nested metadata file was not created")
	}
}

