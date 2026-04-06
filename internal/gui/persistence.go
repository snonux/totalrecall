package gui

import (
	"fmt"
)

// ensureWordDirectoryAndMetadata creates a new card directory and writes word
// metadata. Delegates to CardService.
func (a *Application) ensureWordDirectoryAndMetadata(word string) (string, error) {
	return a.getCardService().EnsureWordDirectoryAndMetadata(word)
}

// ensureCardDirectory ensures a card directory exists for the given word.
// Delegates to CardService.
func (a *Application) ensureCardDirectory(word string) (string, error) {
	return a.getCardService().EnsureCardDirectory(word)
}

// saveTranslation saves the current translation to disk.
// Delegates to CardService.
func (a *Application) saveTranslation() {
	if err := a.getCardService().SaveTranslation(a.currentWord, a.currentTranslation); err != nil {
		a.showError(err)
	}
}

// saveImagePrompt is retained for compatibility but is currently a no-op.
// The image prompt is saved by the image generation callback when the image
// is generated, so there is no need to save it separately here.
func (a *Application) saveImagePrompt() {
	// No-op: the prompt is saved as soon as it is generated via the callback.
}

// savePhoneticInfo saves the current phonetic information to disk.
// Delegates to CardService.
func (a *Application) savePhoneticInfo() {
	if err := a.getCardService().SavePhoneticInfo(a.currentWord, a.currentPhonetic); err != nil {
		a.showError(fmt.Errorf("failed to save phonetic info: %w", err))
	}
}
