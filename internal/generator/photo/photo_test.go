package photo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/falconleon/mock-salesforce/internal/db"
	"github.com/falconleon/mock-salesforce/internal/generator/seed"
)

func TestMetadataStore_LoadSave(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "metadata.json")
	store := NewMetadataStore(path)

	// Load from non-existent file should return empty
	metadata, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(metadata) != 0 {
		t.Errorf("Load() returned %d items, want 0", len(metadata))
	}

	// Save and load
	testMetadata := []PhotoMetadata{
		{
			ID:        "test-1",
			Filename:  "test1.png",
			Gender:    "Male",
			Ethnicity: "White",
			AgeRange:  "30-40",
			HairColor: "Brown",
			Glasses:   true,
			InUseBy:   []string{"entity-1"},
			CreatedAt: time.Now().UTC(),
		},
	}

	if err := store.Save(testMetadata); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("Load() returned %d items, want 1", len(loaded))
	}
	if loaded[0].ID != "test-1" {
		t.Errorf("Load() ID = %q, want %q", loaded[0].ID, "test-1")
	}
}

func TestMetadataStore_MarkInUse(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "metadata.json")
	store := NewMetadataStore(path)

	// Create initial metadata
	initial := []PhotoMetadata{{ID: "photo-1", InUseBy: []string{}}}
	if err := store.Save(initial); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Mark in use
	if err := store.MarkInUse("photo-1", "entity-1"); err != nil {
		t.Fatalf("MarkInUse() error = %v", err)
	}

	// Verify
	loaded, _ := store.Load()
	if len(loaded[0].InUseBy) != 1 || loaded[0].InUseBy[0] != "entity-1" {
		t.Errorf("InUseBy = %v, want [entity-1]", loaded[0].InUseBy)
	}

	// Mark same entity again (should not duplicate)
	if err := store.MarkInUse("photo-1", "entity-1"); err != nil {
		t.Fatalf("MarkInUse() duplicate error = %v", err)
	}
	loaded, _ = store.Load()
	if len(loaded[0].InUseBy) != 1 {
		t.Errorf("InUseBy has %d entries, want 1", len(loaded[0].InUseBy))
	}
}

func TestMetadataStore_ReleaseUsage(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "metadata.json")
	store := NewMetadataStore(path)

	// Create metadata with usage
	initial := []PhotoMetadata{{ID: "photo-1", InUseBy: []string{"entity-1", "entity-2"}}}
	if err := store.Save(initial); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Release one usage
	if err := store.ReleaseUsage("photo-1", "entity-1"); err != nil {
		t.Fatalf("ReleaseUsage() error = %v", err)
	}

	// Verify
	loaded, _ := store.Load()
	if len(loaded[0].InUseBy) != 1 || loaded[0].InUseBy[0] != "entity-2" {
		t.Errorf("InUseBy = %v, want [entity-2]", loaded[0].InUseBy)
	}
}

func TestFindMatch_ExactMatch(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "metadata.json")
	store := NewMetadataStore(path)

	// Create photo matching typical person
	photos := []PhotoMetadata{
		{
			ID:        "photo-match",
			Gender:    "Male",
			Ethnicity: "White",
			AgeRange:  "30-40",
			HairColor: "Brown",
			HairStyle: "short",
			EyeColor:  "Blue",
			Glasses:   false,
			Build:     "athletic",
			InUseBy:   []string{},
		},
	}
	if err := store.Save(photos); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	person := seed.PersonSeed{
		Gender:    "Male",
		Ethnicity: "White",
		Age:       35,
		HairColor: "Brown",
		HairStyle: "short",
		EyeColor:  "Blue",
		Glasses:   false,
		Build:     "athletic",
	}

	match := store.FindMatch(person)
	if match == nil {
		t.Fatal("FindMatch() returned nil, want match")
	}
	if match.ID != "photo-match" {
		t.Errorf("FindMatch() ID = %q, want %q", match.ID, "photo-match")
	}
}

func TestFindMatch_NoMatch_DifferentGender(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "metadata.json")
	store := NewMetadataStore(path)

	// Create female photo
	photos := []PhotoMetadata{
		{ID: "photo-female", Gender: "Female", Ethnicity: "White", AgeRange: "30-40"},
	}
	if err := store.Save(photos); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Search for male
	person := seed.PersonSeed{Gender: "Male", Ethnicity: "White", Age: 35}

	match := store.FindMatch(person)
	if match != nil {
		t.Errorf("FindMatch() = %v, want nil (gender mismatch)", match.ID)
	}
}

func TestFindMatch_PrefersFewerUsers(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "metadata.json")
	store := NewMetadataStore(path)

	// Two identical photos, one with more users
	photos := []PhotoMetadata{
		{ID: "photo-busy", Gender: "Male", Ethnicity: "White", AgeRange: "30-40", InUseBy: []string{"a", "b", "c"}},
		{ID: "photo-free", Gender: "Male", Ethnicity: "White", AgeRange: "30-40", InUseBy: []string{}},
	}
	if err := store.Save(photos); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	person := seed.PersonSeed{Gender: "Male", Ethnicity: "White", Age: 35}

	match := store.FindMatch(person)
	if match == nil {
		t.Fatal("FindMatch() returned nil")
	}
	if match.ID != "photo-free" {
		t.Errorf("FindMatch() ID = %q, want %q (fewer users)", match.ID, "photo-free")
	}
}

func TestAgeToRange(t *testing.T) {
	tests := []struct {
		age  int
		want string
	}{
		{18, "under-20"},
		{25, "20-30"},
		{35, "30-40"},
		{45, "40-50"},
		{55, "50-60"},
		{65, "60+"},
	}

	for _, tc := range tests {
		got := ageToRange(tc.age)
		if got != tc.want {
			t.Errorf("ageToRange(%d) = %q, want %q", tc.age, got, tc.want)
		}
	}
}

func TestBuildPhotoPrompt(t *testing.T) {
	person := seed.PersonSeed{
		Age:         35,
		Ethnicity:   "White",
		Gender:      "Male",
		HairColor:   "Brown",
		HairTexture: "straight",
		HairStyle:   "short",
		EyeColor:    "Blue",
		FacialHair:  "beard",
		Glasses:     true,
		Build:       "athletic",
	}

	prompt := BuildPhotoPrompt(person)

	// Verify key elements present
	expected := []string{
		"35-year-old",
		"White",
		"male",
		"brown",
		"straight",
		"short",
		"blue eyes",
		"beard",
		"wearing glasses",
		"athletic build",
		"professional",
	}

	for _, e := range expected {
		if !containsIgnoreCase(prompt, e) {
			t.Errorf("BuildPhotoPrompt() missing %q in: %s", e, prompt)
		}
	}
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(len(s) >= len(substr)) &&
		(s == substr || containsLower(s, substr))
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if matchIgnoreCase(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func matchIgnoreCase(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func TestPhotoGenerator_Generate(t *testing.T) {
	// Create mock CogView server
	fakeImageData := []byte("fake-png-data")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("missing or wrong authorization header")
		}

		var req CogViewRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if req.Model != CogViewModel {
			t.Errorf("model = %q, want %q", req.Model, CogViewModel)
		}

		// Return fake image
		resp := CogViewResponse{
			Data: []CogViewImageData{
				{B64JSON: base64.StdEncoding.EncodeToString(fakeImageData)},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create generator with mock endpoint
	tmpDir := t.TempDir()
	generator := NewPhotoGenerator("test-api-key", tmpDir)
	generator.endpoint = server.URL

	person := seed.PersonSeed{
		Age:         35,
		Gender:      "Male",
		Ethnicity:   "White",
		HairColor:   "Brown",
		HairTexture: "straight",
		HairStyle:   "short",
		EyeColor:    "Blue",
		Glasses:     false,
		Build:       "average",
	}

	metadata, err := generator.Generate(context.Background(), person)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify metadata
	if metadata.Gender != "Male" {
		t.Errorf("Gender = %q, want Male", metadata.Gender)
	}
	if metadata.Ethnicity != "White" {
		t.Errorf("Ethnicity = %q, want White", metadata.Ethnicity)
	}
	if metadata.AgeRange != "30-40" {
		t.Errorf("AgeRange = %q, want 30-40", metadata.AgeRange)
	}

	// Verify file was saved
	filePath := filepath.Join(tmpDir, metadata.Filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read saved image: %v", err)
	}
	if string(data) != string(fakeImageData) {
		t.Errorf("saved image data mismatch")
	}
}

func TestPhotoGenerator_APIError(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer server.Close()

	generator := NewPhotoGenerator("test-api-key", t.TempDir())
	generator.endpoint = server.URL

	person := seed.PersonSeed{Gender: "Male", Ethnicity: "White", Age: 30}

	_, err := generator.Generate(context.Background(), person)
	if err == nil {
		t.Error("Generate() expected error for API failure")
	}
}

func TestPhotoGenerator_RetryOnTransientError(t *testing.T) {
	attemptCount := 0
	fakeImageData := []byte{0x89, 0x50, 0x4E, 0x47} // PNG header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		// Fail first 2 attempts with 503, succeed on 3rd
		if attemptCount < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error": "service unavailable"}`))
			return
		}

		resp := CogViewResponse{
			Data: []CogViewImageData{
				{B64JSON: base64.StdEncoding.EncodeToString(fakeImageData)},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	generator := NewPhotoGenerator("test-api-key", tmpDir)
	generator.endpoint = server.URL
	// Use faster backoff for tests
	generator.retryConfig = RetryConfig{
		MaxRetries:  3,
		BaseBackoff: 10 * time.Millisecond,
		MaxBackoff:  50 * time.Millisecond,
	}

	person := seed.PersonSeed{Gender: "Male", Ethnicity: "White", Age: 30}

	_, err := generator.Generate(context.Background(), person)
	if err != nil {
		t.Fatalf("Generate() error = %v, expected success after retry", err)
	}

	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}
}

func TestPhotoGenerator_NoRetryOnNonTransientError(t *testing.T) {
	attemptCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		// Return 400 Bad Request (non-transient)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "bad request"}`))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	generator := NewPhotoGenerator("test-api-key", tmpDir)
	generator.endpoint = server.URL
	generator.retryConfig = RetryConfig{
		MaxRetries:  3,
		BaseBackoff: 10 * time.Millisecond,
		MaxBackoff:  50 * time.Millisecond,
	}

	person := seed.PersonSeed{Gender: "Male", Ethnicity: "White", Age: 30}

	_, err := generator.Generate(context.Background(), person)
	if err == nil {
		t.Fatal("Generate() expected error for bad request")
	}

	// Should only attempt once for non-transient errors
	if attemptCount != 1 {
		t.Errorf("Expected 1 attempt for non-transient error, got %d", attemptCount)
	}
}

func TestPhotoGenerator_MaxRetriesExhausted(t *testing.T) {
	attemptCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		// Always return 503
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error": "service unavailable"}`))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	generator := NewPhotoGenerator("test-api-key", tmpDir)
	generator.endpoint = server.URL
	generator.retryConfig = RetryConfig{
		MaxRetries:  2,
		BaseBackoff: 10 * time.Millisecond,
		MaxBackoff:  50 * time.Millisecond,
	}

	person := seed.PersonSeed{Gender: "Male", Ethnicity: "White", Age: 30}

	_, err := generator.Generate(context.Background(), person)
	if err == nil {
		t.Fatal("Generate() expected error when max retries exhausted")
	}

	// Should attempt MaxRetries + 1 (initial + retries)
	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts (1 initial + 2 retries), got %d", attemptCount)
	}
}

func TestTraitExtractor_ExtractTraitsFromContact(t *testing.T) {
	extractor := NewTraitExtractor()

	contact := db.Contact{
		ID:        "contact-123",
		FirstName: "John",
		LastName:  "Smith",
		Title:     "Senior Support Engineer",
	}

	traits := extractor.ExtractTraitsFromContact(contact)

	if traits.FirstName != "John" {
		t.Errorf("FirstName = %q, want %q", traits.FirstName, "John")
	}
	if traits.LastName != "Smith" {
		t.Errorf("LastName = %q, want %q", traits.LastName, "Smith")
	}

	// Should be reproducible with same ID
	traits2 := extractor.ExtractTraitsFromContact(contact)
	if traits.Gender != traits2.Gender {
		t.Errorf("Gender not reproducible: %q vs %q", traits.Gender, traits2.Gender)
	}
}

func TestTraitExtractor_ExtractTraitsFromUser(t *testing.T) {
	extractor := NewTraitExtractor()

	user := db.User{
		ID:        "user-456",
		FirstName: "Jane",
		LastName:  "Doe",
		Title:     "Support Manager",
		UserRole:  "admin",
	}

	traits := extractor.ExtractTraitsFromUser(user)

	if traits.FirstName != "Jane" {
		t.Errorf("FirstName = %q, want %q", traits.FirstName, "Jane")
	}
	if traits.LastName != "Doe" {
		t.Errorf("LastName = %q, want %q", traits.LastName, "Doe")
	}
}

func TestRoleFromTitle(t *testing.T) {
	tests := []struct {
		title    string
		expected seed.RoleLevel
	}{
		{"CEO", seed.RoleExecutive},
		{"CTO", seed.RoleExecutive},
		{"cfo", seed.RoleExecutive},
		{"president", seed.RoleExecutive},
		{"Vice President of Sales", seed.RoleDirector},
		{"VP of Engineering", seed.RoleDirector},
		{"Director of Engineering", seed.RoleDirector},
		{"Support Manager", seed.RoleManager},
		{"Team Lead", seed.RoleSupportL3},
		{"team lead", seed.RoleSupportL3},
		{"Senior Support Engineer", seed.RoleSupportL2},
		{"Lead Developer", seed.RoleSupportL2},
		{"Support Specialist", seed.RoleSupportL1},
		{"", seed.RoleSupportL1},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := roleFromTitle(tt.title)
			if got != tt.expected {
				t.Errorf("roleFromTitle(%q) = %v, want %v", tt.title, got, tt.expected)
			}
		})
	}
}
