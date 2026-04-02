package batch

import (
	"fmt"
	"os"
	"strings"

	"codeberg.org/snonux/totalrecall/internal"
)

// WordEntry represents a word with optional translation
type WordEntry struct {
	Bulgarian   string
	Translation string
	// NeedsTranslation indicates if translation from English to Bulgarian is needed
	NeedsTranslation bool
	// CardType indicates whether this is en-bg or bg-bg card
	CardType internal.CardType
}

// ReadBatchFile reads words from a file and returns WordEntry slice
// Supports formats:
// - Bulgarian word only: "ябълка" (will be translated to English)
// - With translation: "ябълка = apple" (both provided, no translation needed)
// - English only: "= apple" (will be translated to Bulgarian)
// - Bulgarian-Bulgarian: "word1 == definition" (bg-bg card, double equals)
func ReadBatchFile(filename string) ([]WordEntry, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read batch file: %w", err)
	}

	var entries []WordEntry
	// Normalize \r\n to \n before splitting so both Windows and Unix line
	// endings are handled uniformly by strings.Split.
	normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
	for _, line := range strings.Split(normalized, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			entry := parseBatchLine(line)
			if entry != nil {
				entries = append(entries, *entry)
			}
		}
	}

	return entries, nil
}

// parseBatchLine parses a single batch file line and returns the appropriate WordEntry
func parseBatchLine(line string) *WordEntry {
	// Check for Bulgarian-Bulgarian format first (double equals ==)
	if strings.Contains(line, "==") {
		parts := strings.SplitN(line, "==", 2)
		if len(parts) == 2 {
			bulgarian1 := strings.TrimSpace(parts[0])
			bulgarian2 := strings.TrimSpace(parts[1])

			if bulgarian1 != "" && bulgarian2 != "" {
				return &WordEntry{
					Bulgarian:        bulgarian1,
					Translation:      bulgarian2,
					NeedsTranslation: false,
					CardType:         internal.CardTypeBgBg,
				}
			}
		}
		return nil
	}

	// Check for English-Bulgarian format (single equals =)
	if strings.Contains(line, "=") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			bulgarian := strings.TrimSpace(parts[0])
			english := strings.TrimSpace(parts[1])

			if bulgarian == "" && english != "" {
				// Format: "= ENGLISH" - need to translate English to Bulgarian
				return &WordEntry{
					Bulgarian:        "",
					Translation:      english,
					NeedsTranslation: true,
					CardType:         internal.CardTypeEnBg,
				}
			} else if bulgarian != "" && english != "" {
				// Format: "BULGARIAN = ENGLISH" - both provided
				return &WordEntry{
					Bulgarian:        bulgarian,
					Translation:      english,
					NeedsTranslation: false,
					CardType:         internal.CardTypeEnBg,
				}
			}
		}
		return nil
	}

	// Just a Bulgarian word - needs translation to English
	return &WordEntry{
		Bulgarian:        line,
		Translation:      "",
		NeedsTranslation: false,
		CardType:         internal.CardTypeEnBg,
	}
}

