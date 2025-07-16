package audio

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements Provider interface for OpenAI TTS
type OpenAIProvider struct {
	client      *openai.Client
	config      *Config
	cacheDir    string
	enableCache bool
}

// NewOpenAIProvider creates a new OpenAI TTS provider
func NewOpenAIProvider(config *Config) (Provider, error) {
	if config.OpenAIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	client := openai.NewClient(config.OpenAIKey)

	provider := &OpenAIProvider{
		client:      client,
		config:      config,
		cacheDir:    config.CacheDir,
		enableCache: config.EnableCache,
	}

	// Create cache directory if caching is enabled
	if provider.enableCache && provider.cacheDir != "" {
		if err := os.MkdirAll(provider.cacheDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create cache directory: %w", err)
		}
	}

	return provider, nil
}

// GenerateAudio generates audio using OpenAI TTS
func (p *OpenAIProvider) GenerateAudio(ctx context.Context, text string, outputFile string) error {
	// Validate Bulgarian text
	if err := ValidateBulgarianText(text); err != nil {
		return err
	}

	// Check cache first
	if p.enableCache {
		cacheFile := p.getCacheFilePath(text)
		if _, err := os.Stat(cacheFile); err == nil {
			// Cache hit - copy cached file
			return p.copyFile(cacheFile, outputFile)
		}
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

	// Cache the result if caching is enabled
	if p.enableCache {
		cacheFile := p.getCacheFilePath(text)
		_ = p.copyFile(outputFile, cacheFile) // Ignore cache errors
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

// getCacheFilePath generates a cache file path for the given text
func (p *OpenAIProvider) getCacheFilePath(text string) string {
	// Create a hash of the text and settings
	h := md5.New()
	h.Write([]byte(text))
	h.Write([]byte(p.config.OpenAIModel))
	h.Write([]byte(p.config.OpenAIVoice))
	h.Write([]byte(fmt.Sprintf("%.2f", p.config.OpenAISpeed)))
	// Include instruction in cache key for gpt-4o-mini-tts
	if p.config.OpenAIModel == "gpt-4o-mini-tts" && p.config.OpenAIInstruction != "" {
		h.Write([]byte(p.config.OpenAIInstruction))
	}
	hash := hex.EncodeToString(h.Sum(nil))

	// Use first 2 chars as subdirectory for better file system performance
	subdir := hash[:2]
	filename := hash[2:] + ".mp3"

	return filepath.Join(p.cacheDir, subdir, filename)
}

// copyFile copies a file from src to dst
func (p *OpenAIProvider) copyFile(src, dst string) error {
	// Ensure destination directory exists
	dir := filepath.Dir(dst)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

// ClearCache removes all cached audio files
func (p *OpenAIProvider) ClearCache() error {
	if p.cacheDir == "" {
		return nil
	}
	return os.RemoveAll(p.cacheDir)
}

// GetCacheStats returns cache statistics
func (p *OpenAIProvider) GetCacheStats() (fileCount int, totalSize int64, err error) {
	if !p.enableCache || p.cacheDir == "" {
		return 0, 0, nil
	}

	err = filepath.Walk(p.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			fileCount++
			totalSize += info.Size()
		}
		return nil
	})

	return fileCount, totalSize, err
}
