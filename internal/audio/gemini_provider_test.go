package audio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/genai"
)

func TestNewGeminiProvider(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "missing google api key",
			config:  &Config{},
			wantErr: true,
		},
		{
			name: "valid config",
			config: &Config{
				GoogleAPIKey:   "test-key",
				GeminiTTSModel: "gemini-2.5-flash",
				GeminiVoice:    "Kore",
				GeminiSpeed:    1.0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewGeminiProvider(tt.config)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewGeminiProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}

			if provider.Name() != "gemini" {
				t.Fatalf("Name() = %q, want %q", provider.Name(), "gemini")
			}
		})
	}
}

func TestGeminiProviderIsAvailable(t *testing.T) {
	provider := &GeminiProvider{
		config: &Config{GoogleAPIKey: "test-key"},
	}

	if err := provider.IsAvailable(); err != nil {
		t.Fatalf("IsAvailable() unexpected error: %v", err)
	}

	provider.config.GoogleAPIKey = ""
	if err := provider.IsAvailable(); err == nil {
		t.Fatal("IsAvailable() expected error when API key is missing")
	}
}

func TestGeminiProviderBuildPrompt(t *testing.T) {
	provider := &GeminiProvider{
		config: &Config{
			GeminiVoice: "Kore",
			GeminiSpeed: 0.92,
		},
	}

	prompt := provider.buildPrompt("ябълка")

	for _, want := range []string{
		"Bulgarian language",
		"authentic Bulgarian phonetics",
		"Speak slowly and clearly for language learners.",
		"ябълка",
		"voice named Kore",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("buildPrompt() = %q, missing %q", prompt, want)
		}
	}
}

func TestExtractAudioData(t *testing.T) {
	response := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Parts: []*genai.Part{
						{
							InlineData: &genai.Blob{
								Data:     []byte{0x01, 0x02, 0x03},
								MIMEType: "audio/pcm",
							},
						},
					},
				},
			},
		},
	}

	data, mimeType, err := extractAudioData(response)
	if err != nil {
		t.Fatalf("extractAudioData() unexpected error: %v", err)
	}

	if mimeType != "audio/pcm" {
		t.Fatalf("extractAudioData() mimeType = %q, want %q", mimeType, "audio/pcm")
	}

	if len(data) != 3 || data[0] != 0x01 || data[1] != 0x02 || data[2] != 0x03 {
		t.Fatalf("extractAudioData() data = %v, want raw audio bytes", data)
	}
}

func TestWriteGeminiAudioFileWritesWAV(t *testing.T) {
	dir := t.TempDir()
	outputFile := filepath.Join(dir, "output.wav")
	pcmData := []byte{0x11, 0x22, 0x33, 0x44}

	if err := writeGeminiAudioFile(outputFile, pcmData, "audio/pcm"); err != nil {
		t.Fatalf("writeGeminiAudioFile() unexpected error: %v", err)
	}

	fileData, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("ReadFile() unexpected error: %v", err)
	}

	if !strings.HasPrefix(string(fileData[:4]), "RIFF") {
		t.Fatalf("output file does not look like WAV data: %q", fileData[:4])
	}

	if got, want := len(fileData), 44+len(pcmData); got != want {
		t.Fatalf("len(output) = %d, want %d", got, want)
	}
}
