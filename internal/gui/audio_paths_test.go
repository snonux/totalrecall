package gui

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/widget"
	ttwidget "github.com/dweymouth/fyne-tooltip/widget"
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

func TestGUIDiscoveryAndLoadingRecognizeVoiceSuffixedAudio(t *testing.T) {
	fyneApp := fyneapp.New()
	t.Cleanup(func() {
		fyneApp.Quit()
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	tempDir := t.TempDir()
	wordDir := filepath.Join(tempDir, "word-card")
	if err := os.MkdirAll(wordDir, 0755); err != nil {
		t.Fatalf("failed to create word dir: %v", err)
	}

	word := "ябълка"
	if err := os.WriteFile(filepath.Join(wordDir, "word.txt"), []byte(word), 0644); err != nil {
		t.Fatalf("failed to write word file: %v", err)
	}

	voiceAudioPath := filepath.Join(wordDir, "audio_sentinel-gemini-voice.mp3")
	if err := os.WriteFile(voiceAudioPath, []byte("voice-audio"), 0644); err != nil {
		t.Fatalf("failed to write voice-suffixed audio file: %v", err)
	}

	app := &Application{
		config: &Config{
			OutputDir:   tempDir,
			AudioFormat: "wav",
		},
		ctx:                      ctx,
		cancel:                   cancel,
		queue:                    NewWordQueue(context.Background()),
		wordInput:                NewCustomEntry(),
		audioPlayer:              NewAudioPlayer(),
		imageDisplay:             NewImageDisplay(),
		translationEntry:         NewCustomEntry(),
		cardTypeSelect:           widget.NewSelect([]string{"English → Bulgarian", "Bulgarian → Bulgarian"}, nil),
		imagePromptEntry:         NewCustomMultiLineEntry(),
		statusLabel:              widget.NewLabel(""),
		prevWordBtn:              ttwidget.NewButton("", nil),
		nextWordBtn:              ttwidget.NewButton("", nil),
		keepButton:               ttwidget.NewButton("", nil),
		regenerateImageBtn:       ttwidget.NewButton("", nil),
		regenerateRandomImageBtn: ttwidget.NewButton("", nil),
		regenerateAudioBtn:       ttwidget.NewButton("", nil),
		regenerateAllBtn:         ttwidget.NewButton("", nil),
		deleteButton:             ttwidget.NewButton("", nil),
	}

	app.scanExistingWords()
	if len(app.existingWords) != 1 || app.existingWords[0] != word {
		t.Fatalf("scanExistingWords() = %v, want %q", app.existingWords, word)
	}

	app.loadExistingFiles(word)
	time.Sleep(50 * time.Millisecond)

	if app.currentAudioFile != voiceAudioPath {
		t.Fatalf("currentAudioFile = %q, want voice-suffixed path %q", app.currentAudioFile, voiceAudioPath)
	}

	if app.audioPlayer == nil {
		t.Fatal("expected audio player to be initialized")
	}
	if app.audioPlayer.audioFile != voiceAudioPath {
		t.Fatalf("audioPlayer.audioFile = %q, want %q", app.audioPlayer.audioFile, voiceAudioPath)
	}

	if app.currentCardType != "en-bg" {
		t.Fatalf("currentCardType = %q, want %q", app.currentCardType, "en-bg")
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
