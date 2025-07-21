package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

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

	// Handle --list-models flag
	if flags.ListModels {
		lister := models.NewLister(cli.GetOpenAIKey())
		return lister.ListAvailableModels()
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
		if err := proc.GenerateAnkiFile(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to generate Anki file: %v\n", err)
		} else {
			if flags.AnkiCSV {
				fmt.Println("Anki import file created: anki_import.csv")
			} else {
				fmt.Printf("Anki package created: %s.apkg\n", flags.DeckName)
			}
		}
	}

	fmt.Printf("\nDone! Materials saved to: %s\n", flags.OutputDir)
	return nil
}
