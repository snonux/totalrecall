package processor

// CardStore manages the on-disk layout of word card directories.
// It wraps the low-level internal.FindCardDirectory /
// internal.FindOrCreateCardDirectory helpers and adds the higher-level
// isWordFullyProcessed check used by the batch processor to skip words that
// have already been completely generated.
//
// All methods are on *Processor rather than a separate struct to avoid an
// extra layer of indirection while still keeping the concerns separated into
// their own file (SRP at the file level, as recommended for Go packages).

import (
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/snonux/totalrecall/internal"
	"codeberg.org/snonux/totalrecall/internal/anki"
	"codeberg.org/snonux/totalrecall/internal/audio"
)

// findOrCreateWordDirectory returns the existing card directory for word
// inside the configured output directory, creating it when absent.
func (p *Processor) findOrCreateWordDirectory(word string) string {
	return internal.FindOrCreateCardDirectory(p.flags.OutputDir, word)
}

// findCardDirectory searches the configured output directory for an existing
// card directory that contains the given word. Returns an empty string when
// no matching directory is found.
func (p *Processor) findCardDirectory(word string) string {
	return internal.FindCardDirectory(p.flags.OutputDir, word)
}

// isWordFullyProcessed returns true when the word's card directory already
// contains all expected output files (audio, image, translation, phonetic).
// The exact set of required files depends on the --skip-audio / --skip-images
// flags so partially-generated cards are still re-processed when relevant.
func (p *Processor) isWordFullyProcessed(word string) bool {
	wordDir := p.findCardDirectory(word)
	if wordDir == "" {
		return false // No directory exists yet.
	}

	// Base set of required files for every card type.
	requiredFiles := []string{
		"word.txt",
		"translation.txt",
		"phonetic.txt",
	}

	if !p.flags.SkipAudio {
		if !p.hasRequiredAudioFiles(wordDir, &requiredFiles) {
			return false
		}
	}

	if !p.flags.SkipImages {
		if !p.hasRequiredImageFiles(wordDir, &requiredFiles) {
			return false
		}
	}

	// Verify that every file in the required list actually exists on disk.
	for _, file := range requiredFiles {
		if _, err := os.Stat(filepath.Join(wordDir, file)); os.IsNotExist(err) {
			return false
		}
	}

	return true
}

// hasRequiredAudioFiles checks that all expected audio files and their
// attribution sidecars exist in wordDir. It appends extra filenames to
// requiredFiles as a side-effect so they are validated by the caller.
// Returns false as soon as a required audio file is determined to be missing.
func (p *Processor) hasRequiredAudioFiles(wordDir string, requiredFiles *[]string) bool {
	cardType := internal.LoadCardType(wordDir)
	audioFormat := p.effectiveAudioFormat()

	if cardType.IsBgBg() {
		return p.hasBgBgAudioFiles(wordDir, audioFormat)
	}

	return p.hasEnBgAudioFiles(wordDir, audioFormat, requiredFiles)
}

// hasBgBgAudioFiles verifies that both audio_front and audio_back files exist
// along with their attribution sidecars. Used for bg-bg (definition) cards.
func (p *Processor) hasBgBgAudioFiles(wordDir, audioFormat string) bool {
	frontAudioFiles := anki.ResolveAudioPaths(wordDir, "audio_front", audioFormat)
	backAudioFiles := anki.ResolveAudioPaths(wordDir, "audio_back", audioFormat)
	if len(frontAudioFiles) == 0 || len(backAudioFiles) == 0 {
		return false
	}
	for _, audioFile := range append(frontAudioFiles, backAudioFiles...) {
		if _, err := os.Stat(audio.AttributionPath(audioFile)); os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// hasEnBgAudioFiles verifies that at least one resolved audio file exists
// along with its attribution sidecar. Used for en-bg (translation) cards.
// It also appends "audio_metadata.txt" to requiredFiles so the caller checks it.
func (p *Processor) hasEnBgAudioFiles(wordDir, audioFormat string, requiredFiles *[]string) bool {
	*requiredFiles = append(*requiredFiles, "audio_metadata.txt")

	audioFiles := anki.ResolveAudioPaths(wordDir, "audio", audioFormat)
	if len(audioFiles) == 0 {
		return false
	}
	for _, audioFile := range audioFiles {
		if _, err := os.Stat(audio.AttributionPath(audioFile)); os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// hasRequiredImageFiles checks that at least one image file exists and that
// the expected image sidecar files are present. It appends those sidecar
// filenames to requiredFiles as a side-effect.
// Returns false when no image file can be found.
func (p *Processor) hasRequiredImageFiles(wordDir string, requiredFiles *[]string) bool {
	*requiredFiles = append(*requiredFiles,
		"image_attribution.txt",
		"image_prompt.txt",
	)

	// Accept any of the common image extensions and naming conventions.
	imagePatterns := []string{"image_*.jpg", "image_*.png", "image_*.webp", "image.jpg", "image.png", "image.webp"}
	for _, pattern := range imagePatterns {
		if strings.Contains(pattern, "*") {
			matches, _ := filepath.Glob(filepath.Join(wordDir, pattern))
			if len(matches) > 0 {
				return true
			}
		} else {
			if _, err := os.Stat(filepath.Join(wordDir, pattern)); err == nil {
				return true
			}
		}
	}
	return false
}
