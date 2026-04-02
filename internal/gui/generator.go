package gui

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
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

func randomVoice(voices []string) string {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return voices[rng.Intn(len(voices))]
}

func randomOpenAISpeed() float64 {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return 0.90 + rng.Float64()*0.10
}

func (a *Application) audioProviderName() string {
	if a != nil && a.audioConfig != nil {
		if provider := strings.ToLower(strings.TrimSpace(a.audioConfig.Provider)); provider != "" {
			return provider
		}
	}
	return audio.DefaultProviderConfig().Provider
}

func (a *Application) audioVoices() []string {
	switch a.audioProviderName() {
	case "gemini":
		return audio.GeminiVoices
	default:
		return audio.OpenAIVoices
	}
}

func (a *Application) audioVoiceAndSpeed() (string, float64) {
	switch a.audioProviderName() {
	case "gemini":
		if a.audioConfig != nil {
			if voice := strings.TrimSpace(a.audioConfig.GeminiVoice); voice != "" {
				return voice, a.geminiSpeed()
			}
		}
		return randomVoice(a.audioVoices()), a.geminiSpeed()
	default:
		return randomVoice(a.audioVoices()), randomOpenAISpeed()
	}
}

func (a *Application) geminiSpeed() float64 {
	if a != nil && a.audioConfig != nil && a.audioConfig.GeminiSpeed > 0 {
		return a.audioConfig.GeminiSpeed
	}
	return audio.DefaultProviderConfig().GeminiSpeed
}

func (a *Application) geminiVoicePinned() bool {
	return a != nil && a.audioConfig != nil && strings.TrimSpace(a.audioConfig.GeminiVoice) != ""
}

func (a *Application) audioOutputFormat() string {
	if a != nil && a.config != nil && strings.TrimSpace(a.config.AudioFormat) != "" {
		return a.config.AudioFormat
	}

	if a != nil && a.audioConfig != nil && strings.TrimSpace(a.audioConfig.OutputFormat) != "" {
		return a.audioConfig.OutputFormat
	}

	return audio.DefaultProviderConfig().OutputFormat
}

func (a *Application) audioConfigForGeneration(voice string, speed float64) audio.Config {
	audioConfig := audio.Config{}
	if a != nil && a.audioConfig != nil {
		audioConfig = *a.audioConfig
	}

	audioConfig.Provider = a.audioProviderName()
	if a != nil && a.config != nil {
		audioConfig.OutputDir = a.config.OutputDir
	}
	audioConfig.OutputFormat = a.audioOutputFormat()

	switch audioConfig.Provider {
	case "gemini":
		audioConfig.GeminiVoice = voice
		audioConfig.GeminiSpeed = speed
		if strings.TrimSpace(audioConfig.GeminiTTSModel) == "" {
			audioConfig.GeminiTTSModel = audio.DefaultProviderConfig().GeminiTTSModel
		}
	default:
		audioConfig.OpenAIVoice = voice
		audioConfig.OpenAISpeed = speed
	}

	return audioConfig
}

func (a *Application) generateAudioFile(ctx context.Context, text, outputFile, voice string, speed float64) error {
	audioConfig := a.audioConfigForGeneration(voice, speed)

	provider, err := newAudioProvider(&audioConfig)
	if err != nil {
		return err
	}

	return provider.GenerateAudio(ctx, text, outputFile)
}

func (a *Application) generateGeminiAudioWithFallbacks(initialVoice string, generate func(voice string) error) (string, error) {
	attempted := make([]string, 0, len(audio.GeminiVoices))
	var lastErr error

	for i, voice := range audio.GeminiVoiceFallbacks(initialVoice) {
		if i > 0 {
			fmt.Printf("Retrying Gemini audio with voice: %s\n", voice)
		}

		attempted = append(attempted, voice)
		err := generate(voice)
		if err == nil {
			return voice, nil
		}
		if !audio.IsGeminiNoAudioDataError(err) {
			return "", err
		}

		lastErr = err
		fmt.Printf("Warning: Gemini returned no audio for voice %s\n", voice)
	}

	return "", fmt.Errorf("Gemini returned no audio for voices %s: %w", strings.Join(attempted, ", "), lastErr)
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
		audioFile := filepath.Join(cardDir, fmt.Sprintf("audio.%s", a.audioOutputFormat()))
		if _, err := os.Stat(audioFile); err == nil {
			isRegeneration = true
		}
	}

	voice, speed := a.audioVoiceAndSpeed()

	// Log the audio generation details
	if isRegeneration {
		fmt.Printf("Regenerating audio for '%s' with voice: %s, speed: %.2f\n", word, voice, speed)
	} else {
		fmt.Printf("Generating audio for '%s' with voice: %s, speed: %.2f\n", word, voice, speed)
	}

	// Use the provided card directory
	if cardDir == "" {
		return "", fmt.Errorf("card directory not provided")
	}

	// Generate filename in subdirectory
	outputFile := filepath.Join(cardDir, fmt.Sprintf("audio.%s", a.audioOutputFormat()))

	finalVoice := voice
	var err error
	if a.audioProviderName() == "gemini" && !a.geminiVoicePinned() {
		finalVoice, err = a.generateGeminiAudioWithFallbacks(voice, func(candidate string) error {
			return a.generateAudioFile(ctx, word, outputFile, candidate, speed)
		})
	} else {
		err = a.generateAudioFile(ctx, word, outputFile, voice, speed)
	}
	if err != nil {
		return "", err
	}

	audioConfig := a.audioConfigForGeneration(finalVoice, speed)

	// Save audio attribution
	if err := a.saveAudioAttribution(word, outputFile, finalVoice, speed); err != nil {
		// Non-fatal error, just log it
		fmt.Printf("Warning: Failed to save audio attribution: %v\n", err)
	}

	// Save voice metadata for GUI display
	if err := a.saveAudioMetadata(cardDir, audioConfig, finalVoice, speed, "en-bg", outputFile, ""); err != nil {
		fmt.Printf("Warning: Failed to save audio metadata: %v\n", err)
	}

	return outputFile, nil
}

// generateAudioFront generates front audio for a bg-bg card
func (a *Application) generateAudioFront(ctx context.Context, word string, cardDir string) (string, error) {
	if cardDir == "" {
		return "", fmt.Errorf("card directory not provided")
	}

	voice, speed := a.audioVoiceAndSpeed()
	fmt.Printf("Generating front audio for '%s' with voice: %s, speed: %.2f\n", word, voice, speed)
	frontFile := filepath.Join(cardDir, fmt.Sprintf("audio_front.%s", a.audioOutputFormat()))

	finalVoice := voice
	var err error
	if a.audioProviderName() == "gemini" && !a.geminiVoicePinned() {
		finalVoice, err = a.generateGeminiAudioWithFallbacks(voice, func(candidate string) error {
			return a.generateAudioFile(ctx, word, frontFile, candidate, speed)
		})
	} else {
		err = a.generateAudioFile(ctx, word, frontFile, voice, speed)
	}
	if err != nil {
		return "", fmt.Errorf("failed to generate front audio: %w", err)
	}

	audioConfig := a.audioConfigForGeneration(finalVoice, speed)

	if err := a.saveAudioAttribution(word, frontFile, finalVoice, speed); err != nil {
		fmt.Printf("Warning: Failed to save audio attribution: %v\n", err)
	}

	// Update metadata
	if err := a.saveAudioMetadata(cardDir, audioConfig, finalVoice, speed, "bg-bg", frontFile, a.currentAudioFileBack); err != nil {
		fmt.Printf("Warning: Failed to save audio metadata: %v\n", err)
	}

	return frontFile, nil
}

// generateAudioBack generates back audio for a bg-bg card
func (a *Application) generateAudioBack(ctx context.Context, text string, cardDir string) (string, error) {
	if cardDir == "" {
		return "", fmt.Errorf("card directory not provided")
	}

	voice, speed := a.audioVoiceAndSpeed()
	fmt.Printf("Generating back audio for '%s' with voice: %s, speed: %.2f\n", text, voice, speed)
	backFile := filepath.Join(cardDir, fmt.Sprintf("audio_back.%s", a.audioOutputFormat()))

	finalVoice := voice
	var err error
	if a.audioProviderName() == "gemini" && !a.geminiVoicePinned() {
		finalVoice, err = a.generateGeminiAudioWithFallbacks(voice, func(candidate string) error {
			return a.generateAudioFile(ctx, text, backFile, candidate, speed)
		})
	} else {
		err = a.generateAudioFile(ctx, text, backFile, voice, speed)
	}
	if err != nil {
		return "", fmt.Errorf("failed to generate back audio: %w", err)
	}

	audioConfig := a.audioConfigForGeneration(finalVoice, speed)

	if err := a.saveAudioAttribution(text, backFile, finalVoice, speed); err != nil {
		fmt.Printf("Warning: Failed to save audio attribution: %v\n", err)
	}

	// Update metadata
	if err := a.saveAudioMetadata(cardDir, audioConfig, finalVoice, speed, "bg-bg", a.currentAudioFile, backFile); err != nil {
		fmt.Printf("Warning: Failed to save audio metadata: %v\n", err)
	}

	return backFile, nil
}

// generateAudioBgBg generates audio for both sides of a bg-bg card
func (a *Application) generateAudioBgBg(ctx context.Context, front, back, cardDir string) (string, string, error) {
	if cardDir == "" {
		return "", "", fmt.Errorf("card directory not provided")
	}

	voice, speed := a.audioVoiceAndSpeed()

	// Generate front audio
	fmt.Printf("Generating front audio for '%s' with voice: %s, speed: %.2f\n", front, voice, speed)
	frontFile := filepath.Join(cardDir, fmt.Sprintf("audio_front.%s", a.audioOutputFormat()))
	backFile := filepath.Join(cardDir, fmt.Sprintf("audio_back.%s", a.audioOutputFormat()))

	runPair := func(candidate string) error {
		if err := a.generateAudioFile(ctx, front, frontFile, candidate, speed); err != nil {
			return fmt.Errorf("failed to generate front audio: %w", err)
		}

		fmt.Printf("Generating back audio for '%s' with voice: %s, speed: %.2f\n", back, candidate, speed)
		if err := a.generateAudioFile(ctx, back, backFile, candidate, speed); err != nil {
			return fmt.Errorf("failed to generate back audio: %w", err)
		}

		return nil
	}

	finalVoice := voice
	var err error
	if a.audioProviderName() == "gemini" && !a.geminiVoicePinned() {
		finalVoice, err = a.generateGeminiAudioWithFallbacks(voice, runPair)
	} else {
		err = runPair(voice)
	}
	if err != nil {
		return "", "", err
	}

	audioConfig := a.audioConfigForGeneration(finalVoice, speed)

	// Save audio attribution
	if err := a.saveAudioAttribution(front, frontFile, finalVoice, speed); err != nil {
		fmt.Printf("Warning: Failed to save audio attribution: %v\n", err)
	}
	if err := a.saveAudioAttribution(back, backFile, finalVoice, speed); err != nil {
		fmt.Printf("Warning: Failed to save audio attribution: %v\n", err)
	}

	// Save metadata for both sides
	if err := a.saveAudioMetadata(cardDir, audioConfig, finalVoice, speed, "bg-bg", frontFile, backFile); err != nil {
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
		config := a.config
		if config == nil {
			config = DefaultConfig()
		}
		if config.GoogleAPIKey == "" {
			return nil, fmt.Errorf("google API key is required for image generation")
		}

		nanoBananaConfig := &image.NanoBananaConfig{
			APIKey:    config.GoogleAPIKey,
			Model:     config.NanoBananaModel,
			TextModel: config.NanoBananaTextModel,
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
	processedText := audio.ProcessedTextForWord(word)
	var attribution string
	switch a.audioProviderName() {
	case "gemini":
		model := audio.DefaultProviderConfig().GeminiTTSModel
		if a.audioConfig != nil && strings.TrimSpace(a.audioConfig.GeminiTTSModel) != "" {
			model = a.audioConfig.GeminiTTSModel
		}
		attribution = audio.BuildGeminiAttribution(audio.AttributionParams{
			Word:          word,
			Model:         model,
			Voice:         voice,
			Speed:         speed,
			ProcessedText: processedText,
			GeneratedAt:   time.Now(),
		})
	default:
		model := audio.DefaultProviderConfig().OpenAIModel
		instruction := audio.DefaultProviderConfig().OpenAIInstruction
		if a.audioConfig != nil {
			if strings.TrimSpace(a.audioConfig.OpenAIModel) != "" {
				model = a.audioConfig.OpenAIModel
			}
			if strings.TrimSpace(a.audioConfig.OpenAIInstruction) != "" {
				instruction = a.audioConfig.OpenAIInstruction
			}
		}
		attribution = audio.BuildOpenAIAttribution(audio.AttributionParams{
			Word:          word,
			Model:         model,
			Voice:         voice,
			Speed:         speed,
			Instruction:   instruction,
			ProcessedText: processedText,
			GeneratedAt:   time.Now(),
		})
	}

	// Save to file
	attrPath := audio.AttributionPath(audioFile)
	if err := os.WriteFile(attrPath, []byte(attribution), 0644); err != nil {
		return fmt.Errorf("failed to write audio attribution file: %w", err)
	}

	return nil
}

func (a *Application) saveAudioMetadata(cardDir string, audioConfig audio.Config, voice string, speed float64, cardType string, audioFile string, audioFileBack string) error {
	metadataFile := filepath.Join(cardDir, "audio_metadata.txt")
	if cardType == "bg-bg" {
		if audioFile == "" {
			audioFile, _ = a.resolveBgBgAudioFiles(cardDir)
		}
		if audioFileBack == "" {
			_, audioFileBack = a.resolveBgBgAudioFiles(cardDir)
		}
	}

	metadata := audio.BuildSidecarMetadata(audio.SidecarMetadataParams{
		Provider:          audioConfig.Provider,
		OutputFormat:      audioConfig.OutputFormat,
		CardType:          cardType,
		AudioFile:         audioFile,
		AudioFileBack:     audioFileBack,
		OpenAIModel:       audioConfig.OpenAIModel,
		OpenAIVoice:       voice,
		OpenAISpeed:       speed,
		OpenAIInstruction: audioConfig.OpenAIInstruction,
		GeminiTTSModel:    audioConfig.GeminiTTSModel,
		GeminiVoice:       voice,
		GeminiSpeed:       speed,
	})

	if err := os.WriteFile(metadataFile, []byte(metadata), 0644); err != nil {
		return fmt.Errorf("failed to write audio metadata file: %w", err)
	}

	return nil
}
