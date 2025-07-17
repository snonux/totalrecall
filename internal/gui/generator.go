package gui

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"github.com/sashabaranov/go-openai"
	
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
func (a *Application) generateAudio(word string) (string, error) {
	// Get available voices
	allVoices := []string{"alloy", "ash", "ballad", "coral", "echo", "fable", "onyx", "nova", "sage", "shimmer", "verse"}
	
	// Select a random voice
	rand.Seed(time.Now().UnixNano())
	voice := allVoices[rand.Intn(len(allVoices))]
	
	// Update audio config with random voice
	a.audioConfig.OpenAIVoice = voice
	
	// Create audio provider
	provider, err := audio.NewProvider(a.audioConfig)
	if err != nil {
		return "", err
	}
	
	// Generate filename
	filename := sanitizeFilename(word)
	outputFile := filepath.Join(a.config.OutputDir, fmt.Sprintf("%s.%s", filename, a.config.AudioFormat))
	
	// Generate audio
	err = provider.GenerateAudio(a.ctx, word, outputFile)
	if err != nil {
		return "", err
	}
	
	// Save audio attribution
	if err := a.saveAudioAttribution(word, outputFile, voice); err != nil {
		// Non-fatal error, just log it
		fmt.Printf("Warning: Failed to save audio attribution: %v\n", err)
	}
	
	return outputFile, nil
}

// generateImages downloads images for a word
func (a *Application) generateImages(word string) (string, error) {
	return a.generateImagesWithPrompt(word, "", "")
}

// generateImagesWithPrompt downloads a single image for a word with optional custom prompt and translation
func (a *Application) generateImagesWithPrompt(word string, customPrompt string, translation string) (string, error) {
	// Create image searcher based on provider
	var searcher image.ImageSearcher
	var err error
	
	switch a.config.ImageProvider {
	case "openai":
		openaiConfig := &image.OpenAIConfig{
			APIKey:      a.config.OpenAIKey,
			Model:       "dall-e-2",  // DALL-E 2 supports 512x512
			Size:        "512x512",   // Half of 1024x1024
			Quality:     "standard",
			Style:       "natural",
			CacheDir:    "./.image_cache",
			EnableCache: a.config.EnableCache,
		}
		
		searcher = image.NewOpenAIClient(openaiConfig)
		if openaiConfig.APIKey == "" {
			return "", fmt.Errorf("OpenAI API key is required for image generation")
		}
		
	default:
		return "", fmt.Errorf("unknown image provider: %s", a.config.ImageProvider)
	}
	
	// Create downloader
	downloadOpts := &image.DownloadOptions{
		OutputDir:         a.config.OutputDir,
		OverwriteExisting: true,
		CreateDir:         true,
		FileNamePattern:   "{word}",
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
	_, path, err := downloader.DownloadBestMatchWithOptions(a.ctx, searchOpts)
	if err != nil {
		return "", err
	}
	
	// If using OpenAI, get the last used prompt and update the UI
	if a.config.ImageProvider == "openai" {
		if openaiClient, ok := searcher.(*image.OpenAIClient); ok {
			usedPrompt := openaiClient.GetLastPrompt()
			if usedPrompt != "" {
				fyne.Do(func() {
					a.imagePromptEntry.SetText(usedPrompt)
				})
			}
		}
	}
	
	return path, nil
}

// saveAudioAttribution saves attribution info for generated audio
func (a *Application) saveAudioAttribution(word, audioFile, voice string) error {
	attribution := fmt.Sprintf("Audio generated by OpenAI TTS\n\n")
	attribution += fmt.Sprintf("Bulgarian word: %s\n", word)
	attribution += fmt.Sprintf("Model: %s\n", a.audioConfig.OpenAIModel)
	attribution += fmt.Sprintf("Voice: %s\n", voice)
	attribution += fmt.Sprintf("Speed: %.2f\n", a.audioConfig.OpenAISpeed)
	
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

// sanitizeFilename creates a safe filename from a string
func sanitizeFilename(s string) string {
	result := ""
	for _, r := range s {
		if isAlphaNumeric(r) || r == '-' || r == '_' {
			result += string(r)
		} else {
			result += "_"
		}
	}
	return result
}

// isAlphaNumeric checks if a rune is alphanumeric
func isAlphaNumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || 
	       (r >= '0' && r <= '9') || (r >= 'а' && r <= 'я') || 
	       (r >= 'А' && r <= 'Я')
}