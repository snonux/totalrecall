package audio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// Compile-time check that OpenAIProvider implements the Provider interface.
var _ Provider = (*OpenAIProvider)(nil)

// OpenAIProvider implements Provider interface for OpenAI TTS.
// It stores only the OpenAI-specific sub-config so it never sees Gemini fields.
type OpenAIProvider struct {
	client       *openai.Client
	config       OpenAIAudioConfig
	outputFormat string
}

// NewOpenAIProvider creates a new OpenAI TTS provider from the OpenAI-specific
// sub-config. Callers that have a flat Config should use NewProvider instead.
func NewOpenAIProvider(config OpenAIAudioConfig, outputFormat string) (Provider, error) {
	if config.Key == "" {
		return nil, errors.New("OpenAI API key is required")
	}

	return &OpenAIProvider{
		client:       openai.NewClient(config.Key),
		config:       config,
		outputFormat: outputFormat,
	}, nil
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
	fmt.Printf("OpenAI TTS: Using model '%s' with voice '%s' at speed %.2f\n", p.config.Model, p.config.Voice, p.config.Speed)
	if p.config.Instruction != "" && (p.config.Model == "gpt-4o-mini-tts" || p.config.Model == "gpt-4o-mini-audio-preview") {
		fmt.Printf("OpenAI TTS Instruction: '%s'\n", p.config.Instruction)
	}
	fmt.Printf("OpenAI TTS Input: '%s'\n", processedText)

	req := openai.CreateSpeechRequest{
		Model: openai.SpeechModel(p.config.Model),
		Input: processedText,
		Voice: openai.SpeechVoice(p.config.Voice),
		Speed: p.config.Speed,
	}

	// Add instructions for gpt-4o-mini-tts model
	if p.config.Instruction != "" && (p.config.Model == "gpt-4o-mini-tts" || p.config.Model == "gpt-4o-mini-audio-preview") {
		req.Instructions = p.config.Instruction
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
		if strings.Contains(errStr, "does not have access to model") && (p.config.Model == "gpt-4o-mini-tts" || p.config.Model == "gpt-4o-mini-audio-preview") {
			return fmt.Errorf("OpenAI TTS API error: %w\nNote: The %s model requires access. Try using --openai-model tts-1-hd instead", err, p.config.Model)
		}
		return err
	}
	defer func() {
		_ = response.Close()
	}()

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
	defer func() {
		_ = out.Close()
	}()

	// Copy the audio data
	written, err := io.Copy(out, response)
	if err != nil {
		return fmt.Errorf("failed to write audio file: %w", err)
	}

	if written == 0 {
		return errors.New("no audio data received from OpenAI")
	}

	return nil
}

// Name returns the provider name
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// IsAvailable checks if the OpenAI API is accessible
func (p *OpenAIProvider) IsAvailable() error {
	if p.config.Key == "" {
		return errors.New("OpenAI API key not configured")
	}

	// We could make a test API call here, but that would use credits
	// For now, just check that we have a key
	return nil
}

// preprocessBulgarianText prepares Bulgarian text for clearer TTS pronunciation
func (p *OpenAIProvider) preprocessBulgarianText(text string) string {
	return openAIProcessedText(text)
}
