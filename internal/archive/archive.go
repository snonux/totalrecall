package archive

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ArchiveCards moves the cards directory to an archive with timestamp
func ArchiveCards(cardsDir string) error {
	// Check if cards directory exists
	if _, err := os.Stat(cardsDir); os.IsNotExist(err) {
		return fmt.Errorf("cards directory does not exist: %s", cardsDir)
	}

	// Get parent directory and create archive path
	parentDir := filepath.Dir(cardsDir)
	archiveDir := filepath.Join(parentDir, "archive")

	// Create archive directory if it doesn't exist
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return fmt.Errorf("failed to create archive directory: %w", err)
	}

	// Generate timestamp
	timestamp := time.Now().Format("20060102-150405")
	archiveName := fmt.Sprintf("cards-%s", timestamp)
	archivePath := filepath.Join(archiveDir, archiveName)

	// Check if archive already exists (unlikely but possible)
	if _, err := os.Stat(archivePath); err == nil {
		// Add microseconds to make it unique
		timestamp = time.Now().Format("20060102-150405.000000")
		archiveName = fmt.Sprintf("cards-%s", timestamp)
		archivePath = filepath.Join(archiveDir, archiveName)
	}

	// Rename cards directory to archive
	if err := os.Rename(cardsDir, archivePath); err != nil {
		return fmt.Errorf("failed to archive cards directory: %w", err)
	}

	fmt.Printf("Cards directory archived to: %s\n", archivePath)
	return nil
}
