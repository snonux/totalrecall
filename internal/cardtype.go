package internal

import (
	"os"
	"path/filepath"
	"strings"
)

// CardType represents the type of flashcard
type CardType string

const (
	// CardTypeEnBg represents English-Bulgarian cards (default)
	CardTypeEnBg CardType = "en-bg"
	// CardTypeBgBg represents Bulgarian-Bulgarian cards
	CardTypeBgBg CardType = "bg-bg"
)

// String returns the string representation of the card type
func (ct CardType) String() string {
	return string(ct)
}

// IsBgBg returns true if this is a Bulgarian-Bulgarian card
func (ct CardType) IsBgBg() bool {
	return ct == CardTypeBgBg
}

// DisplayName returns a human-readable name for the card type
func (ct CardType) DisplayName() string {
	switch ct {
	case CardTypeBgBg:
		return "Bulgarian → Bulgarian"
	default:
		return "English → Bulgarian"
	}
}

// SaveCardType saves the card type to a file in the card directory
func SaveCardType(cardDir string, cardType CardType) error {
	cardTypePath := filepath.Join(cardDir, "cardtype.txt")
	return os.WriteFile(cardTypePath, []byte(string(cardType)), 0644)
}

// LoadCardType loads the card type from a card directory
// Returns CardTypeEnBg as default for backwards compatibility
func LoadCardType(cardDir string) CardType {
	cardTypePath := filepath.Join(cardDir, "cardtype.txt")
	data, err := os.ReadFile(cardTypePath)
	if err != nil {
		return CardTypeEnBg
	}
	cardType := CardType(strings.TrimSpace(string(data)))
	if cardType == CardTypeBgBg {
		return CardTypeBgBg
	}
	return CardTypeEnBg
}
