package cli

import (
	"os"

	"github.com/spf13/viper"
)

// GetOpenAIKey retrieves the OpenAI API key from environment or config
func GetOpenAIKey() string {
	// First check environment variable
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return key
	}

	// Then check config file
	return viper.GetString("audio.openai_key")
}

// GetGoogleAPIKey retrieves the Google API key from GOOGLE_API_KEY or config.
// It prefers image.google_api_key and falls back to google.api_key for older configs.
func GetGoogleAPIKey() string {
	// First check environment variable
	if key := os.Getenv("GOOGLE_API_KEY"); key != "" {
		return key
	}

	// Then check config file
	if key := viper.GetString("image.google_api_key"); key != "" {
		return key
	}

	// Fall back to the legacy key for compatibility with older configs.
	return viper.GetString("google.api_key")
}
