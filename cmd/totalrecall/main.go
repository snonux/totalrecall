package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"codeberg.org/snonux/totalrecall/internal/archive"
	"codeberg.org/snonux/totalrecall/internal/cli"
	"codeberg.org/snonux/totalrecall/internal/models"
	"codeberg.org/snonux/totalrecall/internal/processor"
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
		return runCommand(cmd, args, flags)
	}

	// Execute command
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runCommand(cmd *cobra.Command, args []string, flags *cli.Flags) error {
	// Check if output directory was set in config file
	if !cmd.Flags().Changed("output") && flags.OutputDir != "" {
		// Output directory already set by flags
	}

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
		lister := models.NewLister(cli.GetOpenAIKey())
		return lister.ListAvailableModels()
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
		return proc.RunGUIMode()
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
