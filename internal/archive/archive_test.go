package archive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestArchiveCards(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create cards directory with some test files
	cardsDir := filepath.Join(tmpDir, "cards")
	if err := os.MkdirAll(cardsDir, 0755); err != nil {
		t.Fatalf("Failed to create cards directory: %v", err)
	}

	// Create some test files in cards directory
	testFile := filepath.Join(cardsDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a subdirectory with a file
	subDir := filepath.Join(cardsDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}
	subFile := filepath.Join(subDir, "subfile.txt")
	if err := os.WriteFile(subFile, []byte("sub content"), 0644); err != nil {
		t.Fatalf("Failed to create sub file: %v", err)
	}

	// Archive the cards directory
	if err := ArchiveCards(cardsDir); err != nil {
		t.Fatalf("ArchiveCards failed: %v", err)
	}

	// Check that cards directory no longer exists
	if _, err := os.Stat(cardsDir); !os.IsNotExist(err) {
		t.Error("Cards directory still exists after archiving")
	}

	// Check that archive directory was created
	archiveDir := filepath.Join(tmpDir, "archive")
	if _, err := os.Stat(archiveDir); os.IsNotExist(err) {
		t.Error("Archive directory was not created")
	}

	// Check that archived directory exists with timestamp
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		t.Fatalf("Failed to read archive directory: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry in archive directory, got %d", len(entries))
	}

	// Verify the archived directory name starts with "cards-"
	archivedName := entries[0].Name()
	if !strings.HasPrefix(archivedName, "cards-") {
		t.Errorf("Archived directory name doesn't start with 'cards-': %s", archivedName)
	}

	// Verify timestamp format (should be cards-YYYYMMDD-HHMMSS)
	parts := strings.Split(archivedName, "-")
	if len(parts) < 3 {
		t.Errorf("Invalid archive name format: %s", archivedName)
	}

	// Check that archived files exist
	archivedPath := filepath.Join(archiveDir, archivedName)
	archivedTestFile := filepath.Join(archivedPath, "test.txt")
	if _, err := os.Stat(archivedTestFile); os.IsNotExist(err) {
		t.Error("Test file not found in archive")
	}

	archivedSubFile := filepath.Join(archivedPath, "subdir", "subfile.txt")
	if _, err := os.Stat(archivedSubFile); os.IsNotExist(err) {
		t.Error("Sub file not found in archive")
	}
}

func TestArchiveCards_NonExistentDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentDir := filepath.Join(tmpDir, "nonexistent")

	err := ArchiveCards(nonExistentDir)
	if err == nil {
		t.Error("Expected error for non-existent directory")
	}

	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Expected 'does not exist' error, got: %v", err)
	}
}

func TestArchiveCards_MultipleArchives(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Archive twice to ensure unique timestamps
	for i := 0; i < 2; i++ {
		// Create cards directory
		cardsDir := filepath.Join(tmpDir, "cards")
		if err := os.MkdirAll(cardsDir, 0755); err != nil {
			t.Fatalf("Failed to create cards directory: %v", err)
		}

		// Create a test file
		testFile := filepath.Join(cardsDir, "test.txt")
		content := []byte("test content " + string(rune(i)))
		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Small delay to ensure different timestamps
		if i == 1 {
			time.Sleep(10 * time.Millisecond)
		}

		// Archive
		if err := ArchiveCards(cardsDir); err != nil {
			t.Fatalf("ArchiveCards failed on iteration %d: %v", i, err)
		}
	}

	// Check that we have 2 archives
	archiveDir := filepath.Join(tmpDir, "archive")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		t.Fatalf("Failed to read archive directory: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries in archive directory, got %d", len(entries))
	}

	// Verify both archives have different names
	if entries[0].Name() == entries[1].Name() {
		t.Error("Archive names are not unique")
	}
}
