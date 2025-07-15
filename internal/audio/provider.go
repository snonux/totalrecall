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

// Config holds common configuration for audio providers
type Config struct {
	Provider     string // Provider name: "espeak" or "openai"
	OutputDir    string // Directory for output files
	OutputFormat string // Output format: "mp3" or "wav"
	
	// ESpeak-specific settings
	ESpeakVoice     string
	ESpeakSpeed     int
	ESpeakPitch     int
	ESpeakAmplitude int
	ESpeakWordGap   int
	
	// OpenAI-specific settings
	OpenAIKey      string
	OpenAIModel    string // "tts-1" or "tts-1-hd"
	OpenAIVoice    string // "alloy", "echo", "fable", "onyx", "nova", "shimmer"
	OpenAISpeed    float64 // 0.25 to 4.0
	
	// Caching settings
	EnableCache bool
	CacheDir    string
}

// DefaultConfig returns default configuration
func DefaultProviderConfig() *Config {
	return &Config{
		Provider:        "openai",
		OutputDir:       "./",
		OutputFormat:    "mp3",
		ESpeakVoice:     "bg",
		ESpeakSpeed:     150,
		ESpeakPitch:     50,
		ESpeakAmplitude: 100,
		ESpeakWordGap:   0,
		OpenAIModel:     "tts-1",
		OpenAIVoice:     "nova",
		OpenAISpeed:     1.0,
		EnableCache:     true,
		CacheDir:        "./.audio_cache",
	}
}

// NewProvider creates the appropriate audio provider based on configuration
func NewProvider(config *Config) (Provider, error) {
	if config == nil {
		config = DefaultProviderConfig()
	}
	
	switch config.Provider {
	case "espeak", "espeak-ng":
		espeakConfig := &ESpeakConfig{
			Voice:     config.ESpeakVoice,
			Speed:     config.ESpeakSpeed,
			Pitch:     config.ESpeakPitch,
			Amplitude: config.ESpeakAmplitude,
			WordGap:   config.ESpeakWordGap,
			OutputDir: config.OutputDir,
		}
		return NewESpeakProvider(espeakConfig)
		
	case "openai":
		if config.OpenAIKey == "" {
			return nil, fmt.Errorf("OpenAI API key is required")
		}
		return NewOpenAIProvider(config)
		
	default:
		return nil, fmt.Errorf("unknown audio provider: %s", config.Provider)
	}
}

// ProviderWithFallback wraps a primary provider with a fallback option
type ProviderWithFallback struct {
	primary   Provider
	fallback  Provider
}

// NewProviderWithFallback creates a provider that falls back to secondary if primary fails
func NewProviderWithFallback(primary, fallback Provider) Provider {
	return &ProviderWithFallback{
		primary:  primary,
		fallback: fallback,
	}
}

// GenerateAudio tries primary provider first, falls back to secondary on error
func (p *ProviderWithFallback) GenerateAudio(ctx context.Context, text string, outputFile string) error {
	err := p.primary.GenerateAudio(ctx, text, outputFile)
	if err != nil {
		// Log the primary error
		fmt.Printf("Primary provider (%s) failed: %v. Falling back to %s\n", 
			p.primary.Name(), err, p.fallback.Name())
		
		// Try fallback
		return p.fallback.GenerateAudio(ctx, text, outputFile)
	}
	return nil
}

// Name returns the provider name
func (p *ProviderWithFallback) Name() string {
	return fmt.Sprintf("%s (fallback: %s)", p.primary.Name(), p.fallback.Name())
}

// IsAvailable checks if at least one provider is available
func (p *ProviderWithFallback) IsAvailable() error {
	primaryErr := p.primary.IsAvailable()
	if primaryErr == nil {
		return nil
	}
	
	fallbackErr := p.fallback.IsAvailable()
	if fallbackErr == nil {
		return nil
	}
	
	return fmt.Errorf("both providers unavailable: primary=%v, fallback=%v", 
		primaryErr, fallbackErr)
}