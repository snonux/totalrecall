package internal

import (
	"strings"

	"codeberg.org/snonux/totalrecall/internal/store"
)

// GenerateCardID creates a unique ID for a card based on timestamp and Bulgarian word.
// Delegates to store.GenerateCardID which is the single source of truth.
// Format: epochMillis_md5(word)[:8]
func GenerateCardID(bulgarianWord string) string {
	return store.GenerateCardID(bulgarianWord)
}

// FindCardDirectory searches outputDir for a subdirectory whose word.txt
// (or legacy _word.txt) matches the given word. Returns the directory path
// or an empty string if not found.
// Delegates to store.FindCardDirectory which is the single source of truth.
func FindCardDirectory(outputDir, word string) string {
	return store.FindCardDirectory(outputDir, word)
}

// FindOrCreateCardDirectory returns the existing card directory for word inside
// outputDir, or creates a new one with a generated card ID. It also writes
// word.txt so subsequent calls can find the directory.
// Delegates to store.FindOrCreateCardDirectory which is the single source of truth.
func FindOrCreateCardDirectory(outputDir, word string) string {
	return store.FindOrCreateCardDirectory(outputDir, word)
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

// isAlphaNumeric checks if a rune is alphanumeric (Latin or Cyrillic).
func isAlphaNumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || (r >= 'а' && r <= 'я') ||
		(r >= 'А' && r <= 'Я')
}
