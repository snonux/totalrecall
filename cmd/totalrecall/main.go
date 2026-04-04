package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"codeberg.org/snonux/totalrecall/internal/archive"
	"codeberg.org/snonux/totalrecall/internal/cli"
	appconfig "codeberg.org/snonux/totalrecall/internal/config"
	"codeberg.org/snonux/totalrecall/internal/gui"
	"codeberg.org/snonux/totalrecall/internal/models"
	"codeberg.org/snonux/totalrecall/internal/processor"
	"codeberg.org/snonux/totalrecall/internal/story"
)

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
		})
		return runner.Run(flags.StoryFile)
	}

	// Auto-adjust image size for DALL-E 3
	if flags.OpenAIImageModel == "dall-e-3" && !cmd.Flags().Changed("openai-image-size") {
		// If user didn't explicitly set size, use 1024x1024 for DALL-E 3
		flags.OpenAIImageSize = "1024x1024"
		fmt.Printf("Note: Using image size 1024x1024 for DALL-E 3 (use --openai-image-size to override)\n")
	}

	// Create processor
	proc := processor.NewProcessor(flags)

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
