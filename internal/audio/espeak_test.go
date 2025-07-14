package audio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateBulgarianText(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantErr bool
	}{
		{
			name:    "valid Bulgarian word",
			text:    "ябълка",
			wantErr: false,
		},
		{
			name:    "valid Bulgarian phrase",
			text:    "добър ден",
			wantErr: false,
		},
		{
			name:    "empty string",
			text:    "",
			wantErr: true,
		},
		{
			name:    "only Latin characters",
			text:    "apple",
			wantErr: true,
		},
		{
			name:    "mixed Cyrillic and Latin",
			text:    "ябълка apple",
			wantErr: false, // Contains at least one Cyrillic
		},
		{
			name:    "numbers only",
			text:    "12345",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBulgarianText(tt.text)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBulgarianText() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestListVoices(t *testing.T) {
	voices := ListVoices()
	
	if len(voices) == 0 {
		t.Error("ListVoices() returned empty slice")
	}
	
	// Check for expected voices
	expectedVoices := []string{"bg", "bg+m1", "bg+f1"}
	for _, expected := range expectedVoices {
		found := false
		for _, voice := range voices {
			if voice == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected voice %s not found in list", expected)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	if config == nil {
		t.Fatal("DefaultConfig() returned nil")
	}
	
	if config.Voice != "bg" {
		t.Errorf("Expected default voice 'bg', got '%s'", config.Voice)
	}
	
	if config.Speed != 150 {
		t.Errorf("Expected default speed 150, got %d", config.Speed)
	}
	
	if config.OutputDir != "./" {
		t.Errorf("Expected default output dir './', got '%s'", config.OutputDir)
	}
}

func TestNew(t *testing.T) {
	// This test will fail if espeak-ng is not installed
	// We'll skip it in that case
	espeak, err := New(nil)
	if err != nil {
		if checkESpeakInstalled() != nil {
			t.Skip("espeak-ng not installed, skipping test")
		}
		t.Fatalf("New() failed: %v", err)
	}
	
	if espeak == nil {
		t.Fatal("New() returned nil ESpeak instance")
	}
	
	if espeak.config == nil {
		t.Fatal("ESpeak instance has nil config")
	}
}

func TestSetSpeed(t *testing.T) {
	config := DefaultConfig()
	espeak := &ESpeak{config: config}
	
	tests := []struct {
		input    int
		expected int
	}{
		{150, 150},     // Normal speed
		{50, 80},       // Below minimum
		{500, 450},     // Above maximum
		{200, 200},     // Valid speed
	}
	
	for _, tt := range tests {
		espeak.SetSpeed(tt.input)
		if espeak.config.Speed != tt.expected {
			t.Errorf("SetSpeed(%d) resulted in speed %d, expected %d",
				tt.input, espeak.config.Speed, tt.expected)
		}
	}
}

func TestGenerateAudio_InvalidInput(t *testing.T) {
	// Skip if espeak-ng not installed
	if checkESpeakInstalled() != nil {
		t.Skip("espeak-ng not installed, skipping test")
	}
	
	espeak, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create ESpeak: %v", err)
	}
	
	// Test with empty text
	err = espeak.GenerateAudio("", "test.wav")
	if err == nil {
		t.Error("GenerateAudio() with empty text should return error")
	}
}

func TestGenerateAudio_Integration(t *testing.T) {
	// Skip if espeak-ng not installed
	if checkESpeakInstalled() != nil {
		t.Skip("espeak-ng not installed, skipping integration test")
	}
	
	// Create temporary directory
	tempDir := t.TempDir()
	
	config := &ESpeakConfig{
		Voice:     "bg",
		Speed:     150,
		OutputDir: tempDir,
	}
	
	espeak, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create ESpeak: %v", err)
	}
	
	// Generate audio file
	outputFile := filepath.Join(tempDir, "test.wav")
	err = espeak.GenerateAudio("ябълка", outputFile)
	if err != nil {
		t.Fatalf("GenerateAudio() failed: %v", err)
	}
	
	// Check if file was created
	info, err := os.Stat(outputFile)
	if err != nil {
		t.Fatalf("Output file not created: %v", err)
	}
	
	// Check file size (WAV file should have some content)
	if info.Size() == 0 {
		t.Error("Output file is empty")
	}
}