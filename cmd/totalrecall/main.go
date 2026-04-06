package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"codeberg.org/snonux/totalrecall/internal/archive"
	"codeberg.org/snonux/totalrecall/internal/cli"
	appconfig "codeberg.org/snonux/totalrecall/internal/config"
	"codeberg.org/snonux/totalrecall/internal/gui"
	"codeberg.org/snonux/totalrecall/internal/models"
	"codeberg.org/snonux/totalrecall/internal/processor"
	"codeberg.org/snonux/totalrecall/internal/story"
)

// newProcessorConfig reads all Viper-sourced settings in a single pass and
// returns a fully-resolved processor.Config. Centralising all Viper access
// here means the processor package is free of any Viper dependency, which
// improves testability and removes tight coupling to the global config singleton.
func newProcessorConfig() *processor.Config {
	return &processor.Config{
		// Translation & phonetic
		TranslationProvider:    strings.TrimSpace(viper.GetString("translation.provider")),
		PhoneticProvider:       strings.TrimSpace(viper.GetString("phonetic.provider")),
		TranslationGeminiModel: viper.GetString("translation.gemini_model"),

		// Audio
		AudioProvider:        strings.ToLower(strings.TrimSpace(viper.GetString("audio.provider"))),
		AudioFormat:          strings.ToLower(strings.TrimSpace(viper.GetString("audio.format"))),
		AudioFormatSet:       viper.IsSet("audio.format"),
		GeminiTTSModel:       strings.TrimSpace(viper.GetString("audio.gemini_tts_model")),
		GeminiVoice:          strings.TrimSpace(viper.GetString("audio.gemini_voice")),
		OpenAIVoice:          strings.TrimSpace(viper.GetString("audio.openai_voice")),
		OpenAIModel:          viper.GetString("audio.openai_model"),
		OpenAIModelSet:       viper.IsSet("audio.openai_model"),
		OpenAISpeed:          viper.GetFloat64("audio.openai_speed"),
		OpenAISpeedSet:       viper.IsSet("audio.openai_speed"),
		OpenAIInstruction:    viper.GetString("audio.openai_instruction"),
		OpenAIInstructionSet: viper.IsSet("audio.openai_instruction"),

		// Image
		ImageProvider:               strings.ToLower(strings.TrimSpace(viper.GetString("image.provider"))),
		ImageOpenAIModel:            viper.GetString("image.openai_model"),
		ImageOpenAIModelSet:         viper.IsSet("image.openai_model"),
		ImageOpenAISize:             viper.GetString("image.openai_size"),
		ImageOpenAISizeSet:          viper.IsSet("image.openai_size"),
		ImageOpenAIQuality:          viper.GetString("image.openai_quality"),
		ImageOpenAIQualitySet:       viper.IsSet("image.openai_quality"),
		ImageOpenAIStyle:            viper.GetString("image.openai_style"),
		ImageOpenAIStyleSet:         viper.IsSet("image.openai_style"),
		ImageNanoBananaModel:        strings.TrimSpace(viper.GetString("image.nanobanana_model")),
		ImageNanoBananaModelSet:     viper.IsSet("image.nanobanana_model"),
		ImageNanoBananaTextModel:    strings.TrimSpace(viper.GetString("image.nanobanana_text_model")),
		ImageNanoBananaTextModelSet: viper.IsSet("image.nanobanana_text_model"),
	}
}

func main() {
	// Create flags instance
	flags := cli.NewFlags()

	// Create root command
	rootCmd := cli.CreateRootCommand(flags)

	// Set up command initialization
	cobra.OnInitialize(func() {
		cli.InitConfig(flags.CfgFile)
	})

	// Set the run function
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		cli.MarkExplicitFlagValues(cmd, flags)
		return runCommand(cmd, args, flags)
	}

	// Execute command
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runCommand(cmd *cobra.Command, args []string, flags *cli.Flags) error {
	// Handle --archive flag
	if flags.Archive {
		home, _ := os.UserHomeDir()
		cardsDir := filepath.Join(home, ".local", "state", "totalrecall", "cards")
		if err := archive.ArchiveCards(cardsDir); err != nil {
			return fmt.Errorf("failed to archive cards: %w", err)
		}
		return nil
	}

	// Handle --list-models flag
	if flags.ListModels {
		lister := models.NewLister(cli.GetOpenAIKey(), cli.GetGoogleAPIKey(), os.Stdout)
		return lister.ListAvailableModels()
	}

	// Handle --story flag: generate a vocabulary story + comic image into CWD.
	// This is deliberately placed before processor creation because it does not
	// need the full processor pipeline (no Anki cards, no per-word audio).
	if flags.StoryFile != "" {
		runner := story.NewRunner(&story.RunnerConfig{
			APIKey:         cli.GetGoogleAPIKey(),
			TextModel:      flags.NanoBananaTextModel,
			ImageModel:     flags.NanoBananaModel,
			ImageTextModel: flags.NanoBananaTextModel,
			OutputDir:      ".",
			Style:          flags.StoryStyle,
			Theme:          flags.StoryTheme,
			UltraRealistic: storyUltraRealistic(flags.StoryNoUltraRealistic),
			NarratorVoice:  flags.NarratorVoice,
			NarrateEnabled: flags.NarrateEnabled,
			Slug:           flags.StorySlug,
		})
		if err := runner.Run(flags.StoryFile); err != nil {
			return err
		}
		return runStoryVideos(flags)
	}

	// Auto-adjust image size for DALL-E 3
	if flags.OpenAIImageModel == "dall-e-3" && !cmd.Flags().Changed("openai-image-size") {
		// If user didn't explicitly set size, use 1024x1024 for DALL-E 3
		flags.OpenAIImageSize = "1024x1024"
		fmt.Printf("Note: Using image size 1024x1024 for DALL-E 3 (use --openai-image-size to override)\n")
	}

	// Resolve all Viper config values once here so the processor never touches
	// the global Viper singleton directly (Dependency Inversion Principle).
	proc := processor.NewProcessor(flags, newProcessorConfig())

	// Handle batch processing
	if flags.BatchFile != "" {
		// Process batch file
		if err := proc.ProcessBatch(); err != nil {
			return err
		}
	} else if len(args) > 0 {
		// Process single word
		if err := proc.ProcessSingleWord(args[0]); err != nil {
			return err
		}
	} else {
		// No input provided - launch GUI mode by default
		return runGUIMode(proc, flags)
	}

	// Generate Anki file if requested
	if flags.GenerateAnki {
		fmt.Printf("\nGenerating Anki import file...\n")
		outputPath, err := proc.GenerateAnkiFile()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to generate Anki file: %v\n", err)
		} else {
			fmt.Printf("Anki package created: %s\n", outputPath)
		}
	}

	fmt.Printf("\nDone! Materials saved to: %s\n", flags.OutputDir)
	return nil
}

// runGUIMode launches the GUI application. It lives in cmd/main.go so that
// gui.New() is called from the composition root rather than from the
// processor package, reducing the processor→gui import coupling.
func runGUIMode(proc *processor.Processor, flags *cli.Flags) error {
	guiConfig := proc.GUIConfig()

	// Only override OutputDir when the user explicitly set a non-default path.
	home, err := appconfig.HomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
	defaultOutputDir := filepath.Join(home, "Downloads")
	if flags.OutputDir != defaultOutputDir {
		guiConfig.OutputDir = flags.OutputDir
	}
	if guiConfig.GoogleAPIKey == "" {
		guiConfig.GoogleAPIKey = cli.GetGoogleAPIKey()
	}

	app := gui.New(guiConfig)
	app.Run()

	return nil
}

// runStoryVideos is called after the story runner completes. When the
// --video flag is true (default), it prompts the user to select gallery pages
// for Veo video generation and then generates the selected videos.
// Passing --video=false skips the prompt entirely.
//
// Video generation failures are intentionally non-fatal: the comic, PDF, and
// narration are already on disk, so a Veo API error should not invalidate
// those outputs. Errors are printed as warnings and the function returns nil.
func runStoryVideos(flags *cli.Flags) error {
	if !flags.VideoEnabled {
		return nil
	}

	// The story runner writes gallery PNGs into ./comics/<slug>/, so we search
	// from "." recursively to find them regardless of the exact slug.
	// PromptForGalleryVideos returns the actual file paths (not just page numbers)
	// so GenerateSelectedVideos can locate them without a second directory search.
	selectedPaths, err := cli.PromptForGalleryVideos(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: video prompt failed: %v\n", err)
		return nil
	}

	if err := cli.GenerateSelectedVideos(cli.GetGoogleAPIKey(), selectedPaths); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: video generation failed: %v\n", err)
	}

	return nil
}

// storyUltraRealistic converts the --no-ultra-realistic bool flag into a *bool
// for RunnerConfig. When noUltraRealistic is true, returns a pointer to false
// (forcing standard comic style). When false (flag not set), returns nil so
// the runner picks randomly 50/50 each run.
func storyUltraRealistic(noUltraRealistic bool) *bool {
	if noUltraRealistic {
		v := false
		return &v
	}
	return nil // nil → random pick in NewRunner
}
