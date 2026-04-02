package gui

import (
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/snonux/totalrecall/internal/anki"
)

func (a *Application) resolveSingleAudioFile(wordDir string) string {
	if audioFile := resolveAudioFileFromMetadata(wordDir, "audio_file"); audioFile != "" {
		return audioFile
	}

	return anki.ResolveAudioFile(wordDir, "audio", "")
}

func (a *Application) resolveBgBgAudioFiles(wordDir string) (string, string) {
	front := resolveAudioFileFromMetadata(wordDir, "audio_file")
	if front == "" {
		front = anki.ResolveAudioFile(wordDir, "audio_front", "")
	}

	back := resolveAudioFileFromMetadata(wordDir, "audio_file_back")
	if back == "" {
		back = anki.ResolveAudioFile(wordDir, "audio_back", "")
	}

	return front, back
}

func (a *Application) hasAnyAudioFile(wordDir string) bool {
	single := a.resolveSingleAudioFile(wordDir)
	if single != "" {
		return true
	}

	front, back := a.resolveBgBgAudioFiles(wordDir)
	return front != "" || back != ""
}

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
