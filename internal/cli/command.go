package cli

import (
	"github.com/spf13/cobra"

	"codeberg.org/snonux/totalrecall/internal"
)

// CreateRootCommand creates and configures the root cobra command
func CreateRootCommand(flags *Flags) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "totalrecall [word]",
		Short: "Bulgarian Anki Flashcard Generator",
		Long: `totalrecall generates Anki flashcard materials from Bulgarian words.

It creates audio pronunciation files using Gemini TTS by default and downloads
representative images. Launching with no arguments opens the interactive GUI, which uses Nano Banana for images by default. Explicit CLI and batch runs also use Nano Banana by default, and can be switched to OpenAI via --image-api openai. Audio can be switched between Gemini and OpenAI with --audio-provider.

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

	setupFlags(rootCmd, flags)

	return rootCmd
}
