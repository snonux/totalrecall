package gui

import (
	"testing"

	"codeberg.org/snonux/totalrecall/internal/translation"
)

func TestDefaultConfigPrefersGeminiTranslationProvider(t *testing.T) {
	config := DefaultConfig()

	if config.TranslationProvider != translation.ProviderGemini {
		t.Fatalf("DefaultConfig() translation provider = %q, want %q", config.TranslationProvider, translation.ProviderGemini)
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
			name: "default to gemini when google key is available",
			config: &Config{
				GoogleAPIKey: "google-key",
			},
			wantProv:   translation.ProviderGemini,
			wantOpen:   "",
			wantGoogle: "google-key",
		},
		{
			name: "fallback to openai when only openai key is available",
			config: &Config{
				OpenAIKey: "openai-key",
			},
			wantProv:   translation.ProviderOpenAI,
			wantOpen:   "openai-key",
			wantGoogle: "",
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
