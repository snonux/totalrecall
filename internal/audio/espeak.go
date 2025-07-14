package audio

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ESpeakConfig holds configuration for espeak-ng audio generation
type ESpeakConfig struct {
	Voice     string // Voice variant (e.g., "bg", "bg+m1", "bg+f1")
	Speed     int    // Speech speed in words per minute (default: 150)
	Pitch     int    // Pitch adjustment, 0 to 99 (default: 50)
	Amplitude int    // Volume/amplitude, 0 to 200 (default: 100)
	WordGap   int    // Gap between words in 10ms units (default: 0)
	OutputDir string // Directory for output files
}

// DefaultConfig returns the default configuration for Bulgarian voice
func DefaultConfig() *ESpeakConfig {
	return &ESpeakConfig{
		Voice:     "bg",
		Speed:     150,
		Pitch:     50,
		Amplitude: 100,
		WordGap:   0,
		OutputDir: "./",
	}
}

// ESpeak provides an interface to the espeak-ng text-to-speech engine
type ESpeak struct {
	config *ESpeakConfig
}

// New creates a new ESpeak instance with the given configuration
func New(config *ESpeakConfig) (*ESpeak, error) {
	// Check if espeak-ng is installed
	if err := checkESpeakInstalled(); err != nil {
		return nil, err
	}
	
	if config == nil {
		config = DefaultConfig()
	}
	
	return &ESpeak{config: config}, nil
}

// GenerateAudio generates an audio file for the given Bulgarian text
func (e *ESpeak) GenerateAudio(text string, outputFile string) error {
	// Validate input
	if text == "" {
		return fmt.Errorf("text cannot be empty")
	}
	
	// Ensure output directory exists
	dir := filepath.Dir(outputFile)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}
	
	// Build espeak-ng command
	args := []string{
		"-v", e.config.Voice,  // Voice selection
		"-s", fmt.Sprintf("%d", e.config.Speed),  // Speed
		"-p", fmt.Sprintf("%d", e.config.Pitch),  // Pitch
		"-a", fmt.Sprintf("%d", e.config.Amplitude),  // Amplitude/volume
	}
	
	// Add word gap if specified
	if e.config.WordGap > 0 {
		args = append(args, "-g", fmt.Sprintf("%d", e.config.WordGap))
	}
	
	// Add output file and text
	args = append(args, "-w", outputFile, text)
	
	cmd := exec.Command("espeak-ng", args...)
	
	// Run the command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("espeak-ng failed: %w\nOutput: %s", err, string(output))
	}
	
	return nil
}

// SetVoice updates the voice variant
func (e *ESpeak) SetVoice(voice string) {
	e.config.Voice = voice
}

// SetSpeed updates the speech speed
func (e *ESpeak) SetSpeed(speed int) {
	if speed < 80 {
		speed = 80
	} else if speed > 450 {
		speed = 450
	}
	e.config.Speed = speed
}

// SetPitch updates the pitch (0-99, 50 is default)
func (e *ESpeak) SetPitch(pitch int) {
	if pitch < 0 {
		pitch = 0
	} else if pitch > 99 {
		pitch = 99
	}
	e.config.Pitch = pitch
}

// SetAmplitude updates the volume/amplitude (0-200, 100 is default)
func (e *ESpeak) SetAmplitude(amplitude int) {
	if amplitude < 0 {
		amplitude = 0
	} else if amplitude > 200 {
		amplitude = 200
	}
	e.config.Amplitude = amplitude
}

// SetWordGap updates the gap between words in 10ms units
func (e *ESpeak) SetWordGap(gap int) {
	if gap < 0 {
		gap = 0
	}
	e.config.WordGap = gap
}

// checkESpeakInstalled verifies that espeak-ng is available on the system
func checkESpeakInstalled() error {
	cmd := exec.Command("espeak-ng", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("espeak-ng is not installed or not in PATH: %w", err)
	}
	return nil
}

// ValidateBulgarianText performs basic validation of Bulgarian text
func ValidateBulgarianText(text string) error {
	if text == "" {
		return fmt.Errorf("text cannot be empty")
	}
	
	// Check if text contains at least one Cyrillic character
	hasCyrillic := false
	for _, r := range text {
		// Bulgarian Cyrillic range
		if (r >= 'А' && r <= 'я') || r == 'Ё' || r == 'ё' {
			hasCyrillic = true
			break
		}
	}
	
	if !hasCyrillic {
		return fmt.Errorf("text must contain Bulgarian Cyrillic characters")
	}
	
	return nil
}

// ListVoices returns available Bulgarian voice variants
func ListVoices() []string {
	return []string{
		"bg",      // Default Bulgarian voice
		"bg+m1",   // Bulgarian male voice 1
		"bg+m2",   // Bulgarian male voice 2
		"bg+m3",   // Bulgarian male voice 3
		"bg+f1",   // Bulgarian female voice 1
		"bg+f2",   // Bulgarian female voice 2
		"bg+f3",   // Bulgarian female voice 3
	}
}

// ConvertWAVToMP3 converts a WAV file to MP3 using ffmpeg
func ConvertWAVToMP3(wavFile, mp3File string) error {
	// Check if ffmpeg is installed
	if err := exec.Command("ffmpeg", "-version").Run(); err != nil {
		return fmt.Errorf("ffmpeg is not installed or not in PATH: %w", err)
	}
	
	cmd := exec.Command("ffmpeg", "-i", wavFile, "-acodec", "mp3", "-y", mp3File)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg conversion failed: %w\nOutput: %s", err, string(output))
	}
	
	return nil
}

// GenerateMP3 generates an MP3 file for the given Bulgarian text
func (e *ESpeak) GenerateMP3(text string, outputFile string) error {
	// Generate temporary WAV file
	tempWAV := strings.TrimSuffix(outputFile, filepath.Ext(outputFile)) + "_temp.wav"
	
	// Generate WAV
	if err := e.GenerateAudio(text, tempWAV); err != nil {
		return err
	}
	
	// Convert to MP3
	if err := ConvertWAVToMP3(tempWAV, outputFile); err != nil {
		// Clean up temporary file
		os.Remove(tempWAV)
		return err
	}
	
	// Clean up temporary file
	return os.Remove(tempWAV)
}