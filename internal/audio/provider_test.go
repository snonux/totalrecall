package audio

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultProviderConfig(t *testing.T) {
	config := DefaultProviderConfig()

	if config.Provider != "gemini" {
		t.Errorf("Expected provider 'gemini', got '%s'", config.Provider)
	}

	if config.OutputFormat != "mp3" {
		t.Errorf("Expected output format 'mp3', got '%s'", config.OutputFormat)
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
	if filepath.Ext(outputFile) != ".mp3" {
		t.Fatalf("DefaultProviderConfig() output file %q does not use the default mp3 extension", outputFile)
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
