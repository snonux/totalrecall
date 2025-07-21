package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"codeberg.org/snonux/totalrecall/internal"
)

// CreateRootCommand creates and configures the root cobra command
func CreateRootCommand(flags *Flags) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "totalrecall [word]",
		Short: "Bulgarian Anki Flashcard Generator",
		Long: `totalrecall generates Anki flashcard materials from Bulgarian words.

It creates audio pronunciation files using OpenAI TTS and downloads
representative images from web search APIs.

Examples:
  totalrecall                     # Launch interactive GUI (default)
  totalrecall ябълка              # Generate materials for "apple" via CLI
  totalrecall --batch words.txt   # Process multiple words from file`,
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
	cmd.PersistentFlags().StringVar(&flags.CfgFile, "config", "", "config file (default is $HOME/.totalrecall.yaml)")

	// Local flags
	cmd.Flags().StringVarP(&flags.OutputDir, "output", "o", defaultOutputDir, "Output directory")
	cmd.Flags().StringVarP(&flags.AudioFormat, "format", "f", flags.AudioFormat, "Audio format (wav or mp3)")
	cmd.Flags().StringVar(&flags.ImageAPI, "image-api", flags.ImageAPI, "Image source (only openai supported)")
	cmd.Flags().StringVar(&flags.BatchFile, "batch", "", "Process words from file (one per line)")
	cmd.Flags().BoolVar(&flags.SkipAudio, "skip-audio", false, "Skip audio generation")
	cmd.Flags().BoolVar(&flags.SkipImages, "skip-images", false, "Skip image download")
	cmd.Flags().BoolVar(&flags.GenerateAnki, "anki", false, "Generate Anki import file (APKG format by default, use --anki-csv for legacy CSV)")
	cmd.Flags().BoolVar(&flags.AnkiCSV, "anki-csv", false, "Generate legacy CSV format instead of APKG when using --anki")
	cmd.Flags().StringVar(&flags.DeckName, "deck-name", flags.DeckName, "Deck name for APKG export")
	cmd.Flags().BoolVar(&flags.ListModels, "list-models", false, "List available OpenAI models for the current API key")
	cmd.Flags().BoolVar(&flags.AllVoices, "all-voices", false, "Generate audio in all available voices (creates multiple files)")
	cmd.Flags().BoolVar(&flags.NoAutoPlay, "no-auto-play", false, "Disable automatic audio playback in GUI mode (auto-play is enabled by default)")

	// OpenAI flags
	cmd.Flags().StringVar(&flags.OpenAIModel, "openai-model", flags.OpenAIModel, "OpenAI TTS model: tts-1, tts-1-hd, gpt-4o-mini-tts")
	cmd.Flags().StringVar(&flags.OpenAIVoice, "openai-voice", "", "OpenAI voice: alloy, ash, ballad, coral, echo, fable, onyx, nova, sage, shimmer, verse (default: random)")
	cmd.Flags().Float64Var(&flags.OpenAISpeed, "openai-speed", flags.OpenAISpeed, "OpenAI speech speed (0.25 to 4.0, may be ignored by gpt-4o-mini-tts)")
	cmd.Flags().StringVar(&flags.OpenAIInstruction, "openai-instruction", "", "Voice instructions for gpt-4o-mini-tts model (e.g., 'speak slowly with a Bulgarian accent')")

	// OpenAI Image Generation flags
	cmd.Flags().StringVar(&flags.OpenAIImageModel, "openai-image-model", flags.OpenAIImageModel, "OpenAI image model: dall-e-2 or dall-e-3")
	cmd.Flags().StringVar(&flags.OpenAIImageSize, "openai-image-size", flags.OpenAIImageSize, "Image size: 256x256, 512x512, 1024x1024 (dall-e-3: also 1024x1792, 1792x1024)")
	cmd.Flags().StringVar(&flags.OpenAIImageQuality, "openai-image-quality", flags.OpenAIImageQuality, "Image quality: standard or hd (dall-e-3 only)")
	cmd.Flags().StringVar(&flags.OpenAIImageStyle, "openai-image-style", flags.OpenAIImageStyle, "Image style: natural or vivid (dall-e-3 only)")

	// Bind flags to viper
	bindFlagsToViper(cmd)
}

func bindFlagsToViper(cmd *cobra.Command) {
	viper.BindPFlag("audio.provider", cmd.Flags().Lookup("audio-provider"))
	viper.BindPFlag("audio.voice", cmd.Flags().Lookup("voice"))
	viper.BindPFlag("audio.format", cmd.Flags().Lookup("format"))
	viper.BindPFlag("audio.pitch", cmd.Flags().Lookup("pitch"))
	viper.BindPFlag("audio.amplitude", cmd.Flags().Lookup("amplitude"))
	viper.BindPFlag("audio.word_gap", cmd.Flags().Lookup("word-gap"))
	viper.BindPFlag("audio.openai_model", cmd.Flags().Lookup("openai-model"))
	viper.BindPFlag("audio.openai_voice", cmd.Flags().Lookup("openai-voice"))
	viper.BindPFlag("audio.openai_speed", cmd.Flags().Lookup("openai-speed"))
	viper.BindPFlag("audio.openai_instruction", cmd.Flags().Lookup("openai-instruction"))
	viper.BindPFlag("output.directory", cmd.Flags().Lookup("output"))
	viper.BindPFlag("image.provider", cmd.Flags().Lookup("image-api"))
	// Bind OpenAI image flags
	viper.BindPFlag("image.openai_model", cmd.Flags().Lookup("openai-image-model"))
	viper.BindPFlag("image.openai_size", cmd.Flags().Lookup("openai-image-size"))
	viper.BindPFlag("image.openai_quality", cmd.Flags().Lookup("openai-image-quality"))
	viper.BindPFlag("image.openai_style", cmd.Flags().Lookup("openai-image-style"))
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
