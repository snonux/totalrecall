package gui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	ttwidget "github.com/dweymouth/fyne-tooltip/widget"
)

// AudioPlayer is a custom widget for playing audio files
type AudioPlayer struct {
	widget.BaseWidget

	container   *fyne.Container
	playButton  *ttwidget.Button
	stopButton  *ttwidget.Button
	statusLabel *widget.Label

	audioFile  string
	isPlaying  bool
	playCmd    *exec.Cmd
	voiceInfo  string // Stores voice and speed info
}

// NewAudioPlayer creates a new audio player widget
func NewAudioPlayer() *AudioPlayer {
	p := &AudioPlayer{}

	// Create controls with tooltips
	p.playButton = ttwidget.NewButton("", p.onPlay)
	p.playButton.Icon = theme.MediaPlayIcon()
	p.playButton.SetToolTip("Play audio (P)")

	p.stopButton = ttwidget.NewButton("", p.onStop)
	p.stopButton.Icon = theme.MediaStopIcon()
	p.stopButton.SetToolTip("Stop audio")

	p.statusLabel = widget.NewLabel("No audio loaded")

	// Initially disable controls
	p.playButton.Disable()
	p.stopButton.Disable()

	// Create main container
	p.container = container.NewHBox(
		p.playButton,
		p.stopButton,
		layout.NewSpacer(),
		p.statusLabel,
	)

	p.ExtendBaseWidget(p)
	return p
}

// CreateRenderer implements fyne.Widget
func (p *AudioPlayer) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(p.container)
}

// SetAudioFile sets the audio file to play
func (p *AudioPlayer) SetAudioFile(audioFile string) {
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
		
		// Format status text with voice and speed info
		statusText := fmt.Sprintf("Audio: %s%s", filepath.Base(audioFile), p.voiceInfo)
		p.statusLabel.SetText(statusText)
	} else {
		p.Clear()
	}
}

// Clear clears the audio player
func (p *AudioPlayer) Clear() {
	p.onStop() // Stop any playing audio
	p.audioFile = ""
	p.isPlaying = false
	p.voiceInfo = ""
	p.playButton.Disable()
	p.stopButton.Disable()
	p.statusLabel.SetText("No audio loaded")
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

// onStop handles stop button click
func (p *AudioPlayer) onStop() {
	if p.playCmd != nil && p.playCmd.Process != nil {
		p.playCmd.Process.Kill()
		p.playCmd = nil
	}

	p.isPlaying = false
	p.playButton.SetIcon(theme.MediaPlayIcon())
	p.stopButton.Disable()
	p.statusLabel.SetText(fmt.Sprintf("Stopped: %s%s", filepath.Base(p.audioFile), p.voiceInfo))
}

// Play triggers audio playback
func (p *AudioPlayer) Play() {
	if !p.playButton.Disabled() {
		p.onPlay()
	}
}

// startPlayback starts audio playback using platform-specific commands
func (p *AudioPlayer) startPlayback() error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin": // macOS
		cmd = exec.Command("afplay", p.audioFile)
	case "linux":
		// Try multiple commands in order of preference
		// mpg123 first since it handles MP3 files best
		if _, err := exec.LookPath("mpg123"); err == nil {
			cmd = exec.Command("mpg123", "-q", p.audioFile) // -q for quiet mode
		} else if _, err := exec.LookPath("ffplay"); err == nil {
			cmd = exec.Command("ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet", p.audioFile)
		} else if _, err := exec.LookPath("play"); err == nil {
			// SoX play command
			cmd = exec.Command("play", "-q", p.audioFile)
		} else if _, err := exec.LookPath("paplay"); err == nil {
			cmd = exec.Command("paplay", p.audioFile)
		} else if _, err := exec.LookPath("aplay"); err == nil {
			cmd = exec.Command("aplay", "-q", p.audioFile)
		} else {
			return fmt.Errorf("no audio player found. Install mpg123, ffplay, sox, paplay, or aplay")
		}
	case "windows":
		// Use Windows Media Player
		cmd = exec.Command("cmd", "/c", "start", "/min", p.audioFile)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	// Store the command so we can stop it later
	p.playCmd = cmd

	// Start playback in background
	go func() {
		err := cmd.Run()
		if err == nil {
			// Playback finished normally
			fyne.Do(func() {
				p.isPlaying = false
				p.playButton.SetIcon(theme.MediaPlayIcon())
				p.stopButton.Disable()
				p.statusLabel.SetText(fmt.Sprintf("Finished: %s%s", filepath.Base(p.audioFile), p.voiceInfo))
			})
		}
	}()

	return nil
}
