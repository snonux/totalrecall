package processor

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"

	"codeberg.org/snonux/totalrecall/internal"
	"codeberg.org/snonux/totalrecall/internal/anki"
	"codeberg.org/snonux/totalrecall/internal/audio"
	"codeberg.org/snonux/totalrecall/internal/batch"
	"codeberg.org/snonux/totalrecall/internal/cli"
	"codeberg.org/snonux/totalrecall/internal/gui"
	"codeberg.org/snonux/totalrecall/internal/image"
	"codeberg.org/snonux/totalrecall/internal/phonetic"
	"codeberg.org/snonux/totalrecall/internal/translation"
)

// Processor handles the main word processing logic
type Processor struct {
	flags            *cli.Flags
	translator       *translation.Translator
	translationCache *translation.TranslationCache
	phoneticFetcher  *phonetic.Fetcher
}

var newOpenAIImageClient = func(config *image.OpenAIConfig) image.ImageSearcher {
	return image.NewOpenAIClient(config)
}

var newNanoBananaImageClient = func(config *image.NanoBananaConfig) image.ImageSearcher {
	return image.NewNanoBananaClient(config)
}

var newAudioProvider = audio.NewProvider

// NewProcessor creates a new word processor
func NewProcessor(flags *cli.Flags) *Processor {
	openAIKey := cli.GetOpenAIKey()
	googleAPIKey := cli.GetGoogleAPIKey()
	translationProvider := translation.Provider(viper.GetString("translation.provider"))
	phoneticProvider := phonetic.Provider(viper.GetString("phonetic.provider"))
	return &Processor{
		flags:            flags,
		translator:       translation.NewTranslator(&translation.Config{Provider: translationProvider, OpenAIKey: openAIKey, GoogleAPIKey: googleAPIKey}),
		translationCache: translation.NewTranslationCache(),
		phoneticFetcher:  phonetic.NewFetcher(&phonetic.Config{Provider: phoneticProvider, OpenAIKey: openAIKey, GoogleAPIKey: googleAPIKey}),
	}
}

// ProcessBatch processes multiple words from a batch file
func (p *Processor) ProcessBatch() error {
	entries, err := batch.ReadBatchFile(p.flags.BatchFile)
	if err != nil {
		return err
	}

	// Create output directory (including parent directories)
	if err := os.MkdirAll(p.flags.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// First pass: handle entries that need English to Bulgarian translation
	for i, entry := range entries {
		if entry.NeedsTranslation && entry.Translation != "" {
			// Translate English to Bulgarian
			bulgarian, err := p.translator.TranslateEnglishToBulgarian(entry.Translation)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error translating '%s' to Bulgarian: %v\n", entry.Translation, err)
				continue
			}
			entries[i].Bulgarian = bulgarian
			fmt.Printf("Translated '%s' to Bulgarian: %s\n", entry.Translation, bulgarian)
		}
	}

	// Validate Bulgarian words
	for _, entry := range entries {
		if entry.Bulgarian != "" {
			if err := audio.ValidateBulgarianText(entry.Bulgarian); err != nil {
				return fmt.Errorf("invalid word '%s': %w", entry.Bulgarian, err)
			}
		}
	}

	// Track statistics
	skippedCount := 0
	processedCount := 0
	errorCount := 0

	// Process each entry
	for i, entry := range entries {
		if entry.Bulgarian == "" {
			continue // Skip entries without Bulgarian word
		}

		fmt.Printf("\nProcessing %d/%d: %s\n", i+1, len(entries), entry.Bulgarian)

		// Check if word already exists and has all required files
		if os.Getenv("DEBUG_BATCH") != "" {
			fmt.Printf("  [DEBUG] Checking if word is fully processed...\n")
		}
		if p.isWordFullyProcessed(entry.Bulgarian) {
			wordDir := p.findCardDirectory(entry.Bulgarian)
			fmt.Printf("  ✓ Skipping '%s' - already fully processed in %s\n", entry.Bulgarian, filepath.Base(wordDir))
			skippedCount++
			continue
		}
		if os.Getenv("DEBUG_BATCH") != "" {
			fmt.Printf("  [DEBUG] Word is not fully processed, will process it\n")
		}

		if err := p.ProcessWordWithTranslationAndType(entry.Bulgarian, entry.Translation, entry.CardType); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing '%s': %v\n", entry.Bulgarian, err)
			errorCount++
		} else {
			processedCount++
		}
	}

	// Print summary
	fmt.Printf("\n=== Batch Processing Summary ===\n")
	fmt.Printf("Total words: %d\n", len(entries))
	fmt.Printf("Processed: %d\n", processedCount)
	fmt.Printf("Skipped (already complete): %d\n", skippedCount)
	if errorCount > 0 {
		fmt.Printf("Errors: %d\n", errorCount)
	}
	fmt.Printf("================================\n")

	return nil
}

// ProcessSingleWord processes a single word from command line
func (p *Processor) ProcessSingleWord(word string) error {
	// Validate word
	if err := audio.ValidateBulgarianText(word); err != nil {
		return fmt.Errorf("invalid word '%s': %w", word, err)
	}

	// Create output directory (including parent directories)
	if err := os.MkdirAll(p.flags.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Printf("\nProcessing: %s\n", word)
	return p.ProcessWordWithTranslation(word, "")
}

// ProcessWordWithTranslation processes a word with optional provided translation (en-bg mode)
func (p *Processor) ProcessWordWithTranslation(word, providedTranslation string) error {
	return p.ProcessWordWithTranslationAndType(word, providedTranslation, internal.CardTypeEnBg)
}

// ProcessWordWithTranslationAndType processes a word with optional provided translation and card type
func (p *Processor) ProcessWordWithTranslationAndType(word, providedTranslation string, cardType internal.CardType) error {
	var translationText string

	// For bg-bg cards, translation is the back side (Bulgarian definition)
	// For en-bg cards, translation is the English word
	if providedTranslation != "" {
		translationText = providedTranslation
		if cardType.IsBgBg() {
			fmt.Printf("  Using provided definition: %s\n", translationText)
		} else {
			fmt.Printf("  Using provided translation: %s\n", translationText)
		}
	} else if !cardType.IsBgBg() {
		// Only translate to English for en-bg cards
		fmt.Printf("  Translating to English...\n")
		var err error
		translationText, err = p.translator.TranslateWord(word)
		if err != nil {
			fmt.Printf("  Warning: Translation failed: %v\n", err)
			translationText = ""
		} else {
			fmt.Printf("  Translation: %s\n", translationText)
		}
	}

	// Find or create word directory
	wordDir := p.findOrCreateWordDirectory(word)

	// Save card type
	if err := internal.SaveCardType(wordDir, cardType); err != nil {
		fmt.Printf("  Warning: Failed to save card type: %v\n", err)
	}

	// Store translation for Anki export
	if translationText != "" {
		p.translationCache.Add(word, translationText)

		// Check if translation file already exists
		translationFile := filepath.Join(wordDir, "translation.txt")
		if _, err := os.Stat(translationFile); os.IsNotExist(err) {
			if err := translation.SaveTranslation(wordDir, word, translationText); err != nil {
				fmt.Printf("  Warning: Failed to save translation: %v\n", err)
			}
		} else {
			fmt.Printf("  Translation file already exists\n")
		}
	}

	// Fetch phonetic information
	fmt.Printf("  Fetching phonetic information...\n")
	if err := p.phoneticFetcher.FetchAndSave(word, wordDir); err != nil {
		fmt.Printf("  Warning: Failed to fetch phonetic info: %v\n", err)
	} else {
		fmt.Printf("  Saved phonetic information\n")
	}

	// Generate audio
	if !p.flags.SkipAudio {
		fmt.Printf("  Generating audio...\n")
		if cardType.IsBgBg() {
			// Generate audio for both sides
			if err := p.generateAudioBgBg(word, translationText); err != nil {
				return fmt.Errorf("audio generation failed: %w", err)
			}
		} else {
			if err := p.generateAudio(word); err != nil {
				return fmt.Errorf("audio generation failed: %w", err)
			}
		}
	}

	// Download images - pass the translation for better image generation
	if !p.flags.SkipImages {
		fmt.Printf("  Downloading images...\n")
		if err := p.downloadImagesWithTranslation(word, translationText); err != nil {
			return fmt.Errorf("image download failed: %w", err)
		}
	}

	return nil
}

func (p *Processor) audioProviderName() string {
	if provider := strings.ToLower(strings.TrimSpace(viper.GetString("audio.provider"))); provider != "" {
		return provider
	}
	if p != nil && p.flags != nil {
		return strings.ToLower(strings.TrimSpace(p.flags.AudioProvider))
	}
	return ""
}

func (p *Processor) effectiveAudioFormat() string {
	if p.audioProviderName() == "gemini" {
		return "wav"
	}

	if viper.IsSet("audio.format") {
		if format := strings.ToLower(strings.TrimSpace(viper.GetString("audio.format"))); format != "" {
			return format
		}
	}

	if p != nil && p.flags != nil {
		if format := strings.ToLower(strings.TrimSpace(p.flags.AudioFormat)); format != "" {
			return format
		}
	}

	return "mp3"
}

func (p *Processor) geminiTTSModel() string {
	if model := strings.TrimSpace(viper.GetString("audio.gemini_tts_model")); model != "" {
		return model
	}
	if p != nil && p.flags != nil {
		return strings.TrimSpace(p.flags.GeminiTTSModel)
	}
	return ""
}

func (p *Processor) geminiVoice() string {
	if voice := strings.TrimSpace(viper.GetString("audio.gemini_voice")); voice != "" {
		return voice
	}
	if p != nil && p.flags != nil {
		return strings.TrimSpace(p.flags.GeminiVoice)
	}
	return ""
}

func (p *Processor) openAIVoice() string {
	if voice := strings.TrimSpace(viper.GetString("audio.openai_voice")); voice != "" {
		return voice
	}
	if p != nil && p.flags != nil {
		return strings.TrimSpace(p.flags.OpenAIVoice)
	}
	return ""
}

func (p *Processor) audioVoicesForProvider() []string {
	switch p.audioProviderName() {
	case "gemini":
		return audio.GeminiVoices
	default:
		return audio.OpenAIVoices
	}
}

func (p *Processor) audioVoiceForProvider() string {
	switch p.audioProviderName() {
	case "gemini":
		return p.geminiVoice()
	default:
		if voice := p.openAIVoice(); voice != "" {
			return voice
		}
		voices := p.audioVoicesForProvider()
		return voices[rand.Intn(len(voices))]
	}
}

// generateAudio generates audio files for a word
func (p *Processor) generateAudio(word string) error {
	provider := p.audioProviderName()

	// Get the provider-specific voice list.
	var voices []string
	if p.flags.AllVoices {
		voices = p.audioVoicesForProvider()
	} else {
		voice := p.audioVoiceForProvider()
		switch provider {
		case "gemini":
			if voice != "" {
				fmt.Printf("  Using specified Gemini voice: %s\n", voice)
			} else {
				fmt.Printf("  Using Gemini model default voice\n")
			}
		default:
			if p.openAIVoice() != "" {
				fmt.Printf("  Using specified voice: %s\n", voice)
			} else {
				fmt.Printf("  Using random voice: %s\n", voice)
			}
		}
		voices = []string{voice}
	}

	// Generate audio for each voice
	for i, voice := range voices {
		if p.flags.AllVoices {
			fmt.Printf("  Generating audio %d/%d (voice: %s)...\n", i+1, len(voices), voice)
		}
		if err := p.generateAudioWithVoice(word, voice); err != nil {
			return fmt.Errorf("failed to generate audio with voice %s: %w", voice, err)
		}
	}

	return nil
}

// generateAudioBgBg generates audio files for both sides of a bg-bg card
func (p *Processor) generateAudioBgBg(front, back string) error {
	provider := p.audioProviderName()

	voice := p.audioVoiceForProvider()
	switch provider {
	case "gemini":
		if voice != "" {
			fmt.Printf("  Using specified Gemini voice: %s\n", voice)
		} else {
			fmt.Printf("  Using Gemini model default voice\n")
		}
	default:
		if p.openAIVoice() != "" {
			fmt.Printf("  Using specified voice: %s\n", voice)
		} else {
			fmt.Printf("  Using random voice: %s\n", voice)
		}
	}

	// Find or create the word directory ONCE (for the front word)
	// Both audio files will be saved to this same directory
	wordDir := p.findOrCreateWordDirectory(front)

	// Generate front audio
	fmt.Printf("  Generating front audio for '%s'...\n", front)
	if err := p.generateAudioWithVoiceAndFilenameInDir(front, voice, "audio_front", wordDir); err != nil {
		return fmt.Errorf("failed to generate front audio: %w", err)
	}

	// Generate back audio
	fmt.Printf("  Generating back audio for '%s'...\n", back)
	if err := p.generateAudioWithVoiceAndFilenameInDir(back, voice, "audio_back", wordDir); err != nil {
		return fmt.Errorf("failed to generate back audio: %w", err)
	}

	return nil
}

// generateAudioWithVoice generates audio for a word with a specific voice
func (p *Processor) generateAudioWithVoice(word, voice string) error {
	return p.generateAudioWithVoiceAndFilename(word, voice, "audio")
}

// generateAudioWithVoiceAndFilename generates audio for a word with a specific voice and filename
func (p *Processor) generateAudioWithVoiceAndFilename(word, voice, filenameBase string) error {
	wordDir := p.findOrCreateWordDirectory(word)
	return p.generateAudioWithVoiceAndFilenameInDir(word, voice, filenameBase, wordDir)
}

// generateAudioWithVoiceAndFilenameInDir generates audio for a word and saves it to a specific directory
func (p *Processor) generateAudioWithVoiceAndFilenameInDir(word, voice, filenameBase, wordDir string) error {
	audioProvider := p.audioProviderName()
	audioFormat := p.effectiveAudioFormat()

	// Generate random speed between 0.90 and 1.00 if not explicitly set
	speed := p.flags.OpenAISpeed
	if audioProvider == "openai" && p.flags.OpenAISpeed == 0.9 && !viper.IsSet("audio.openai_speed") {
		// Default was used, generate random speed
		speed = 0.90 + rand.Float64()*0.10
	}

	// Create audio provider configuration
	providerConfig := audio.DefaultProviderConfig()
	providerConfig.Provider = audioProvider
	providerConfig.OutputDir = p.flags.OutputDir
	providerConfig.OpenAIKey = cli.GetOpenAIKey()
	providerConfig.GoogleAPIKey = cli.GetGoogleAPIKey()

	switch audioProvider {
	case "gemini":
		providerConfig.OutputFormat = audioFormat
		providerConfig.GeminiTTSModel = p.geminiTTSModel()
		if voice != "" {
			providerConfig.GeminiVoice = voice
		} else {
			providerConfig.GeminiVoice = p.geminiVoice()
		}
		providerConfig.GeminiSpeed = 1.0
	default:
		providerConfig.OutputFormat = audioFormat
		providerConfig.OpenAIModel = p.flags.OpenAIModel
		providerConfig.OpenAIVoice = voice
		providerConfig.OpenAISpeed = speed
		providerConfig.OpenAIInstruction = p.flags.OpenAIInstruction

		// Use config file values if not overridden by flags
		if p.flags.OpenAIModel == "gpt-4o-mini-tts" && viper.IsSet("audio.openai_model") {
			providerConfig.OpenAIModel = viper.GetString("audio.openai_model")
		}
		if p.flags.OpenAISpeed == 0.9 && viper.IsSet("audio.openai_speed") {
			providerConfig.OpenAISpeed = viper.GetFloat64("audio.openai_speed")
		}
		if p.flags.OpenAIInstruction == "" && viper.IsSet("audio.openai_instruction") {
			providerConfig.OpenAIInstruction = viper.GetString("audio.openai_instruction")
		}
	}

	// Create the audio provider
	provider, err := newAudioProvider(providerConfig)
	if err != nil {
		return err
	}

	// Generate audio file
	ctx := context.Background()

	// Build filename using the provided base
	outputFormat := providerConfig.OutputFormat
	var outputFile string
	if p.flags.AllVoices && filenameBase == "audio" {
		outputFile = filepath.Join(wordDir, fmt.Sprintf("%s_%s.%s", filenameBase, voice, outputFormat))
	} else {
		outputFile = filepath.Join(wordDir, fmt.Sprintf("%s.%s", filenameBase, outputFormat))
	}

	// Generate the audio
	err = provider.GenerateAudio(ctx, word, outputFile)
	if err != nil {
		return err
	}

	// Save audio attribution
	if err := p.saveAudioAttribution(word, outputFile, providerConfig); err != nil {
		fmt.Printf("  Warning: Failed to save audio attribution: %v\n", err)
	}

	return nil
}

// downloadImagesWithTranslation downloads images for a word
func (p *Processor) downloadImagesWithTranslation(word, translationText string) error {
	searcher, err := p.newImageSearcher()
	if err != nil {
		return err
	}

	// Find existing card directory or create new one
	wordDir := p.findOrCreateWordDirectory(word)

	// Create downloader
	downloadOpts := &image.DownloadOptions{
		OutputDir:         wordDir,
		OverwriteExisting: true,
		CreateDir:         true,
		FileNamePattern:   "image",
		MaxSizeBytes:      5 * 1024 * 1024, // 5MB
	}

	downloader := image.NewDownloader(searcher, downloadOpts)

	// Create search options with translation if provided
	searchOpts := image.DefaultSearchOptions(word)
	if translationText != "" {
		searchOpts.Translation = translationText
	}

	type promptSetter interface {
		SetPromptCallback(func(prompt string))
	}
	if promptAware, ok := searcher.(promptSetter); ok {
		promptFile := filepath.Join(wordDir, "image_prompt.txt")
		promptAware.SetPromptCallback(func(prompt string) {
			if prompt == "" {
				return
			}
			if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
				fmt.Printf("  Warning: Failed to save image prompt: %v\n", err)
			}
		})
	}

	// Download single image
	ctx := context.Background()
	_, path, err := downloader.DownloadBestMatchWithOptions(ctx, searchOpts)
	if err != nil {
		return err
	}
	fmt.Printf("    Downloaded: %s\n", path)

	p.saveImagePrompt(wordDir, searcher)

	return nil
}

// GenerateAnkiFile generates the Anki import file and returns the output path
func (p *Processor) GenerateAnkiFile() (string, error) {
	// When --anki is used from CLI, save to home directory
	var outputDir string
	if p.flags.GenerateAnki {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		outputDir = homeDir
	} else {
		outputDir = p.flags.OutputDir
	}

	// Create Anki generator
	audioFormat := p.effectiveAudioFormat()
	gen := anki.NewGenerator(&anki.GeneratorOptions{
		OutputPath:     filepath.Join(outputDir, "anki_import.csv"),
		MediaFolder:    p.flags.OutputDir,
		IncludeHeaders: true,
		AudioFormat:    audioFormat,
	})

	// Use the translation cache as the source of truth for cards
	translations := p.translationCache.GetAll()
	if len(translations) == 0 {
		fmt.Println("  No translations found in cache, generating cards from directory...")
		// Fallback to old method if cache is empty but files might exist
		if err := gen.GenerateFromDirectory(p.flags.OutputDir); err != nil {
			return "", fmt.Errorf("failed to generate cards from directory: %w", err)
		}
	} else {
		fmt.Printf("  Generating cards from %d translations in cache...\n", len(translations))
		for bulgarian, english := range translations {
			card := anki.Card{
				Bulgarian:   bulgarian,
				Translation: english,
			}

			// Find associated media files in the output directory
			wordDir := p.findCardDirectory(bulgarian)
			if wordDir != "" {
				cardType := internal.LoadCardType(wordDir)
				if cardType.IsBgBg() {
					card.AudioFile = anki.ResolveAudioFile(wordDir, "audio_front", audioFormat)
					card.AudioFileBack = anki.ResolveAudioFile(wordDir, "audio_back", audioFormat)
				} else {
					card.AudioFile = anki.ResolveAudioFile(wordDir, "audio", audioFormat)
				}

				// Look for image file
				imageFile := filepath.Join(wordDir, "image.jpg") // Assuming jpg, adjust if needed
				if _, err := os.Stat(imageFile); err == nil {
					card.ImageFile = imageFile
				}

				// Load phonetic information as notes
				phoneticFile := filepath.Join(wordDir, "phonetic.txt")
				if data, err := os.ReadFile(phoneticFile); err == nil {
					notes := strings.TrimSpace(string(data))
					card.Notes = strings.ReplaceAll(notes, "\n", "<br>")
				}
			}
			gen.AddCard(card)
		}
	}

	var outputPath string
	if p.flags.AnkiCSV {
		// Generate CSV
		outputPath = filepath.Join(outputDir, "anki_import.csv")
		if err := gen.GenerateCSV(); err != nil {
			return "", fmt.Errorf("failed to generate CSV: %w", err)
		}
	} else {
		// Generate APKG
		outputPath = filepath.Join(outputDir, fmt.Sprintf("%s.apkg", internal.SanitizeFilename(p.flags.DeckName)))
		if err := gen.GenerateAPKG(outputPath, p.flags.DeckName); err != nil {
			return "", fmt.Errorf("failed to generate APKG: %w", err)
		}
	}

	// Print stats
	total, withAudio, withImages := gen.Stats()
	fmt.Printf("  Generated %d cards (%d with audio, %d with images)\n",
		total, withAudio, withImages)

	return outputPath, nil
}

// RunGUIMode launches the GUI application
func (p *Processor) RunGUIMode() error {
	guiConfig := p.guiConfigForRunMode()

	// Only set OutputDir if it was explicitly provided via flag
	// Check if the outputDir is different from the default
	home, _ := os.UserHomeDir()
	defaultOutputDir := filepath.Join(home, "Downloads")
	if p.flags.OutputDir != defaultOutputDir {
		// User explicitly set a different output directory
		guiConfig.OutputDir = p.flags.OutputDir
	}
	if guiConfig.GoogleAPIKey == "" {
		guiConfig.GoogleAPIKey = cli.GetGoogleAPIKey()
	}
	// Otherwise, gui.New will use its own default (XDG state directory)

	// Create and run GUI application
	app := gui.New(guiConfig)
	app.Run()

	return nil
}

func (p *Processor) guiConfigForRunMode() *gui.Config {
	imageProvider := p.flags.ImageAPI
	if !p.flags.ImageAPISpecified {
		imageProvider = gui.DefaultConfig().ImageProvider
	}

	return &gui.Config{
		AudioFormat:         p.effectiveAudioFormat(),
		AudioProvider:       p.audioProviderName(),
		ImageProvider:       imageProvider,
		OpenAIKey:           cli.GetOpenAIKey(),
		GoogleAPIKey:        cli.GetGoogleAPIKey(),
		NanoBananaModel:     p.nanoBananaModelForRunMode(),
		NanoBananaTextModel: p.nanoBananaTextModelForRunMode(),
		GeminiTTSModel:      p.geminiTTSModel(),
		GeminiVoice:         p.geminiVoice(),
		TranslationProvider: translation.Provider(viper.GetString("translation.provider")),
		PhoneticProvider:    phonetic.Provider(viper.GetString("phonetic.provider")),
		AutoPlay:            !p.flags.NoAutoPlay, // Invert the flag (--no-auto-play disables auto-play)
	}
}

func (p *Processor) nanoBananaModelForRunMode() string {
	if p != nil && p.flags != nil && p.flags.NanoBananaModelSpecified {
		if model := strings.TrimSpace(p.flags.NanoBananaModel); model != "" {
			return model
		}
	}

	if model := strings.TrimSpace(viper.GetString("image.nanobanana_model")); model != "" {
		return model
	}

	if p != nil && p.flags != nil {
		if model := strings.TrimSpace(p.flags.NanoBananaModel); model != "" {
			return model
		}
	}

	return image.DefaultNanoBananaModel
}

func (p *Processor) nanoBananaTextModelForRunMode() string {
	if p != nil && p.flags != nil && p.flags.NanoBananaTextModelSpecified {
		if model := strings.TrimSpace(p.flags.NanoBananaTextModel); model != "" {
			return model
		}
	}

	if model := strings.TrimSpace(viper.GetString("image.nanobanana_text_model")); model != "" {
		return model
	}

	if p != nil && p.flags != nil {
		if model := strings.TrimSpace(p.flags.NanoBananaTextModel); model != "" {
			return model
		}
	}

	return image.DefaultNanoBananaTextModel
}

func (p *Processor) newImageSearcher() (image.ImageSearcher, error) {
	provider := p.imageProviderForRunMode()

	switch provider {
	case "openai":
		return p.newOpenAIImageSearcher()
	case "nanobanana":
		return p.newNanoBananaImageSearcher()
	default:
		return nil, fmt.Errorf("unknown image provider: %s", provider)
	}
}

func (p *Processor) imageProviderForRunMode() string {
	if p.flags.ImageAPISpecified {
		return strings.ToLower(strings.TrimSpace(p.flags.ImageAPI))
	}

	if provider := strings.ToLower(strings.TrimSpace(viper.GetString("image.provider"))); provider != "" {
		return provider
	}

	return strings.ToLower(strings.TrimSpace(p.flags.ImageAPI))
}

func (p *Processor) newOpenAIImageSearcher() (image.ImageSearcher, error) {
	openaiConfig := &image.OpenAIConfig{
		APIKey:  cli.GetOpenAIKey(),
		Model:   p.flags.OpenAIImageModel,
		Size:    p.flags.OpenAIImageSize,
		Quality: p.flags.OpenAIImageQuality,
		Style:   p.flags.OpenAIImageStyle,
	}

	if p.flags.OpenAIImageModel == "dall-e-2" && viper.IsSet("image.openai_model") {
		openaiConfig.Model = viper.GetString("image.openai_model")
	}
	if p.flags.OpenAIImageSize == "512x512" && viper.IsSet("image.openai_size") {
		openaiConfig.Size = viper.GetString("image.openai_size")
	}
	if p.flags.OpenAIImageQuality == "standard" && viper.IsSet("image.openai_quality") {
		openaiConfig.Quality = viper.GetString("image.openai_quality")
	}
	if p.flags.OpenAIImageStyle == "natural" && viper.IsSet("image.openai_style") {
		openaiConfig.Style = viper.GetString("image.openai_style")
	}

	if openaiConfig.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required for image generation")
	}

	return newOpenAIImageClient(openaiConfig), nil
}

func (p *Processor) newNanoBananaImageSearcher() (image.ImageSearcher, error) {
	nanoBananaConfig := &image.NanoBananaConfig{
		APIKey:    cli.GetGoogleAPIKey(),
		Model:     p.flags.NanoBananaModel,
		TextModel: p.flags.NanoBananaTextModel,
	}

	if !p.flags.NanoBananaModelSpecified && viper.IsSet("image.nanobanana_model") {
		nanoBananaConfig.Model = viper.GetString("image.nanobanana_model")
	}
	if !p.flags.NanoBananaTextModelSpecified && viper.IsSet("image.nanobanana_text_model") {
		nanoBananaConfig.TextModel = viper.GetString("image.nanobanana_text_model")
	}

	if nanoBananaConfig.APIKey == "" {
		return nil, fmt.Errorf("Google API key is required for image generation")
	}

	return newNanoBananaImageClient(nanoBananaConfig), nil
}

func (p *Processor) saveImagePrompt(wordDir string, searcher image.ImageSearcher) {
	type promptGetter interface {
		GetLastPrompt() string
	}

	promptSource, ok := searcher.(promptGetter)
	if !ok {
		return
	}

	usedPrompt := promptSource.GetLastPrompt()
	if usedPrompt == "" {
		return
	}

	promptFile := filepath.Join(wordDir, "image_prompt.txt")
	if err := os.WriteFile(promptFile, []byte(usedPrompt), 0644); err != nil {
		fmt.Printf("  Warning: Failed to save image prompt: %v\n", err)
	}
}

// Helper methods

func (p *Processor) findOrCreateWordDirectory(word string) string {
	// Try to find existing directory first
	if dir := p.findCardDirectory(word); dir != "" {
		return dir
	}

	// No existing directory, create new one with card ID
	cardID := internal.GenerateCardID(word)
	wordDir := filepath.Join(p.flags.OutputDir, cardID)
	if err := os.MkdirAll(wordDir, 0755); err != nil {
		fmt.Printf("Warning: failed to create word directory: %v\n", err)
		return p.flags.OutputDir // Fallback to output directory
	}

	// Save word metadata
	metadataFile := filepath.Join(wordDir, "word.txt")
	if err := os.WriteFile(metadataFile, []byte(word), 0644); err != nil {
		fmt.Printf("Warning: failed to save word metadata: %v\n", err)
	}

	return wordDir
}

func (p *Processor) findCardDirectory(word string) string {
	entries, err := os.ReadDir(p.flags.OutputDir)
	if err != nil {
		return ""
	}

	// Look through all directories to find one with matching word.txt
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		dirPath := filepath.Join(p.flags.OutputDir, entry.Name())
		wordFile := filepath.Join(dirPath, "word.txt")

		// Read the word file to check if it matches
		if data, err := os.ReadFile(wordFile); err == nil {
			storedWord := strings.TrimSpace(string(data))
			if storedWord == word {
				return dirPath
			}
		}
	}

	return ""
}

// isWordFullyProcessed checks if a word has already been fully processed
func (p *Processor) isWordFullyProcessed(word string) bool {
	// Find the word directory
	wordDir := p.findCardDirectory(word)
	if wordDir == "" {
		return false // No directory exists
	}

	// Debug logging
	if os.Getenv("DEBUG_BATCH") != "" {
		fmt.Printf("  [DEBUG] Checking word directory: %s\n", wordDir)
	}

	// Check for required files
	requiredFiles := []string{
		"word.txt",        // Word metadata
		"translation.txt", // Translation file
		"phonetic.txt",    // Phonetic information
	}

	// Check for audio-related files (unless skipped)
	if !p.flags.SkipAudio {
		// Load card type to determine required audio files
		cardType := internal.LoadCardType(wordDir)
		audioFormat := p.effectiveAudioFormat()

		if cardType.IsBgBg() {
			frontAudioFiles := anki.ResolveAudioPaths(wordDir, "audio_front", audioFormat)
			backAudioFiles := anki.ResolveAudioPaths(wordDir, "audio_back", audioFormat)
			if len(frontAudioFiles) == 0 || len(backAudioFiles) == 0 {
				if os.Getenv("DEBUG_BATCH") != "" {
					fmt.Printf("  [DEBUG] No bg-bg audio files found in %s\n", wordDir)
				}
				return false
			}
			for _, audioFile := range append(frontAudioFiles, backAudioFiles...) {
				if _, err := os.Stat(audio.AttributionPath(audioFile)); os.IsNotExist(err) {
					if os.Getenv("DEBUG_BATCH") != "" {
						fmt.Printf("  [DEBUG] Missing attribution for audio file: %s\n", audioFile)
					}
					return false
				}
			}
		} else {
			// For en-bg cards, check for at least one resolved audio file and its metadata.
			requiredFiles = append(requiredFiles, "audio_metadata.txt")

			audioFiles := anki.ResolveAudioPaths(wordDir, "audio", audioFormat)
			if len(audioFiles) == 0 {
				if os.Getenv("DEBUG_BATCH") != "" {
					fmt.Printf("  [DEBUG] No audio files found in %s\n", wordDir)
				}
				return false
			}
			for _, audioFile := range audioFiles {
				if _, err := os.Stat(audio.AttributionPath(audioFile)); os.IsNotExist(err) {
					if os.Getenv("DEBUG_BATCH") != "" {
						fmt.Printf("  [DEBUG] Missing attribution for audio file: %s\n", audioFile)
					}
					return false
				}
			}
		}
	}

	// Check for image-related files (unless skipped)
	if !p.flags.SkipImages {
		// Add image-related files to required list
		requiredFiles = append(requiredFiles,
			"image_attribution.txt",
			"image_prompt.txt",
		)

		// Check for at least one image file
		imagePatterns := []string{"image_*.jpg", "image_*.png", "image_*.webp", "image.jpg", "image.png", "image.webp"}
		hasImage := false
		for _, pattern := range imagePatterns {
			if strings.Contains(pattern, "*") {
				matches, _ := filepath.Glob(filepath.Join(wordDir, pattern))
				if len(matches) > 0 {
					hasImage = true
					break
				}
			} else {
				// Direct file check
				if _, err := os.Stat(filepath.Join(wordDir, pattern)); err == nil {
					hasImage = true
					break
				}
			}
		}
		if !hasImage {
			if os.Getenv("DEBUG_BATCH") != "" {
				fmt.Printf("  [DEBUG] No image files found in %s\n", wordDir)
			}
			return false // No image files found
		}
	}

	// Check all required files exist
	for _, file := range requiredFiles {
		filePath := filepath.Join(wordDir, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			if os.Getenv("DEBUG_BATCH") != "" {
				fmt.Printf("  [DEBUG] Required file missing: %s\n", filePath)
			}
			return false // Required file missing
		}
	}

	if os.Getenv("DEBUG_BATCH") != "" {
		fmt.Printf("  [DEBUG] All required files exist, word is fully processed\n")
	}
	return true // All required files exist
}
func (p *Processor) saveAudioAttribution(word, audioFile string, config *audio.Config) error {
	processedText := audio.ProcessedTextForProvider(config.Provider, word)
	instruction := audio.InstructionForProvider(config.Provider, config)

	var attribution string
	switch strings.ToLower(strings.TrimSpace(config.Provider)) {
	case "gemini":
		attribution = audio.BuildGeminiAttribution(audio.AttributionParams{
			Word:          word,
			Model:         config.GeminiTTSModel,
			Voice:         config.GeminiVoice,
			Speed:         config.GeminiSpeed,
			Instruction:   instruction,
			ProcessedText: processedText,
			GeneratedAt:   time.Now(),
		})
	default:
		attribution = audio.BuildOpenAIAttribution(audio.AttributionParams{
			Word:          word,
			Model:         config.OpenAIModel,
			Voice:         config.OpenAIVoice,
			Speed:         config.OpenAISpeed,
			Instruction:   instruction,
			ProcessedText: processedText,
			GeneratedAt:   time.Now(),
		})
	}

	// Save to file
	attrPath := audio.AttributionPath(audioFile)
	if err := os.WriteFile(attrPath, []byte(attribution), 0644); err != nil {
		return fmt.Errorf("failed to write audio attribution file: %w", err)
	}

	// Also save metadata for GUI display
	wordDir := filepath.Dir(audioFile)
	metadataFile := filepath.Join(wordDir, "audio_metadata.txt")
	metadata := p.buildAudioMetadata(config, audioFile)
	if err := os.WriteFile(metadataFile, []byte(metadata), 0644); err != nil {
		// Non-fatal error, just log it
		fmt.Printf("Warning: Failed to save audio metadata: %v\n", err)
	}

	return nil
}

func (p *Processor) buildAudioMetadata(config *audio.Config, audioFile string) string {
	audioFileHint, audioFileBackHint := p.audioMetadataFileHints(audioFile)
	return audio.BuildSidecarMetadata(audio.SidecarMetadataParams{
		Provider:          config.Provider,
		OutputFormat:      config.OutputFormat,
		AudioFile:         audioFileHint,
		AudioFileBack:     audioFileBackHint,
		OpenAIModel:       config.OpenAIModel,
		OpenAIVoice:       config.OpenAIVoice,
		OpenAISpeed:       config.OpenAISpeed,
		OpenAIInstruction: config.OpenAIInstruction,
		GeminiTTSModel:    config.GeminiTTSModel,
		GeminiVoice:       config.GeminiVoice,
		GeminiSpeed:       config.GeminiSpeed,
	})
}

func (p *Processor) audioMetadataFileHints(audioFile string) (string, string) {
	if strings.TrimSpace(audioFile) == "" {
		return "", ""
	}

	wordDir := filepath.Dir(audioFile)
	base := filepath.Base(audioFile)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	switch name {
	case "audio":
		return audioFile, ""
	case "audio_front":
		backFile := filepath.Join(wordDir, "audio_back"+ext)
		if _, err := os.Stat(backFile); err == nil {
			return audioFile, backFile
		}
		return audioFile, ""
	case "audio_back":
		frontFile := filepath.Join(wordDir, "audio_front"+ext)
		return frontFile, audioFile
	default:
		return audioFile, ""
	}
}
