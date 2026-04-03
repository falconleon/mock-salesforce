package zaiclient

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testOutputDir returns the path to the output directory, creating it if needed.
func testOutputDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(".", "output")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create output directory: %v", err)
	}
	return dir
}

// testTimestamp returns a timestamp string for dated filenames.
func testTimestamp() string {
	return time.Now().Format("2006-01-02_150405")
}

// writeTestReport writes a text report to the output directory with a dated filename.
func writeTestReport(t *testing.T, name string, content string) string {
	t.Helper()
	dir := testOutputDir(t)
	filename := fmt.Sprintf("%s_%s", testTimestamp(), name)
	fullPath := filepath.Join(dir, filename)
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Logf("Warning: failed to write report to %s: %v", fullPath, err)
		return ""
	}
	t.Logf("Report written to: %s", fullPath)
	return fullPath
}
