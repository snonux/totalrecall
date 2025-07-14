package audio

import (
	"context"
	"path/filepath"
	"strings"
)

// ESpeakProvider implements Provider interface for espeak-ng
type ESpeakProvider struct {
	espeak *ESpeak
	format string
}

// NewESpeakProvider creates a new espeak-ng provider
func NewESpeakProvider(config *ESpeakConfig) (Provider, error) {
	espeak, err := New(config)
	if err != nil {
		return nil, err
	}
	
	return &ESpeakProvider{
		espeak: espeak,
		format: "mp3", // default format
	}, nil
}

// GenerateAudio generates audio using espeak-ng
func (p *ESpeakProvider) GenerateAudio(ctx context.Context, text string, outputFile string) error {
	// Validate Bulgarian text
	if err := ValidateBulgarianText(text); err != nil {
		return err
	}
	
	// Determine format from output file extension
	ext := strings.ToLower(filepath.Ext(outputFile))
	
	switch ext {
	case ".mp3":
		return p.espeak.GenerateMP3(text, outputFile)
	case ".wav":
		return p.espeak.GenerateAudio(text, outputFile)
	default:
		// Default to MP3 if extension is unclear
		if !strings.HasSuffix(outputFile, ".mp3") {
			outputFile += ".mp3"
		}
		return p.espeak.GenerateMP3(text, outputFile)
	}
}

// Name returns the provider name
func (p *ESpeakProvider) Name() string {
	return "espeak-ng"
}

// IsAvailable checks if espeak-ng is installed
func (p *ESpeakProvider) IsAvailable() error {
	return checkESpeakInstalled()
}

// SetFormat sets the output format preference
func (p *ESpeakProvider) SetFormat(format string) {
	p.format = format
}