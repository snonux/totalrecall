package audio

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements Provider interface for OpenAI TTS
type OpenAIProvider struct {
	client *openai.Client
	config *Config
}

// NewOpenAIProvider creates a new OpenAI TTS provider
func NewOpenAIProvider(config *Config) (Provider, error) {
	if config.OpenAIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	client := openai.NewClient(config.OpenAIKey)

	provider := &OpenAIProvider{
		client: client,
		config: config,
	}

	return provider, nil
}

// GenerateAudio generates audio using OpenAI TTS
func (p *OpenAIProvider) GenerateAudio(ctx context.Context, text string, outputFile string) error {
	// Validate Bulgarian text
	if err := ValidateBulgarianText(text); err != nil {
		return err
	}

	// Preprocess text for clearer Bulgarian pronunciation
	processedText := p.preprocessBulgarianText(text)

	// Prepare the TTS request
	// OpenAI TTS will automatically detect and pronounce Bulgarian text
	fmt.Printf("OpenAI TTS: Using model '%s' with voice '%s' at speed %.2f\n", p.config.OpenAIModel, p.config.OpenAIVoice, p.config.OpenAISpeed)
	if p.config.OpenAIInstruction != "" && (p.config.OpenAIModel == "gpt-4o-mini-tts" || p.config.OpenAIModel == "gpt-4o-mini-audio-preview") {
		fmt.Printf("OpenAI TTS Instruction: '%s'\n", p.config.OpenAIInstruction)
	}
	fmt.Printf("OpenAI TTS Input: '%s'\n", processedText)

	req := openai.CreateSpeechRequest{
		Model: openai.SpeechModel(p.config.OpenAIModel),
		Input: processedText,
		Voice: openai.SpeechVoice(p.config.OpenAIVoice),
		Speed: p.config.OpenAISpeed,
	}

	// Add instructions for gpt-4o-mini-tts model
	if p.config.OpenAIInstruction != "" && (p.config.OpenAIModel == "gpt-4o-mini-tts" || p.config.OpenAIModel == "gpt-4o-mini-audio-preview") {
		req.Instructions = p.config.OpenAIInstruction
	}

	// Determine response format based on output file extension
	ext := strings.ToLower(filepath.Ext(outputFile))
	switch ext {
	case ".mp3":
		req.ResponseFormat = openai.SpeechResponseFormatMp3
	case ".wav":
		req.ResponseFormat = openai.SpeechResponseFormatWav
	case ".opus":
		req.ResponseFormat = openai.SpeechResponseFormatOpus
	case ".aac":
		req.ResponseFormat = openai.SpeechResponseFormatAac
	case ".flac":
		req.ResponseFormat = openai.SpeechResponseFormatFlac
	default:
		req.ResponseFormat = openai.SpeechResponseFormatMp3
		if !strings.HasSuffix(outputFile, ".mp3") {
			outputFile += ".mp3"
		}
	}

	// Make the API call
	response, err := p.client.CreateSpeech(ctx, req)
	if err != nil {
		// Check if it's a model access error
		errStr := err.Error()
		if strings.Contains(errStr, "does not have access to model") && (p.config.OpenAIModel == "gpt-4o-mini-tts" || p.config.OpenAIModel == "gpt-4o-mini-audio-preview") {
			return fmt.Errorf("OpenAI TTS API error: %w\nNote: The %s model requires access. Try using --openai-model tts-1-hd instead", err, p.config.OpenAIModel)
		}
		return fmt.Errorf("OpenAI TTS API error: %w", err)
	}
	defer response.Close()

	// Ensure output directory exists
	dir := filepath.Dir(outputFile)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// Create output file
	out, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()

	// Copy the audio data
	written, err := io.Copy(out, response)
	if err != nil {
		return fmt.Errorf("failed to write audio file: %w", err)
	}

	if written == 0 {
		return fmt.Errorf("no audio data received from OpenAI")
	}

	return nil
}

// Name returns the provider name
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// IsAvailable checks if the OpenAI API is accessible
func (p *OpenAIProvider) IsAvailable() error {
	if p.config.OpenAIKey == "" {
		return fmt.Errorf("OpenAI API key not configured")
	}

	// We could make a test API call here, but that would use credits
	// For now, just check that we have a key
	return nil
}

// preprocessBulgarianText prepares Bulgarian text for clearer TTS pronunciation
func (p *OpenAIProvider) preprocessBulgarianText(text string) string {
	// First, clean the text and remove punctuation that shouldn't be spoken
	cleanedText := strings.TrimSpace(text)

	// Remove common punctuation marks that shouldn't be pronounced
	punctuationToRemove := []string{"!", "?", ".", ",", ";", ":", "\"", "'", "(", ")", "[", "]", "{", "}", "-", "—", "–"}
	for _, punct := range punctuationToRemove {
		cleanedText = strings.ReplaceAll(cleanedText, punct, "")
	}

	// Trim any remaining whitespace
	cleanedText = strings.TrimSpace(cleanedText)

	// For single words, we add subtle punctuation to create natural pauses
	// This helps the TTS engine pronounce it more carefully
	processedText := cleanedText // fmt.Sprintf("%s...", cleanedText)

	return processedText
}
