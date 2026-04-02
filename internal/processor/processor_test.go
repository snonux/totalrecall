package processor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
	lastPrompt     string
	searchErr      error
	downloadErr    error
	promptCallback func(string)
}

func (s *stubImageSearcher) Search(ctx context.Context, opts *image.SearchOptions) ([]image.SearchResult, error) {
	if s.searchErr != nil {
		return nil, s.searchErr
	}

	s.lastPrompt = "stub nanobanana prompt"
	if s.promptCallback != nil {
		s.promptCallback(s.lastPrompt)
	}
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
	if s.downloadErr != nil {
		return nil, s.downloadErr
	}
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

func (s *stubImageSearcher) SetPromptCallback(callback func(string)) {
	s.promptCallback = callback
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

func captureStdout(t *testing.T, fn func()) (output string) {
	t.Helper()

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}

	os.Stdout = writer
	outputCh := make(chan string, 1)
	defer func() {
		os.Stdout = originalStdout
		if err := writer.Close(); err != nil {
			t.Fatalf("failed to close stdout pipe: %v", err)
		}
		output = <-outputCh
		if err := reader.Close(); err != nil {
			t.Fatalf("failed to close stdout reader: %v", err)
		}
	}()

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, reader)
		outputCh <- buf.String()
	}()

	fn()

	return output
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

func TestNewProcessor_DefaultPhoneticProviderUsesGemini(t *testing.T) {
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

	if got := p.phoneticFetcher.Provider(); got != phonetic.ProviderGemini {
		t.Fatalf("expected default phonetic provider %q, got %q", phonetic.ProviderGemini, got)
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

func TestNewProcessor_DefaultTranslationProviderUsesGemini(t *testing.T) {
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
		t.Fatal("Expected error for missing Google API key")
	}
	if err.Error() != "google API key not found" {
		t.Fatalf("Expected Gemini default provider error, got: %v", err)
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
	if err.Error() != "google API key not found" {
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
	viper.Set("image.nanobanana_model", "config-image-model")
	viper.Set("image.nanobanana_text_model", "config-text-model")

	flags := cli.NewFlags()
	flags.AudioFormat = "mp3"
	flags.ImageAPI = "openai"
	flags.ImageAPISpecified = false
	p := NewProcessor(flags)

	guiConfig := p.guiConfigForRunMode()
	if guiConfig.ImageProvider != gui.DefaultConfig().ImageProvider {
		t.Fatalf("guiConfig.ImageProvider = %q, want GUI default %q", guiConfig.ImageProvider, gui.DefaultConfig().ImageProvider)
	}
	if guiConfig.AudioProvider != "gemini" {
		t.Fatalf("guiConfig.AudioProvider = %q, want %q", guiConfig.AudioProvider, "gemini")
	}
	if guiConfig.AudioFormat != "mp3" {
		t.Fatalf("guiConfig.AudioFormat = %q, want %q", guiConfig.AudioFormat, "mp3")
	}
	if guiConfig.NanoBananaModel != "config-image-model" {
		t.Fatalf("guiConfig.NanoBananaModel = %q, want %q", guiConfig.NanoBananaModel, "config-image-model")
	}
	if guiConfig.NanoBananaTextModel != "config-text-model" {
		t.Fatalf("guiConfig.NanoBananaTextModel = %q, want %q", guiConfig.NanoBananaTextModel, "config-text-model")
	}
	if guiConfig.GeminiTTSModel != "gemini-2.5-flash-preview-tts" {
		t.Fatalf("guiConfig.GeminiTTSModel = %q, want %q", guiConfig.GeminiTTSModel, "gemini-2.5-flash-preview-tts")
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
	if guiConfig.AudioProvider != "gemini" {
		t.Fatalf("guiConfig.AudioProvider = %q, want %q", guiConfig.AudioProvider, "gemini")
	}
	if guiConfig.GoogleAPIKey != "test-google-key" {
		t.Fatalf("guiConfig.GoogleAPIKey = %q, want %q", guiConfig.GoogleAPIKey, "test-google-key")
	}
}

func TestGUIConfigForRunModeHonorsExplicitNanoBananaModelFlags(t *testing.T) {
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

	flags := cli.NewFlags()
	flags.ImageAPI = "openai"
	flags.ImageAPISpecified = false
	flags.NanoBananaModel = "flag-image-model"
	flags.NanoBananaModelSpecified = true
	flags.NanoBananaTextModel = "flag-text-model"
	flags.NanoBananaTextModelSpecified = true
	p := NewProcessor(flags)

	guiConfig := p.guiConfigForRunMode()
	if guiConfig.NanoBananaModel != "flag-image-model" {
		t.Fatalf("guiConfig.NanoBananaModel = %q, want %q", guiConfig.NanoBananaModel, "flag-image-model")
	}
	if guiConfig.NanoBananaTextModel != "flag-text-model" {
		t.Fatalf("guiConfig.NanoBananaTextModel = %q, want %q", guiConfig.NanoBananaTextModel, "flag-text-model")
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

	wordDir := p.findCardDirectory("ябълка")
	if wordDir == "" {
		t.Fatal("expected generated word directory")
	}

	metadataData, err := os.ReadFile(filepath.Join(wordDir, "audio_metadata.txt"))
	if err != nil {
		t.Fatalf("expected metadata file: %v", err)
	}
	metadata := string(metadataData)
	for _, want := range []string{
		"provider=openai",
		"cardtype=bg-bg",
		"audio_file=audio_front.mp3",
		"audio_file_back=audio_back.mp3",
	} {
		if !strings.Contains(metadata, want) {
			t.Fatalf("metadata = %q, missing %q", metadata, want)
		}
	}

	for i, outputFile := range fakeProvider.outputFiles {
		attrPath := audio.AttributionPath(outputFile)
		attributionData, err := os.ReadFile(attrPath)
		if err != nil {
			t.Fatalf("expected attribution file %q: %v", attrPath, err)
		}
		attribution := string(attributionData)
		wantText := []string{"ябълка", "круша"}[i]
		if !strings.Contains(attribution, "Processed text sent to TTS: "+wantText) {
			t.Fatalf("bg-bg attribution missing exact processed text %q: %q", wantText, attribution)
		}
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
	if err := p.generateAudio("ябълка!?"); err != nil {
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
	if capturedConfig.OutputFormat != "mp3" {
		t.Fatalf("captured OutputFormat = %q, want %q", capturedConfig.OutputFormat, "mp3")
	}
	if fakeProvider.generateCalls != 1 {
		t.Fatalf("GenerateAudio() calls = %d, want %d", fakeProvider.generateCalls, 1)
	}
	if !strings.HasSuffix(fakeProvider.lastOutputFile, "audio.mp3") {
		t.Fatalf("GenerateAudio() output file = %q, want mp3 output", fakeProvider.lastOutputFile)
	}

	wordDir := p.findCardDirectory("ябълка!?")
	if wordDir == "" {
		t.Fatal("expected generated word directory")
	}
	metadataData, err := os.ReadFile(filepath.Join(wordDir, "audio_metadata.txt"))
	if err != nil {
		t.Fatalf("expected metadata file: %v", err)
	}
	metadata := string(metadataData)
	for _, want := range []string{
		"provider=gemini",
		"model=gemini-2.5-flash-preview-tts",
		"voice=Kore",
		"speed=1.00",
		"format=mp3",
		"audio_file=audio.mp3",
		"cardtype=en-bg",
	} {
		if !strings.Contains(metadata, want) {
			t.Fatalf("metadata = %q, missing %q", metadata, want)
		}
	}

	attrPath := audio.AttributionPath(fakeProvider.lastOutputFile)
	attributionData, err := os.ReadFile(attrPath)
	if err != nil {
		t.Fatalf("expected attribution file %q: %v", attrPath, err)
	}
	attribution := string(attributionData)
	if !strings.Contains(attribution, "Processed text sent to TTS: ябълка!?") {
		t.Fatalf("gemini attribution missing exact processed text: %q", attribution)
	}
	if !strings.Contains(attribution, "Speak at a natural pace.") {
		t.Fatalf("gemini attribution missing speed hint semantics: %q", attribution)
	}
	if !strings.Contains(attribution, "voice named Kore") {
		t.Fatalf("gemini attribution missing voice semantics: %q", attribution)
	}
}

func TestGenerateAudioUsesGeminiModelDefaultWhenVoiceNotSet(t *testing.T) {
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

	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()
	viper.Reset()
	viper.Set("audio.provider", "gemini")

	flags := cli.NewFlags()
	flags.OutputDir = t.TempDir()
	flags.AudioFormat = "mp3"
	flags.AudioProvider = "gemini"

	p := NewProcessor(flags)
	if err := p.generateAudio("ябълка!?"); err != nil {
		t.Fatalf("generateAudio() unexpected error: %v", err)
	}

	if capturedConfig == nil {
		t.Fatal("expected audio provider config to be captured")
	}
	if capturedConfig.GeminiVoice != "sentinel-gemini-voice" {
		t.Fatalf("captured GeminiVoice = %q, want %q", capturedConfig.GeminiVoice, "sentinel-gemini-voice")
	}
	if fakeProvider.generateCalls != 1 {
		t.Fatalf("GenerateAudio() calls = %d, want %d", fakeProvider.generateCalls, 1)
	}
	if !strings.HasSuffix(fakeProvider.lastOutputFile, "audio.mp3") {
		t.Fatalf("GenerateAudio() output file = %q, want mp3 output", fakeProvider.lastOutputFile)
	}

	wordDir := p.findCardDirectory("ябълка!?")
	if wordDir == "" {
		t.Fatal("expected generated word directory")
	}
	metadataData, err := os.ReadFile(filepath.Join(wordDir, "audio_metadata.txt"))
	if err != nil {
		t.Fatalf("expected metadata file: %v", err)
	}
	metadata := string(metadataData)
	for _, want := range []string{
		"provider=gemini",
		"voice=sentinel-gemini-voice",
		"audio_file=audio.mp3",
		"cardtype=en-bg",
	} {
		if !strings.Contains(metadata, want) {
			t.Fatalf("metadata = %q, missing %q", metadata, want)
		}
	}

	attrPath := audio.AttributionPath(fakeProvider.lastOutputFile)
	attributionData, err := os.ReadFile(attrPath)
	if err != nil {
		t.Fatalf("expected attribution file %q: %v", attrPath, err)
	}
	attribution := string(attributionData)
	if !strings.Contains(attribution, "Processed text sent to TTS: ябълка!?") {
		t.Fatalf("gemini attribution missing exact processed text: %q", attribution)
	}
	if !strings.Contains(attribution, "Speak at a natural pace.") {
		t.Fatalf("gemini attribution missing speed hint semantics: %q", attribution)
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

	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()
	viper.Reset()
	viper.Set("audio.provider", "gemini")

	flags := cli.NewFlags()
	flags.OutputDir = t.TempDir()
	flags.AudioProvider = "gemini"

	p := NewProcessor(flags)
	p.randomIntn = func(int) int { return 0 }
	output := captureStdout(t, func() {
		if err := p.generateAudio("ябълка"); err != nil {
			t.Fatalf("generateAudio() unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "  Warning: Gemini returned no audio for voice Charon") {
		t.Fatalf("stdout missing indented Gemini warning: %q", output)
	}
	if !strings.Contains(output, "  Retrying Gemini audio with voice: Kore") {
		t.Fatalf("stdout missing retry message: %q", output)
	}

	if got, want := strings.Join(attemptedVoices, ","), "Charon,Kore"; got != want {
		t.Fatalf("attempted voices = %q, want %q", got, want)
	}

	wordDir := p.findCardDirectory("ябълка")
	if wordDir == "" {
		t.Fatal("expected generated word directory")
	}

	metadataData, err := os.ReadFile(filepath.Join(wordDir, "audio_metadata.txt"))
	if err != nil {
		t.Fatalf("expected metadata file: %v", err)
	}
	metadata := string(metadataData)
	if !strings.Contains(metadata, "voice=Kore") {
		t.Fatalf("metadata should use the successful fallback voice: %q", metadata)
	}
}

func TestGenerateAudioReturnsExhaustedGeminiFallbackError(t *testing.T) {
	originalFactory := newAudioProvider
	t.Cleanup(func() {
		newAudioProvider = originalFactory
	})

	originalVoices := append([]string(nil), audio.GeminiVoices...)
	t.Cleanup(func() {
		audio.GeminiVoices = originalVoices
	})
	audio.GeminiVoices = []string{"Charon"}

	newAudioProvider = func(*audio.Config) (audio.Provider, error) {
		return &fakeAudioProvider{
			generateFunc: func(_ string, _ string) error {
				return audio.ErrGeminiNoAudioData
			},
		}, nil
	}

	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()
	viper.Reset()
	viper.Set("audio.provider", "gemini")

	flags := cli.NewFlags()
	flags.OutputDir = t.TempDir()
	flags.AudioProvider = "gemini"

	p := NewProcessor(flags)
	err := p.generateAudio("ябълка")
	if !errors.Is(err, audio.ErrGeminiNoAudioData) {
		t.Fatalf("generateAudio() error = %v, want wrapped ErrGeminiNoAudioData", err)
	}
	if got, want := err.Error(), "Gemini returned no audio for voices Charon: no audio data returned from Gemini"; got != want {
		t.Fatalf("generateAudio() error text = %q, want %q", got, want)
	}
}

func TestGenerateAudioBgBgUsesGeminiModelDefaultWhenVoiceNotSet(t *testing.T) {
	originalFactory := newAudioProvider
	t.Cleanup(func() {
		newAudioProvider = originalFactory
	})
	originalVoices := append([]string(nil), audio.GeminiVoices...)
	t.Cleanup(func() {
		audio.GeminiVoices = originalVoices
	})
	audio.GeminiVoices = []string{"sentinel-bg-gemini-voice"}

	fakeProvider := &fakeAudioProvider{}
	var capturedConfigs []*audio.Config
	newAudioProvider = func(config *audio.Config) (audio.Provider, error) {
		copyConfig := *config
		capturedConfigs = append(capturedConfigs, &copyConfig)
		return fakeProvider, nil
	}

	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()
	viper.Reset()
	viper.Set("audio.provider", "gemini")

	flags := cli.NewFlags()
	flags.OutputDir = t.TempDir()
	flags.AudioFormat = "mp3"
	flags.AudioProvider = "gemini"

	p := NewProcessor(flags)
	if err := p.generateAudioBgBg("ябълка!?", "круша."); err != nil {
		t.Fatalf("generateAudioBgBg() unexpected error: %v", err)
	}

	if len(capturedConfigs) != 2 {
		t.Fatalf("captured config count = %d, want %d", len(capturedConfigs), 2)
	}
	for i, capturedConfig := range capturedConfigs {
		if capturedConfig.GeminiVoice != "sentinel-bg-gemini-voice" {
			t.Fatalf("captured config %d GeminiVoice = %q, want %q", i, capturedConfig.GeminiVoice, "sentinel-bg-gemini-voice")
		}
	}
	if fakeProvider.generateCalls != 2 {
		t.Fatalf("GenerateAudio() calls = %d, want %d", fakeProvider.generateCalls, 2)
	}
	for i, outputFile := range fakeProvider.outputFiles {
		attrPath := audio.AttributionPath(outputFile)
		attributionData, err := os.ReadFile(attrPath)
		if err != nil {
			t.Fatalf("expected attribution file %q: %v", attrPath, err)
		}
		attribution := string(attributionData)
		wantText := []string{"ябълка!?", "круша."}[i]
		if !strings.Contains(attribution, "Processed text sent to TTS: "+wantText) {
			t.Fatalf("bg-bg attribution missing exact processed text %q: %q", wantText, attribution)
		}
		if !strings.Contains(attribution, "Speak at a natural pace.") {
			t.Fatalf("bg-bg attribution missing speed hint semantics: %q", attribution)
		}
	}
}

func TestGenerateAudioUsesConfiguredAudioFormatWhenOpenAIConfigIsSetOnly(t *testing.T) {
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
	viper.Set("audio.provider", "openai")
	viper.Set("audio.format", "mp3")

	flags := cli.NewFlags()
	flags.OutputDir = t.TempDir()
	flags.AudioProvider = "openai"
	flags.AudioFormat = "wav"

	p := NewProcessor(flags)
	wordDir := p.findOrCreateWordDirectory("ябълка!?")
	if err := p.generateAudioWithVoiceAndFilenameInDir("ябълка!?", "alloy", "audio", wordDir); err != nil {
		t.Fatalf("generateAudioWithVoiceAndFilenameInDir() unexpected error: %v", err)
	}

	if capturedConfig == nil {
		t.Fatal("expected audio provider config to be captured")
	}
	if capturedConfig.OutputFormat != "mp3" {
		t.Fatalf("captured OutputFormat = %q, want %q", capturedConfig.OutputFormat, "mp3")
	}
	if fakeProvider.generateCalls != 1 {
		t.Fatalf("GenerateAudio() calls = %d, want %d", fakeProvider.generateCalls, 1)
	}
	if !strings.HasSuffix(fakeProvider.lastOutputFile, "audio.mp3") {
		t.Fatalf("GenerateAudio() output file = %q, want mp3 output", fakeProvider.lastOutputFile)
	}

	metadataData, err := os.ReadFile(filepath.Join(wordDir, "audio_metadata.txt"))
	if err != nil {
		t.Fatalf("expected metadata file: %v", err)
	}
	metadata := string(metadataData)
	for _, want := range []string{
		"provider=openai",
		"model=gpt-4o-mini-tts",
		"voice=alloy",
		"format=mp3",
		"cardtype=en-bg",
	} {
		if !strings.Contains(metadata, want) {
			t.Fatalf("metadata = %q, missing %q", metadata, want)
		}
	}

	attrPath := audio.AttributionPath(fakeProvider.lastOutputFile)
	attributionData, err := os.ReadFile(attrPath)
	if err != nil {
		t.Fatalf("expected attribution file %q: %v", attrPath, err)
	}
	attribution := string(attributionData)
	if !strings.Contains(attribution, "Processed text sent to TTS: ябълка") {
		t.Fatalf("openai attribution missing exact processed text: %q", attribution)
	}
	if strings.Contains(attribution, "Voice instructions:") {
		t.Fatalf("openai attribution unexpectedly recorded instructions: %q", attribution)
	}
}

func TestGenerateAudioUsesConfiguredOpenAIVoiceFromConfig(t *testing.T) {
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
	viper.Set("audio.provider", "openai")
	viper.Set("audio.openai_voice", "shimmer")

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
	if capturedConfig.OpenAIVoice != "shimmer" {
		t.Fatalf("captured OpenAIVoice = %q, want %q", capturedConfig.OpenAIVoice, "shimmer")
	}
	if fakeProvider.generateCalls != 1 {
		t.Fatalf("GenerateAudio() calls = %d, want %d", fakeProvider.generateCalls, 1)
	}
	if !strings.HasSuffix(fakeProvider.lastOutputFile, "audio.mp3") {
		t.Fatalf("GenerateAudio() output file = %q, want single-voice output file", fakeProvider.lastOutputFile)
	}

	wordDir := p.findCardDirectory("ябълка")
	if wordDir == "" {
		t.Fatal("expected generated word directory")
	}
	metadataData, err := os.ReadFile(filepath.Join(wordDir, "audio_metadata.txt"))
	if err != nil {
		t.Fatalf("expected metadata file: %v", err)
	}
	metadata := string(metadataData)
	for _, want := range []string{
		"provider=openai",
		"voice=shimmer",
		"audio_file=audio.mp3",
	} {
		if !strings.Contains(metadata, want) {
			t.Fatalf("metadata = %q, missing %q", metadata, want)
		}
	}

	attrPath := audio.AttributionPath(fakeProvider.lastOutputFile)
	attributionData, err := os.ReadFile(attrPath)
	if err != nil {
		t.Fatalf("expected attribution file %q: %v", attrPath, err)
	}
	attribution := string(attributionData)
	if !strings.Contains(attribution, "Processed text sent to TTS: ябълка") {
		t.Fatalf("openai attribution missing exact processed text: %q", attribution)
	}
}

func TestGenerateAudioOmitsOpenAIInstructionsForUnsupportedModel(t *testing.T) {
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
	viper.Set("audio.provider", "openai")
	viper.Set("audio.openai_instruction", "Speak clearly.")

	flags := cli.NewFlags()
	flags.OutputDir = t.TempDir()
	flags.AudioProvider = "openai"
	flags.AudioFormat = "mp3"
	flags.OpenAIModel = "tts-1"

	p := NewProcessor(flags)
	if err := p.generateAudio("ябълка!?"); err != nil {
		t.Fatalf("generateAudio() unexpected error: %v", err)
	}

	if capturedConfig == nil {
		t.Fatal("expected audio provider config to be captured")
	}
	if capturedConfig.OpenAIModel != "tts-1" {
		t.Fatalf("captured OpenAIModel = %q, want %q", capturedConfig.OpenAIModel, "tts-1")
	}
	if capturedConfig.OpenAIInstruction != "Speak clearly." {
		t.Fatalf("captured OpenAIInstruction = %q, want %q", capturedConfig.OpenAIInstruction, "Speak clearly.")
	}

	wordDir := p.findCardDirectory("ябълка!?")
	if wordDir == "" {
		t.Fatal("expected generated word directory")
	}
	metadataData, err := os.ReadFile(filepath.Join(wordDir, "audio_metadata.txt"))
	if err != nil {
		t.Fatalf("expected metadata file: %v", err)
	}
	metadata := string(metadataData)
	if strings.Contains(metadata, "instruction=Speak clearly.") {
		t.Fatalf("openai metadata unexpectedly recorded unsupported instructions: %q", metadata)
	}
	if !strings.Contains(metadata, "provider=openai") || !strings.Contains(metadata, "model=tts-1") {
		t.Fatalf("openai metadata missing provider/model semantics: %q", metadata)
	}

	attrPath := audio.AttributionPath(fakeProvider.lastOutputFile)
	attributionData, err := os.ReadFile(attrPath)
	if err != nil {
		t.Fatalf("expected attribution file %q: %v", attrPath, err)
	}
	attribution := string(attributionData)
	if strings.Contains(attribution, "Voice instructions:") {
		t.Fatalf("openai attribution unexpectedly recorded unsupported instructions: %q", attribution)
	}
	if !strings.Contains(attribution, "Processed text sent to TTS: ябълка") {
		t.Fatalf("openai attribution missing cleaned processed text: %q", attribution)
	}
}

func TestGenerateAnkiFileUsesEffectiveAudioFormatForGemini(t *testing.T) {
	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()
	viper.Reset()
	viper.Set("audio.provider", "gemini")

	tempDir := t.TempDir()
	flags := cli.NewFlags()
	flags.OutputDir = tempDir
	flags.AudioProvider = "gemini"
	flags.AudioFormat = "mp3"
	flags.AnkiCSV = true

	p := NewProcessor(flags)
	p.translationCache.Add("ябълка", "apple")

	wordDir := p.findOrCreateWordDirectory("ябълка")
	for name, content := range map[string]string{
		"audio_metadata.txt":          "provider=gemini\nmodel=gemini-2.5-flash-preview-tts\nvoice=Kore\nspeed=1.00\nformat=wav\n",
		"phonetic.txt":                "phonetic",
		"audio_alpha.wav":             "audio data",
		"audio_beta.wav":              "audio data",
		"audio_alpha_attribution.txt": "attribution",
		"audio_beta_attribution.txt":  "attribution",
		"translation.txt":             "ябълка = apple",
	} {
		if err := os.WriteFile(filepath.Join(wordDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("failed to create %s: %v", name, err)
		}
	}

	outputPath, err := p.GenerateAnkiFile()
	if err != nil {
		t.Fatalf("GenerateAnkiFile() unexpected error: %v", err)
	}
	if !strings.HasSuffix(outputPath, "anki_import.csv") {
		t.Fatalf("GenerateAnkiFile() output = %q, want CSV output", outputPath)
	}

	csvData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read generated CSV: %v", err)
	}
	cardID := filepath.Base(wordDir)
	if !strings.Contains(string(csvData), fmt.Sprintf("[sound:%s_audio_alpha.wav]", cardID)) {
		t.Fatalf("generated CSV did not reference the resolved multi-voice wav audio for card %q: %s", cardID, csvData)
	}
}

func TestIsWordFullyProcessedUsesMultiVoiceAttributionFiles(t *testing.T) {
	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()
	viper.Reset()
	viper.Set("audio.provider", "gemini")

	flags := cli.NewFlags()
	flags.OutputDir = t.TempDir()
	flags.AudioProvider = "gemini"
	flags.AudioFormat = "mp3"
	flags.SkipImages = true

	p := NewProcessor(flags)
	wordDir := p.findOrCreateWordDirectory("ябълка")
	files := map[string]string{
		"translation.txt":             "ябълка = apple",
		"phonetic.txt":                "phonetic",
		"audio_metadata.txt":          "provider=gemini\nmodel=gemini-2.5-flash-preview-tts\nvoice=Kore\nspeed=1.00\nformat=wav\n",
		"audio_alpha.wav":             "audio data",
		"audio_beta.wav":              "audio data",
		"audio_alpha_attribution.txt": "attribution",
		"audio_beta_attribution.txt":  "attribution",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(wordDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("failed to create %s: %v", name, err)
		}
	}

	if !p.isWordFullyProcessed("ябълка") {
		t.Fatal("expected Gemini word with multi-voice wav audio to be treated as fully processed")
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

func TestDownloadImagesWithTranslationPersistsPromptWhenDownloadFails(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-openai-key")
	t.Setenv("GOOGLE_API_KEY", "test-google-key")

	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()
	viper.Reset()
	viper.Set("image.provider", "nanobanana")

	originalConstructor := newNanoBananaImageClient
	stubSearcher := &stubImageSearcher{downloadErr: errors.New("download failed")}
	newNanoBananaImageClient = func(config *image.NanoBananaConfig) image.ImageSearcher {
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
	err := p.downloadImagesWithTranslation("ябълка", "apple")
	if err == nil {
		t.Fatal("downloadImagesWithTranslation() expected error from failed download")
	}

	wordDir := p.findCardDirectory("ябълка")
	if wordDir == "" {
		t.Fatal("expected word directory to be created")
	}

	promptData, err := os.ReadFile(filepath.Join(wordDir, "image_prompt.txt"))
	if err != nil {
		t.Fatalf("expected prompt file after failed download: %v", err)
	}
	if got := strings.TrimSpace(string(promptData)); got != "stub nanobanana prompt" {
		t.Fatalf("prompt file = %q, want %q", got, "stub nanobanana prompt")
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
	if got := err.Error(); got != "google API key is required for image generation" {
		t.Fatalf("newImageSearcher() error = %q, want %q", got, "google API key is required for image generation")
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
