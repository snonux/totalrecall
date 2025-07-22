package audio

import (
	"context"
	"strings"
	"testing"
)

func TestNewOpenAIProvider(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "missing API key",
			config: &Config{
				OpenAIKey: "",
			},
			wantErr: true,
			errMsg:  "OpenAI API key is required",
		},
		{
			name: "valid config",
			config: &Config{
				OpenAIKey: "test-key",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewOpenAIProvider(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewOpenAIProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && err.Error() != tt.errMsg {
				t.Errorf("NewOpenAIProvider() error = %v, want %v", err.Error(), tt.errMsg)
			}

			// Check provider properties
			if !tt.wantErr && provider != nil {
				if provider.Name() != "openai" {
					t.Errorf("Name() = %v, want %v", provider.Name(), "openai")
				}
			}
		})
	}
}

func TestOpenAIProviderIsAvailable(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "with API key",
			config: &Config{
				OpenAIKey: "test-key",
			},
			wantErr: false,
		},
		{
			name: "without API key",
			config: &Config{
				OpenAIKey: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &OpenAIProvider{
				config: tt.config,
			}
			err := provider.IsAvailable()
			if (err != nil) != tt.wantErr {
				t.Errorf("IsAvailable() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPreprocessBulgarianText(t *testing.T) {
	provider := &OpenAIProvider{
		config: &Config{},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple word",
			input:    "ябълка",
			expected: "ябълка",
		},
		{
			name:     "word with punctuation",
			input:    "ябълка!",
			expected: "ябълка",
		},
		{
			name:     "word with multiple punctuation",
			input:    "\"ябълка?\"",
			expected: "ябълка",
		},
		{
			name:     "word with spaces",
			input:    "  ябълка  ",
			expected: "ябълка",
		},
		{
			name:     "word with dashes",
			input:    "ябълка-круша",
			expected: "ябълкакруша",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.preprocessBulgarianText(tt.input)
			if result != tt.expected {
				t.Errorf("preprocessBulgarianText(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateAudioValidation(t *testing.T) {
	provider := &OpenAIProvider{
		config: &Config{
			OpenAIKey: "test-key",
		},
	}

	ctx := context.Background()

	// Test with non-Bulgarian text
	err := provider.GenerateAudio(ctx, "hello", "output.mp3")
	if err == nil {
		t.Error("Expected error for non-Bulgarian text")
	}
	if !strings.Contains(err.Error(), "must contain Cyrillic characters") {
		t.Errorf("Expected Bulgarian validation error, got: %v", err)
	}

	// Test with empty text
	err = provider.GenerateAudio(ctx, "", "output.mp3")
	if err == nil {
		t.Error("Expected error for empty text")
	}
}
