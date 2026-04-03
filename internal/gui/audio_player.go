package gui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	ttwidget "github.com/dweymouth/fyne-tooltip/widget"
)

// AudioPlayer is a custom widget for playing audio files
type AudioPlayer struct {
	widget.BaseWidget

	container       *fyne.Container
	playButton      *ttwidget.Button
	playButtonLabel *widget.Label    // Label for front audio button
	playBackButton  *ttwidget.Button // Play back audio for bg-bg cards
	playBackLabel   *widget.Label    // Label for back audio button
	stopButton      *ttwidget.Button
	statusLabel     *widget.Label
	phoneticLabel   *widget.Label

	audioFile       string
	audioFileBack   string // Back audio file for bg-bg cards
	isBgBg          bool   // Track if this is a bg-bg card
	isPlaying       bool
	playCmd         *exec.Cmd
	voiceInfo       string // Stores voice and speed info
	autoPlayEnabled *bool  // Pointer to parent's auto-play state
}

type audioCommandCandidate struct {
	name string
	args []string
}

// NewAudioPlayer creates a new audio player widget
func NewAudioPlayer() *AudioPlayer {
	p := &AudioPlayer{}

	// Create controls (tooltips will be set later after tooltip layer is created)
	p.playButton = ttwidget.NewButton("", p.onPlay)
	p.playButton.Icon = theme.MediaPlayIcon()

	p.playButtonLabel = widget.NewLabel("")
	p.playButtonLabel.TextStyle = fyne.TextStyle{Bold: true}

	p.playBackButton = ttwidget.NewButton("", p.onPlayBack)
	p.playBackButton.Icon = theme.MediaPlayIcon() // Same icon as front button

	p.playBackLabel = widget.NewLabel("")
	p.playBackLabel.TextStyle = fyne.TextStyle{Bold: true}

	p.stopButton = ttwidget.NewButton("", p.onStop)
	p.stopButton.Icon = theme.MediaStopIcon()

	p.statusLabel = widget.NewLabel("No audio loaded")

	// Create phonetic label — wraps so long IPA strings are never clipped.
	p.phoneticLabel = widget.NewLabel("")
	p.phoneticLabel.TextStyle = fyne.TextStyle{
		Bold:   true,
		Italic: true,
	}
	p.phoneticLabel.Wrapping = fyne.TextWrapWord

	// Initially disable controls
	p.playButton.Disable()
	p.playBackButton.Disable()
	p.playBackButton.Hide() // Only show for bg-bg cards
	p.playBackLabel.Hide()
	p.stopButton.Disable()

	// Layout: buttons on the left, status on the right, phonetic fills the middle.
	// Using NewBorder so the phonetic label expands horizontally rather than being
	// squeezed to its minimum width in an HBox.
	buttons := container.NewVBox(
		container.NewHBox(p.playButton, p.playButtonLabel),
		container.NewHBox(p.playBackButton, p.playBackLabel),
	)
	leftControls := container.NewHBox(buttons, p.stopButton)
	p.container = container.NewBorder(nil, nil, leftControls, p.statusLabel, p.phoneticLabel)

	p.ExtendBaseWidget(p)
	return p
}

// CreateRenderer implements fyne.Widget
func (p *AudioPlayer) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(p.container)
}

// SetAudioFile sets the audio file to play and optionally auto-plays it
func (p *AudioPlayer) SetAudioFile(audioFile string) {
	p.setAudioFileInternal(audioFile, true) // Enable auto-play
}

// SetAudioFileNoAutoPlay sets the audio file without auto-playing
// Used when regenerating audio on bg-bg cards - we only want to play when explicitly requested
func (p *AudioPlayer) SetAudioFileNoAutoPlay(audioFile string) {
	p.setAudioFileInternal(audioFile, false) // Disable auto-play
}

// setAudioFileInternal is the internal implementation for setting audio files
func (p *AudioPlayer) setAudioFileInternal(audioFile string, allowAutoPlay bool) {
	p.audioFile = audioFile
	p.isPlaying = false

	if audioFile != "" {
		p.playButton.Enable()

		// Try to load voice metadata
		wordDir := filepath.Dir(audioFile)
		metadataFile := filepath.Join(wordDir, "audio_metadata.txt")
		voice := ""
		speed := ""

		if data, err := os.ReadFile(metadataFile); err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "voice=") {
					voice = strings.TrimPrefix(line, "voice=")
				} else if strings.HasPrefix(line, "speed=") {
					speed = strings.TrimPrefix(line, "speed=")
				}
			}
		}

		// Store voice info
		if voice != "" && speed != "" {
			p.voiceInfo = fmt.Sprintf(" (voice: %s, speed: %s)", voice, speed)
		} else {
			p.voiceInfo = ""
		}

		// Update button label based on whether this is bg-bg
		if p.isBgBg {
			p.playButtonLabel.SetText("Front")
		} else {
			p.playButtonLabel.SetText("")
		}

		// Format status text with voice and speed info
		statusText := fmt.Sprintf("Audio: %s%s", filepath.Base(audioFile), p.voiceInfo)
		p.statusLabel.SetText(statusText)

		// Auto-play if enabled and allowed. AfterFunc fires the callback after
		// the UI has had a chance to render without blocking a goroutine.
		if allowAutoPlay && p.autoPlayEnabled != nil && *p.autoPlayEnabled {
			time.AfterFunc(100*time.Millisecond, func() {
				fyne.Do(p.onPlay)
			})
		}
	} else {
		p.Clear()
	}
}

// SetBackAudioFile sets the back audio file for bg-bg cards
func (p *AudioPlayer) SetBackAudioFile(audioFile string) {
	p.audioFileBack = audioFile
	if audioFile != "" {
		p.isBgBg = true
		p.playBackButton.Enable()
		p.playBackButton.Show()
		p.playBackLabel.SetText("Back")
		p.playBackLabel.Show()
		// Update front label now that we know it's bg-bg
		p.playButtonLabel.SetText("Front")
		// NOTE: Do NOT auto-play back audio
		// Back audio regeneration just prepares the file
		// User should press 'P' to listen to it
	} else {
		p.isBgBg = false
		p.playBackButton.Disable()
		p.playBackButton.Hide()
		p.playBackLabel.SetText("")
		p.playBackLabel.Hide()
		// Clear front label if not bg-bg
		p.playButtonLabel.SetText("")
	}
	// Refresh container to update layout after show/hide
	if p.container != nil {
		p.container.Refresh()
	}
}

// Clear clears the audio player
func (p *AudioPlayer) Clear() {
	p.onStop()
	p.audioFile = ""
	p.audioFileBack = ""
	p.isBgBg = false
	p.isPlaying = false
	p.voiceInfo = ""
	p.playButton.Disable()
	p.playBackButton.Disable()
	p.playBackButton.Hide()
	p.playButtonLabel.SetText("")
	p.playBackLabel.SetText("")
	p.playBackLabel.Hide()
	p.stopButton.Disable()
	p.statusLabel.SetText("No audio loaded")
	p.phoneticLabel.SetText("")
	// Refresh container to update layout after hiding back button
	if p.container != nil {
		p.container.Refresh()
	}
}

// SetPhonetic sets the phonetic transcription text
func (p *AudioPlayer) SetPhonetic(phonetic string) {
	p.phoneticLabel.SetText(phonetic)
	p.phoneticLabel.Refresh()
	// Also refresh the container to ensure layout updates
	if p.container != nil {
		p.container.Refresh()
	}
}

// SetAutoPlayEnabled sets the reference to the auto-play state
func (p *AudioPlayer) SetAutoPlayEnabled(autoPlayEnabled *bool) {
	p.autoPlayEnabled = autoPlayEnabled
}

// onPlay handles play button click
func (p *AudioPlayer) onPlay() {
	if p.audioFile == "" {
		return
	}

	if p.isPlaying {
		// Pause functionality - just stop for now
		p.onStop()
		return
	}

	// Start playing
	if err := p.startPlayback(); err != nil {
		p.statusLabel.SetText(fmt.Sprintf("Error: %v", err))
		return
	}

	p.isPlaying = true
	p.playButton.SetIcon(theme.MediaPauseIcon())
	p.stopButton.Enable()
	p.statusLabel.SetText(fmt.Sprintf("Playing: %s%s", filepath.Base(p.audioFile), p.voiceInfo))
}

// onPlayBack handles back audio button click (for bg-bg cards)
func (p *AudioPlayer) onPlayBack() {
	if p.audioFileBack == "" {
		return
	}

	if p.isPlaying {
		p.onStop()
	}

	// Start playback using back audio file directly
	if err := p.startPlaybackForFile(p.audioFileBack); err != nil {
		p.statusLabel.SetText(fmt.Sprintf("Error: %v", err))
		return
	}

	p.isPlaying = true
	p.playBackButton.SetIcon(theme.MediaPauseIcon()) // Back button, not front
	p.stopButton.Enable()
	p.statusLabel.SetText(fmt.Sprintf("Playing back audio: %s", filepath.Base(p.audioFileBack)))
}

// onStop handles stop button click
func (p *AudioPlayer) onStop() {
	if p.playCmd != nil && p.playCmd.Process != nil {
		if err := p.playCmd.Process.Kill(); err != nil {
			p.statusLabel.SetText(fmt.Sprintf("failed to stop playback: %v", err))
		}
		p.playCmd = nil
	}

	p.isPlaying = false
	// Set correct button icon based on which audio was playing
	if p.isBgBg && p.audioFileBack != "" {
		p.playBackButton.SetIcon(theme.MediaPlayIcon()) // Back button if it was playing
	} else {
		p.playButton.SetIcon(theme.MediaPlayIcon()) // Front button otherwise
	}
	p.stopButton.Disable()
	p.statusLabel.SetText(fmt.Sprintf("Stopped: %s%s", filepath.Base(p.audioFile), p.voiceInfo))
}

// Play triggers audio playback
func (p *AudioPlayer) Play() {
	if !p.playButton.Disabled() {
		fyne.Do(func() {
			p.onPlay()
		})
	}
}

// PlayBack triggers back audio playback (for bg-bg cards)
func (p *AudioPlayer) PlayBack() {
	if !p.playBackButton.Disabled() {
		fyne.Do(func() {
			p.onPlayBack()
		})
	}
}

// startPlayback starts audio playback using platform-specific commands
// This plays the front audio file (p.audioFile)
func (p *AudioPlayer) startPlayback() error {
	return p.startPlaybackForFile(p.audioFile)
}

// startPlaybackForFile starts playback of a specific audio file
// This allows playing either front or back audio without modifying state
func (p *AudioPlayer) startPlaybackForFile(audioFile string) error {
	cmd, err := audioPlaybackCommand(runtime.GOOS, audioFile, exec.LookPath)
	if err != nil {
		return err
	}

	// Store the command so we can stop it later
	p.playCmd = cmd

	// Start playback in background
	// Capture whether this is playing back audio or front audio for proper icon reset
	isPlayingBack := audioFile == p.audioFileBack
	go func() {
		err := cmd.Run()
		if err == nil {
			// Playback finished normally
			fyne.Do(func() {
				p.isPlaying = false
				// Reset correct button icon based on which audio was playing
				if isPlayingBack {
					p.playBackButton.SetIcon(theme.MediaPlayIcon())
					p.statusLabel.SetText(fmt.Sprintf("Finished: %s", filepath.Base(p.audioFileBack)))
				} else {
					p.playButton.SetIcon(theme.MediaPlayIcon())
					p.statusLabel.SetText(fmt.Sprintf("Finished: %s%s", filepath.Base(p.audioFile), p.voiceInfo))
				}
				p.stopButton.Disable()
			})
		}
	}()

	return nil
}

func audioPlaybackCommand(goos, audioFile string, lookPath func(string) (string, error)) (*exec.Cmd, error) {
	switch goos {
	case "darwin":
		return exec.Command("afplay", audioFile), nil
	case "linux":
		return linuxAudioPlaybackCommand(audioFile, lookPath)
	case "windows":
		return exec.Command("cmd", "/c", "start", "/min", audioFile), nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", goos)
	}
}

func linuxAudioPlaybackCommand(audioFile string, lookPath func(string) (string, error)) (*exec.Cmd, error) {
	candidates := linuxAudioCommandCandidates(audioFile)
	for _, candidate := range candidates {
		path, err := lookPath(candidate.name)
		if err != nil {
			continue
		}

		args := append([]string(nil), candidate.args...)
		return exec.Command(path, args...), nil
	}

	return nil, errors.New("no compatible audio player found. Install ffplay, sox, paplay, aplay, or mpg123 for mp3 files")
}

func linuxAudioCommandCandidates(audioFile string) []audioCommandCandidate {
	ext := strings.ToLower(filepath.Ext(audioFile))
	switch ext {
	case ".mp3":
		return []audioCommandCandidate{
			{name: "mpg123", args: []string{"-q", audioFile}},
			{name: "ffplay", args: []string{"-nodisp", "-autoexit", "-loglevel", "quiet", audioFile}},
			{name: "play", args: []string{"-q", audioFile}},
			{name: "paplay", args: []string{audioFile}},
			{name: "aplay", args: []string{"-q", audioFile}},
		}
	default:
		return []audioCommandCandidate{
			{name: "ffplay", args: []string{"-nodisp", "-autoexit", "-loglevel", "quiet", audioFile}},
			{name: "play", args: []string{"-q", audioFile}},
			{name: "paplay", args: []string{audioFile}},
			{name: "aplay", args: []string{"-q", audioFile}},
		}
	}
}
