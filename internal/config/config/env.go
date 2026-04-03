// Package config provides configuration utilities.
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// LoadEnvFromRepoRoot finds the repository root and loads .env file from there.
// Returns the path to the repo root, or empty string if not found.
func LoadEnvFromRepoRoot() string {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return ""
	}

	envPath := filepath.Join(repoRoot, ".env")
	_ = loadEnvFile(envPath) // Ignore error if file doesn't exist
	return repoRoot
}

// findRepoRoot walks up the directory tree until it finds .git (directory or file)
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		// Check for .git (directory for normal repos, file for worktrees)
		gitPath := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// loadEnvFile reads a .env file and sets environment variables.
// Only sets variables that are not already set (environment takes precedence).
func loadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove surrounding quotes
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		// Only set if not already set
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}

