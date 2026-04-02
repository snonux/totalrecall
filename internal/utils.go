package internal

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
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