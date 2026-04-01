package image

import "google.golang.org/genai"

const (
	// DefaultNanoBananaModel is the model name planned for Nano Banana image generation.
	DefaultNanoBananaModel = "gemini-3.1-flash-image-preview"

	// DefaultNanoBananaTextModel is the text model planned for prompt and translation work.
	DefaultNanoBananaTextModel = "gemini-2.5-flash"
)

// NanoBananaConfig holds the settings needed to build a Gemini-backed image generator.
type NanoBananaConfig struct {
	APIKey    string
	Model     string
	TextModel string
}

// NewNanoBananaConfig returns a normalized Nano Banana configuration.
func NewNanoBananaConfig(apiKey string) *NanoBananaConfig {
	return &NanoBananaConfig{
		APIKey:    apiKey,
		Model:     DefaultNanoBananaModel,
		TextModel: DefaultNanoBananaTextModel,
	}
}

// ClientConfig returns the Google GenAI client config for this provider.
func (c *NanoBananaConfig) ClientConfig() *genai.ClientConfig {
	return &genai.ClientConfig{APIKey: c.APIKey}
}
