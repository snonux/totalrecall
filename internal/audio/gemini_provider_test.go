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
		name      string
		config    *Config
		wantErr   bool
		wantModel string
		wantSpeed float64
	}{
		{
			name:    "missing google api key",
			config:  &Config{},
			wantErr: true,
		},
		{
			name: "valid config",
			config: &Config{
				GoogleAPIKey: "test-key",
			},
			wantErr:   false,
			wantModel: defaultGeminiTTSModel,
			wantSpeed: 1.0,
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

			geminiProvider, ok := provider.(*GeminiProvider)
			if !ok {
				t.Fatalf("NewGeminiProvider() returned %T, want *GeminiProvider", provider)
			}
			if geminiProvider.config.GeminiTTSModel != tt.wantModel {
				t.Fatalf("GeminiTTSModel = %q, want %q", geminiProvider.config.GeminiTTSModel, tt.wantModel)
			}
			if geminiProvider.config.GeminiSpeed != tt.wantSpeed {
				t.Fatalf("GeminiSpeed = %v, want %v", geminiProvider.config.GeminiSpeed, tt.wantSpeed)
			}
		})
	}
}

func TestGeminiProviderIsAvailable(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "with API key",
			config:  &Config{GoogleAPIKey: "test-key"},
			wantErr: false,
		},
		{
			name:    "without API key",
			config:  &Config{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &GeminiProvider{config: tt.config}
			err := provider.IsAvailable()
			if (err != nil) != tt.wantErr {
				t.Fatalf("IsAvailable() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
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

func TestExtractAudioDataErrors(t *testing.T) {
	tests := []struct {
		name      string
		response  *genai.GenerateContentResponse
		wantError string
	}{
		{
			name:      "nil response",
			response:  nil,
			wantError: "no response from Gemini",
		},
		{
			name: "response without audio",
			response: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					nil,
					{Content: nil},
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								nil,
								{InlineData: nil},
								{InlineData: &genai.Blob{Data: nil, MIMEType: "audio/pcm"}},
							},
						},
					},
				},
			},
			wantError: "no audio data returned from Gemini",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, mimeType, err := extractAudioData(tt.response)
			if err == nil {
				t.Fatal("extractAudioData() expected error")
			}
			if err.Error() != tt.wantError {
				t.Fatalf("extractAudioData() error = %q, want %q", err.Error(), tt.wantError)
			}
			if data != nil {
				t.Fatalf("extractAudioData() data = %v, want nil", data)
			}
			if mimeType != "" {
				t.Fatalf("extractAudioData() mimeType = %q, want empty string", mimeType)
			}
		})
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

func TestWriteGeminiAudioFileRejectsUnsupportedFormats(t *testing.T) {
	dir := t.TempDir()
	outputFile := filepath.Join(dir, "output.mp3")

	err := writeGeminiAudioFile(outputFile, []byte{0x11, 0x22}, "audio/pcm")
	if err == nil {
		t.Fatal("writeGeminiAudioFile() expected error for non-wav output")
	}

	if !strings.Contains(err.Error(), "only supports .wav output files") {
		t.Fatalf("writeGeminiAudioFile() error = %v, want unsupported-format message", err)
	}

	if _, statErr := os.Stat(outputFile); !os.IsNotExist(statErr) {
		t.Fatalf("expected no output file to be written, statErr=%v", statErr)
	}
}

func TestNewGeminiProviderWithGoogleAPIKey(t *testing.T) {
	apiKey := strings.TrimSpace(os.Getenv("GOOGLE_API_KEY"))
	if apiKey == "" {
		t.Skip("Skipping smoke test: GOOGLE_API_KEY not set")
	}

	provider, err := NewGeminiProvider(&Config{GoogleAPIKey: apiKey})
	if err != nil {
		t.Fatalf("NewGeminiProvider() unexpected error: %v", err)
	}

	geminiProvider, ok := provider.(*GeminiProvider)
	if !ok {
		t.Fatalf("NewGeminiProvider() returned %T, want *GeminiProvider", provider)
	}

	if geminiProvider.Name() != "gemini" {
		t.Fatalf("Name() = %q, want %q", geminiProvider.Name(), "gemini")
	}

	if err := geminiProvider.IsAvailable(); err != nil {
		t.Fatalf("IsAvailable() unexpected error with GOOGLE_API_KEY set: %v", err)
	}
}
