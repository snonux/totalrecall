package gui

import (
	"testing"

	"codeberg.org/snonux/totalrecall/internal/audio"
	"codeberg.org/snonux/totalrecall/internal/image"
	"codeberg.org/snonux/totalrecall/internal/phonetic"
	"codeberg.org/snonux/totalrecall/internal/translation"
)

func TestDefaultConfigUsesGeminiLanguageProviders(t *testing.T) {
	config := DefaultConfig()
	audioDefaults := audio.DefaultProviderConfig()

	if config.TranslationProvider != translation.ProviderGemini {
		t.Fatalf("DefaultConfig() translation provider = %q, want %q", config.TranslationProvider, translation.ProviderGemini)
	}
	if config.PhoneticProvider != phonetic.ProviderGemini {
		t.Fatalf("DefaultConfig() phonetic provider = %q, want %q", config.PhoneticProvider, phonetic.ProviderGemini)
	}
	if config.ImageProvider != image.ImageProviderNanoBanana {
		t.Fatalf("DefaultConfig() image provider = %q, want %q", config.ImageProvider, image.ImageProviderNanoBanana)
	}
	if config.AudioProvider != audioDefaults.Provider {
		t.Fatalf("DefaultConfig() audio provider = %q, want %q", config.AudioProvider, audioDefaults.Provider)
	}
	if config.AudioFormat != audioDefaults.OutputFormat {
		t.Fatalf("DefaultConfig() audio format = %q, want %q", config.AudioFormat, audioDefaults.OutputFormat)
	}
	if config.NanoBananaModel != image.DefaultNanoBananaModel {
		t.Fatalf("DefaultConfig() NanoBananaModel = %q, want %q", config.NanoBananaModel, image.DefaultNanoBananaModel)
	}
	if config.NanoBananaTextModel != image.DefaultNanoBananaTextModel {
		t.Fatalf("DefaultConfig() NanoBananaTextModel = %q, want %q", config.NanoBananaTextModel, image.DefaultNanoBananaTextModel)
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
			name: "default to gemini when provider is unset and only openai key is available",
			config: &Config{
				OpenAIKey: "openai-key",
			},
			wantProv:   translation.ProviderGemini,
			wantOpen:   "openai-key",
			wantGoogle: "",
		},
		{
			name: "default to gemini when provider is unset and both keys are available",
			config: &Config{
				OpenAIKey:    "openai-key",
				GoogleAPIKey: "google-key",
			},
			wantProv:   translation.ProviderGemini,
			wantOpen:   "openai-key",
			wantGoogle: "google-key",
		},
		{
			name: "default to gemini when provider is unset and only google key is available",
			config: &Config{
				GoogleAPIKey: "google-key",
			},
			wantProv:   translation.ProviderGemini,
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
			name:       "nil config still uses gemini defaults",
			config:     nil,
			wantProv:   translation.ProviderGemini,
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
