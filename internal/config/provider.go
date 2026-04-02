package config

import "strings"

const (
	// ProviderGemini is the canonical name for the Google Gemini provider.
	// It is the default across all subsystems (audio, translation, phonetic).
	ProviderGemini = "gemini"

	// ProviderOpenAI is the canonical name for the OpenAI provider.
	ProviderOpenAI = "openai"
)

// NormalizeProvider returns a canonical, lowercase provider name.
// An empty input resolves to ProviderGemini (the default). The function is
// shared by the audio, translation, and phonetic packages so the same
// normalization rule has a single authoritative home.
func NormalizeProvider(provider string) string {
	normalized := strings.ToLower(strings.TrimSpace(provider))
	if normalized == "" {
		return ProviderGemini
	}
	return normalized
}
