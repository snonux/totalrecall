package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"codeberg.org/snonux/totalrecall/internal"
	"codeberg.org/snonux/totalrecall/internal/audio"
)

// CreateRootCommand creates and configures the root cobra command
func CreateRootCommand(flags *Flags) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "totalrecall [word]",
		Short: "Bulgarian Anki Flashcard Generator",
		Long: `totalrecall generates Anki flashcard materials from Bulgarian words.

It creates audio pronunciation files using Gemini TTS by default and downloads
representative images. Launching with no arguments opens the interactive GUI, which uses Nano Banana for images by default. Explicit CLI runs can use OpenAI or Nano Banana via --image-api, and audio can be switched between Gemini and OpenAI with --audio-provider.

Gemini audio model and voice flags are available for Gemini TTS generation.

Examples:
  totalrecall                     # Launch interactive GUI (default)
  totalrecall ябълка              # Generate materials for "apple" via CLI
  totalrecall --batch words.txt   # Process multiple words from file
  totalrecall --archive           # Archive existing cards directory

Batch file formats:
  ябълка                          # Bulgarian word (will be translated to English)
  ябълка = apple                  # English→Bulgarian card (single equals)
  ябълка == определение           # Bulgarian→Bulgarian card (double equals)
  = apple                         # English only (will be translated to Bulgarian)`,
		Args:    cobra.MaximumNArgs(1),
		Version: internal.Version,
	}

	// Set up flags
	setupFlags(rootCmd, flags)

	return rootCmd
}

func setupFlags(cmd *cobra.Command, flags *Flags) {
	// Set default output directory to match GUI mode
	home, _ := os.UserHomeDir()
	defaultOutputDir := filepath.Join(home, ".local", "state", "totalrecall", "cards")

	// Global flags
	// AGENT: The default config file location should be ~/.config/totalrecall/config.yaml
	cmd.PersistentFlags().StringVar(&flags.CfgFile, "config", "", "config file (default is $HOME/.totalrecall.yaml)")

	// Local flags
	cmd.Flags().StringVarP(&flags.OutputDir, "output", "o", defaultOutputDir, "Output directory")
	cmd.Flags().StringVarP(&flags.AudioFormat, "format", "f", flags.AudioFormat, "Audio format (wav or mp3; Gemini TTS always writes wav)")
	cmd.Flags().StringVar(&flags.ImageAPI, "image-api", flags.ImageAPI, "Image source for explicit CLI runs (OpenAI or Nano Banana; config file image.provider also applies when unset)")
	cmd.Flags().StringVar(&flags.BatchFile, "batch", "", "Process words from file (one per line)")
	cmd.Flags().BoolVar(&flags.SkipAudio, "skip-audio", false, "Skip audio generation")
	cmd.Flags().BoolVar(&flags.SkipImages, "skip-images", false, "Skip image download")
	cmd.Flags().BoolVar(&flags.GenerateAnki, "anki", false, "Generate Anki import file (APKG format by default, use --anki-csv for legacy CSV)")
	cmd.Flags().BoolVar(&flags.AnkiCSV, "anki-csv", false, "Generate legacy CSV format instead of APKG when using --anki")
	cmd.Flags().StringVar(&flags.DeckName, "deck-name", flags.DeckName, "Deck name for APKG export")
	cmd.Flags().BoolVar(&flags.ListModels, "list-models", false, "List available OpenAI and Gemini models for the configured API keys")
	cmd.Flags().BoolVar(&flags.AllVoices, "all-voices", false, "Generate audio in all available voices (creates multiple files)")
	cmd.Flags().BoolVar(&flags.NoAutoPlay, "no-auto-play", false, "Disable automatic audio playback in GUI mode (auto-play is enabled by default)")
	cmd.Flags().BoolVar(&flags.Archive, "archive", false, "Archive existing cards directory with timestamp")

	// OpenAI flags
	cmd.Flags().StringVar(&flags.OpenAIModel, "openai-model", flags.OpenAIModel, "OpenAI TTS model: tts-1, tts-1-hd, gpt-4o-mini-tts")
	cmd.Flags().StringVar(&flags.OpenAIVoice, "openai-voice", "", openAIVoiceUsage())
	cmd.Flags().Float64Var(&flags.OpenAISpeed, "openai-speed", flags.OpenAISpeed, "OpenAI speech speed (0.25 to 4.0, may be ignored by gpt-4o-mini-tts)")
	cmd.Flags().StringVar(&flags.OpenAIInstruction, "openai-instruction", "", "Voice instructions for gpt-4o-mini-tts model (e.g., 'speak slowly with a Bulgarian accent')")

	// Gemini audio flags
	cmd.Flags().StringVar(&flags.AudioProvider, "audio-provider", flags.AudioProvider, "Audio provider (gemini or openai; config file audio.provider also applies)")
	cmd.Flags().StringVar(&flags.GeminiTTSModel, "gemini-tts-model", flags.GeminiTTSModel, "Gemini TTS model (config file audio.gemini_tts_model also applies)")
	cmd.Flags().StringVar(&flags.GeminiVoice, "gemini-voice", flags.GeminiVoice, geminiVoiceUsage())

	// OpenAI Image Generation flags
	cmd.Flags().StringVar(&flags.OpenAIImageModel, "openai-image-model", flags.OpenAIImageModel, "OpenAI image model: dall-e-2 or dall-e-3")
	cmd.Flags().StringVar(&flags.OpenAIImageSize, "openai-image-size", flags.OpenAIImageSize, "Image size: 256x256, 512x512, 1024x1024 (dall-e-3: also 1024x1792, 1792x1024)")
	cmd.Flags().StringVar(&flags.OpenAIImageQuality, "openai-image-quality", flags.OpenAIImageQuality, "Image quality: standard or hd (dall-e-3 only)")
	cmd.Flags().StringVar(&flags.OpenAIImageStyle, "openai-image-style", flags.OpenAIImageStyle, "Image style: natural or vivid (dall-e-3 only)")

	// Nano Banana Image Generation flags
	cmd.Flags().StringVar(&flags.NanoBananaModel, "nanobanana-model", flags.NanoBananaModel, "Nano Banana image model used when Nano Banana image generation is selected")
	cmd.Flags().StringVar(&flags.NanoBananaTextModel, "nanobanana-text-model", flags.NanoBananaTextModel, "Nano Banana text model used when Nano Banana image generation is selected")

	// Bind flags to viper
	if err := bindFlagsToViper(cmd); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to bind flags to config: %v\n", err)
	}
}

// MarkExplicitFlagValues records which CLI flags were explicitly set by the user.
func MarkExplicitFlagValues(cmd *cobra.Command, flags *Flags) {
	flags.ImageAPISpecified = cmd.Flags().Changed("image-api")
	flags.NanoBananaModelSpecified = cmd.Flags().Changed("nanobanana-model")
	flags.NanoBananaTextModelSpecified = cmd.Flags().Changed("nanobanana-text-model")
}

func bindFlagsToViper(cmd *cobra.Command) error {
	bindings := map[string]string{
		"audio.format":                "format",
		"audio.provider":              "audio-provider",
		"audio.openai_model":          "openai-model",
		"audio.openai_voice":          "openai-voice",
		"audio.openai_speed":          "openai-speed",
		"audio.openai_instruction":    "openai-instruction",
		"audio.gemini_tts_model":      "gemini-tts-model",
		"audio.gemini_voice":          "gemini-voice",
		"output.directory":            "output",
		"image.provider":              "image-api",
		"image.openai_model":          "openai-image-model",
		"image.openai_size":           "openai-image-size",
		"image.openai_quality":        "openai-image-quality",
		"image.openai_style":          "openai-image-style",
		"image.nanobanana_model":      "nanobanana-model",
		"image.nanobanana_text_model": "nanobanana-text-model",
	}

	var errs []error
	for key, flagName := range bindings {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			errs = append(errs, fmt.Errorf("flag %q not found for key %q", flagName, key))
			continue
		}
		if err := viper.BindPFlag(key, flag); err != nil {
			errs = append(errs, fmt.Errorf("bind %q to %q: %w", key, flagName, err))
		}
	}

	return errors.Join(errs...)
}

// InitConfig initializes viper configuration
func InitConfig(cfgFile string) {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
			return
		}

		// Search config in home directory with name ".totalrecall" (without extension)
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".totalrecall")
	}

	// Environment variables
	viper.SetEnvPrefix("TOTALRECALL")
	viper.AutomaticEnv()

	// Read config file
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

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

func openAIVoiceUsage() string {
	return "OpenAI voice: " + strings.Join(audio.OpenAIVoices, ", ") + " (default: random)"
}

func geminiVoiceUsage() string {
	return "Gemini voice: " + strings.Join(audio.GeminiVoices, ", ") + " (default: model default)"
}
