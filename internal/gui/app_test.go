package gui

import (
	"testing"

	"codeberg.org/snonux/totalrecall/internal/audio"
	"codeberg.org/snonux/totalrecall/internal/translation"
)

func TestDefaultConfigUsesOpenAITranslationProvider(t *testing.T) {
	config := DefaultConfig()
	audioDefaults := audio.DefaultProviderConfig()

	if config.TranslationProvider != translation.ProviderOpenAI {
		t.Fatalf("DefaultConfig() translation provider = %q, want %q", config.TranslationProvider, translation.ProviderOpenAI)
	}
	if config.ImageProvider != imageProviderNanoBanana {
		t.Fatalf("DefaultConfig() image provider = %q, want %q", config.ImageProvider, imageProviderNanoBanana)
	}
	if config.AudioProvider != audioDefaults.Provider {
		t.Fatalf("DefaultConfig() audio provider = %q, want %q", config.AudioProvider, audioDefaults.Provider)
	}
	if config.AudioFormat != audioDefaults.OutputFormat {
		t.Fatalf("DefaultConfig() audio format = %q, want %q", config.AudioFormat, audioDefaults.OutputFormat)
	}
	if config.GeminiTTSModel != audioDefaults.GeminiTTSModel {
		t.Fatalf("DefaultConfig() GeminiTTSModel = %q, want %q", config.GeminiTTSModel, audioDefaults.GeminiTTSModel)
	}
}

func TestTranslationConfigForApp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		config     *Config
		wantProv   translation.Provider
		wantOpen   string
		wantGoogle string
	}{
		{
			name: "default to openai when provider is unset and only openai key is available",
			config: &Config{
				OpenAIKey: "openai-key",
			},
			wantProv:   translation.ProviderOpenAI,
			wantOpen:   "openai-key",
			wantGoogle: "",
		},
		{
			name: "default to openai when provider is unset and both keys are available",
			config: &Config{
				OpenAIKey:    "openai-key",
				GoogleAPIKey: "google-key",
			},
			wantProv:   translation.ProviderOpenAI,
			wantOpen:   "openai-key",
			wantGoogle: "google-key",
		},
		{
			name: "default to openai when provider is unset and only google key is available",
			config: &Config{
				GoogleAPIKey: "google-key",
			},
			wantProv:   translation.ProviderOpenAI,
			wantOpen:   "",
			wantGoogle: "google-key",
		},
		{
			name: "honor explicit gemini provider",
			config: &Config{
				TranslationProvider: translation.ProviderGemini,
				GoogleAPIKey:        "google-key",
				OpenAIKey:           "openai-key",
			},
			wantProv:   translation.ProviderGemini,
			wantOpen:   "openai-key",
			wantGoogle: "google-key",
		},
		{
			name: "honor explicit openai provider",
			config: &Config{
				TranslationProvider: translation.ProviderOpenAI,
				OpenAIKey:           "openai-key",
				GoogleAPIKey:        "google-key",
			},
			wantProv:   translation.ProviderOpenAI,
			wantOpen:   "openai-key",
			wantGoogle: "google-key",
		},
		{
			name:       "nil config still uses openai defaults",
			config:     nil,
			wantProv:   translation.ProviderOpenAI,
			wantOpen:   "",
			wantGoogle: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := translationConfigForApp(tt.config)
			if got.Provider != tt.wantProv {
				t.Fatalf("Provider = %q, want %q", got.Provider, tt.wantProv)
			}
			if got.OpenAIKey != tt.wantOpen {
				t.Fatalf("OpenAIKey = %q, want %q", got.OpenAIKey, tt.wantOpen)
			}
			if got.GoogleAPIKey != tt.wantGoogle {
				t.Fatalf("GoogleAPIKey = %q, want %q", got.GoogleAPIKey, tt.wantGoogle)
			}
		})
	}
}
