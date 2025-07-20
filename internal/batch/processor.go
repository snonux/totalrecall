package batch

import (
	"fmt"
	"os"
	"strings"
)

// WordEntry represents a word with optional translation
type WordEntry struct {
	Bulgarian   string
	Translation string
}

// ReadBatchFile reads words from a file and returns WordEntry slice
// Supports formats:
// - Bulgarian word only: "ябълка"
// - With translation: "ябълка = apple"
func ReadBatchFile(filename string) ([]WordEntry, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read batch file: %w", err)
	}

	var entries []WordEntry
	lines := string(content)

	for _, line := range splitLines(lines) {
		if line = trimSpace(line); line != "" {
			// Check if line contains '=' for bulgarian = english format
			if strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					bulgarian := strings.TrimSpace(parts[0])
					english := strings.TrimSpace(parts[1])
					if bulgarian != "" {
						entries = append(entries, WordEntry{
							Bulgarian:   bulgarian,
							Translation: english,
						})
					}
				}
			} else {
				// Just a bulgarian word
				entries = append(entries, WordEntry{
					Bulgarian:   line,
					Translation: "",
				})
			}
		}
	}

	return entries, nil
}

// splitLines splits a string by newlines
func splitLines(s string) []string {
	var lines []string
	current := ""
	for _, r := range s {
		if r == '\n' {
			lines = append(lines, current)
			current = ""
		} else if r != '\r' {
			current += string(r)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

// trimSpace trims whitespace from string
func trimSpace(s string) string {
	start := 0
	end := len(s)

	// Trim from start
	for start < end && isSpace(rune(s[start])) {
		start++
	}

	// Trim from end
	for end > start && isSpace(rune(s[end-1])) {
		end--
	}

	return s[start:end]
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}
