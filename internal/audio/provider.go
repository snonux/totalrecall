package audio

import (
	"context"
	"fmt"
)

// Provider defines the interface for text-to-speech providers
type Provider interface {
	// GenerateAudio generates audio from text and saves it to the specified file
	GenerateAudio(ctx context.Context, text string, outputFile string) error

	// Name returns the provider name
	Name() string

	// IsAvailable checks if the provider is properly configured and available
	IsAvailable() error
}

// OpenAIAudioConfig holds settings specific to the OpenAI TTS backend.
// Callers that only use Gemini never need to populate these fields.
type OpenAIAudioConfig struct {
	Key         string
	Model       string  // "tts-1", "tts-1-hd", or "gpt-4o-mini-tts"
	Voice       string  // One of OpenAIVoices.
	Speed       float64 // 0.25 to 4.0
	Instruction string  // Voice instructions for gpt-4o-mini-tts model
}

// GeminiAudioConfig holds settings specific to the Gemini TTS backend.
// Callers that only use OpenAI never need to populate these fields.
type GeminiAudioConfig struct {
	APIKey   string
	TTSModel string  // "gemini-2.5-flash-preview-tts"
	Voice    string  // One of GeminiVoices; empty lets the caller choose a random voice.
	Speed    float64 // Prompt hint for desired speech speed
}

// Config holds common configuration for audio providers. Provider-specific
// settings are grouped into OpenAI and Gemini sub-configs so callers and
// implementations only see the fields relevant to their backend.
type Config struct {
	Provider     string // Provider name: "openai" or "gemini"
	OutputDir    string // Directory for output files
	OutputFormat string // Output format: "mp3" or "wav"

	// OpenAI-specific settings — ignored when Provider == "gemini".
	OpenAIKey         string
	OpenAIModel       string  // "tts-1", "tts-1-hd", or "gpt-4o-mini-tts"
	OpenAIVoice       string  // One of OpenAIVoices.
	OpenAISpeed       float64 // 0.25 to 4.0
	OpenAIInstruction string  // Voice instructions for gpt-4o-mini-tts model

	// Gemini-specific settings — ignored when Provider == "openai".
	GoogleAPIKey   string
	GeminiTTSModel string  // "gemini-2.5-flash-preview-tts"
	GeminiVoice    string  // One of GeminiVoices; empty lets the caller choose a random voice.
	GeminiSpeed    float64 // Prompt hint for desired speech speed
}

// openAIAudioConfigFrom extracts the OpenAI-specific sub-config from the flat Config.
// A nil Config produces a zero-value OpenAIAudioConfig.
func openAIAudioConfigFrom(c *Config) OpenAIAudioConfig {
	if c == nil {
		return OpenAIAudioConfig{}
	}
	return OpenAIAudioConfig{
		Key:         c.OpenAIKey,
		Model:       c.OpenAIModel,
		Voice:       c.OpenAIVoice,
		Speed:       c.OpenAISpeed,
		Instruction: c.OpenAIInstruction,
	}
}

// geminiAudioConfigFrom extracts the Gemini-specific sub-config from the flat Config.
// A nil Config produces a zero-value GeminiAudioConfig.
func geminiAudioConfigFrom(c *Config) GeminiAudioConfig {
	if c == nil {
		return GeminiAudioConfig{}
	}
	return GeminiAudioConfig{
		APIKey:   c.GoogleAPIKey,
		TTSModel: c.GeminiTTSModel,
		Voice:    c.GeminiVoice,
		Speed:    c.GeminiSpeed,
	}
}

// DefaultConfig returns default configuration
func DefaultProviderConfig() *Config {
	return &Config{
		Provider:     "gemini",
		OutputDir:    "./",
		OutputFormat: "mp3",
		OpenAIModel:  "gpt-4o-mini-tts", // New model with voice instructions support
		OpenAIVoice:  "alloy",
		OpenAISpeed:  1.0,
		// OpenAISpeed:       0.98, // Default speed for clarity
		OpenAIInstruction: "You are speaking Bulgarian language (български език). Pronounce the Bulgarian text with authentic Bulgarian phonetics, not Russian. Speak slowly and clearly for language learners.",
		GeminiTTSModel:    "gemini-2.5-flash-preview-tts",
		GeminiSpeed:       1.0,
	}
}

// NewProvider creates the appropriate audio provider based on configuration.
// It extracts provider-specific sub-configs so each implementation only
// receives the fields it needs (ISP).
func NewProvider(config *Config) (Provider, error) {
	if config == nil {
		config = DefaultProviderConfig()
	}

	switch config.Provider {
	case "openai":
		if config.OpenAIKey == "" {
			return nil, fmt.Errorf("OpenAI API key is required")
		}
		return NewOpenAIProvider(openAIAudioConfigFrom(config), config.OutputFormat)
	case "gemini":
		if config.GoogleAPIKey == "" {
			return nil, fmt.Errorf("google API key is required")
		}
		return NewGeminiProvider(geminiAudioConfigFrom(config), config.OutputFormat)
	default:
		return nil, fmt.Errorf("unknown audio provider: %s", config.Provider)
	}
}

