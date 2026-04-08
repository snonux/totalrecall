package gui

import (
	"strings"

	"codeberg.org/snonux/totalrecall/internal/audio"
)

// VoiceSelector picks voice and speed for a generation run from an
// AudioConfigResolver. For Gemini with a pinned voice the configured voice is
// used; otherwise a random voice is selected from the available list.
type VoiceSelector struct {
	resolver *AudioConfigResolver
}

// NewVoiceSelector constructs a VoiceSelector backed by the given resolver.
func NewVoiceSelector(resolver *AudioConfigResolver) *VoiceSelector {
	return &VoiceSelector{resolver: resolver}
}

// VoiceAndSpeed selects the voice and speed for a generation run.
func (v *VoiceSelector) VoiceAndSpeed() (string, float64) {
	switch v.resolver.ProviderName() {
	case "gemini":
		if v.resolver.audioConfig != nil {
			if voice := strings.TrimSpace(v.resolver.audioConfig.GeminiVoice); voice != "" {
				return voice, v.GeminiSpeed()
			}
		}
		return randomVoice(v.resolver.Voices()), v.GeminiSpeed()
	default:
		return randomVoice(v.resolver.Voices()), randomOpenAISpeed()
	}
}

// GeminiSpeed returns the configured Gemini TTS speed or the default.
func (v *VoiceSelector) GeminiSpeed() float64 {
	if v.resolver.audioConfig != nil && v.resolver.audioConfig.GeminiSpeed > 0 {
		return v.resolver.audioConfig.GeminiSpeed
	}
	return audio.DefaultProviderConfig().GeminiSpeed
}

// GeminiVoicePinned reports whether a specific Gemini voice is locked in
// config, meaning fallback voice selection should be skipped.
func (v *VoiceSelector) GeminiVoicePinned() bool {
	return v.resolver.audioConfig != nil && strings.TrimSpace(v.resolver.audioConfig.GeminiVoice) != ""
}
