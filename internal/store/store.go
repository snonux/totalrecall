// Package store provides the CardStore type, a single shared repository for
// locating and creating on-disk card directories. Both the processor and gui
// packages use CardStore so the directory-scanning logic lives in exactly one
// place (DRY / SRP).
//
// Dependency position: store imports only the standard library, so every other
// internal package may import it without creating import cycles.
package store

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CardStore manages the on-disk layout of word card directories under a single
// output directory. It is safe to create multiple instances pointing at the
// same directory; all operations are stateless file-system reads/writes.
type CardStore struct {
	outputDir string
}

// New constructs a CardStore rooted at outputDir.
func New(outputDir string) *CardStore {
	return &CardStore{outputDir: outputDir}
}

// OutputDir returns the root output directory this store operates on.
func (cs *CardStore) OutputDir() string {
	return cs.outputDir
}

// FindCardDirectory searches the output directory for a subdirectory whose
// word.txt (or legacy _word.txt) matches word. Returns the directory path or
// an empty string when no matching directory is found.
func (cs *CardStore) FindCardDirectory(word string) string {
	return FindCardDirectory(cs.outputDir, word)
}

// FindOrCreateCardDirectory returns the existing card directory for word, or
// creates a new one with a generated card ID and writes word.txt so subsequent
// calls can locate the directory.
func (cs *CardStore) FindOrCreateCardDirectory(word string) string {
	return FindOrCreateCardDirectory(cs.outputDir, word)
}

// ScanWords scans the output directory for subdirectories that contain at
// least one content file (word.txt or legacy _word.txt) and passes a basic
// content check provided by the caller. It returns a sorted list of Bulgarian
// words found. The hasContent predicate receives the full path of each
// candidate directory; pass nil to accept all directories that have a word
// file.
func (cs *CardStore) ScanWords(hasContent func(wordDir string) bool) []string {
	entries, err := os.ReadDir(cs.outputDir)
	if err != nil {
		// Output directory does not exist yet; return empty list silently.
		return []string{}
	}

	words := make([]string, 0, len(entries))

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		wordDir := filepath.Join(cs.outputDir, entry.Name())

		word, ok := readWordFromDir(wordDir)
		if !ok {
			continue
		}

		if hasContent == nil || hasContent(wordDir) {
			words = append(words, word)
		}
	}

	sort.Strings(words)
	return words
}

// GenerateCardID creates a unique ID for a card based on the current timestamp
// and an MD5 hash of the Bulgarian word.
// Format: epochMillis_md5(word)[:8]
func GenerateCardID(bulgarianWord string) string {
	epochMillis := time.Now().UnixNano() / 1_000_000
	hash := md5.Sum([]byte(bulgarianWord))
	hashStr := hex.EncodeToString(hash[:])[:8]
	return fmt.Sprintf("%d_%s", epochMillis, hashStr)
}

// FindCardDirectory is the package-level (non-method) version of the directory
// search. It searches outputDir for a subdirectory whose word.txt (or legacy
// _word.txt) content matches word and returns its path, or "" if not found.
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
		if found := readWordMatch(dirPath, word); found {
			return dirPath
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

// readWordFromDir reads the Bulgarian word stored in word.txt (or the legacy
// _word.txt) inside a card directory. Returns the word and true on success.
func readWordFromDir(wordDir string) (string, bool) {
	wordFile := filepath.Join(wordDir, "word.txt")
	data, err := os.ReadFile(wordFile)
	if err != nil {
		// Backward-compatible fallback: old format used _word.txt.
		wordFile = filepath.Join(wordDir, "_word.txt")
		data, err = os.ReadFile(wordFile)
		if err != nil {
			return "", false
		}
	}

	word := strings.TrimSpace(string(data))
	if word == "" {
		return "", false
	}

	return word, true
}

// readWordMatch returns true when the card directory at dirPath contains a
// word file whose trimmed content equals word.
func readWordMatch(dirPath, word string) bool {
	found, ok := readWordFromDir(dirPath)
	return ok && found == word
}
