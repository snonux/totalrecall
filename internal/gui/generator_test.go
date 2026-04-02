package gui

import (
	"context"
	"errors"
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
	texts          []string
	outputFiles    []string
	lastText       string
	lastOutputFile string
	generateFunc   func(text, outputFile string) error
}

func (f *fakeAudioProvider) GenerateAudio(_ context.Context, text, outputFile string) error {
	f.generateCalls++
	f.texts = append(f.texts, text)
	f.outputFiles = append(f.outputFiles, outputFile)
	f.lastText = text
	f.lastOutputFile = outputFile
	if f.generateFunc != nil {
		return f.generateFunc(text, outputFile)
	}
	return nil
}

func (f *fakeAudioProvider) Name() string {
	return "fake-audio"
}

func (f *fakeAudioProvider) IsAvailable() error {
	return nil
}

func (f *fakeAudioProvider) Voices() []string {
	return nil
}

func (f *fakeAudioProvider) BuildAttribution(params audio.AttributionParams) string {
	return ""
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
			ImageProvider:       imageProviderNanoBanana,
			GoogleAPIKey:        "google-key",
			NanoBananaModel:     "custom-image-model",
			NanoBananaTextModel: "custom-text-model",
			OutputDir:           tempDir,
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
	if capturedConfig.Model != "custom-image-model" {
		t.Fatalf("Nano Banana model = %q, want %q", capturedConfig.Model, "custom-image-model")
	}
	if capturedConfig.TextModel != "custom-text-model" {
		t.Fatalf("Nano Banana text model = %q, want %q", capturedConfig.TextModel, "custom-text-model")
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
			GeminiTTSModel:    "sentinel-gemini-model",
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

	metadataData, err := os.ReadFile(filepath.Join(cardDir, "audio_metadata.txt"))
	if err != nil {
		t.Fatalf("expected metadata file: %v", err)
	}
	metadata := string(metadataData)
	if !strings.Contains(metadata, "model=gpt-4o-mini-tts") {
		t.Fatalf("openai metadata missing active model: %q", metadata)
	}
	if strings.Contains(metadata, "sentinel-gemini-model") {
		t.Fatalf("openai metadata should not use Gemini model when provider is OpenAI: %q", metadata)
	}
	if !strings.Contains(metadata, "cardtype=en-bg") {
		t.Fatalf("openai metadata missing card type: %q", metadata)
	}

	attrPath := audio.AttributionPath(outputPath)
	attributionData, err := os.ReadFile(attrPath)
	if err != nil {
		t.Fatalf("expected attribution file %q: %v", attrPath, err)
	}
	attribution := string(attributionData)
	if !strings.Contains(attribution, "Processed text sent to TTS: ябълка...") {
		t.Fatalf("openai attribution missing processed text: %q", attribution)
	}
}

func TestGenerateAudioUsesRandomGeminiVoiceAndAttribution(t *testing.T) {
	originalFactory := newAudioProvider
	t.Cleanup(func() {
		newAudioProvider = originalFactory
	})

	originalVoices := append([]string(nil), audio.GeminiVoices...)
	t.Cleanup(func() {
		audio.GeminiVoices = originalVoices
	})

	audio.GeminiVoices = []string{"sentinel-gemini-voice"}

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
			Provider:       "gemini",
			OutputDir:      tempDir,
			GoogleAPIKey:   "google-key",
			GeminiTTSModel: "gemini-2.5-flash-preview-tts",
			GeminiVoice:    "",
		},
	}

	outputPath, err := app.generateAudio(context.Background(), "ябълка", cardDir)
	if err != nil {
		t.Fatalf("generateAudio() unexpected error: %v", err)
	}

	if capturedConfig == nil {
		t.Fatal("expected audio provider config to be captured")
	}
	if capturedConfig.Provider != "gemini" {
		t.Fatalf("captured Provider = %q, want %q", capturedConfig.Provider, "gemini")
	}
	if capturedConfig.GeminiVoice != "sentinel-gemini-voice" {
		t.Fatalf("captured GeminiVoice = %q, want %q", capturedConfig.GeminiVoice, "sentinel-gemini-voice")
	}
	if capturedConfig.OutputFormat != "mp3" {
		t.Fatalf("captured OutputFormat = %q, want %q", capturedConfig.OutputFormat, "mp3")
	}
	if fakeProvider.generateCalls != 1 {
		t.Fatalf("GenerateAudio() calls = %d, want %d", fakeProvider.generateCalls, 1)
	}
	if !strings.HasSuffix(outputPath, "audio.mp3") {
		t.Fatalf("outputPath = %q, want an MP3 output file", outputPath)
	}

	attrPath := audio.AttributionPath(outputPath)
	attributionData, err := os.ReadFile(attrPath)
	if err != nil {
		t.Fatalf("expected attribution file %q: %v", attrPath, err)
	}
	attribution := string(attributionData)
	if !strings.Contains(attribution, "Audio generated by Google Gemini TTS") {
		t.Fatalf("gemini attribution missing header: %q", attribution)
	}
	if !strings.Contains(attribution, "sentinel-gemini-voice") {
		t.Fatalf("gemini attribution should use the selected random Gemini voice: %q", attribution)
	}
	if !strings.Contains(attribution, "Processed text sent to TTS: ябълка...") {
		t.Fatalf("gemini attribution missing processed text: %q", attribution)
	}

	metadataData, err := os.ReadFile(filepath.Join(cardDir, "audio_metadata.txt"))
	if err != nil {
		t.Fatalf("expected metadata file: %v", err)
	}
	metadata := string(metadataData)
	if !strings.Contains(metadata, "audio_file=audio.mp3") {
		t.Fatalf("gemini metadata missing fresh audio file reference: %q", metadata)
	}
	if !strings.Contains(metadata, "voice=sentinel-gemini-voice") {
		t.Fatalf("gemini metadata missing selected random voice: %q", metadata)
	}
	if !strings.Contains(metadata, "format=mp3") {
		t.Fatalf("gemini metadata missing format: %q", metadata)
	}
	if !strings.Contains(metadata, "cardtype=en-bg") {
		t.Fatalf("gemini metadata missing card type: %q", metadata)
	}
}

func TestGenerateGeminiAudioWithFallbacksRetriesAlternateVoice(t *testing.T) {
	originalFactory := newAudioProvider
	t.Cleanup(func() {
		newAudioProvider = originalFactory
	})

	originalVoices := append([]string(nil), audio.GeminiVoices...)
	t.Cleanup(func() {
		audio.GeminiVoices = originalVoices
	})
	audio.GeminiVoices = []string{"Charon", "Kore", "Leda"}

	var attemptedVoices []string
	newAudioProvider = func(config *audio.Config) (audio.Provider, error) {
		attemptedVoices = append(attemptedVoices, config.GeminiVoice)
		return &fakeAudioProvider{
			generateFunc: func(_ string, _ string) error {
				if config.GeminiVoice == "Charon" {
					return audio.ErrGeminiNoAudioData
				}
				return nil
			},
		}, nil
	}

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "audio.wav")
	app := &Application{
		config: &Config{
			OutputDir:   tempDir,
			AudioFormat: "wav",
		},
		audioConfig: &audio.Config{
			Provider:       "gemini",
			OutputDir:      tempDir,
			GoogleAPIKey:   "google-key",
			GeminiTTSModel: "gemini-2.5-flash-preview-tts",
		},
	}

	voice, err := audio.RunWithVoiceFallbacks("Charon", func(candidate string) error {
		return app.generateAudioFile(context.Background(), "ябълка", outputPath, candidate, 1.0)
	}, nil)
	if err != nil {
		t.Fatalf("RunWithVoiceFallbacks() unexpected error: %v", err)
	}

	if voice != "Kore" {
		t.Fatalf("final voice = %q, want %q", voice, "Kore")
	}
	if got, want := strings.Join(attemptedVoices, ","), "Charon,Kore"; got != want {
		t.Fatalf("attempted voices = %q, want %q", got, want)
	}
}

func TestGenerateAudioBgBgUsesSharedOpenAIVoices(t *testing.T) {
	originalFactory := newAudioProvider
	t.Cleanup(func() {
		newAudioProvider = originalFactory
	})

	originalVoices := append([]string(nil), audio.OpenAIVoices...)
	t.Cleanup(func() {
		audio.OpenAIVoices = originalVoices
	})

	audio.OpenAIVoices = []string{"sentinel-bg-gui-voice"}

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

	frontPath, backPath, err := app.generateAudioBgBg(context.Background(), "ябълка", "круша", cardDir)
	if err != nil {
		t.Fatalf("generateAudioBgBg() unexpected error: %v", err)
	}

	if capturedConfig == nil {
		t.Fatal("expected audio provider config to be captured")
	}
	if capturedConfig.OpenAIVoice != "sentinel-bg-gui-voice" {
		t.Fatalf("captured OpenAIVoice = %q, want %q", capturedConfig.OpenAIVoice, "sentinel-bg-gui-voice")
	}
	if fakeProvider.generateCalls != 2 {
		t.Fatalf("GenerateAudio() calls = %d, want %d", fakeProvider.generateCalls, 2)
	}
	if len(fakeProvider.outputFiles) != 2 {
		t.Fatalf("output file count = %d, want %d", len(fakeProvider.outputFiles), 2)
	}
	if !strings.HasSuffix(fakeProvider.outputFiles[0], "audio_front.mp3") {
		t.Fatalf("front output file = %q, want audio_front.mp3", fakeProvider.outputFiles[0])
	}
	if !strings.HasSuffix(fakeProvider.outputFiles[1], "audio_back.mp3") {
		t.Fatalf("back output file = %q, want audio_back.mp3", fakeProvider.outputFiles[1])
	}
	if !strings.HasSuffix(frontPath, "audio_front.mp3") {
		t.Fatalf("frontPath = %q, want audio_front.mp3", frontPath)
	}
	if !strings.HasSuffix(backPath, "audio_back.mp3") {
		t.Fatalf("backPath = %q, want audio_back.mp3", backPath)
	}

	metadataData, err := os.ReadFile(filepath.Join(cardDir, "audio_metadata.txt"))
	if err != nil {
		t.Fatalf("expected metadata file: %v", err)
	}
	metadata := string(metadataData)
	if !strings.Contains(metadata, "audio_file=audio_front.mp3") {
		t.Fatalf("bg-bg metadata missing front audio reference: %q", metadata)
	}
	if !strings.Contains(metadata, "audio_file_back=audio_back.mp3") {
		t.Fatalf("bg-bg metadata missing back audio reference: %q", metadata)
	}
	if !strings.Contains(metadata, "cardtype=bg-bg") {
		t.Fatalf("bg-bg metadata missing card type: %q", metadata)
	}

	for _, tc := range []struct {
		audioPath string
		wantText  string
	}{
		{audioPath: frontPath, wantText: "ябълка..."},
		{audioPath: backPath, wantText: "круша..."},
	} {
		attrPath := audio.AttributionPath(tc.audioPath)
		attributionData, err := os.ReadFile(attrPath)
		if err != nil {
			t.Fatalf("expected attribution file %q: %v", attrPath, err)
		}
		attribution := string(attributionData)
		if !strings.Contains(attribution, "Processed text sent to TTS: "+tc.wantText) {
			t.Fatalf("bg-bg attribution missing processed text %q: %q", tc.wantText, attribution)
		}
	}
}

func TestGenerateAudioFrontUsesSharedOpenAIVoices(t *testing.T) {
	originalFactory := newAudioProvider
	t.Cleanup(func() {
		newAudioProvider = originalFactory
	})

	originalVoices := append([]string(nil), audio.OpenAIVoices...)
	t.Cleanup(func() {
		audio.OpenAIVoices = originalVoices
	})

	audio.OpenAIVoices = []string{"sentinel-front-voice"}

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

	outputPath, err := app.generateAudioFront(context.Background(), "ябълка", cardDir)
	if err != nil {
		t.Fatalf("generateAudioFront() unexpected error: %v", err)
	}

	if capturedConfig == nil {
		t.Fatal("expected audio provider config to be captured")
	}
	if capturedConfig.OpenAIVoice != "sentinel-front-voice" {
		t.Fatalf("captured OpenAIVoice = %q, want %q", capturedConfig.OpenAIVoice, "sentinel-front-voice")
	}
	if fakeProvider.generateCalls != 1 {
		t.Fatalf("GenerateAudio() calls = %d, want %d", fakeProvider.generateCalls, 1)
	}
	if fakeProvider.lastText != "ябълка" {
		t.Fatalf("GenerateAudio() text = %q, want %q", fakeProvider.lastText, "ябълка")
	}
	if !strings.HasSuffix(fakeProvider.lastOutputFile, "audio_front.mp3") {
		t.Fatalf("GenerateAudio() output file = %q, want audio_front.mp3", fakeProvider.lastOutputFile)
	}
	if !strings.HasSuffix(outputPath, "audio_front.mp3") {
		t.Fatalf("outputPath = %q, want audio_front.mp3", outputPath)
	}

	attrPath := audio.AttributionPath(outputPath)
	attributionData, err := os.ReadFile(attrPath)
	if err != nil {
		t.Fatalf("expected attribution file %q: %v", attrPath, err)
	}
	attribution := string(attributionData)
	if !strings.Contains(attribution, "Audio generated by OpenAI TTS") {
		t.Fatalf("front attribution missing header: %q", attribution)
	}
	if !strings.Contains(attribution, "Voice: sentinel-front-voice") {
		t.Fatalf("front attribution missing voice: %q", attribution)
	}
}

func TestGenerateAudioBackUsesSharedOpenAIVoices(t *testing.T) {
	originalFactory := newAudioProvider
	t.Cleanup(func() {
		newAudioProvider = originalFactory
	})

	originalVoices := append([]string(nil), audio.OpenAIVoices...)
	t.Cleanup(func() {
		audio.OpenAIVoices = originalVoices
	})

	audio.OpenAIVoices = []string{"sentinel-back-voice"}

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

	outputPath, err := app.generateAudioBack(context.Background(), "круша", cardDir)
	if err != nil {
		t.Fatalf("generateAudioBack() unexpected error: %v", err)
	}

	if capturedConfig == nil {
		t.Fatal("expected audio provider config to be captured")
	}
	if capturedConfig.OpenAIVoice != "sentinel-back-voice" {
		t.Fatalf("captured OpenAIVoice = %q, want %q", capturedConfig.OpenAIVoice, "sentinel-back-voice")
	}
	if fakeProvider.generateCalls != 1 {
		t.Fatalf("GenerateAudio() calls = %d, want %d", fakeProvider.generateCalls, 1)
	}
	if fakeProvider.lastText != "круша" {
		t.Fatalf("GenerateAudio() text = %q, want %q", fakeProvider.lastText, "круша")
	}
	if !strings.HasSuffix(fakeProvider.lastOutputFile, "audio_back.mp3") {
		t.Fatalf("GenerateAudio() output file = %q, want audio_back.mp3", fakeProvider.lastOutputFile)
	}
	if !strings.HasSuffix(outputPath, "audio_back.mp3") {
		t.Fatalf("outputPath = %q, want audio_back.mp3", outputPath)
	}

	attrPath := audio.AttributionPath(outputPath)
	attributionData, err := os.ReadFile(attrPath)
	if err != nil {
		t.Fatalf("expected attribution file %q: %v", attrPath, err)
	}
	attribution := string(attributionData)
	if !strings.Contains(attribution, "Audio generated by OpenAI TTS") {
		t.Fatalf("back attribution missing header: %q", attribution)
	}
	if !strings.Contains(attribution, "Bulgarian word: круша") {
		t.Fatalf("back attribution missing spoken text: %q", attribution)
	}
}

func TestGenerateAudioProviderFactoryError(t *testing.T) {
	originalFactory := newAudioProvider
	t.Cleanup(func() {
		newAudioProvider = originalFactory
	})

	originalVoices := append([]string(nil), audio.OpenAIVoices...)
	t.Cleanup(func() {
		audio.OpenAIVoices = originalVoices
	})

	audio.OpenAIVoices = []string{"sentinel-error-voice"}
	newAudioProvider = func(*audio.Config) (audio.Provider, error) {
		return nil, errors.New("provider factory failed")
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

	_, err := app.generateAudioFront(context.Background(), "ябълка", cardDir)
	if err == nil {
		t.Fatal("generateAudioFront() expected error from provider factory")
	}
	if !strings.Contains(err.Error(), "provider factory failed") {
		t.Fatalf("generateAudioFront() error = %q, want it to contain %q", err.Error(), "provider factory failed")
	}
}
