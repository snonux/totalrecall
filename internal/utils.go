package internal

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GenerateCardID creates a unique ID for a card based on timestamp and Bulgarian word
// Format: epochMillis_md5(word)[:8]
func GenerateCardID(bulgarianWord string) string {
	// Get current timestamp in milliseconds
	now := time.Now()
	epochMillis := now.UnixNano() / 1000000

	// Calculate MD5 hash of the word
	hash := md5.Sum([]byte(bulgarianWord))
	hashStr := hex.EncodeToString(hash[:])[:8] // Use first 8 chars of MD5

	// Combine timestamp and hash
	return fmt.Sprintf("%d_%s", epochMillis, hashStr)
}

// FindCardDirectory searches outputDir for a subdirectory whose word.txt
// (or legacy _word.txt) matches the given word. Returns the directory path
// or an empty string if not found.
func FindCardDirectory(outputDir, word string) string {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		dirPath := filepath.Join(outputDir, entry.Name())
		wordFile := filepath.Join(dirPath, "word.txt")

		if data, err := os.ReadFile(wordFile); err == nil {
			if strings.TrimSpace(string(data)) == word {
				return dirPath
			}
		} else {
			// Backward-compatible fallback: old format used _word.txt
			wordFile = filepath.Join(dirPath, "_word.txt")
			if data, err := os.ReadFile(wordFile); err == nil {
				if strings.TrimSpace(string(data)) == word {
					return dirPath
				}
			}
		}
	}

	return ""
}

// FindOrCreateCardDirectory returns the existing card directory for word inside
// outputDir, or creates a new one with a generated card ID. It also writes
// word.txt so subsequent calls can find the directory.
func FindOrCreateCardDirectory(outputDir, word string) string {
	if dir := FindCardDirectory(outputDir, word); dir != "" {
		return dir
	}

	cardID := GenerateCardID(word)
	wordDir := filepath.Join(outputDir, cardID)
	if err := os.MkdirAll(wordDir, 0755); err != nil {
		fmt.Printf("Warning: failed to create word directory: %v\n", err)
		return outputDir
	}

	if err := os.WriteFile(filepath.Join(wordDir, "word.txt"), []byte(word), 0644); err != nil {
		fmt.Printf("Warning: failed to save word metadata: %v\n", err)
	}

	return wordDir
}

// SanitizeFilename creates a safe filename from a string.
// Uses strings.Builder to avoid per-rune heap allocations.
func SanitizeFilename(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if isAlphaNumeric(r) || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// isAlphaNumeric checks if a rune is alphanumeric
func isAlphaNumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || (r >= 'а' && r <= 'я') ||
		(r >= 'А' && r <= 'Я')
}
