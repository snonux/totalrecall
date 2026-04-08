package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"codeberg.org/snonux/totalrecall/internal/audio"
	"codeberg.org/snonux/totalrecall/internal/config"
)

// Flags holds all command-line flag values
type Flags struct {
	// General flags
	CfgFile     string
	OutputDir   string
	AudioFormat string
	// AudioFormatSpecified records whether the audio format was explicitly set on the CLI.
	AudioFormatSpecified bool
	// AudioProvider selects the text-to-speech backend ("gemini" or "openai").
	AudioProvider         string
	ImageAPI              string
	ImageAPISpecified     bool
	BatchFile             string
	StoryFile             string // --story <file>: generate vocabulary story + comic image
	StoryStyle            string // --story-style: override the random art style (empty = random)
	StoryTheme            string // --story-theme: override the random genre pick (empty = random)
	StoryNoUltraRealistic bool   // --no-ultra-realistic: disable photorealistic rendering requirement
	StoryUltraRealistic   bool   // --ultra-realistic: force photorealistic rendering (overrides random 50/50)
	StorySlug             string // --story-slug: force a specific output slug/directory (empty = auto from title)
	NarratorVoice         string // --narrator-voice: Gemini voice for cinematic narration (empty = random)
	NarrateEnabled        bool   // --narrate: generate cinematic MP3 narration after --story (default false)
	VideoEnabled          bool   // --video: whether to prompt for Veo video generation after --story completes
	SkipAudio             bool
	SkipImages            bool
	GenerateAnki          bool
	AnkiCSV               bool
	DeckName              string
	ListModels            bool
	AllVoices             bool
	NoAutoPlay            bool
	Archive               bool

	// OpenAI flags
	OpenAIModel       string
	OpenAIVoice       string
	OpenAISpeed       float64
	OpenAIInstruction string

	// OpenAI Image flags
	OpenAIImageModel   string
	OpenAIImageSize    string
	OpenAIImageQuality string
	OpenAIImageStyle   string

	// Gemini audio flags
	// GeminiTTSModel is the Gemini TTS model used when Gemini audio is selected.
	GeminiTTSModel string
	// GeminiVoice selects a specific Gemini voice; empty picks a random Gemini voice.
	GeminiVoice string

	// NanoBananaModel is the Gemini image model used for Nano Banana generation.
	NanoBananaModel string
	// NanoBananaModelSpecified records whether the Nano Banana image model was explicitly set on the CLI.
	NanoBananaModelSpecified bool
	// NanoBananaTextModel is the Gemini text model used for Nano Banana prompt generation.
	NanoBananaTextModel string
	// NanoBananaTextModelSpecified records whether the Nano Banana text model was explicitly set on the CLI.
	NanoBananaTextModelSpecified bool
}

// NewFlags creates a new Flags instance with default values
func NewFlags() *Flags {
	defaults := audio.DefaultProviderConfig()

	return &Flags{
		AudioFormat:         defaults.OutputFormat,
		AudioProvider:       defaults.Provider,
		ImageAPI:            "nanobanana",
		VideoEnabled:        true,
		DeckName:            "Bulgarian Vocabulary",
		OpenAIModel:         "gpt-4o-mini-tts",
		OpenAISpeed:         0.9,
		OpenAIImageModel:    "dall-e-2",
		OpenAIImageSize:     "512x512",
		OpenAIImageQuality:  "standard",
		OpenAIImageStyle:    "natural",
		GeminiTTSModel:      defaults.GeminiTTSModel,
		NanoBananaModel:     config.DefaultNanoBananaModel,
		NanoBananaTextModel: config.DefaultNanoBananaTextModel,
	}
}

func setupFlags(cmd *cobra.Command, flags *Flags) {
	// Set default output directory to match GUI mode
	home, err := config.HomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
	defaultOutputDir := filepath.Join(home, ".local", "state", "totalrecall", "cards")

	// Global flags
	// AGENT: The default config file location should be ~/.config/totalrecall/config.yaml
	cmd.PersistentFlags().StringVar(&flags.CfgFile, "config", "", "config file (default is $HOME/.totalrecall.yaml)")

	// Local flags
	cmd.Flags().StringVarP(&flags.OutputDir, "output", "o", defaultOutputDir, "Output directory")
	cmd.Flags().StringVarP(&flags.AudioFormat, "format", "f", flags.AudioFormat, "Audio format (wav or mp3; Gemini TTS writes wav natively and auto-converts to mp3 with ffmpeg, which is now the default)")
	cmd.Flags().StringVar(&flags.ImageAPI, "image-api", flags.ImageAPI, "Image source for explicit CLI runs (default: Nano Banana; use openai to switch, config file image.provider also applies when unset)")
	cmd.Flags().StringVar(&flags.BatchFile, "batch", "", "Process words from file (one per line)")
	cmd.Flags().StringVar(&flags.StoryFile, "story", "", "Generate a vocabulary story + comic image from a batch-format file (outputs to current directory)")
	cmd.Flags().StringVar(&flags.StoryStyle, "story-style", "", "Art style for comic pages (default: random). E.g. \"ultra realistic comic strip with photographic detail and dramatic lighting\"")
	cmd.Flags().StringVar(&flags.StoryTheme, "story-theme", "", "Genre/theme for the story (default: random). E.g. \"a thrilling space adventure with aliens and spaceships\"")
	cmd.Flags().BoolVar(&flags.StoryNoUltraRealistic, "no-ultra-realistic", false, "Disable photorealistic rendering requirement; produces standard comic-book style output")
	cmd.Flags().BoolVar(&flags.StoryUltraRealistic, "ultra-realistic", false, "Force photorealistic rendering for all pages (overrides the default random 50/50 pick)")
	cmd.Flags().StringVar(&flags.StorySlug, "story-slug", "",
		"Force the output directory slug for --story (e.g. \"ai-jungle-quest\"). "+
			"Use this to repair a partial run: existing pages are skipped, missing ones are generated.")
	cmd.Flags().StringVar(&flags.NarratorVoice, "narrator-voice", "",
		"Gemini voice for cinematic story narration (default: random from cinematic pool). "+
			"Valid values: Charon, Fenrir, Enceladus, Algieba, Aoede, Schedar")
	cmd.Flags().BoolVar(&flags.NarrateEnabled, "narrate", false,
		"Generate a cinematic MP3 narration of the story after --story completes (default false). "+
			"Requires GOOGLE_API_KEY. Use --narrator-voice to pick a specific voice.")
	cmd.Flags().BoolVar(&flags.VideoEnabled, "video", flags.VideoEnabled,
		"Prompt to generate Veo videos after comic generation (default true; use --video=false to skip)")
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
	flags.AudioFormatSpecified = cmd.Flags().Changed("format")
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

func openAIVoiceUsage() string {
	return "OpenAI voice: " + strings.Join(audio.OpenAIVoices, ", ") + " (default: random)"
}

func geminiVoiceUsage() string {
	return "Gemini voice: " + strings.Join(audio.GeminiVoices, ", ") + " (default: random)"
}
