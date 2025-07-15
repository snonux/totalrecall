package audio

import (
	"fmt"
	"strings"
	"unicode"
)

// ValidateBulgarianText validates that the input text contains valid Bulgarian text
func ValidateBulgarianText(text string) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("text cannot be empty")
	}
	
	hasCyrillic := false
	for _, r := range text {
		if unicode.In(r, unicode.Cyrillic) {
			hasCyrillic = true
			break
		}
	}
	
	if !hasCyrillic {
		return fmt.Errorf("text must contain Cyrillic characters")
	}
	
	return nil
}