package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFromRepoRoot(t *testing.T) {
	// This test runs from within the repo, so it should find the root
	repoRoot := LoadEnvFromRepoRoot()
	if repoRoot == "" {
		t.Skip("Could not find repo root - may be running outside repo")
	}

	// Verify we found a valid repo root (has .git or go.mod)
	gitPath := filepath.Join(repoRoot, ".git")
	goModPath := filepath.Join(repoRoot, "go.mod")

	hasGit := false
	hasGoMod := false

	if info, err := os.Stat(gitPath); err == nil && info.IsDir() {
		hasGit = true
	}
	if _, err := os.Stat(goModPath); err == nil {
		hasGoMod = true
	}

	if !hasGit && !hasGoMod {
		t.Errorf("LoadEnvFromRepoRoot returned %q but neither .git nor go.mod exists there", repoRoot)
	}
}

func TestFindRepoRoot(t *testing.T) {
	root, err := findRepoRoot()
	if err != nil {
		t.Skip("Could not find repo root - may be running outside repo")
	}

	// Should have go.mod at root
	goModPath := filepath.Join(root, "go.mod")
	if _, err := os.Stat(goModPath); err != nil {
		// Or .git
		gitPath := filepath.Join(root, ".git")
		if _, err := os.Stat(gitPath); err != nil {
			t.Errorf("findRepoRoot returned %q but it doesn't have go.mod or .git", root)
		}
	}
}

func TestLoadEnvFile(t *testing.T) {
	// Create a temp .env file
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	content := `# Test comment
TEST_VAR_1=value1
TEST_VAR_2="quoted value"
TEST_VAR_3='single quoted'
EMPTY_VAR=
`
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test .env file: %v", err)
	}

	// Clear any existing values
	os.Unsetenv("TEST_VAR_1")
	os.Unsetenv("TEST_VAR_2")
	os.Unsetenv("TEST_VAR_3")
	os.Unsetenv("EMPTY_VAR")

	// Load the file
	if err := loadEnvFile(envPath); err != nil {
		t.Fatalf("loadEnvFile failed: %v", err)
	}

	// Check values
	tests := []struct {
		key      string
		expected string
	}{
		{"TEST_VAR_1", "value1"},
		{"TEST_VAR_2", "quoted value"},
		{"TEST_VAR_3", "single quoted"},
		{"EMPTY_VAR", ""},
	}

	for _, tt := range tests {
		got := os.Getenv(tt.key)
		if got != tt.expected {
			t.Errorf("Getenv(%q) = %q, want %q", tt.key, got, tt.expected)
		}
	}

	// Clean up
	for _, tt := range tests {
		os.Unsetenv(tt.key)
	}
}

func TestLoadEnvFileDoesNotOverwrite(t *testing.T) {
	// Set a value before loading
	os.Setenv("PREEXISTING_VAR", "original")
	defer os.Unsetenv("PREEXISTING_VAR")

	// Create a temp .env file that tries to override
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	content := `PREEXISTING_VAR=overwritten`
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test .env file: %v", err)
	}

	if err := loadEnvFile(envPath); err != nil {
		t.Fatalf("loadEnvFile failed: %v", err)
	}

	// Should still have original value
	got := os.Getenv("PREEXISTING_VAR")
	if got != "original" {
		t.Errorf("Getenv(PREEXISTING_VAR) = %q, want %q (should not overwrite)", got, "original")
	}
}

