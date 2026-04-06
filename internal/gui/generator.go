package gui

import (
	"context"
	"math/rand"
	"time"

	"codeberg.org/snonux/totalrecall/internal/audio"
	"codeberg.org/snonux/totalrecall/internal/image"
)

// promptAwareImageClient extends ImageClient with prompt-callback support
// used by the GUI to capture and display the last generated image prompt.
type promptAwareImageClient interface {
	image.ImageClient
	SetPromptCallback(func(prompt string))
}

// randomVoice picks a random voice from the provided list.
// Used by GenerationOrchestrator for both OpenAI and Gemini voice selection.
func randomVoice(voices []string) string {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return voices[rng.Intn(len(voices))]
}

// randomOpenAISpeed picks a random speed in [0.90, 1.00) for OpenAI TTS to
// add slight variation across generations.
func randomOpenAISpeed() float64 {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return 0.90 + rng.Float64()*0.10
}

// --- Application delegation methods ---
// Each method delegates to getOrchestrator() so tests that create Application
// directly (setting newAudioProvider / audioConfig / config) continue to work
// without modification, while production code uses the pre-built orchestrator.

// audioProviderName returns the lowercase TTS provider name.
func (a *Application) audioProviderName() string {
	return a.getOrchestrator().audioProviderName()
}

// audioOutputFormat resolves the effective audio output format.
func (a *Application) audioOutputFormat() string {
	return a.getOrchestrator().audioOutputFormat()
}

// translateWord translates a Bulgarian word to English.
func (a *Application) translateWord(word string) (string, error) {
	return a.getOrchestrator().TranslateWord(word)
}

// translateEnglishToBulgarian translates an English word to Bulgarian.
func (a *Application) translateEnglishToBulgarian(word string) (string, error) {
	return a.getOrchestrator().TranslateEnglishToBulgarian(word)
}

// generateAudio generates audio for an en-bg card's single audio file.
func (a *Application) generateAudio(ctx context.Context, word, cardDir string) (string, error) {
	return a.getOrchestrator().GenerateAudio(ctx, word, cardDir)
}

// generateAudioFront generates the front audio file for a bg-bg card.
func (a *Application) generateAudioFront(ctx context.Context, word, cardDir string) (string, error) {
	return a.getOrchestrator().GenerateAudioFront(ctx, word, cardDir)
}

// generateAudioBack generates the back audio file for a bg-bg card.
func (a *Application) generateAudioBack(ctx context.Context, text, cardDir string) (string, error) {
	return a.getOrchestrator().GenerateAudioBack(ctx, text, cardDir)
}

// generateAudioBgBg generates audio for both sides of a bg-bg card.
func (a *Application) generateAudioBgBg(ctx context.Context, front, back, cardDir string) (string, string, error) {
	return a.getOrchestrator().GenerateAudioBgBg(ctx, front, back, cardDir)
}

// generateAudioFile generates a single audio file using the given voice/speed.
// Kept as a thin wrapper so the fallback tests in generator_test.go can call it.
func (a *Application) generateAudioFile(ctx context.Context, text, outputFile, voice string, speed float64) error {
	return a.getOrchestrator().generateAudioFile(ctx, text, outputFile, voice, speed)
}

// generateImagesWithPrompt downloads a single image for a word with optional
// custom prompt and translation hint.
func (a *Application) generateImagesWithPrompt(ctx context.Context, word, customPrompt, translation, cardDir string) (string, error) {
	o := a.getOrchestrator()

	// Wrap with a UI update callback for the current word's image prompt entry.
	promptUI := func(prompt string) {
		a.mu.Lock()
		isCurrentWord := a.currentWord == word
		a.mu.Unlock()

		if isCurrentWord && a.imagePromptEntry != nil {
			a.imagePromptEntry.SetText(prompt)
		}
	}

	return o.generateImagesWithPromptAndNotify(ctx, word, customPrompt, translation, cardDir, promptUI)
}

// getPhoneticInfo fetches phonetic information for a Bulgarian word.
func (a *Application) getPhoneticInfo(word string) (string, error) {
	return a.getOrchestrator().GetPhoneticInfo(word)
}

// saveAudioAttribution saves attribution metadata for a generated audio file.
func (a *Application) saveAudioAttribution(word, audioFile, voice string, speed float64) error {
	return a.getOrchestrator().saveAudioAttribution(word, audioFile, voice, speed)
}

// saveAudioMetadata writes the sidecar metadata file for a generated audio file.
func (a *Application) saveAudioMetadata(cardDir string, audioCfg audio.Config, voice string, speed float64, cardType, audioFile, audioFileBack string) error {
	return a.getOrchestrator().saveAudioMetadata(cardDir, audioCfg, voice, speed, cardType, audioFile, audioFileBack)
}
