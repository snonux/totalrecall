package anki

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ResolveAudioPaths returns the matching audio files for a logical base name.
// It prefers multi-voice outputs (audio_<voice>.<ext>) over a stale single file
// and uses metadata hints before falling back to common formats.
func ResolveAudioPaths(wordDir, baseName, preferredFormat string) []string {
	formats := audioFormatsToTry(wordDir, preferredFormat)

	// Prefer voice-specific files so multi-voice output wins over any stale
	// single-file audio that may be left behind in the directory.
	for _, format := range formats {
		globPattern := filepath.Join(wordDir, baseName+"_*."+format)
		matches, err := filepath.Glob(globPattern)
		if err == nil && len(matches) > 0 {
			sort.Strings(matches)
			return matches
		}
	}

	for _, format := range formats {
		exactPath := filepath.Join(wordDir, baseName+"."+format)
		if fileExists(exactPath) {
			return []string{exactPath}
		}
	}

	return nil
}

// ResolveAudioFile returns the first resolved audio file for a logical base name.
func ResolveAudioFile(wordDir, baseName, preferredFormat string) string {
	paths := ResolveAudioPaths(wordDir, baseName, preferredFormat)
	if len(paths) == 0 {
		return ""
	}

	return paths[0]
}

func audioFormatsToTry(wordDir, preferredFormat string) []string {
	var candidates []string
	appendFormat := func(format string) {
		format = strings.ToLower(strings.TrimSpace(format))
		if format == "" || containsString(candidates, format) {
			return
		}
		candidates = append(candidates, format)
	}

	appendFormat(readAudioFormatHint(wordDir))
	appendFormat(preferredFormat)
	appendFormat("wav")
	appendFormat("mp3")

	return candidates
}

func readAudioFormatHint(wordDir string) string {
	metadataFile := filepath.Join(wordDir, "audio_metadata.txt")
	data, err := os.ReadFile(metadataFile)
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "format=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "format="))
		}
	}

	return ""
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
