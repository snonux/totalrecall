package processor

// AnkiExporter builds Anki import artifacts (CSV or APKG) from the in-memory
// translation cache and on-disk card directories.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/snonux/totalrecall/internal"
	"codeberg.org/snonux/totalrecall/internal/anki"
)

// AnkiExporter generates Anki deck output using Processor state (flags, cache,
// card directory lookups).
type AnkiExporter struct {
	p *Processor
}

// GenerateAnkiFile generates the Anki import file and returns the output path.
// When --anki is specified the file is placed in the user's home directory;
// otherwise it goes into the configured output directory.
func (e *AnkiExporter) GenerateAnkiFile() (string, error) {
	p := e.p
	outputDir, err := e.resolveAnkiOutputDir()
	if err != nil {
		return "", err
	}

	audioFormat := p.EffectiveAudioFormat()
	gen := anki.NewGenerator(&anki.GeneratorOptions{
		OutputPath:     filepath.Join(outputDir, "anki_import.csv"),
		MediaFolder:    p.Flags.OutputDir,
		IncludeHeaders: true,
		AudioFormat:    audioFormat,
	})

	if err := e.populateAnkiGenerator(gen, audioFormat); err != nil {
		return "", err
	}

	return e.writeAnkiOutput(gen, outputDir)
}

// resolveAnkiOutputDir returns the directory where the Anki file should be
// written. When --anki is set it resolves to the user's home directory.
func (e *AnkiExporter) resolveAnkiOutputDir() (string, error) {
	p := e.p
	if p.Flags.GenerateAnki {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return homeDir, nil
	}
	return p.Flags.OutputDir, nil
}

// populateAnkiGenerator fills the generator with cards. When the translation
// cache is populated it is used as the authoritative source; otherwise the
// generator falls back to scanning the output directory for existing cards.
func (e *AnkiExporter) populateAnkiGenerator(gen *anki.Generator, audioFormat string) error {
	p := e.p
	translations := p.translationCache.GetAll()
	if len(translations) == 0 {
		fmt.Println("  No translations found in cache, generating cards from directory...")
		if err := gen.GenerateFromDirectory(p.Flags.OutputDir); err != nil {
			return fmt.Errorf("failed to generate cards from directory: %w", err)
		}
		return nil
	}

	fmt.Printf("  Generating cards from %d translations in cache...\n", len(translations))
	for bulgarian, english := range translations {
		card := e.buildAnkiCard(bulgarian, english, audioFormat)
		gen.AddCard(card)
	}
	return nil
}

// buildAnkiCard constructs an anki.Card for a word, resolving all associated
// media files (audio, image, phonetic) from the word's card directory.
func (e *AnkiExporter) buildAnkiCard(bulgarian, english, audioFormat string) anki.Card {
	p := e.p
	card := anki.Card{
		Bulgarian:   bulgarian,
		Translation: english,
	}

	wordDir := p.findCardDirectory(bulgarian)
	if wordDir == "" {
		return card
	}

	cardType := internal.LoadCardType(wordDir)
	if cardType.IsBgBg() {
		card.AudioFile = anki.ResolveAudioFile(wordDir, "audio_front", audioFormat)
		card.AudioFileBack = anki.ResolveAudioFile(wordDir, "audio_back", audioFormat)
	} else {
		card.AudioFile = anki.ResolveAudioFile(wordDir, "audio", audioFormat)
	}

	imageFile := filepath.Join(wordDir, "image.jpg")
	if _, err := os.Stat(imageFile); err == nil {
		card.ImageFile = imageFile
	}

	phoneticFile := filepath.Join(wordDir, "phonetic.txt")
	if data, err := os.ReadFile(phoneticFile); err == nil {
		notes := strings.TrimSpace(string(data))
		card.Notes = strings.ReplaceAll(notes, "\n", "<br>")
	}

	return card
}

// writeAnkiOutput generates either a CSV or APKG file depending on the
// --anki-csv flag and returns the output path.
func (e *AnkiExporter) writeAnkiOutput(gen *anki.Generator, outputDir string) (string, error) {
	p := e.p
	if p.Flags.AnkiCSV {
		outputPath := filepath.Join(outputDir, "anki_import.csv")
		if err := gen.GenerateCSV(); err != nil {
			return "", fmt.Errorf("failed to generate CSV: %w", err)
		}
		e.printAnkiStats(gen)
		return outputPath, nil
	}

	outputPath := filepath.Join(outputDir, fmt.Sprintf("%s.apkg", internal.SanitizeFilename(p.Flags.DeckName)))
	if err := gen.GenerateAPKG(outputPath, p.Flags.DeckName); err != nil {
		return "", fmt.Errorf("failed to generate APKG: %w", err)
	}
	e.printAnkiStats(gen)
	return outputPath, nil
}

// printAnkiStats logs the card generation statistics to stdout.
func (e *AnkiExporter) printAnkiStats(gen *anki.Generator) {
	total, withAudio, withImages := gen.Stats()
	fmt.Printf("  Generated %d cards (%d with audio, %d with images)\n", total, withAudio, withImages)
}
