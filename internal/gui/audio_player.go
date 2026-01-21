package gui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

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

	container       *fyne.Container
	playButton      *ttwidget.Button
	playButtonLabel *widget.Label        // Label for front audio button
	playBackButton  *ttwidget.Button     // Play back audio for bg-bg cards
	playBackLabel   *widget.Label        // Label for back audio button
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

	// Create phonetic label
	p.phoneticLabel = widget.NewLabel("")
	p.phoneticLabel.TextStyle = fyne.TextStyle{
		Bold:   true,
		Italic: true,
	}

	// Initially disable controls
	p.playButton.Disable()
	p.playBackButton.Disable()
	p.playBackButton.Hide() // Only show for bg-bg cards
	p.playBackLabel.Hide()
	p.stopButton.Disable()

	// Create main container with phonetic display
	p.container = container.NewHBox(
		container.NewVBox(
			container.NewHBox(p.playButton, p.playButtonLabel),
			container.NewHBox(p.playBackButton, p.playBackLabel),
		),
		p.stopButton,
		p.phoneticLabel,
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

		// Auto-play if enabled and allowed
		if allowAutoPlay && p.autoPlayEnabled != nil && *p.autoPlayEnabled {
			// Small delay to ensure UI is ready
			go func() {
				// Wait a tiny bit for UI to be ready
				time.Sleep(100 * time.Millisecond)
				fyne.Do(func() {
					p.onPlay()
				})
			}()
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
	fmt.Printf("DEBUG (onPlay): Starting playback\n")
	fmt.Printf("  - audioFile: %s\n", p.audioFile)
	fmt.Printf("  - audioFileBack: %s\n", p.audioFileBack)
	fmt.Printf("  - isBgBg: %v\n", p.isBgBg)
	fmt.Printf("  - isPlaying: %v\n", p.isPlaying)
	
	if p.audioFile == "" {
		fmt.Printf("DEBUG (onPlay): No audioFile set, returning\n")
		return
	}

	if p.isPlaying {
		// Pause functionality - just stop for now
		fmt.Printf("DEBUG (onPlay): Already playing, stopping\n")
		p.onStop()
		return
	}

	// Start playing
	fmt.Printf("DEBUG (onPlay): About to start playback for: %s\n", filepath.Base(p.audioFile))
	if err := p.startPlayback(); err != nil {
		fmt.Printf("DEBUG (onPlay): Error starting playback: %v\n", err)
		p.statusLabel.SetText(fmt.Sprintf("Error: %v", err))
		return
	}

	p.isPlaying = true
	p.playButton.SetIcon(theme.MediaPauseIcon())
	p.stopButton.Enable()
	p.statusLabel.SetText(fmt.Sprintf("Playing: %s%s", filepath.Base(p.audioFile), p.voiceInfo))
	fmt.Printf("DEBUG (onPlay): Playback started successfully\n")
}

// onPlayBack handles back audio button click (for bg-bg cards)
func (p *AudioPlayer) onPlayBack() {
	fmt.Printf("DEBUG (onPlayBack): Starting back audio playback\n")
	fmt.Printf("  - audioFile: %s\n", p.audioFile)
	fmt.Printf("  - audioFileBack: %s\n", p.audioFileBack)
	fmt.Printf("  - isBgBg: %v\n", p.isBgBg)
	fmt.Printf("  - isPlaying: %v\n", p.isPlaying)
	
	if p.audioFileBack == "" {
		fmt.Printf("DEBUG (onPlayBack): No audioFileBack set, returning\n")
		return
	}

	if p.isPlaying {
		fmt.Printf("DEBUG (onPlayBack): Already playing, stopping first\n")
		p.onStop()
	}

	// Start playback using back audio file directly
	fmt.Printf("DEBUG (onPlayBack): About to start playback for back audio: %s\n", filepath.Base(p.audioFileBack))
	if err := p.startPlaybackForFile(p.audioFileBack); err != nil {
		fmt.Printf("DEBUG (onPlayBack): Error starting playback: %v\n", err)
		p.statusLabel.SetText(fmt.Sprintf("Error: %v", err))
		return
	}

	p.isPlaying = true
	p.playBackButton.SetIcon(theme.MediaPauseIcon())  // Back button, not front
	p.stopButton.Enable()
	p.statusLabel.SetText(fmt.Sprintf("Playing back audio: %s", filepath.Base(p.audioFileBack)))
	fmt.Printf("DEBUG (onPlayBack): Back audio playback started successfully\n")
}

// onStop handles stop button click
func (p *AudioPlayer) onStop() {
	if p.playCmd != nil && p.playCmd.Process != nil {
		p.playCmd.Process.Kill()
		p.playCmd = nil
	}

	p.isPlaying = false
	// Set correct button icon based on which audio was playing
	if p.isBgBg && p.audioFileBack != "" {
		p.playBackButton.SetIcon(theme.MediaPlayIcon())  // Back button if it was playing
	} else {
		p.playButton.SetIcon(theme.MediaPlayIcon())  // Front button otherwise
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
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin": // macOS
		cmd = exec.Command("afplay", audioFile)
	case "linux":
		// Try multiple commands in order of preference
		// mpg123 first since it handles MP3 files best
		if _, err := exec.LookPath("mpg123"); err == nil {
			cmd = exec.Command("mpg123", "-q", audioFile) // -q for quiet mode
		} else if _, err := exec.LookPath("ffplay"); err == nil {
			cmd = exec.Command("ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet", audioFile)
		} else if _, err := exec.LookPath("play"); err == nil {
			// SoX play command
			cmd = exec.Command("play", "-q", audioFile)
		} else if _, err := exec.LookPath("paplay"); err == nil {
			cmd = exec.Command("paplay", audioFile)
		} else if _, err := exec.LookPath("aplay"); err == nil {
			cmd = exec.Command("aplay", "-q", audioFile)
		} else {
			return fmt.Errorf("no audio player found. Install mpg123, ffplay, sox, paplay, or aplay")
		}
	case "windows":
		// Use Windows Media Player
		cmd = exec.Command("cmd", "/c", "start", "/min", audioFile)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
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
