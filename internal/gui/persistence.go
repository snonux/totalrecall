package gui

import (
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"

	"codeberg.org/snonux/totalrecall/internal"
)

// ensureWordDirectoryAndMetadata creates a new card directory and writes word metadata.
func (a *Application) ensureWordDirectoryAndMetadata(word string) (string, error) {
	cardID := internal.GenerateCardID(word)
	wordDir := filepath.Join(a.config.OutputDir, cardID)
	if err := os.MkdirAll(wordDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create card directory: %w", err)
	}

	metadataFile := filepath.Join(wordDir, "word.txt")
	if err := os.WriteFile(metadataFile, []byte(word), 0644); err != nil {
		return "", fmt.Errorf("failed to save word metadata: %w", err)
	}

	return wordDir, nil
}

// ensureCardDirectory ensures a card directory exists for the given word and returns its path.
func (a *Application) ensureCardDirectory(word string) (string, error) {
	wordDir := a.findCardDirectory(word)
	if wordDir != "" {
		return wordDir, nil
	}

	return a.ensureWordDirectoryAndMetadata(word)
}

// saveTranslation saves the current translation to a file.
func (a *Application) saveTranslation() {
	if a.currentWord == "" || a.currentTranslation == "" {
		return
	}

	wordDir := a.findCardDirectory(a.currentWord)
	if wordDir == "" {
		newWordDir, err := a.ensureWordDirectoryAndMetadata(a.currentWord)
		if err != nil {
			a.showError(err)
			return
		}
		wordDir = newWordDir
	}

	translationFile := filepath.Join(wordDir, "translation.txt")
	content := fmt.Sprintf("%s = %s\n", a.currentWord, a.currentTranslation)
	if err := os.WriteFile(translationFile, []byte(content), 0644); err != nil {
		a.showError(fmt.Errorf("failed to save translation: %w", err))
	}
}

// saveImagePrompt saves the current image prompt to a file.
func (a *Application) saveImagePrompt() {
	// With timestamp-based card IDs, we can't update existing prompts.
	// The prompt is saved when the image is generated.
	// This function is kept for compatibility but does nothing.
}

// savePhoneticInfo saves the phonetic information to a file.
func (a *Application) savePhoneticInfo() {
	phoneticText := a.currentPhonetic
	if a.currentWord == "" || phoneticText == "" || phoneticText == "Failed to fetch phonetic information" {
		return
	}

	wordDir := a.findCardDirectory(a.currentWord)
	if wordDir == "" {
		newWordDir, err := a.ensureWordDirectoryAndMetadata(a.currentWord)
		if err != nil {
			a.showError(err)
			return
		}
		wordDir = newWordDir
	}

	phoneticFile := filepath.Join(wordDir, "phonetic.txt")
	if err := os.WriteFile(phoneticFile, []byte(phoneticText), 0644); err != nil {
		a.showError(fmt.Errorf("failed to save phonetic info: %w", err))
	}
}

// loadPhoneticInfo loads phonetic information from a file if it exists.
func (a *Application) loadPhoneticInfo(word string) {
	wordDir := a.findCardDirectory(word)
	if wordDir == "" {
		return
	}

	phoneticFile := filepath.Join(wordDir, "phonetic.txt")
	if data, err := os.ReadFile(phoneticFile); err == nil {
		phoneticText := string(data)
		a.currentPhonetic = phoneticText
		fyne.Do(func() {
			if phoneticText != "" {
				a.audioPlayer.SetPhonetic(phoneticText)
			} else {
				a.audioPlayer.SetPhonetic("")
			}
		})
	}
}
