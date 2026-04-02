package gui

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResolveSingleAudioFileFindsLegacyMp3WhenGuiDefaultIsWav(t *testing.T) {
	tempDir := t.TempDir()
	wordDir := filepath.Join(tempDir, "word")
	if err := os.MkdirAll(wordDir, 0755); err != nil {
		t.Fatalf("failed to create word dir: %v", err)
	}

	mp3Path := filepath.Join(wordDir, "audio.mp3")
	if err := os.WriteFile(mp3Path, []byte("mp3"), 0644); err != nil {
		t.Fatalf("failed to write mp3 file: %v", err)
	}

	app := &Application{
		config: &Config{AudioFormat: "wav"},
	}

	got := app.resolveSingleAudioFile(wordDir)
	if got != mp3Path {
		t.Fatalf("resolveSingleAudioFile() = %q, want %q", got, mp3Path)
	}
}

func TestResolveSingleAudioFilePrefersNewerOnDiskAudio(t *testing.T) {
	tempDir := t.TempDir()
	wordDir := filepath.Join(tempDir, "word")
	if err := os.MkdirAll(wordDir, 0755); err != nil {
		t.Fatalf("failed to create word dir: %v", err)
	}

	mp3Path := filepath.Join(wordDir, "audio.mp3")
	wavPath := filepath.Join(wordDir, "audio.wav")
	if err := os.WriteFile(mp3Path, []byte("mp3"), 0644); err != nil {
		t.Fatalf("failed to write mp3 file: %v", err)
	}
	if err := os.WriteFile(wavPath, []byte("wav"), 0644); err != nil {
		t.Fatalf("failed to write wav file: %v", err)
	}

	older := time.Now().Add(-time.Hour)
	newer := time.Now()
	if err := os.Chtimes(mp3Path, older, older); err != nil {
		t.Fatalf("failed to set mp3 file time: %v", err)
	}
	if err := os.Chtimes(wavPath, newer, newer); err != nil {
		t.Fatalf("failed to set wav file time: %v", err)
	}

	app := &Application{
		config: &Config{AudioFormat: "wav"},
	}

	got := app.resolveSingleAudioFile(wordDir)
	if got != wavPath {
		t.Fatalf("resolveSingleAudioFile() = %q, want newer wav %q", got, wavPath)
	}
}

func TestResolveBgBgAudioFilesFindLegacyMp3Files(t *testing.T) {
	tempDir := t.TempDir()
	wordDir := filepath.Join(tempDir, "word")
	if err := os.MkdirAll(wordDir, 0755); err != nil {
		t.Fatalf("failed to create word dir: %v", err)
	}

	frontPath := filepath.Join(wordDir, "audio_front.mp3")
	backPath := filepath.Join(wordDir, "audio_back.mp3")
	if err := os.WriteFile(frontPath, []byte("front"), 0644); err != nil {
		t.Fatalf("failed to write front file: %v", err)
	}
	if err := os.WriteFile(backPath, []byte("back"), 0644); err != nil {
		t.Fatalf("failed to write back file: %v", err)
	}

	older := time.Now().Add(-time.Hour)
	if err := os.Chtimes(frontPath, older, older); err != nil {
		t.Fatalf("failed to set front file time: %v", err)
	}

	app := &Application{
		config: &Config{AudioFormat: "wav"},
	}

	gotFront, gotBack := app.resolveBgBgAudioFiles(wordDir)
	if gotFront != frontPath {
		t.Fatalf("resolveBgBgAudioFiles() front = %q, want %q", gotFront, frontPath)
	}
	if gotBack != backPath {
		t.Fatalf("resolveBgBgAudioFiles() back = %q, want %q", gotBack, backPath)
	}
}
