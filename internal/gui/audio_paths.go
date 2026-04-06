package gui

import (
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/snonux/totalrecall/internal/anki"
)

// resolveSingleAudioFile resolves the en-bg audio file path for a card directory.
// Delegates to the package-level helper used by CardService.
func (a *Application) resolveSingleAudioFile(wordDir string) string {
	return resolveSingleAudioFileInDir(wordDir)
}

// resolveBgBgAudioFiles resolves front and back audio file paths for a bg-bg
// card directory. Delegates to the package-level helper used by CardService.
func (a *Application) resolveBgBgAudioFiles(wordDir string) (string, string) {
	return resolveBgBgAudioFilesInDir(wordDir)
}

// hasAnyAudioFile returns true if the card directory contains any audio file.
// Delegates to the package-level helper used by CardService.
func (a *Application) hasAnyAudioFile(wordDir string) bool {
	return hasAnyAudioFileInDir(wordDir)
}

// resolveAudioFileFromMetadata reads audio_metadata.txt from wordDir and returns
// the path stored under key. Returns empty string when the key is absent or the
// file does not exist on disk.
func resolveAudioFileFromMetadata(wordDir, key string) string {
	metadata := readAudioMetadata(wordDir)
	value := strings.TrimSpace(metadata[key])
	if value == "" {
		return ""
	}

	if !filepath.IsAbs(value) {
		value = filepath.Join(wordDir, value)
	}

	if _, err := os.Stat(value); err == nil {
		return value
	}

	return ""
}

// readAudioMetadata parses audio_metadata.txt in wordDir into a key→value map.
// Returns an empty map when the file does not exist or cannot be read.
func readAudioMetadata(wordDir string) map[string]string {
	metadataFile := filepath.Join(wordDir, "audio_metadata.txt")
	data, err := os.ReadFile(metadataFile)
	if err != nil {
		return map[string]string{}
	}

	values := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		values[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}

	return values
}

// anki.ResolveAudioFile is kept here to avoid importing anki in card_service.go
// directly. The package-level helpers reference it via this file.
var _ = anki.ResolveAudioFile
