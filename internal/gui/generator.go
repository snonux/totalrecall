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
	"github.com/sashabaranov/go-openai"

	"codeberg.org/snonux/totalrecall/internal"
	"codeberg.org/snonux/totalrecall/internal/audio"
	"codeberg.org/snonux/totalrecall/internal/image"
)

// translateWord translates a Bulgarian word to English
func (a *Application) translateWord(word string) (string, error) {
	if a.config.OpenAIKey == "" {
		return "", fmt.Errorf("OpenAI API key not configured")
	}

	client := openai.NewClient(a.config.OpenAIKey)

	req := openai.ChatCompletionRequest{
		Model: openai.GPT4oMini,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: fmt.Sprintf("Translate the Bulgarian word '%s' to English. Respond with only the English translation, nothing else.", word),
			},
		},
		MaxTokens:   50,
		Temperature: 0.3,
	}

	resp, err := client.CreateChatCompletion(a.ctx, req)
	if err != nil {
		return "", fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no translation returned")
	}

	translation := strings.TrimSpace(resp.Choices[0].Message.Content)
	return translation, nil
}

// translateEnglishToBulgarian translates an English word to Bulgarian
func (a *Application) translateEnglishToBulgarian(word string) (string, error) {
	if a.config.OpenAIKey == "" {
		return "", fmt.Errorf("OpenAI API key not configured")
	}

	client := openai.NewClient(a.config.OpenAIKey)

	req := openai.ChatCompletionRequest{
		Model: openai.GPT4oMini,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: fmt.Sprintf("Translate the English word '%s' to Bulgarian. Respond with only the Bulgarian translation in Cyrillic script, nothing else.", word),
			},
		},
		MaxTokens:   50,
		Temperature: 0.3,
	}

	resp, err := client.CreateChatCompletion(a.ctx, req)
	if err != nil {
		return "", fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no translation returned")
	}

	translation := strings.TrimSpace(resp.Choices[0].Message.Content)
	return translation, nil
}

// generateAudio generates audio for a word
func (a *Application) generateAudio(ctx context.Context, word string) (string, error) {
	// Check if this is a regeneration by looking for existing audio file
	wordDir := a.findCardDirectory(word)
	isRegeneration := false
	if wordDir != "" {
		audioFile := filepath.Join(wordDir, fmt.Sprintf("audio.%s", a.config.AudioFormat))
		if _, err := os.Stat(audioFile); err == nil {
			isRegeneration = true
		}
	}

	// For regeneration, use random voice and speed; otherwise use defaults
	var voice string
	var speed float64

	if isRegeneration {
		// Get available voices
		allVoices := []string{"alloy", "ash", "ballad", "coral", "echo", "fable", "onyx", "nova", "sage", "shimmer", "verse"}

		// Select a random voice
		rand.Seed(time.Now().UnixNano())
		voice = allVoices[rand.Intn(len(allVoices))]

		// Generate random speed between 0.90 and 1.00
		speed = 0.90 + rand.Float64()*0.10
	} else {
		// Use defaults for first generation
		voice = "alloy"
		speed = 1.0
	}

	// Update audio config with selected voice and speed
	a.audioConfig.OpenAIVoice = voice
	a.audioConfig.OpenAISpeed = speed

	// Create audio provider
	provider, err := audio.NewProvider(a.audioConfig)
	if err != nil {
		return "", err
	}

	// Find existing card directory or create new one again after provider creation
	wordDir = a.findCardDirectory(word)
	if wordDir == "" {
		// No existing directory, create new one with card ID
		cardID := internal.GenerateCardID(word)
		wordDir = filepath.Join(a.config.OutputDir, cardID)
		if err := os.MkdirAll(wordDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create word directory: %w", err)
		}

		// Save the original Bulgarian word in a metadata file
		metadataFile := filepath.Join(wordDir, "word.txt")
		if err := os.WriteFile(metadataFile, []byte(word), 0644); err != nil {
			return "", fmt.Errorf("failed to save word metadata: %w", err)
		}
	}

	// Generate filename in subdirectory
	outputFile := filepath.Join(wordDir, fmt.Sprintf("audio.%s", a.config.AudioFormat))

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
	metadataFile := filepath.Join(wordDir, "audio_metadata.txt")
	metadata := fmt.Sprintf("voice=%s\nspeed=%.2f\n", voice, speed)
	if err := os.WriteFile(metadataFile, []byte(metadata), 0644); err != nil {
		fmt.Printf("Warning: Failed to save audio metadata: %v\n", err)
	}

	return outputFile, nil
}

// generateImages downloads images for a word
func (a *Application) generateImages(ctx context.Context, word string) (string, error) {
	return a.generateImagesWithPrompt(ctx, word, "", "")
}

// generateImagesWithPrompt downloads a single image for a word with optional custom prompt and translation
func (a *Application) generateImagesWithPrompt(ctx context.Context, word string, customPrompt string, translation string) (string, error) {
	// Create image searcher based on provider
	var searcher image.ImageSearcher
	var err error

	switch a.config.ImageProvider {
	case "openai":
		openaiConfig := &image.OpenAIConfig{
			APIKey:  a.config.OpenAIKey,
			Model:   "dall-e-2", // DALL-E 2 supports 512x512
			Size:    "512x512",  // Half of 1024x1024
			Quality: "standard",
			Style:   "natural",
		}

		searcher = image.NewOpenAIClient(openaiConfig)
		if openaiConfig.APIKey == "" {
			return "", fmt.Errorf("OpenAI API key is required for image generation")
		}

	default:
		return "", fmt.Errorf("unknown image provider: %s", a.config.ImageProvider)
	}

	// Find existing card directory or create new one
	wordDir := a.findCardDirectory(word)
	if wordDir == "" {
		// No existing directory, create new one with card ID
		cardID := internal.GenerateCardID(word)
		wordDir = filepath.Join(a.config.OutputDir, cardID)
		if err := os.MkdirAll(wordDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create word directory: %w", err)
		}

		// Save the original Bulgarian word in a metadata file
		metadataFile := filepath.Join(wordDir, "word.txt")
		if err := os.WriteFile(metadataFile, []byte(word), 0644); err != nil {
			return "", fmt.Errorf("failed to save word metadata: %w", err)
		}
	}

	// Create downloader
	downloadOpts := &image.DownloadOptions{
		OutputDir:         wordDir,
		OverwriteExisting: true,
		CreateDir:         true,
		FileNamePattern:   "image",
		MaxSizeBytes:      5 * 1024 * 1024, // 5MB
	}

	downloader := image.NewDownloader(searcher, downloadOpts)

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

	// If using OpenAI, get the last used prompt
	if a.config.ImageProvider == "openai" {
		if openaiClient, ok := searcher.(*image.OpenAIClient); ok {
			usedPrompt := openaiClient.GetLastPrompt()
			if usedPrompt != "" {
				// Save the prompt to disk immediately for this word
				promptFile := filepath.Join(wordDir, "image_prompt.txt")
				os.WriteFile(promptFile, []byte(usedPrompt), 0644)

				// Only update UI if this word is still the current word
				a.mu.Lock()
				isCurrentWord := a.currentWord == word
				a.mu.Unlock()

				if isCurrentWord {
					fyne.Do(func() {
						a.imagePromptEntry.SetText(usedPrompt)
					})
				}
			}
		}
	}

	return path, nil
}

// saveAudioAttribution saves attribution info for generated audio
func (a *Application) saveAudioAttribution(word, audioFile, voice string, speed float64) error {
	attribution := fmt.Sprintf("Audio generated by OpenAI TTS\n\n")
	attribution += fmt.Sprintf("Bulgarian word: %s\n", word)
	attribution += fmt.Sprintf("Model: %s\n", a.audioConfig.OpenAIModel)
	attribution += fmt.Sprintf("Voice: %s\n", voice)
	attribution += fmt.Sprintf("Speed: %.2f\n", speed)

	if a.audioConfig.OpenAIInstruction != "" {
		attribution += fmt.Sprintf("\nVoice instructions:\n%s\n", a.audioConfig.OpenAIInstruction)
	}

	attribution += fmt.Sprintf("\nGenerated at: %s\n", time.Now().Format("2006-01-02 15:04:05"))

	// Save to file
	attrPath := strings.TrimSuffix(audioFile, filepath.Ext(audioFile)) + "_attribution.txt"
	if err := os.WriteFile(attrPath, []byte(attribution), 0644); err != nil {
		return fmt.Errorf("failed to write audio attribution file: %w", err)
	}

	return nil
}
