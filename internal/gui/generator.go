package gui

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"

	"codeberg.org/snonux/totalrecall/internal/audio"
	"codeberg.org/snonux/totalrecall/internal/image"
)

type promptAwareImageClient interface {
	image.ImageSearcher
	SetPromptCallback(func(prompt string))
}

var newOpenAIImageClient = func(config *image.OpenAIConfig) promptAwareImageClient {
	return image.NewOpenAIClient(config)
}

var newNanoBananaImageClient = func(config *image.NanoBananaConfig) promptAwareImageClient {
	return image.NewNanoBananaClient(config)
}

var newAudioProvider = audio.NewProvider

func randomVoiceAndSpeed(voices []string) (string, float64) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	voice := voices[rng.Intn(len(voices))]
	speed := 0.90 + rng.Float64()*0.10
	return voice, speed
}

// translateWord translates a Bulgarian word to English
func (a *Application) translateWord(word string) (string, error) {
	if a.translator == nil {
		return "", fmt.Errorf("translation service not configured")
	}

	return a.translator.TranslateWord(word)
}

// translateEnglishToBulgarian translates an English word to Bulgarian
func (a *Application) translateEnglishToBulgarian(word string) (string, error) {
	if a.translator == nil {
		return "", fmt.Errorf("translation service not configured")
	}

	return a.translator.TranslateEnglishToBulgarian(word)
}

// generateAudio generates audio for a word
func (a *Application) generateAudio(ctx context.Context, word string, cardDir string) (string, error) {
	// Check if this is a regeneration by looking for existing audio file
	isRegeneration := false
	if cardDir != "" {
		audioFile := filepath.Join(cardDir, fmt.Sprintf("audio.%s", a.config.AudioFormat))
		if _, err := os.Stat(audioFile); err == nil {
			isRegeneration = true
		}
	}

	// Select a random voice
	voice, speed := randomVoiceAndSpeed(audio.OpenAIVoices)

	// Create a copy of audio config with selected voice and speed
	audioConfig := *a.audioConfig
	audioConfig.OpenAIVoice = voice
	audioConfig.OpenAISpeed = speed
	audioConfig.OutputDir = a.config.OutputDir // Ensure correct output directory

	// Log the audio generation details
	if isRegeneration {
		fmt.Printf("Regenerating audio for '%s' with voice: %s, speed: %.2f\n", word, voice, speed)
	} else {
		fmt.Printf("Generating audio for '%s' with voice: %s, speed: %.2f\n", word, voice, speed)
	}

	// Create audio provider
	provider, err := newAudioProvider(&audioConfig)
	if err != nil {
		return "", err
	}

	// Use the provided card directory
	if cardDir == "" {
		return "", fmt.Errorf("card directory not provided")
	}

	// Generate filename in subdirectory
	outputFile := filepath.Join(cardDir, fmt.Sprintf("audio.%s", a.config.AudioFormat))

	// Generate audio
	err = provider.GenerateAudio(ctx, word, outputFile)
	if err != nil {
		return "", err
	}

	// Save audio attribution
	if err := a.saveAudioAttribution(word, outputFile, voice, speed); err != nil {
		// Non-fatal error, just log it
		fmt.Printf("Warning: Failed to save audio attribution: %v\n", err)
	}

	// Save voice metadata for GUI display
	metadataFile := filepath.Join(cardDir, "audio_metadata.txt")
	metadata := fmt.Sprintf("voice=%s\nspeed=%.2f\n", voice, speed)
	if err := os.WriteFile(metadataFile, []byte(metadata), 0644); err != nil {
		fmt.Printf("Warning: Failed to save audio metadata: %v\n", err)
	}

	return outputFile, nil
}

// generateAudioFront generates front audio for a bg-bg card
func (a *Application) generateAudioFront(ctx context.Context, word string, cardDir string) (string, error) {
	fmt.Printf("DEBUG (generateAudioFront): Called with word: %s, cardDir: %s\n", word, cardDir)

	if cardDir == "" {
		fmt.Printf("DEBUG (generateAudioFront): Card directory not provided, returning error\n")
		return "", fmt.Errorf("card directory not provided")
	}

	voice, speed := randomVoiceAndSpeed(audio.OpenAIVoices)

	audioConfig := *a.audioConfig
	audioConfig.OpenAIVoice = voice
	audioConfig.OpenAISpeed = speed
	audioConfig.OutputDir = a.config.OutputDir

	provider, err := newAudioProvider(&audioConfig)
	if err != nil {
		fmt.Printf("DEBUG (generateAudioFront): Failed to create audio provider: %v\n", err)
		return "", err
	}

	fmt.Printf("DEBUG (generateAudioFront): Generating front audio for '%s' with voice: %s, speed: %.2f\n", word, voice, speed)
	fmt.Printf("Generating front audio for '%s' with voice: %s, speed: %.2f\n", word, voice, speed)
	frontFile := filepath.Join(cardDir, fmt.Sprintf("audio_front.%s", a.config.AudioFormat))
	fmt.Printf("DEBUG (generateAudioFront): Will write to: %s\n", frontFile)
	if err := provider.GenerateAudio(ctx, word, frontFile); err != nil {
		return "", fmt.Errorf("failed to generate front audio: %w", err)
	}
	fmt.Printf("DEBUG (generateAudioFront): Successfully wrote front audio to: %s\n", frontFile)

	// Update metadata
	metadataFile := filepath.Join(cardDir, "audio_metadata.txt")
	metadata := fmt.Sprintf("voice=%s\nspeed=%.2f\ncardtype=bg-bg\n", voice, speed)
	if err := os.WriteFile(metadataFile, []byte(metadata), 0644); err != nil {
		fmt.Printf("Warning: Failed to save audio metadata: %v\n", err)
	}

	return frontFile, nil
}

// generateAudioBack generates back audio for a bg-bg card
func (a *Application) generateAudioBack(ctx context.Context, text string, cardDir string) (string, error) {
	fmt.Printf("DEBUG (generateAudioBack): Called with text: %s, cardDir: %s\n", text, cardDir)

	if cardDir == "" {
		fmt.Printf("DEBUG (generateAudioBack): Card directory not provided, returning error\n")
		return "", fmt.Errorf("card directory not provided")
	}

	voice, speed := randomVoiceAndSpeed(audio.OpenAIVoices)

	audioConfig := *a.audioConfig
	audioConfig.OpenAIVoice = voice
	audioConfig.OpenAISpeed = speed
	audioConfig.OutputDir = a.config.OutputDir

	provider, err := newAudioProvider(&audioConfig)
	if err != nil {
		fmt.Printf("DEBUG (generateAudioBack): Failed to create audio provider: %v\n", err)
		return "", err
	}

	fmt.Printf("DEBUG (generateAudioBack): Generating back audio for '%s' with voice: %s, speed: %.2f\n", text, voice, speed)
	fmt.Printf("Generating back audio for '%s' with voice: %s, speed: %.2f\n", text, voice, speed)
	backFile := filepath.Join(cardDir, fmt.Sprintf("audio_back.%s", a.config.AudioFormat))
	fmt.Printf("DEBUG (generateAudioBack): Will write to: %s\n", backFile)
	if err := provider.GenerateAudio(ctx, text, backFile); err != nil {
		return "", fmt.Errorf("failed to generate back audio: %w", err)
	}
	fmt.Printf("DEBUG (generateAudioBack): Successfully wrote back audio to: %s\n", backFile)

	return backFile, nil
}

// generateAudioBgBg generates audio for both sides of a bg-bg card
func (a *Application) generateAudioBgBg(ctx context.Context, front, back, cardDir string) (string, string, error) {
	if cardDir == "" {
		return "", "", fmt.Errorf("card directory not provided")
	}

	voice, speed := randomVoiceAndSpeed(audio.OpenAIVoices)

	audioConfig := *a.audioConfig
	audioConfig.OpenAIVoice = voice
	audioConfig.OpenAISpeed = speed
	audioConfig.OutputDir = a.config.OutputDir

	provider, err := audio.NewProvider(&audioConfig)
	if err != nil {
		return "", "", err
	}

	// Generate front audio
	fmt.Printf("Generating front audio for '%s' with voice: %s, speed: %.2f\n", front, voice, speed)
	frontFile := filepath.Join(cardDir, fmt.Sprintf("audio_front.%s", a.config.AudioFormat))
	if err := provider.GenerateAudio(ctx, front, frontFile); err != nil {
		return "", "", fmt.Errorf("failed to generate front audio: %w", err)
	}

	// Generate back audio
	fmt.Printf("Generating back audio for '%s' with voice: %s, speed: %.2f\n", back, voice, speed)
	backFile := filepath.Join(cardDir, fmt.Sprintf("audio_back.%s", a.config.AudioFormat))
	if err := provider.GenerateAudio(ctx, back, backFile); err != nil {
		return frontFile, "", fmt.Errorf("failed to generate back audio: %w", err)
	}

	// Save audio attribution
	if err := a.saveAudioAttribution(front, frontFile, voice, speed); err != nil {
		fmt.Printf("Warning: Failed to save audio attribution: %v\n", err)
	}

	// Save voice metadata
	metadataFile := filepath.Join(cardDir, "audio_metadata.txt")
	metadata := fmt.Sprintf("voice=%s\nspeed=%.2f\ncardtype=bg-bg\n", voice, speed)
	if err := os.WriteFile(metadataFile, []byte(metadata), 0644); err != nil {
		fmt.Printf("Warning: Failed to save audio metadata: %v\n", err)
	}

	return frontFile, backFile, nil
}

// generateImagesWithPrompt downloads a single image for a word with optional custom prompt and translation
func (a *Application) generateImagesWithPrompt(ctx context.Context, word string, customPrompt string, translation string, cardDir string) (string, error) {
	searcher, err := a.newImageSearcher()
	if err != nil {
		return "", err
	}

	// Use the provided card directory
	if cardDir == "" {
		return "", fmt.Errorf("card directory not provided")
	}

	// Create downloader
	downloadOpts := &image.DownloadOptions{
		OutputDir:         cardDir,
		OverwriteExisting: true,
		CreateDir:         true,
		FileNamePattern:   "image",
		MaxSizeBytes:      5 * 1024 * 1024, // 5MB
	}

	downloader := image.NewDownloader(searcher, downloadOpts)

	// Set up a prompt callback so the GUI and on-disk metadata update as soon as the prompt exists.
	searcher.SetPromptCallback(a.imagePromptCallback(cardDir, word))

	// Create search options with custom prompt and translation if provided
	searchOpts := image.DefaultSearchOptions(word)
	if customPrompt != "" {
		searchOpts.CustomPrompt = customPrompt
	}
	if translation != "" {
		searchOpts.Translation = translation
	}

	// Download single image
	_, path, err := downloader.DownloadBestMatchWithOptions(ctx, searchOpts)
	if err != nil {
		return "", err
	}

	// The prompt has already been saved and UI updated via the callback

	return path, nil
}

func (a *Application) newImageSearcher() (promptAwareImageClient, error) {
	switch a.config.ImageProvider {
	case imageProviderOpenAI:
		if a.config.OpenAIKey == "" {
			return nil, fmt.Errorf("OpenAI API key is required for image generation")
		}

		openaiConfig := &image.OpenAIConfig{
			APIKey:  a.config.OpenAIKey,
			Model:   "dall-e-2", // DALL-E 2 supports 512x512
			Size:    "512x512",  // Half of 1024x1024
			Quality: "standard",
			Style:   "natural",
		}

		return newOpenAIImageClient(openaiConfig), nil
	case imageProviderNanoBanana:
		if a.config.GoogleAPIKey == "" {
			return nil, fmt.Errorf("Google API key is required for image generation")
		}

		nanoBananaConfig := &image.NanoBananaConfig{
			APIKey: a.config.GoogleAPIKey,
		}

		return newNanoBananaImageClient(nanoBananaConfig), nil
	default:
		return nil, fmt.Errorf("unknown image provider: %s", a.config.ImageProvider)
	}
}

func (a *Application) imagePromptCallback(cardDir, word string) func(prompt string) {
	return func(prompt string) {
		// Save the prompt to disk immediately for this word.
		promptFile := filepath.Join(cardDir, "image_prompt.txt")
		if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
			fmt.Printf("Warning: Failed to save prompt for '%s': %v\n", word, err)
		}

		// Only update UI if this word is still the current word.
		a.mu.Lock()
		isCurrentWord := a.currentWord == word
		a.mu.Unlock()

		if isCurrentWord && a.imagePromptEntry != nil {
			fyne.Do(func() {
				a.imagePromptEntry.SetText(prompt)
			})
		}
	}
}

// saveAudioAttribution saves attribution info for generated audio
func (a *Application) saveAudioAttribution(word, audioFile, voice string, speed float64) error {
	attribution := audio.BuildOpenAIAttribution(audio.AttributionParams{
		Word:        word,
		Model:       a.audioConfig.OpenAIModel,
		Voice:       voice,
		Speed:       speed,
		Instruction: a.audioConfig.OpenAIInstruction,
		GeneratedAt: time.Now(),
	})

	// Save to file
	attrPath := audio.AttributionPath(audioFile)
	if err := os.WriteFile(attrPath, []byte(attribution), 0644); err != nil {
		return fmt.Errorf("failed to write audio attribution file: %w", err)
	}

	return nil
}
