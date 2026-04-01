package processor

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/snonux/totalrecall/internal/audio"
	"codeberg.org/snonux/totalrecall/internal/cli"
	"codeberg.org/snonux/totalrecall/internal/gui"
	"codeberg.org/snonux/totalrecall/internal/image"
	"codeberg.org/snonux/totalrecall/internal/phonetic"
	"github.com/spf13/viper"
)

type stubImageSearcher struct {
	lastPrompt string
	searchErr  error
}

func (s *stubImageSearcher) Search(ctx context.Context, opts *image.SearchOptions) ([]image.SearchResult, error) {
	if s.searchErr != nil {
		return nil, s.searchErr
	}

	s.lastPrompt = "stub nanobanana prompt"
	return []image.SearchResult{
		{
			ID:           "stub-image",
			URL:          "data:image/png;base64,AAAA",
			ThumbnailURL: "data:image/png;base64,AAAA",
			Width:        1,
			Height:       1,
			Description:  "stub image",
			Attribution:  "stub attribution",
			Source:       "nanobanana",
		},
	}, nil
}

func (s *stubImageSearcher) Download(ctx context.Context, url string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("mock image data")), nil
}

func (s *stubImageSearcher) GetAttribution(result *image.SearchResult) string {
	return result.Attribution
}

func (s *stubImageSearcher) Name() string {
	return "nanobanana"
}

func (s *stubImageSearcher) GetLastPrompt() string {
	return s.lastPrompt
}

type fakeAudioProvider struct {
	generateCalls  int
	texts          []string
	outputFiles    []string
	lastText       string
	lastOutputFile string
}

func (f *fakeAudioProvider) GenerateAudio(_ context.Context, text, outputFile string) error {
	f.generateCalls++
	f.texts = append(f.texts, text)
	f.outputFiles = append(f.outputFiles, outputFile)
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

func TestNewProcessor(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-openai-key")
	t.Setenv("GOOGLE_API_KEY", "test-google-key")

	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()
	viper.Reset()

	flags := cli.NewFlags()
	p := NewProcessor(flags)

	if p == nil {
		t.Fatal("NewProcessor returned nil")
	}

	if p.flags != flags {
		t.Error("Processor flags not set correctly")
	}

	if p.translator == nil {
		t.Error("Translator not initialized")
	}

	if p.translationCache == nil {
		t.Error("Translation cache not initialized")
	}

	if p.phoneticFetcher == nil {
		t.Error("Phonetic fetcher not initialized")
	}
}

func TestNewProcessor_DefaultPhoneticProviderUsesOpenAI(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")

	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()
	viper.Reset()

	flags := cli.NewFlags()
	p := NewProcessor(flags)

	if got := p.phoneticFetcher.Provider(); got != phonetic.ProviderOpenAI {
		t.Fatalf("expected default phonetic provider %q, got %q", phonetic.ProviderOpenAI, got)
	}
}

func TestNewProcessor_ExplicitGeminiPhoneticProvider(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")

	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()
	viper.Reset()
	viper.Set("phonetic.provider", "gemini")

	flags := cli.NewFlags()
	p := NewProcessor(flags)

	if got := p.phoneticFetcher.Provider(); got != phonetic.ProviderGemini {
		t.Fatalf("expected gemini phonetic provider %q, got %q", phonetic.ProviderGemini, got)
	}
}

func TestNewProcessor_DefaultTranslationProviderUsesOpenAI(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")

	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()
	viper.Reset()

	flags := cli.NewFlags()
	p := NewProcessor(flags)

	_, err := p.translator.TranslateWord("ябълка")
	if err == nil {
		t.Fatal("Expected error for missing OpenAI API key")
	}
	if err.Error() != "OpenAI API key not found" {
		t.Fatalf("Expected OpenAI default provider error, got: %v", err)
	}
}

func TestNewProcessor_ExplicitGeminiTranslationProvider(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")

	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()
	viper.Reset()
	viper.Set("translation.provider", "gemini")

	flags := cli.NewFlags()
	p := NewProcessor(flags)

	_, err := p.translator.TranslateWord("ябълка")
	if err == nil {
		t.Fatal("Expected error for missing Google API key")
	}
	if err.Error() != "Google API key not found" {
		t.Fatalf("Expected Gemini provider error, got: %v", err)
	}
}

func TestGUIConfigForRunModeUsesNanoBananaDefaultWhenImageAPIIsNotSpecified(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-openai-key")
	t.Setenv("GOOGLE_API_KEY", "test-google-key")

	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()
	viper.Reset()

	flags := cli.NewFlags()
	flags.AudioFormat = "wav"
	flags.ImageAPI = "openai"
	flags.ImageAPISpecified = false
	p := NewProcessor(flags)

	guiConfig := p.guiConfigForRunMode()
	if guiConfig.ImageProvider != gui.DefaultConfig().ImageProvider {
		t.Fatalf("guiConfig.ImageProvider = %q, want GUI default %q", guiConfig.ImageProvider, gui.DefaultConfig().ImageProvider)
	}
	if guiConfig.AudioFormat != "wav" {
		t.Fatalf("guiConfig.AudioFormat = %q, want %q", guiConfig.AudioFormat, "wav")
	}
	if guiConfig.GoogleAPIKey != "test-google-key" {
		t.Fatalf("guiConfig.GoogleAPIKey = %q, want %q", guiConfig.GoogleAPIKey, "test-google-key")
	}
}

func TestGUIConfigForRunModeHonorsExplicitImageAPI(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-openai-key")
	t.Setenv("GOOGLE_API_KEY", "test-google-key")

	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()
	viper.Reset()

	flags := cli.NewFlags()
	flags.ImageAPI = "openai"
	flags.ImageAPISpecified = true
	p := NewProcessor(flags)

	guiConfig := p.guiConfigForRunMode()
	if guiConfig.ImageProvider != "openai" {
		t.Fatalf("guiConfig.ImageProvider = %q, want %q", guiConfig.ImageProvider, "openai")
	}
	if guiConfig.GoogleAPIKey != "test-google-key" {
		t.Fatalf("guiConfig.GoogleAPIKey = %q, want %q", guiConfig.GoogleAPIKey, "test-google-key")
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

	audio.OpenAIVoices = []string{"sentinel-openai-voice"}

	fakeProvider := &fakeAudioProvider{}
	var capturedConfig *audio.Config
	newAudioProvider = func(config *audio.Config) (audio.Provider, error) {
		copyConfig := *config
		capturedConfig = &copyConfig
		return fakeProvider, nil
	}

	tempDir := t.TempDir()
	flags := cli.NewFlags()
	flags.OutputDir = tempDir
	flags.AudioFormat = "mp3"
	flags.AllVoices = true
	flags.AudioProvider = "openai"

	p := NewProcessor(flags)

	if err := p.generateAudio("ябълка"); err != nil {
		t.Fatalf("generateAudio() unexpected error: %v", err)
	}

	if capturedConfig == nil {
		t.Fatal("expected audio provider config to be captured")
	}
	if capturedConfig.OpenAIVoice != "sentinel-openai-voice" {
		t.Fatalf("captured OpenAIVoice = %q, want %q", capturedConfig.OpenAIVoice, "sentinel-openai-voice")
	}
	if fakeProvider.generateCalls != 1 {
		t.Fatalf("GenerateAudio() calls = %d, want %d", fakeProvider.generateCalls, 1)
	}
	if fakeProvider.lastText != "ябълка" {
		t.Fatalf("GenerateAudio() text = %q, want %q", fakeProvider.lastText, "ябълка")
	}
	if !strings.HasSuffix(fakeProvider.lastOutputFile, "audio_sentinel-openai-voice.mp3") {
		t.Fatalf("GenerateAudio() output file = %q, want shared voice name in filename", fakeProvider.lastOutputFile)
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

	audio.OpenAIVoices = []string{"sentinel-bg-voice"}

	fakeProvider := &fakeAudioProvider{}
	var capturedConfig *audio.Config
	newAudioProvider = func(config *audio.Config) (audio.Provider, error) {
		copyConfig := *config
		capturedConfig = &copyConfig
		return fakeProvider, nil
	}

	tempDir := t.TempDir()
	flags := cli.NewFlags()
	flags.OutputDir = tempDir
	flags.AudioFormat = "mp3"
	flags.AudioProvider = "openai"

	p := NewProcessor(flags)
	if err := p.generateAudioBgBg("ябълка", "круша"); err != nil {
		t.Fatalf("generateAudioBgBg() unexpected error: %v", err)
	}

	if capturedConfig == nil {
		t.Fatal("expected audio provider config to be captured")
	}
	if capturedConfig.OpenAIVoice != "sentinel-bg-voice" {
		t.Fatalf("captured OpenAIVoice = %q, want %q", capturedConfig.OpenAIVoice, "sentinel-bg-voice")
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

	audio.OpenAIVoices = []string{"sentinel-failure-voice"}
	newAudioProvider = func(*audio.Config) (audio.Provider, error) {
		return nil, errors.New("provider factory failed")
	}

	tempDir := t.TempDir()
	flags := cli.NewFlags()
	flags.OutputDir = tempDir
	flags.AudioFormat = "mp3"
	flags.AudioProvider = "openai"

	p := NewProcessor(flags)
	err := p.generateAudio("ябълка")
	if err == nil {
		t.Fatal("generateAudio() expected error from provider factory")
	}
	if !strings.Contains(err.Error(), "provider factory failed") {
		t.Fatalf("generateAudio() error = %q, want it to contain %q", err.Error(), "provider factory failed")
	}
}

func TestGenerateAudioUsesConfiguredGeminiVoiceAndModel(t *testing.T) {
	originalFactory := newAudioProvider
	t.Cleanup(func() {
		newAudioProvider = originalFactory
	})

	fakeProvider := &fakeAudioProvider{}
	var capturedConfig *audio.Config
	newAudioProvider = func(config *audio.Config) (audio.Provider, error) {
		copyConfig := *config
		capturedConfig = &copyConfig
		return fakeProvider, nil
	}

	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()
	viper.Reset()
	viper.Set("audio.provider", "gemini")
	viper.Set("audio.gemini_tts_model", "gemini-2.5-flash-preview-tts")
	viper.Set("audio.gemini_voice", "Kore")

	flags := cli.NewFlags()
	flags.OutputDir = t.TempDir()
	flags.AudioFormat = "mp3"

	p := NewProcessor(flags)
	if err := p.generateAudio("ябълка"); err != nil {
		t.Fatalf("generateAudio() unexpected error: %v", err)
	}

	if capturedConfig == nil {
		t.Fatal("expected audio provider config to be captured")
	}
	if capturedConfig.Provider != "gemini" {
		t.Fatalf("captured provider = %q, want %q", capturedConfig.Provider, "gemini")
	}
	if capturedConfig.GeminiTTSModel != "gemini-2.5-flash-preview-tts" {
		t.Fatalf("captured GeminiTTSModel = %q, want %q", capturedConfig.GeminiTTSModel, "gemini-2.5-flash-preview-tts")
	}
	if capturedConfig.GeminiVoice != "Kore" {
		t.Fatalf("captured GeminiVoice = %q, want %q", capturedConfig.GeminiVoice, "Kore")
	}
	if capturedConfig.OutputFormat != "wav" {
		t.Fatalf("captured OutputFormat = %q, want %q", capturedConfig.OutputFormat, "wav")
	}
	if fakeProvider.generateCalls != 1 {
		t.Fatalf("GenerateAudio() calls = %d, want %d", fakeProvider.generateCalls, 1)
	}
	if !strings.HasSuffix(fakeProvider.lastOutputFile, "audio.wav") {
		t.Fatalf("GenerateAudio() output file = %q, want wav output", fakeProvider.lastOutputFile)
	}
}

func TestDownloadImagesWithTranslationUsesNanoBananaConfigAndSavesPrompt(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-openai-key")
	t.Setenv("GOOGLE_API_KEY", "test-google-key")

	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()
	viper.Reset()
	viper.Set("image.nanobanana_model", "custom-image-model")
	viper.Set("image.nanobanana_text_model", "custom-text-model")

	originalConstructor := newNanoBananaImageClient
	stubSearcher := &stubImageSearcher{}
	capturedConfig := new(image.NanoBananaConfig)
	newNanoBananaImageClient = func(config *image.NanoBananaConfig) image.ImageSearcher {
		*capturedConfig = *config
		return stubSearcher
	}
	t.Cleanup(func() {
		newNanoBananaImageClient = originalConstructor
	})

	flags := cli.NewFlags()
	flags.OutputDir = t.TempDir()
	flags.ImageAPI = "nanobanana"
	flags.ImageAPISpecified = true

	p := NewProcessor(flags)
	if err := p.downloadImagesWithTranslation("ябълка", "apple"); err != nil {
		t.Fatalf("downloadImagesWithTranslation() unexpected error: %v", err)
	}

	if capturedConfig.APIKey != "test-google-key" {
		t.Fatalf("NanoBanana APIKey = %q, want %q", capturedConfig.APIKey, "test-google-key")
	}
	if capturedConfig.Model != "custom-image-model" {
		t.Fatalf("NanoBanana Model = %q, want %q", capturedConfig.Model, "custom-image-model")
	}
	if capturedConfig.TextModel != "custom-text-model" {
		t.Fatalf("NanoBanana TextModel = %q, want %q", capturedConfig.TextModel, "custom-text-model")
	}

	wordDir := p.findCardDirectory("ябълка")
	if wordDir == "" {
		t.Fatal("expected word directory to be created")
	}

	promptData, err := os.ReadFile(filepath.Join(wordDir, "image_prompt.txt"))
	if err != nil {
		t.Fatalf("expected prompt file: %v", err)
	}
	if got := strings.TrimSpace(string(promptData)); got != stubSearcher.GetLastPrompt() {
		t.Fatalf("prompt file = %q, want %q", got, stubSearcher.GetLastPrompt())
	}
}

func TestDownloadImagesWithTranslationUsesConfiguredNanoBananaWhenImageAPINotSpecified(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-openai-key")
	t.Setenv("GOOGLE_API_KEY", "test-google-key")

	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()
	viper.Reset()
	viper.Set("image.provider", "nanobanana")
	viper.Set("image.nanobanana_model", "config-image-model")
	viper.Set("image.nanobanana_text_model", "config-text-model")

	originalConstructor := newNanoBananaImageClient
	stubSearcher := &stubImageSearcher{}
	capturedConfig := new(image.NanoBananaConfig)
	newNanoBananaImageClient = func(config *image.NanoBananaConfig) image.ImageSearcher {
		*capturedConfig = *config
		return stubSearcher
	}
	t.Cleanup(func() {
		newNanoBananaImageClient = originalConstructor
	})

	flags := cli.NewFlags()
	flags.OutputDir = t.TempDir()
	flags.ImageAPI = "openai"
	flags.ImageAPISpecified = false

	p := NewProcessor(flags)
	if err := p.downloadImagesWithTranslation("ябълка", "apple"); err != nil {
		t.Fatalf("downloadImagesWithTranslation() unexpected error: %v", err)
	}

	if capturedConfig.APIKey != "test-google-key" {
		t.Fatalf("NanoBanana APIKey = %q, want %q", capturedConfig.APIKey, "test-google-key")
	}
	if capturedConfig.Model != "config-image-model" {
		t.Fatalf("NanoBanana Model = %q, want %q", capturedConfig.Model, "config-image-model")
	}
	if capturedConfig.TextModel != "config-text-model" {
		t.Fatalf("NanoBanana TextModel = %q, want %q", capturedConfig.TextModel, "config-text-model")
	}

	wordDir := p.findCardDirectory("ябълка")
	if wordDir == "" {
		t.Fatal("expected word directory to be created")
	}

	promptData, err := os.ReadFile(filepath.Join(wordDir, "image_prompt.txt"))
	if err != nil {
		t.Fatalf("expected prompt file: %v", err)
	}
	if got := strings.TrimSpace(string(promptData)); got != stubSearcher.GetLastPrompt() {
		t.Fatalf("prompt file = %q, want %q", got, stubSearcher.GetLastPrompt())
	}
}

func TestNewImageSearcherRejectsUnknownConfiguredProvider(t *testing.T) {
	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()
	viper.Reset()
	viper.Set("image.provider", "not-a-real-provider")

	flags := cli.NewFlags()
	flags.ImageAPI = "openai"
	flags.ImageAPISpecified = false
	p := NewProcessor(flags)

	_, err := p.newImageSearcher()
	if err == nil {
		t.Fatal("expected error for unknown configured provider")
	}
	if got := err.Error(); got != "unknown image provider: not-a-real-provider" {
		t.Fatalf("newImageSearcher() error = %q, want %q", got, "unknown image provider: not-a-real-provider")
	}
}

func TestNewImageSearcherConfiguredNanoBananaRequiresGoogleAPIKey(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "test-openai-key")

	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()
	viper.Reset()
	viper.Set("image.provider", "nanobanana")

	flags := cli.NewFlags()
	flags.ImageAPI = "openai"
	flags.ImageAPISpecified = false
	p := NewProcessor(flags)

	_, err := p.newImageSearcher()
	if err == nil {
		t.Fatal("expected error when Google API key is missing for Nano Banana")
	}
	if got := err.Error(); got != "Google API key is required for image generation" {
		t.Fatalf("newImageSearcher() error = %q, want %q", got, "Google API key is required for image generation")
	}
}

func TestNewNanoBananaImageSearcherExplicitDefaultWinsOverConfig(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-openai-key")
	t.Setenv("GOOGLE_API_KEY", "test-google-key")

	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()
	viper.Reset()
	viper.Set("image.nanobanana_model", "config-image-model")
	viper.Set("image.nanobanana_text_model", "config-text-model")

	originalConstructor := newNanoBananaImageClient
	capturedConfig := new(image.NanoBananaConfig)
	newNanoBananaImageClient = func(config *image.NanoBananaConfig) image.ImageSearcher {
		*capturedConfig = *config
		return &stubImageSearcher{}
	}
	t.Cleanup(func() {
		newNanoBananaImageClient = originalConstructor
	})

	flags := cli.NewFlags()
	flags.OutputDir = t.TempDir()
	flags.ImageAPI = "nanobanana"
	flags.ImageAPISpecified = true
	flags.NanoBananaModel = image.DefaultNanoBananaModel
	flags.NanoBananaModelSpecified = true
	flags.NanoBananaTextModel = image.DefaultNanoBananaTextModel
	flags.NanoBananaTextModelSpecified = true

	p := NewProcessor(flags)
	searcher, err := p.newNanoBananaImageSearcher()
	if err != nil {
		t.Fatalf("newNanoBananaImageSearcher() unexpected error: %v", err)
	}
	if searcher == nil {
		t.Fatal("expected searcher")
	}

	if capturedConfig.Model != image.DefaultNanoBananaModel {
		t.Fatalf("NanoBanana Model = %q, want explicit CLI default %q", capturedConfig.Model, image.DefaultNanoBananaModel)
	}
	if capturedConfig.TextModel != image.DefaultNanoBananaTextModel {
		t.Fatalf("NanoBanana TextModel = %q, want explicit CLI default %q", capturedConfig.TextModel, image.DefaultNanoBananaTextModel)
	}
}

func TestProcessSingleWord_InvalidWord(t *testing.T) {
	flags := cli.NewFlags()
	flags.OutputDir = t.TempDir()
	p := NewProcessor(flags)

	// Test with non-Bulgarian text
	err := p.ProcessSingleWord("hello")
	if err == nil {
		t.Error("Expected error for non-Bulgarian word")
	}

	// Test with empty string
	err = p.ProcessSingleWord("")
	if err == nil {
		t.Error("Expected error for empty word")
	}
}

func TestProcessSingleWord_ValidWord(t *testing.T) {
	// Skip if no OpenAI API key
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("Skipping test: OPENAI_API_KEY must be set")
	}

	flags := cli.NewFlags()
	flags.OutputDir = t.TempDir()
	flags.SkipAudio = true
	flags.SkipImages = true
	p := NewProcessor(flags)

	err := p.ProcessSingleWord("ябълка")
	if err != nil {
		t.Errorf("ProcessSingleWord failed: %v", err)
	}

	// Check that output directory was created
	if _, err := os.Stat(flags.OutputDir); os.IsNotExist(err) {
		t.Error("Output directory was not created")
	}
}

func TestProcessBatch_InvalidFile(t *testing.T) {
	flags := cli.NewFlags()
	flags.BatchFile = "/nonexistent/file.txt"
	p := NewProcessor(flags)

	err := p.ProcessBatch()
	if err == nil {
		t.Error("Expected error for non-existent batch file")
	}
}

func TestProcessBatch_ValidFile(t *testing.T) {
	// Create test batch file
	tmpDir := t.TempDir()
	batchFile := filepath.Join(tmpDir, "batch.txt")
	content := `ябълка
котка = cat
куче`
	err := os.WriteFile(batchFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test batch file: %v", err)
	}

	flags := cli.NewFlags()
	flags.OutputDir = tmpDir
	flags.BatchFile = batchFile
	flags.SkipAudio = true
	flags.SkipImages = true
	p := NewProcessor(flags)

	// Skip if no OpenAI API key
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("Skipping test: OPENAI_API_KEY must be set")
	}

	err = p.ProcessBatch()
	if err != nil {
		t.Errorf("ProcessBatch failed: %v", err)
	}
}

func TestProcessWordWithTranslation_ProvidedTranslation(t *testing.T) {
	flags := cli.NewFlags()
	flags.OutputDir = t.TempDir()
	flags.SkipAudio = true
	flags.SkipImages = true
	p := NewProcessor(flags)

	// Skip if no API key (needed for phonetic fetcher)
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("Skipping test: OPENAI_API_KEY not set")
	}

	err := p.ProcessWordWithTranslation("ябълка", "apple")
	if err != nil {
		t.Errorf("ProcessWordWithTranslation failed: %v", err)
	}

	// Check that translation was cached
	cached, found := p.translationCache.Get("ябълка")
	if !found || cached != "apple" {
		t.Errorf("Expected cached translation 'apple', got '%s' (found: %v)", cached, found)
	}
}

func TestFindOrCreateWordDirectory(t *testing.T) {
	flags := cli.NewFlags()
	flags.OutputDir = t.TempDir()
	p := NewProcessor(flags)

	// First call should create directory
	dir1 := p.findOrCreateWordDirectory("тест")
	if dir1 == "" {
		t.Error("findOrCreateWordDirectory returned empty string")
	}

	// Check directory exists
	if _, err := os.Stat(dir1); os.IsNotExist(err) {
		t.Error("Directory was not created")
	}

	// Check word.txt was created
	wordFile := filepath.Join(dir1, "word.txt")
	content, err := os.ReadFile(wordFile)
	if err != nil {
		t.Errorf("Failed to read word.txt: %v", err)
	}
	if string(content) != "тест" {
		t.Errorf("Expected word.txt to contain 'тест', got '%s'", string(content))
	}

	// Second call should find existing directory
	dir2 := p.findOrCreateWordDirectory("тест")
	if dir2 != dir1 {
		t.Errorf("Expected to find existing directory %s, got %s", dir1, dir2)
	}
}

func TestFindCardDirectory(t *testing.T) {
	flags := cli.NewFlags()
	flags.OutputDir = t.TempDir()
	p := NewProcessor(flags)

	// Test with non-existent word
	dir := p.findCardDirectory("несъществуваща")
	if dir != "" {
		t.Error("Expected empty string for non-existent word")
	}

	// Create a word directory
	wordDir := p.findOrCreateWordDirectory("тест")

	// Now should find it
	dir = p.findCardDirectory("тест")
	if dir != wordDir {
		t.Errorf("Expected to find directory %s, got %s", wordDir, dir)
	}
}

func TestGenerateAnkiFile(t *testing.T) {
	flags := cli.NewFlags()
	flags.OutputDir = t.TempDir()
	flags.GenerateAnki = true
	flags.AnkiCSV = true // Test CSV format
	p := NewProcessor(flags)

	// Add some test translations
	p.translationCache.Add("ябълка", "apple")
	p.translationCache.Add("котка", "cat")

	// Create dummy word directories and files
	p.findOrCreateWordDirectory("ябълка")
	p.findOrCreateWordDirectory("котка")

	_, err := p.GenerateAnkiFile()
	if err != nil {
		t.Errorf("GenerateAnkiFile failed: %v", err)
	}

	// Check CSV file was created in home directory
	homeDir, _ := os.UserHomeDir()
	csvFile := filepath.Join(homeDir, "anki_import.csv")
	if _, err := os.Stat(csvFile); os.IsNotExist(err) {
		t.Error("CSV file was not created in home directory")
	}
	if err := os.Remove(csvFile); err != nil && !os.IsNotExist(err) {
		t.Errorf("Failed to remove CSV file: %v", err)
	}
}

func TestGenerateAnkiFile_APKG(t *testing.T) {
	flags := cli.NewFlags()
	flags.OutputDir = t.TempDir()
	flags.GenerateAnki = true
	flags.AnkiCSV = false // Test APKG format
	flags.DeckName = "Test Deck"
	p := NewProcessor(flags)

	// Create test word directories with files
	word1Dir := p.findOrCreateWordDirectory("ябълка")
	word2Dir := p.findOrCreateWordDirectory("котка")

	// Add test translations
	p.translationCache.Add("ябълка", "apple")
	p.translationCache.Add("котка", "cat")

	// Create dummy audio and image files
	if err := os.WriteFile(filepath.Join(word1Dir, "audio.mp3"), []byte("audio1"), 0644); err != nil {
		t.Fatalf("Failed to create test audio1 file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(word2Dir, "audio.mp3"), []byte("audio2"), 0644); err != nil {
		t.Fatalf("Failed to create test audio2 file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(word1Dir, "image.jpg"), []byte("image1"), 0644); err != nil {
		t.Fatalf("Failed to create test image file: %v", err)
	}

	_, err := p.GenerateAnkiFile()
	if err != nil {
		t.Errorf("GenerateAnkiFile (APKG) failed: %v", err)
	}

	// Check that an .apkg file was created in the home directory
	homeDir, _ := os.UserHomeDir()
	files, err := filepath.Glob(filepath.Join(homeDir, "*.apkg"))
	if err != nil {
		t.Fatalf("Error finding apkg file: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("No .apkg file found in home directory")
	}

	// Clean up the created file
	for _, file := range files {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			t.Errorf("Failed to remove APKG file %s: %v", file, err)
		}
	}
}
