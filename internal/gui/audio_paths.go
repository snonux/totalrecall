package gui

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

var supportedAudioExtensions = map[string]struct{}{
	".aac":  {},
	".flac": {},
	".mp3":  {},
	".opus": {},
	".wav":  {},
}

func (a *Application) resolveSingleAudioFile(wordDir string) string {
	return resolveAudioFileByBaseName(wordDir, "audio")
}

func (a *Application) resolveBgBgAudioFiles(wordDir string) (string, string) {
	return resolveAudioFileByBaseName(wordDir, "audio_front"), resolveAudioFileByBaseName(wordDir, "audio_back")
}

func (a *Application) hasAnyAudioFile(wordDir string) bool {
	single := a.resolveSingleAudioFile(wordDir)
	if single != "" {
		return true
	}

	front, back := a.resolveBgBgAudioFiles(wordDir)
	return front != "" || back != ""
}

func resolveAudioFileByBaseName(wordDir, baseName string) string {
	entries, err := os.ReadDir(wordDir)
	if err != nil {
		return ""
	}

	prefix := baseName + "."
	var resolved string
	var resolvedModTime time.Time

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}

		ext := strings.ToLower(filepath.Ext(name))
		if _, ok := supportedAudioExtensions[ext]; !ok {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		candidate := filepath.Join(wordDir, name)
		if resolved == "" || info.ModTime().After(resolvedModTime) || (info.ModTime().Equal(resolvedModTime) && candidate < resolved) {
			resolved = candidate
			resolvedModTime = info.ModTime()
		}
	}

	return resolved
}
