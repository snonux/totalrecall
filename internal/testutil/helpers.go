package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// CreateTestDirectory creates a temporary directory structure for testing
func CreateTestDirectory(t *testing.T) string {
	t.Helper()

	tempDir := t.TempDir()

	// Create common test structure
	dirs := []string{
		"audio",
		"images",
		"output",
		"cache",
	}

	for _, dir := range dirs {
		path := filepath.Join(tempDir, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatalf("Failed to create test directory %s: %v", path, err)
		}
	}

	return tempDir
}

// CreateTestFile creates a test file with content
func CreateTestFile(t *testing.T, path string, content []byte) {
	t.Helper()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create directory for test file: %v", err)
	}

	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("Failed to create test file %s: %v", path, err)
	}
}

// CreateTestWordDirectory creates a test word directory with standard files
func CreateTestWordDirectory(t *testing.T, baseDir, word string) string {
	t.Helper()

	wordDir := filepath.Join(baseDir, word)
	if err := os.MkdirAll(wordDir, 0755); err != nil {
		t.Fatalf("Failed to create word directory: %v", err)
	}

	// Create standard files
	files := map[string]string{
		"word.txt":        word,
		"translation.txt": word + " = test translation",
		"phonetic.txt":    "test phonetic info",
	}

	for filename, content := range files {
		path := filepath.Join(wordDir, filename)
		CreateTestFile(t, path, []byte(content))
	}

	// Create mock audio file
	audioPath := filepath.Join(wordDir, "audio.mp3")
	CreateTestFile(t, audioPath, []byte{0xFF, 0xFB, 0x90, 0x00})

	// Create mock image file
	imagePath := filepath.Join(wordDir, "image.jpg")
	CreateTestFile(t, imagePath, []byte{0xFF, 0xD8, 0xFF, 0xE0})

	return wordDir
}

// AssertFileExists checks if a file exists
func AssertFileExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("Expected file to exist: %s", path)
	}
}

// AssertFileNotExists checks if a file does not exist
func AssertFileNotExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err == nil {
		t.Errorf("Expected file to not exist: %s", path)
	}
}

// AssertFileContent checks if a file has expected content
func AssertFileContent(t *testing.T, path string, expected []byte) {
	t.Helper()

	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", path, err)
	}

	if string(actual) != string(expected) {
		t.Errorf("File content mismatch in %s\nExpected: %q\nActual: %q", path, expected, actual)
	}
}

// AssertFileContains checks if a file contains a substring
func AssertFileContains(t *testing.T, path string, substring string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", path, err)
	}

	if !contains(string(content), substring) {
		t.Errorf("File %s does not contain expected substring: %q", path, substring)
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr) >= 0))
}

// findSubstring finds the index of substr in s, or -1 if not found
func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// CompareDirectories compares two directories recursively
func CompareDirectories(t *testing.T, dir1, dir2 string) {
	t.Helper()

	err := filepath.Walk(dir1, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(dir1, path)
		if err != nil {
			return err
		}

		// Check if corresponding file exists in dir2
		path2 := filepath.Join(dir2, relPath)
		info2, err := os.Stat(path2)
		if err != nil {
			t.Errorf("File missing in second directory: %s", relPath)
			return nil
		}

		// Compare file types
		if info.IsDir() != info2.IsDir() {
			t.Errorf("File type mismatch for %s", relPath)
			return nil
		}

		// Compare file sizes (for files only)
		if !info.IsDir() && info.Size() != info2.Size() {
			t.Errorf("File size mismatch for %s: %d vs %d", relPath, info.Size(), info2.Size())
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Failed to compare directories: %v", err)
	}
}

// CaptureOutput captures stdout/stderr during test execution
func CaptureOutput(t *testing.T, f func()) (stdout, stderr string) {
	t.Helper()

	// Save current stdout/stderr
	oldStdout := os.Stdout
	oldStderr := os.Stderr

	// Create pipes
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()

	// Redirect stdout/stderr
	os.Stdout = wOut
	os.Stderr = wErr

	// Run function
	f()

	// Close writers
	wOut.Close()
	wErr.Close()

	// Read output
	outBytes := make([]byte, 1024)
	errBytes := make([]byte, 1024)

	nOut, _ := rOut.Read(outBytes)
	nErr, _ := rErr.Read(errBytes)

	// Restore stdout/stderr
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	return string(outBytes[:nOut]), string(errBytes[:nErr])
}
