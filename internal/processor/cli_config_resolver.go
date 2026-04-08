package processor

// CLIConfigResolver applies CLI-flag vs config-file precedence for run-mode
// settings used by the CLI, GUI wiring, and downstream helpers (audio format,
// provider names, Nano Banana models). It does not own API clients or I/O;
// those stay on Processor.

import (
	"strings"

	"codeberg.org/snonux/totalrecall/internal/audio"
	"codeberg.org/snonux/totalrecall/internal/cli"
	"codeberg.org/snonux/totalrecall/internal/gui"
	"codeberg.org/snonux/totalrecall/internal/image"
	"codeberg.org/snonux/totalrecall/internal/phonetic"
	"codeberg.org/snonux/totalrecall/internal/translation"
)

// CLIConfigResolver holds the resolved CLI flags and config snapshot from the
// composition root. Methods implement precedence rules between explicit flags
// and YAML config without importing Viper.
type CLIConfigResolver struct {
	Flags  *cli.Flags
	Config *Config
}

// AudioProviderName returns the configured audio provider name, preferring the
// config-file value over the CLI flag so config-file settings win.
func (r *CLIConfigResolver) AudioProviderName() string {
	if r.Config.AudioProvider != "" {
		return r.Config.AudioProvider
	}
	if r != nil && r.Flags != nil {
		return strings.ToLower(strings.TrimSpace(r.Flags.AudioProvider))
	}
	return ""
}

// EffectiveAudioFormat resolves the audio format from flags and config, with
// CLI flag taking precedence, then the config-file value, then provider-specific defaults.
func (r *CLIConfigResolver) EffectiveAudioFormat() string {
	if r != nil && r.Flags != nil && r.Flags.AudioFormatSpecified {
		if format := strings.ToLower(strings.TrimSpace(r.Flags.AudioFormat)); format != "" {
			return format
		}
	}

	if r.Config.AudioFormatSet && r.Config.AudioFormat != "" {
		return r.Config.AudioFormat
	}

	if r != nil && r.Flags != nil {
		if format := strings.ToLower(strings.TrimSpace(r.Flags.AudioFormat)); format != "" {
			return format
		}
	}

	if r.AudioProviderName() == "gemini" {
		return audio.DefaultProviderConfig().OutputFormat
	}

	return "mp3"
}

// GeminiTTSModel returns the Gemini TTS model, preferring the config-file value over the CLI flag.
func (r *CLIConfigResolver) GeminiTTSModel() string {
	if r.Config.GeminiTTSModel != "" {
		return r.Config.GeminiTTSModel
	}
	if r != nil && r.Flags != nil {
		return strings.TrimSpace(r.Flags.GeminiTTSModel)
	}
	return ""
}

// GeminiVoice returns the Gemini voice, preferring the config-file value over the CLI flag.
func (r *CLIConfigResolver) GeminiVoice() string {
	if r.Config.GeminiVoice != "" {
		return r.Config.GeminiVoice
	}
	if r != nil && r.Flags != nil {
		return strings.TrimSpace(r.Flags.GeminiVoice)
	}
	return ""
}

// OpenAIVoice returns the OpenAI voice, preferring the config-file value over the CLI flag.
func (r *CLIConfigResolver) OpenAIVoice() string {
	if r.Config.OpenAIVoice != "" {
		return r.Config.OpenAIVoice
	}
	if r != nil && r.Flags != nil {
		return strings.TrimSpace(r.Flags.OpenAIVoice)
	}
	return ""
}

// GUIConfig returns a gui.Config populated from flags and config.
// Callers (typically cmd/main.go) use this to construct the GUI application
// so that gui.New() lives outside the processor package and the processor→gui
// dependency is limited to the Config type only.
func (r *CLIConfigResolver) GUIConfig() *gui.Config {
	imageProvider := r.Flags.ImageAPI
	if !r.Flags.ImageAPISpecified {
		imageProvider = gui.DefaultConfig().ImageProvider
	}

	openAIKey := cli.GetOpenAIKey()
	googleAPIKey := cli.GetGoogleAPIKey()
	translationProvider := translation.Provider(r.Config.TranslationProvider)
	phoneticProvider := phonetic.Provider(r.Config.PhoneticProvider)

	phoneticFetcher := phonetic.NewFetcher(&phonetic.Config{
		Provider:     phoneticProvider,
		OpenAIKey:    openAIKey,
		GoogleAPIKey: googleAPIKey,
	})
	translator := translation.NewTranslator(&translation.Config{
		Provider:    translationProvider,
		OpenAIKey:   openAIKey,
		GeminiModel: r.Config.TranslationGeminiModel,
	})

	return &gui.Config{
		AudioFormat:         r.EffectiveAudioFormat(),
		AudioProvider:       r.AudioProviderName(),
		ImageProvider:       imageProvider,
		OpenAIKey:           openAIKey,
		GoogleAPIKey:        googleAPIKey,
		NanoBananaModel:     r.NanoBananaModelForRunMode(),
		NanoBananaTextModel: r.NanoBananaTextModelForRunMode(),
		GeminiTTSModel:      r.GeminiTTSModel(),
		GeminiVoice:         r.GeminiVoice(),
		TranslationProvider: translationProvider,
		PhoneticProvider:    phoneticProvider,
		AutoPlay:            !r.Flags.NoAutoPlay, // Invert the flag (--no-auto-play disables auto-play)
		PhoneticFetcher:     phoneticFetcher,
		Translator:          translator,
	}
}

// NanoBananaModelForRunMode resolves the NanoBanana image model, preferring
// the explicit CLI flag value when set, then the config-file value, then the
// package default.
func (r *CLIConfigResolver) NanoBananaModelForRunMode() string {
	if r != nil && r.Flags != nil && r.Flags.NanoBananaModelSpecified {
		if model := strings.TrimSpace(r.Flags.NanoBananaModel); model != "" {
			return model
		}
	}
	if r.Config.ImageNanoBananaModel != "" {
		return r.Config.ImageNanoBananaModel
	}
	if r != nil && r.Flags != nil {
		if model := strings.TrimSpace(r.Flags.NanoBananaModel); model != "" {
			return model
		}
	}
	return image.DefaultNanoBananaModel
}

// NanoBananaTextModelForRunMode resolves the NanoBanana text (prompt) model
// using the same CLI-flag-over-config precedence as NanoBananaModelForRunMode.
func (r *CLIConfigResolver) NanoBananaTextModelForRunMode() string {
	if r != nil && r.Flags != nil && r.Flags.NanoBananaTextModelSpecified {
		if model := strings.TrimSpace(r.Flags.NanoBananaTextModel); model != "" {
			return model
		}
	}
	if r.Config.ImageNanoBananaTextModel != "" {
		return r.Config.ImageNanoBananaTextModel
	}
	if r != nil && r.Flags != nil {
		if model := strings.TrimSpace(r.Flags.NanoBananaTextModel); model != "" {
			return model
		}
	}
	return image.DefaultNanoBananaTextModel
}
