package anki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveAudioFilePrefersVoiceSpecificFilesOverStaleSingleFile(t *testing.T) {
	tempDir := t.TempDir()
	wordDir := filepath.Join(tempDir, "ябълка")
	if err := os.MkdirAll(wordDir, 0755); err != nil {
		t.Fatalf("failed to create word dir: %v", err)
	}

	files := map[string]string{
		"audio.mp3":       "stale audio",
		"audio_alpha.wav": "voice alpha audio",
		"audio_beta.wav":  "voice beta audio",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(wordDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}

	got := ResolveAudioFile(wordDir, "audio", "mp3")
	if !strings.HasSuffix(got, "audio_alpha.wav") {
		t.Fatalf("ResolveAudioFile() = %q, want voice-specific wav file", got)
	}
}
