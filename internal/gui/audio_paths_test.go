package gui

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/widget"
	ttwidget "github.com/dweymouth/fyne-tooltip/widget"
)

func newGUIAudioTestApp(t *testing.T, tempDir string) *Application {
	t.Helper()

	fyneApp := fyneapp.New()
	t.Cleanup(func() {
		fyneApp.Quit()
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	return &Application{
		config: &Config{
			OutputDir:   tempDir,
			AudioFormat: "wav",
		},
		ctx:                      ctx,
		cancel:                   cancel,
		queue:                    NewWordQueue(ctx),
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
}

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

	app := newGUIAudioTestApp(t, tempDir)

	app.scanExistingWords()
	if len(app.existingWords) != 1 || app.existingWords[0] != word {
		t.Fatalf("scanExistingWords() = %v, want %q", app.existingWords, word)
	}

	app.loadExistingFiles(word)

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

func TestLoadExistingFilesPrefersFreshMetadataAudioOverLegacyVoiceSpecificFile(t *testing.T) {
	tempDir := t.TempDir()
	wordDir := filepath.Join(tempDir, "word-card")
	if err := os.MkdirAll(wordDir, 0755); err != nil {
		t.Fatalf("failed to create word dir: %v", err)
	}

	word := "ябълка"
	if err := os.WriteFile(filepath.Join(wordDir, "word.txt"), []byte(word), 0644); err != nil {
		t.Fatalf("failed to write word file: %v", err)
	}

	staleLegacyAudio := filepath.Join(wordDir, "audio_legacy-voice.wav")
	if err := os.WriteFile(staleLegacyAudio, []byte("stale-legacy-audio"), 0644); err != nil {
		t.Fatalf("failed to write stale legacy audio file: %v", err)
	}

	freshAudio := filepath.Join(wordDir, "audio.wav")
	if err := os.WriteFile(freshAudio, []byte("fresh-audio"), 0644); err != nil {
		t.Fatalf("failed to write fresh audio file: %v", err)
	}

	metadata := "provider=gemini\nvoice=\nspeed=1.00\nformat=wav\ncardtype=en-bg\naudio_file=audio.wav\n"
	if err := os.WriteFile(filepath.Join(wordDir, "audio_metadata.txt"), []byte(metadata), 0644); err != nil {
		t.Fatalf("failed to write audio metadata: %v", err)
	}

	app := newGUIAudioTestApp(t, tempDir)

	app.loadExistingFiles(word)

	if app.currentAudioFile != freshAudio {
		t.Fatalf("currentAudioFile = %q, want fresh generated audio %q", app.currentAudioFile, freshAudio)
	}
	if app.audioPlayer.audioFile != freshAudio {
		t.Fatalf("audioPlayer.audioFile = %q, want fresh generated audio %q", app.audioPlayer.audioFile, freshAudio)
	}
}

func TestCompletedBgBgJobKeepsBackAudioForSessionNavigation(t *testing.T) {
	tempDir := t.TempDir()
	app := newGUIAudioTestApp(t, tempDir)

	job := app.queue.AddWord("ябълка")
	job.CardType = "bg-bg"

	frontAudio := filepath.Join(tempDir, "card", "audio_front.wav")
	backAudio := filepath.Join(tempDir, "card", "audio_back.wav")
	app.queue.CompleteJob(job.ID, "определение", frontAudio, backAudio, "")

	app.loadWordByIndex(0)

	if app.queue.GetCompletedJobs()[0].AudioFileBack != backAudio {
		t.Fatalf("completed job back audio = %q, want %q", app.queue.GetCompletedJobs()[0].AudioFileBack, backAudio)
	}
	if app.currentAudioFileBack != backAudio {
		t.Fatalf("currentAudioFileBack = %q, want %q", app.currentAudioFileBack, backAudio)
	}
	if app.currentCardType != "bg-bg" {
		t.Fatalf("currentCardType = %q, want %q", app.currentCardType, "bg-bg")
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
