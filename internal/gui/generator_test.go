package gui

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/snonux/totalrecall/internal/audio"
	"codeberg.org/snonux/totalrecall/internal/image"
)

type fakePromptAwareImageClient struct {
	searchOpts     *image.SearchOptions
	promptCallback func(string)
}

func (f *fakePromptAwareImageClient) Search(_ context.Context, opts *image.SearchOptions) ([]image.SearchResult, error) {
	copyOpts := *opts
	f.searchOpts = &copyOpts
	if f.promptCallback != nil {
		f.promptCallback("nanobanana prompt")
	}

	return []image.SearchResult{
		{
			ID:           "fake-id",
			URL:          "https://example.com/image.png",
			ThumbnailURL: "https://example.com/image.png",
			Width:        1,
			Height:       1,
			Description:  "fake result",
			Attribution:  "fake attribution",
			Source:       imageProviderNanoBanana,
		},
	}, nil
}

func (f *fakePromptAwareImageClient) Download(_ context.Context, _ string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("fake image bytes")), nil
}

func (f *fakePromptAwareImageClient) GetAttribution(*image.SearchResult) string {
	return "fake attribution"
}

func (f *fakePromptAwareImageClient) Name() string {
	return imageProviderNanoBanana
}

func (f *fakePromptAwareImageClient) SetPromptCallback(callback func(prompt string)) {
	f.promptCallback = callback
}

type fakeAudioProvider struct {
	generateCalls  int
	lastText       string
	lastOutputFile string
}

func (f *fakeAudioProvider) GenerateAudio(_ context.Context, text, outputFile string) error {
	f.generateCalls++
	f.lastText = text
	f.lastOutputFile = outputFile
	return nil
}

func (f *fakeAudioProvider) Name() string {
	return "fake-audio"
}

func (f *fakeAudioProvider) IsAvailable() error {
	return nil
}

func TestGenerateImagesWithPromptUsesNanoBananaProvider(t *testing.T) {
	originalNanoBananaClient := newNanoBananaImageClient
	originalOpenAIClient := newOpenAIImageClient
	t.Cleanup(func() {
		newNanoBananaImageClient = originalNanoBananaClient
		newOpenAIImageClient = originalOpenAIClient
	})

	fakeClient := &fakePromptAwareImageClient{}
	var capturedConfig *image.NanoBananaConfig

	newNanoBananaImageClient = func(config *image.NanoBananaConfig) promptAwareImageClient {
		capturedConfig = &image.NanoBananaConfig{
			APIKey:    config.APIKey,
			Model:     config.Model,
			TextModel: config.TextModel,
		}
		return fakeClient
	}
	newOpenAIImageClient = func(*image.OpenAIConfig) promptAwareImageClient {
		t.Fatal("unexpected OpenAI image client construction")
		return nil
	}

	tempDir := t.TempDir()
	app := &Application{
		config: &Config{
			ImageProvider: imageProviderNanoBanana,
			GoogleAPIKey:  "google-key",
			OutputDir:     tempDir,
		},
		currentWord: "друго",
	}

	outputPath, err := app.generateImagesWithPrompt(context.Background(), "ябълка", "custom prompt", "apple", tempDir)
	if err != nil {
		t.Fatalf("generateImagesWithPrompt() unexpected error: %v", err)
	}

	if capturedConfig == nil {
		t.Fatal("expected Nano Banana client constructor to be called")
	}
	if capturedConfig.APIKey != "google-key" {
		t.Fatalf("Nano Banana API key = %q, want %q", capturedConfig.APIKey, "google-key")
	}
	if fakeClient.searchOpts == nil {
		t.Fatal("expected search options to be captured")
	}
	if fakeClient.searchOpts.Query != "ябълка" {
		t.Fatalf("search query = %q, want %q", fakeClient.searchOpts.Query, "ябълка")
	}
	if fakeClient.searchOpts.CustomPrompt != "custom prompt" {
		t.Fatalf("custom prompt = %q, want %q", fakeClient.searchOpts.CustomPrompt, "custom prompt")
	}
	if fakeClient.searchOpts.Translation != "apple" {
		t.Fatalf("translation = %q, want %q", fakeClient.searchOpts.Translation, "apple")
	}

	promptPath := filepath.Join(tempDir, "image_prompt.txt")
	promptData, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("expected prompt file %q: %v", promptPath, err)
	}
	if got := strings.TrimSpace(string(promptData)); got != "nanobanana prompt" {
		t.Fatalf("prompt file = %q, want %q", got, "nanobanana prompt")
	}

	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("expected downloaded image at %q: %v", outputPath, err)
	}
	if !strings.HasSuffix(outputPath, ".png") {
		t.Fatalf("outputPath = %q, want a PNG output file", outputPath)
	}
}

func TestGenerateAudioUsesSharedOpenAIVoices(t *testing.T) {
	originalFactory := newAudioProvider
	t.Cleanup(func() {
		newAudioProvider = originalFactory
	})

	originalVoices := append([]string(nil), audio.OpenAIVoices...)
	t.Cleanup(func() {
		audio.OpenAIVoices = originalVoices
	})

	audio.OpenAIVoices = []string{"sentinel-gui-voice"}

	fakeProvider := &fakeAudioProvider{}
	var capturedConfig *audio.Config
	newAudioProvider = func(config *audio.Config) (audio.Provider, error) {
		copyConfig := *config
		capturedConfig = &copyConfig
		return fakeProvider, nil
	}

	tempDir := t.TempDir()
	cardDir := filepath.Join(tempDir, "card")
	if err := os.MkdirAll(cardDir, 0755); err != nil {
		t.Fatalf("failed to create card dir: %v", err)
	}

	app := &Application{
		config: &Config{
			OutputDir:   tempDir,
			AudioFormat: "mp3",
		},
		audioConfig: &audio.Config{
			Provider:          "openai",
			OutputDir:         tempDir,
			OpenAIModel:       "gpt-4o-mini-tts",
			OpenAIInstruction: "Speak clearly.",
		},
	}

	outputPath, err := app.generateAudio(context.Background(), "ябълка", cardDir)
	if err != nil {
		t.Fatalf("generateAudio() unexpected error: %v", err)
	}

	if capturedConfig == nil {
		t.Fatal("expected audio provider config to be captured")
	}
	if capturedConfig.OpenAIVoice != "sentinel-gui-voice" {
		t.Fatalf("captured OpenAIVoice = %q, want %q", capturedConfig.OpenAIVoice, "sentinel-gui-voice")
	}
	if fakeProvider.generateCalls != 1 {
		t.Fatalf("GenerateAudio() calls = %d, want %d", fakeProvider.generateCalls, 1)
	}
	if fakeProvider.lastText != "ябълка" {
		t.Fatalf("GenerateAudio() text = %q, want %q", fakeProvider.lastText, "ябълка")
	}
	if fakeProvider.lastOutputFile != outputPath {
		t.Fatalf("GenerateAudio() output file = %q, want %q", fakeProvider.lastOutputFile, outputPath)
	}
	if !strings.HasSuffix(outputPath, "audio.mp3") {
		t.Fatalf("outputPath = %q, want shared audio filename", outputPath)
	}
}
