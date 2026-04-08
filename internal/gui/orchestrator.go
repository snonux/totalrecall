package gui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"

	"codeberg.org/snonux/totalrecall/internal/audio"
	"codeberg.org/snonux/totalrecall/internal/image"
	"codeberg.org/snonux/totalrecall/internal/phonetic"
	"codeberg.org/snonux/totalrecall/internal/translation"
)

// GenerationOrchestrator coordinates audio, image, and phonetics generation
// for a single card. It holds all injectable factory functions so tests can
// substitute fakes without touching the UI layer.
// image.ClientFactories groups the two image-factory functions so the field
// definitions are not duplicated between this type and processor.Processor.
type GenerationOrchestrator struct {
	config *Config

	// audioResolver derives effective TTS settings from GUI + audio.Config.
	audioResolver *AudioConfigResolver
	// voiceSelector picks voice and speed per generation run.
	voiceSelector *VoiceSelector

	phonetics  *phonetic.Fetcher
	translator *translation.Translator

	// imageFactories groups the two image-provider construction functions.
	// Production code uses image.DefaultClientFactories(); tests replace fields.
	imageFactories image.ClientFactories

	// newAudioProvider constructs an audio.Provider from a Config.
	// Production code uses audio.NewProvider; tests replace it with a fake.
	newAudioProvider audio.ProviderFactory

	parallelRunner ParallelRunner
}

// NewGenerationOrchestrator constructs an orchestrator wired to the given app
// configuration and service dependencies. imageFactories and newAudio are the
// injectable test seams — pass image.DefaultClientFactories() and
// audio.NewProvider for production behaviour.
func NewGenerationOrchestrator(
	config *Config,
	audioCfg *audio.Config,
	phonetics *phonetic.Fetcher,
	translator *translation.Translator,
	imageFactories image.ClientFactories,
	newAudio audio.ProviderFactory,
) *GenerationOrchestrator {
	resolver := NewAudioConfigResolver(config, audioCfg)
	return &GenerationOrchestrator{
		config:           config,
		audioResolver:    resolver,
		voiceSelector:    NewVoiceSelector(resolver),
		phonetics:        phonetics,
		translator:       translator,
		imageFactories:   imageFactories,
		newAudioProvider: newAudio,
	}
}

// --- Translation helpers ---

// TranslateWord translates a Bulgarian word to English.
func (o *GenerationOrchestrator) TranslateWord(word string) (string, error) {
	if o.translator == nil {
		return "", fmt.Errorf("translation service not configured")
	}
	return o.translator.TranslateWord(word)
}

// TranslateEnglishToBulgarian translates an English word to Bulgarian.
func (o *GenerationOrchestrator) TranslateEnglishToBulgarian(word string) (string, error) {
	if o.translator == nil {
		return "", fmt.Errorf("translation service not configured")
	}
	return o.translator.TranslateEnglishToBulgarian(word)
}

// --- Audio provider helpers ---

// audioProviderName returns the lowercase provider name from config, defaulting
// to the shared audio default when none is set. Delegates to AudioConfigResolver
// so Application and tests keep a stable method on GenerationOrchestrator.
func (o *GenerationOrchestrator) audioProviderName() string {
	return o.audioResolver.ProviderName()
}

// audioOutputFormat resolves the effective output format (e.g. "mp3" or "wav").
func (o *GenerationOrchestrator) audioOutputFormat() string {
	return o.audioResolver.OutputFormat()
}

// generateAudioFile generates a single audio file for text using the given
// voice and speed. It is the lowest-level generation call.
func (o *GenerationOrchestrator) generateAudioFile(ctx context.Context, text, outputFile, voice string, speed float64) error {
	audioConfig := o.audioResolver.ConfigForGeneration(voice, speed)

	provider, err := o.newAudioProvider(&audioConfig)
	if err != nil {
		return err
	}

	return provider.GenerateAudio(ctx, text, outputFile)
}

// --- Audio generation public methods ---

// GenerateAudio generates audio for an en-bg card's single audio file.
// Returns the path to the generated file.
func (o *GenerationOrchestrator) GenerateAudio(ctx context.Context, word, cardDir string) (string, error) {
	if cardDir == "" {
		return "", fmt.Errorf("card directory not provided")
	}

	// Check if this is a regeneration by looking for an existing audio file.
	isRegeneration := false
	audioFile := filepath.Join(cardDir, fmt.Sprintf("audio.%s", o.audioOutputFormat()))
	if _, err := os.Stat(audioFile); err == nil {
		isRegeneration = true
	}

	voice, speed := o.voiceSelector.VoiceAndSpeed()

	if isRegeneration {
		fmt.Printf("Regenerating audio for '%s' with voice: %s, speed: %.2f\n", word, voice, speed)
	} else {
		fmt.Printf("Generating audio for '%s' with voice: %s, speed: %.2f\n", word, voice, speed)
	}

	finalVoice, err := o.runAudioWithFallbacks(ctx, word, audioFile, voice, speed)
	if err != nil {
		return "", err
	}

	audioCfg := o.audioResolver.ConfigForGeneration(finalVoice, speed)

	if err := o.saveAudioAttribution(word, audioFile, finalVoice, speed); err != nil {
		fmt.Printf("Warning: Failed to save audio attribution: %v\n", err)
	}

	if err := o.saveAudioMetadata(cardDir, audioCfg, finalVoice, speed, "en-bg", audioFile, ""); err != nil {
		fmt.Printf("Warning: Failed to save audio metadata: %v\n", err)
	}

	return audioFile, nil
}

// GenerateAudioFront generates the front audio file for a bg-bg card.
func (o *GenerationOrchestrator) GenerateAudioFront(ctx context.Context, word, cardDir string) (string, error) {
	if cardDir == "" {
		return "", fmt.Errorf("card directory not provided")
	}

	voice, speed := o.voiceSelector.VoiceAndSpeed()
	fmt.Printf("Generating front audio for '%s' with voice: %s, speed: %.2f\n", word, voice, speed)
	frontFile := filepath.Join(cardDir, fmt.Sprintf("audio_front.%s", o.audioOutputFormat()))

	finalVoice, err := o.runAudioWithFallbacks(ctx, word, frontFile, voice, speed)
	if err != nil {
		return "", fmt.Errorf("failed to generate front audio: %w", err)
	}

	audioCfg := o.audioResolver.ConfigForGeneration(finalVoice, speed)

	if err := o.saveAudioAttribution(word, frontFile, finalVoice, speed); err != nil {
		fmt.Printf("Warning: Failed to save audio attribution: %v\n", err)
	}

	// Resolve the existing back audio path to keep the metadata complete.
	_, existingBack := resolveBgBgAudioFilesInDir(cardDir)
	if err := o.saveAudioMetadata(cardDir, audioCfg, finalVoice, speed, "bg-bg", frontFile, existingBack); err != nil {
		fmt.Printf("Warning: Failed to save audio metadata: %v\n", err)
	}

	return frontFile, nil
}

// GenerateAudioBack generates the back audio file for a bg-bg card.
func (o *GenerationOrchestrator) GenerateAudioBack(ctx context.Context, text, cardDir string) (string, error) {
	if cardDir == "" {
		return "", fmt.Errorf("card directory not provided")
	}

	voice, speed := o.voiceSelector.VoiceAndSpeed()
	fmt.Printf("Generating back audio for '%s' with voice: %s, speed: %.2f\n", text, voice, speed)
	backFile := filepath.Join(cardDir, fmt.Sprintf("audio_back.%s", o.audioOutputFormat()))

	finalVoice, err := o.runAudioWithFallbacks(ctx, text, backFile, voice, speed)
	if err != nil {
		return "", fmt.Errorf("failed to generate back audio: %w", err)
	}

	audioCfg := o.audioResolver.ConfigForGeneration(finalVoice, speed)

	if err := o.saveAudioAttribution(text, backFile, finalVoice, speed); err != nil {
		fmt.Printf("Warning: Failed to save audio attribution: %v\n", err)
	}

	// Resolve the existing front audio path to keep the metadata complete.
	existingFront, _ := resolveBgBgAudioFilesInDir(cardDir)
	if err := o.saveAudioMetadata(cardDir, audioCfg, finalVoice, speed, "bg-bg", existingFront, backFile); err != nil {
		fmt.Printf("Warning: Failed to save audio metadata: %v\n", err)
	}

	return backFile, nil
}

// GenerateAudioBgBg generates audio for both sides of a bg-bg card in a single
// call, using the same voice for both to maintain consistency.
func (o *GenerationOrchestrator) GenerateAudioBgBg(ctx context.Context, front, back, cardDir string) (string, string, error) {
	if cardDir == "" {
		return "", "", fmt.Errorf("card directory not provided")
	}

	voice, speed := o.voiceSelector.VoiceAndSpeed()

	fmt.Printf("Generating front audio for '%s' with voice: %s, speed: %.2f\n", front, voice, speed)
	frontFile := filepath.Join(cardDir, fmt.Sprintf("audio_front.%s", o.audioOutputFormat()))
	backFile := filepath.Join(cardDir, fmt.Sprintf("audio_back.%s", o.audioOutputFormat()))

	// runPair generates both files using the given candidate voice.
	runPair := func(candidate string) error {
		if err := o.generateAudioFile(ctx, front, frontFile, candidate, speed); err != nil {
			return fmt.Errorf("failed to generate front audio: %w", err)
		}
		fmt.Printf("Generating back audio for '%s' with voice: %s, speed: %.2f\n", back, candidate, speed)
		if err := o.generateAudioFile(ctx, back, backFile, candidate, speed); err != nil {
			return fmt.Errorf("failed to generate back audio: %w", err)
		}
		return nil
	}

	finalVoice, err := o.runPairWithFallbacks(voice, runPair)
	if err != nil {
		return "", "", err
	}

	audioCfg := o.audioResolver.ConfigForGeneration(finalVoice, speed)

	if err := o.saveAudioAttribution(front, frontFile, finalVoice, speed); err != nil {
		fmt.Printf("Warning: Failed to save audio attribution: %v\n", err)
	}
	if err := o.saveAudioAttribution(back, backFile, finalVoice, speed); err != nil {
		fmt.Printf("Warning: Failed to save audio attribution: %v\n", err)
	}

	if err := o.saveAudioMetadata(cardDir, audioCfg, finalVoice, speed, "bg-bg", frontFile, backFile); err != nil {
		fmt.Printf("Warning: Failed to save audio metadata: %v\n", err)
	}

	return frontFile, backFile, nil
}

// runAudioWithFallbacks runs a single-file audio generation with Gemini voice
// fallback support. Returns the voice that was ultimately used.
func (o *GenerationOrchestrator) runAudioWithFallbacks(ctx context.Context, text, outputFile, voice string, speed float64) (string, error) {
	if o.audioResolver.ProviderName() == "gemini" && !o.voiceSelector.GeminiVoicePinned() {
		return audio.RunWithVoiceFallbacks(voice, func(candidate string) error {
			if candidate != voice {
				fmt.Printf("Retrying Gemini audio with voice: %s\n", candidate)
			}
			return o.generateAudioFile(ctx, text, outputFile, candidate, speed)
		}, nil)
	}

	return voice, o.generateAudioFile(ctx, text, outputFile, voice, speed)
}

// runPairWithFallbacks runs a pair-generation function with Gemini voice
// fallback support. Returns the voice that was ultimately used.
func (o *GenerationOrchestrator) runPairWithFallbacks(voice string, runPair func(string) error) (string, error) {
	if o.audioResolver.ProviderName() == "gemini" && !o.voiceSelector.GeminiVoicePinned() {
		return audio.RunWithVoiceFallbacks(voice, func(candidate string) error {
			if candidate != voice {
				fmt.Printf("Retrying Gemini audio with voice: %s\n", candidate)
			}
			return runPair(candidate)
		}, nil)
	}

	return voice, runPair(voice)
}

// saveAudioAttribution saves attribution metadata for a generated audio file.
// Uses BuildAttributionFor so no switch on provider name is needed here.
func (o *GenerationOrchestrator) saveAudioAttribution(word, audioFile, voice string, speed float64) error {
	processedText := audio.ProcessedTextForWord(word)
	providerName := o.audioResolver.ProviderName()

	cfg := o.audioResolver.BaseConfigForAttribution()

	// Override voice and speed with the values used for this specific generation.
	cfgCopy := *cfg
	cfgCopy.Provider = providerName
	cfgCopy.GeminiVoice = voice
	cfgCopy.GeminiSpeed = speed
	cfgCopy.OpenAIVoice = voice
	cfgCopy.OpenAISpeed = speed

	instruction := audio.InstructionForProvider(providerName, &cfgCopy)
	params := audio.AttributionParamsFrom(&cfgCopy, word, instruction, processedText, time.Now())
	attribution := audio.BuildAttributionFor(providerName, params)

	attrPath := audio.AttributionPath(audioFile)
	if err := os.WriteFile(attrPath, []byte(attribution), 0644); err != nil {
		return fmt.Errorf("failed to write audio attribution file: %w", err)
	}

	return nil
}

// saveAudioMetadata writes a sidecar metadata file alongside the audio file.
func (o *GenerationOrchestrator) saveAudioMetadata(cardDir string, audioCfg audio.Config, voice string, speed float64, cardType, audioFile, audioFileBack string) error {
	metadataFile := filepath.Join(cardDir, "audio_metadata.txt")

	if cardType == "bg-bg" {
		if audioFile == "" {
			audioFile, _ = resolveBgBgAudioFilesInDir(cardDir)
		}
		if audioFileBack == "" {
			_, audioFileBack = resolveBgBgAudioFilesInDir(cardDir)
		}
	}

	metadata := audio.BuildSidecarMetadata(audio.SidecarMetadataParams{
		Provider:          audioCfg.Provider,
		OutputFormat:      audioCfg.OutputFormat,
		CardType:          cardType,
		AudioFile:         audioFile,
		AudioFileBack:     audioFileBack,
		OpenAIModel:       audioCfg.OpenAIModel,
		OpenAIVoice:       voice,
		OpenAISpeed:       speed,
		OpenAIInstruction: audioCfg.OpenAIInstruction,
		GeminiTTSModel:    audioCfg.GeminiTTSModel,
		GeminiVoice:       voice,
		GeminiSpeed:       speed,
	})

	if err := os.WriteFile(metadataFile, []byte(metadata), 0644); err != nil {
		return fmt.Errorf("failed to write audio metadata file: %w", err)
	}

	return nil
}

// --- Image generation ---

// GenerateImagesWithPrompt downloads a single image for a word, using an
// optional custom prompt and translation hint.
func (o *GenerationOrchestrator) GenerateImagesWithPrompt(ctx context.Context, word, customPrompt, translation, cardDir string) (string, error) {
	searcher, err := o.newImageSearcher()
	if err != nil {
		return "", err
	}

	if cardDir == "" {
		return "", fmt.Errorf("card directory not provided")
	}

	downloadOpts := &image.DownloadOptions{
		OutputDir:         cardDir,
		OverwriteExisting: true,
		CreateDir:         true,
		FileNamePattern:   "image",
		MaxSizeBytes:      5 * 1024 * 1024, // 5 MB
	}

	downloader := image.NewDownloader(searcher, downloadOpts)

	// Set up a prompt callback so the on-disk metadata and UI update as soon
	// as the prompt is known (before the image download completes).
	searcher.SetPromptCallback(o.imagePromptCallback(cardDir, word))

	searchOpts := image.DefaultSearchOptions(word)
	if customPrompt != "" {
		searchOpts.CustomPrompt = customPrompt
	}
	if translation != "" {
		searchOpts.Translation = translation
	}

	_, path, err := downloader.DownloadBestMatchWithOptions(ctx, searchOpts)
	if err != nil {
		return "", err
	}

	// The prompt has already been saved and UI updated via the callback.
	return path, nil
}

// imagePromptCallback returns a closure that saves the image prompt to disk
// and notifies the current-word UI update if this word is still current.
// The closure captures the orchestrator's promptUpdateFn to avoid a direct
// dependency on Application.
func (o *GenerationOrchestrator) imagePromptCallback(cardDir, word string) func(prompt string) {
	return func(prompt string) {
		promptFile := filepath.Join(cardDir, "image_prompt.txt")
		if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
			fmt.Printf("Warning: Failed to save prompt for '%s': %v\n", word, err)
		}
	}
}

// newImageSearcher constructs the appropriate image client based on the
// configured image provider. Returns image.PromptAwareClient so callers can
// call SetPromptCallback directly without a type-assertion. The factory
// functions are sourced from imageFactories (the shared image.ClientFactories
// value) to avoid duplicating the factory signatures in this package.
func (o *GenerationOrchestrator) newImageSearcher() (image.PromptAwareClient, error) {
	switch o.config.ImageProvider {
	case imageProviderOpenAI:
		if o.config.OpenAIKey == "" {
			return nil, fmt.Errorf("OpenAI API key is required for image generation")
		}

		openaiConfig := &image.OpenAIConfig{
			APIKey:  o.config.OpenAIKey,
			Model:   "dall-e-2", // DALL-E 2 supports 512×512
			Size:    "512x512",
			Quality: "standard",
			Style:   "natural",
		}

		return o.imageFactories.NewOpenAIClient(openaiConfig), nil

	case imageProviderNanoBanana:
		cfg := o.config
		if cfg == nil {
			cfg = DefaultConfig()
		}
		if cfg.GoogleAPIKey == "" {
			return nil, fmt.Errorf("google API key is required for image generation")
		}

		nanoBananaConfig := &image.NanoBananaConfig{
			APIKey:    cfg.GoogleAPIKey,
			Model:     cfg.NanoBananaModel,
			TextModel: cfg.NanoBananaTextModel,
		}

		return o.imageFactories.NewNanoBananaClient(nanoBananaConfig), nil

	default:
		return nil, fmt.Errorf("unknown image provider: %s", o.config.ImageProvider)
	}
}

// --- Phonetics ---

// GetPhoneticInfo fetches phonetic information for a Bulgarian word.
func (o *GenerationOrchestrator) GetPhoneticInfo(word string) (string, error) {
	if o.phonetics == nil {
		return "", fmt.Errorf("phonetic fetcher not initialized")
	}

	phoneticInfo, err := o.phonetics.Fetch(word)
	if err != nil {
		return "", fmt.Errorf("failed to get phonetic info: %w", err)
	}

	return phoneticInfo, nil
}

// --- Parallel generation (orchestration) ---

// GenerateMaterials generates audio, image, and phonetics in parallel for a
// word. translation is the existing translation (may be empty). isBgBg flags
// bg-bg card type. imagePrompt is an optional custom prompt; imageTranslation
// is the translation hint for image prompts.
// The promptUI callback is called on the generating goroutine when the image
// prompt becomes known so callers can update the UI.
// Returns a GenerateResult or an error if any mandatory step fails.
func (o *GenerationOrchestrator) GenerateMaterials(
	ctx context.Context,
	word, translation, cardDir string,
	isBgBg bool,
	imagePrompt string,
	promptUI func(prompt string),
) (GenerateResult, error) {
	return o.parallelRunner.GenerateMaterials(o, ctx, word, translation, cardDir, isBgBg, imagePrompt, promptUI)
}

// generateImagesWithPromptAndNotify is a thin wrapper around
// GenerateImagesWithPrompt that additionally calls promptUI after saving the
// prompt file, so the UI can display the prompt immediately.
func (o *GenerationOrchestrator) generateImagesWithPromptAndNotify(
	ctx context.Context,
	word, customPrompt, translation, cardDir string,
	promptUI func(prompt string),
) (string, error) {
	searcher, err := o.newImageSearcher()
	if err != nil {
		return "", err
	}

	if cardDir == "" {
		return "", fmt.Errorf("card directory not provided")
	}

	downloadOpts := &image.DownloadOptions{
		OutputDir:         cardDir,
		OverwriteExisting: true,
		CreateDir:         true,
		FileNamePattern:   "image",
		MaxSizeBytes:      5 * 1024 * 1024,
	}

	downloader := image.NewDownloader(searcher, downloadOpts)

	// Wrap the prompt callback so the UI is notified in addition to the file save.
	searcher.SetPromptCallback(func(prompt string) {
		promptFile := filepath.Join(cardDir, "image_prompt.txt")
		if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
			fmt.Printf("Warning: Failed to save prompt for '%s': %v\n", word, err)
		}
		if promptUI != nil {
			fyne.Do(func() {
				promptUI(prompt)
			})
		}
	})

	searchOpts := image.DefaultSearchOptions(word)
	if customPrompt != "" {
		searchOpts.CustomPrompt = customPrompt
	}
	if translation != "" {
		searchOpts.Translation = translation
	}

	_, path, err := downloader.DownloadBestMatchWithOptions(ctx, searchOpts)
	return path, err
}
