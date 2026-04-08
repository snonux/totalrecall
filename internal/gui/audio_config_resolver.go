package gui

import (
	"strings"

	"codeberg.org/snonux/totalrecall/internal/audio"
)

// AudioConfigResolver derives effective TTS provider settings from the GUI
// config and the shared audio.Config. It centralizes provider name, voice list,
// output format, and per-run audio.Config construction so orchestration code
// does not repeat string handling and defaults.
type AudioConfigResolver struct {
	guiConfig   *Config
	audioConfig *audio.Config
}

// NewAudioConfigResolver builds a resolver for the given GUI and audio
// settings. Either pointer may be nil; defaults match the previous
// GenerationOrchestrator behaviour.
func NewAudioConfigResolver(guiConfig *Config, audioConfig *audio.Config) *AudioConfigResolver {
	return &AudioConfigResolver{
		guiConfig:   guiConfig,
		audioConfig: audioConfig,
	}
}

// ProviderName returns the lowercase provider name from config, defaulting to
// the shared audio default when none is set.
func (r *AudioConfigResolver) ProviderName() string {
	if r.audioConfig != nil {
		if provider := strings.ToLower(strings.TrimSpace(r.audioConfig.Provider)); provider != "" {
			return provider
		}
	}
	return audio.DefaultProviderConfig().Provider
}

// Voices returns the configured provider's voice list.
func (r *AudioConfigResolver) Voices() []string {
	return audio.VoicesFor(r.ProviderName())
}

// OutputFormat resolves the effective output format (e.g. "mp3" or "wav").
func (r *AudioConfigResolver) OutputFormat() string {
	if r.guiConfig != nil && strings.TrimSpace(r.guiConfig.AudioFormat) != "" {
		return r.guiConfig.AudioFormat
	}

	if r.audioConfig != nil && strings.TrimSpace(r.audioConfig.OutputFormat) != "" {
		return r.audioConfig.OutputFormat
	}

	return audio.DefaultProviderConfig().OutputFormat
}

// ConfigForGeneration builds an audio.Config for a single generation call,
// overriding the voice and speed with the values selected for this run.
func (r *AudioConfigResolver) ConfigForGeneration(voice string, speed float64) audio.Config {
	audioConfig := audio.Config{}
	if r.audioConfig != nil {
		audioConfig = *r.audioConfig
	}

	audioConfig.Provider = r.ProviderName()
	if r.guiConfig != nil {
		audioConfig.OutputDir = r.guiConfig.OutputDir
	}
	audioConfig.OutputFormat = r.OutputFormat()

	switch audioConfig.Provider {
	case "gemini":
		audioConfig.GeminiVoice = voice
		audioConfig.GeminiSpeed = speed
		if strings.TrimSpace(audioConfig.GeminiTTSModel) == "" {
			audioConfig.GeminiTTSModel = audio.DefaultProviderConfig().GeminiTTSModel
		}
	default:
		audioConfig.OpenAIVoice = voice
		audioConfig.OpenAISpeed = speed
	}

	return audioConfig
}

// BaseConfigForAttribution returns the configured audio.Config pointer, or the
// package default when unset — matching how attribution sidecars resolve the
// base parameters before per-run voice/speed overrides.
func (r *AudioConfigResolver) BaseConfigForAttribution() *audio.Config {
	if r.audioConfig != nil {
		return r.audioConfig
	}
	return audio.DefaultProviderConfig()
}
