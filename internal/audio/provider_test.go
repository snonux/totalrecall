package audio

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// mockProvider implements Provider interface for testing
type mockProvider struct {
	name          string
	generateErr   error
	availableErr  error
	generateCalls int
}

func (m *mockProvider) GenerateAudio(ctx context.Context, text string, outputFile string) error {
	m.generateCalls++
	return m.generateErr
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) IsAvailable() error {
	return m.availableErr
}

func TestDefaultProviderConfig(t *testing.T) {
	config := DefaultProviderConfig()

	if config.Provider != "gemini" {
		t.Errorf("Expected provider 'gemini', got '%s'", config.Provider)
	}

	if config.OutputFormat != "wav" {
		t.Errorf("Expected output format 'wav', got '%s'", config.OutputFormat)
	}

	if config.OpenAIModel != "gpt-4o-mini-tts" {
		t.Errorf("Expected OpenAI model 'gpt-4o-mini-tts', got '%s'", config.OpenAIModel)
	}

	if config.OpenAIVoice != "alloy" {
		t.Errorf("Expected OpenAI voice 'alloy', got '%s'", config.OpenAIVoice)
	}

	if config.OpenAISpeed != 1.0 {
		t.Errorf("Expected OpenAI speed 1.0, got %f", config.OpenAISpeed)
	}

	if config.GeminiTTSModel != "gemini-2.5-flash-preview-tts" {
		t.Errorf("Expected Gemini TTS model 'gemini-2.5-flash-preview-tts', got '%s'", config.GeminiTTSModel)
	}

	if config.GeminiSpeed != 1.0 {
		t.Errorf("Expected Gemini speed 1.0, got %f", config.GeminiSpeed)
	}
}

func TestDefaultProviderConfigIsGeminiCompatible(t *testing.T) {
	config := DefaultProviderConfig()

	if config.Provider != "gemini" {
		t.Fatalf("DefaultProviderConfig() Provider = %q, want %q", config.Provider, "gemini")
	}

	if config.GeminiTTSModel != defaultGeminiTTSModel {
		t.Fatalf("DefaultProviderConfig() GeminiTTSModel = %q, want %q", config.GeminiTTSModel, defaultGeminiTTSModel)
	}

	outputFile := filepath.Join(t.TempDir(), "audio."+config.OutputFormat)
	if filepath.Ext(outputFile) != ".wav" {
		t.Fatalf("DefaultProviderConfig() output file %q is incompatible with Gemini TTS", outputFile)
	}

	if !strings.HasSuffix(config.GeminiTTSModel, "-tts") {
		t.Fatalf("DefaultProviderConfig() GeminiTTSModel = %q, want a TTS model variant", config.GeminiTTSModel)
	}

	if err := writeGeminiAudioFile(outputFile, []byte{0x11, 0x22}, "audio/pcm"); err != nil {
		t.Fatalf("writeGeminiAudioFile() with default Gemini output failed: %v", err)
	}
}

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		wantErr      bool
		errMsg       string
		wantProvider string
	}{
		{
			name:    "nil config uses defaults",
			config:  nil,
			wantErr: true,
			errMsg:  "google API key is required",
		},
		{
			name: "openai provider without key",
			config: &Config{
				Provider: "openai",
			},
			wantErr: true,
			errMsg:  "OpenAI API key is required",
		},
		{
			name: "unknown provider",
			config: &Config{
				Provider: "unknown",
			},
			wantErr: true,
			errMsg:  "unknown audio provider: unknown",
		},
		{
			name: "gemini provider with key",
			config: &Config{
				Provider:     "gemini",
				GoogleAPIKey: "test-google-key",
			},
			wantErr:      false,
			wantProvider: "gemini",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewProvider(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && err.Error() != tt.errMsg {
				t.Errorf("NewProvider() error = %v, want %v", err.Error(), tt.errMsg)
			}
			if !tt.wantErr && tt.wantProvider != "" {
				if provider == nil {
					t.Fatalf("NewProvider() returned nil provider")
				}
				if provider.Name() != tt.wantProvider {
					t.Fatalf("NewProvider() Name() = %v, want %v", provider.Name(), tt.wantProvider)
				}
				if tt.wantProvider == "gemini" {
					if _, ok := provider.(*GeminiProvider); !ok {
						t.Fatalf("NewProvider() returned %T, want *GeminiProvider", provider)
					}
				}
			}
		})
	}
}

func TestProviderWithFallback(t *testing.T) {
	primary := &mockProvider{name: "primary"}
	fallback := &mockProvider{name: "fallback"}

	provider := NewProviderWithFallback(primary, fallback)

	// Test successful primary
	ctx := context.Background()
	err := provider.GenerateAudio(ctx, "test", "output.mp3")
	if err != nil {
		t.Errorf("GenerateAudio() unexpected error: %v", err)
	}
	if primary.generateCalls != 1 {
		t.Errorf("Expected 1 primary call, got %d", primary.generateCalls)
	}
	if fallback.generateCalls != 0 {
		t.Errorf("Expected 0 fallback calls, got %d", fallback.generateCalls)
	}

	// Test primary failure, fallback success
	primary.generateErr = errors.New("primary failed")
	primary.generateCalls = 0

	err = provider.GenerateAudio(ctx, "test", "output.mp3")
	if err != nil {
		t.Errorf("GenerateAudio() unexpected error: %v", err)
	}
	if primary.generateCalls != 1 {
		t.Errorf("Expected 1 primary call, got %d", primary.generateCalls)
	}
	if fallback.generateCalls != 1 {
		t.Errorf("Expected 1 fallback call, got %d", fallback.generateCalls)
	}

	// Test both fail
	fallback.generateErr = errors.New("fallback failed")
	primary.generateCalls = 0
	fallback.generateCalls = 0

	err = provider.GenerateAudio(ctx, "test", "output.mp3")
	if err == nil {
		t.Error("GenerateAudio() expected error when both providers fail")
	}
}

func TestProviderWithFallbackName(t *testing.T) {
	primary := &mockProvider{name: "primary"}
	fallback := &mockProvider{name: "fallback"}

	provider := NewProviderWithFallback(primary, fallback)

	expected := "primary (fallback: fallback)"
	if provider.Name() != expected {
		t.Errorf("Name() = %v, want %v", provider.Name(), expected)
	}
}

func TestProviderWithFallbackIsAvailable(t *testing.T) {
	primary := &mockProvider{name: "primary"}
	fallback := &mockProvider{name: "fallback"}

	provider := NewProviderWithFallback(primary, fallback)

	// Both available
	err := provider.IsAvailable()
	if err != nil {
		t.Errorf("IsAvailable() unexpected error: %v", err)
	}

	// Primary unavailable, fallback available
	primary.availableErr = errors.New("primary unavailable")
	err = provider.IsAvailable()
	if err != nil {
		t.Errorf("IsAvailable() unexpected error when fallback available: %v", err)
	}

	// Primary available, fallback unavailable
	primary.availableErr = nil
	fallback.availableErr = errors.New("fallback unavailable")
	err = provider.IsAvailable()
	if err != nil {
		t.Errorf("IsAvailable() unexpected error when primary available: %v", err)
	}

	// Both unavailable
	primary.availableErr = errors.New("primary unavailable")
	err = provider.IsAvailable()
	if err == nil {
		t.Error("IsAvailable() expected error when both providers unavailable")
	}
}
