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

// viperConfig holds all Viper-sourced settings captured once in NewProcessor.
// Storing them in a struct avoids repeated global Viper access in method bodies
// and makes the values testable without mutating process-wide Viper state.
type viperConfig struct {
	// Translation & phonetic settings
	translationProvider    string
	phoneticProvider       string
	translationGeminiModel string

	// Audio settings
	audioProvider        string
	audioFormat          string
	audioFormatSet       bool
	geminiTTSModel       string
	geminiVoice          string
	openAIVoice          string
	openAIModel          string
	openAIModelSet       bool
	openAISpeed          float64
	openAISpeedSet       bool
	openAIInstruction    string
	openAIInstructionSet bool

	// Image settings
	imageProvider               string
	imageOpenAIModel            string
	imageOpenAIModelSet         bool
	imageOpenAISize             string
	imageOpenAISizeSet          bool
	imageOpenAIQuality          string
	imageOpenAIQualitySet       bool
	imageOpenAIStyle            string
	imageOpenAIStyleSet         bool
	imageNanoBananaModel        string
	imageNanoBananaModelSet     bool
	imageNanoBananaTextModel    string
	imageNanoBananaTextModelSet bool
}

// newViperConfig reads all Viper settings in one pass. Called once from NewProcessor
// so the processor methods never touch the global Viper instance directly.
func newViperConfig() viperConfig {
	return viperConfig{
		translationProvider:    strings.TrimSpace(viper.GetString("translation.provider")),
		phoneticProvider:       strings.TrimSpace(viper.GetString("phonetic.provider")),
		translationGeminiModel: viper.GetString("translation.gemini_model"),

		audioProvider:        strings.ToLower(strings.TrimSpace(viper.GetString("audio.provider"))),
		audioFormat:          strings.ToLower(strings.TrimSpace(viper.GetString("audio.format"))),
		audioFormatSet:       viper.IsSet("audio.format"),
		geminiTTSModel:       strings.TrimSpace(viper.GetString("audio.gemini_tts_model")),
		geminiVoice:          strings.TrimSpace(viper.GetString("audio.gemini_voice")),
		openAIVoice:          strings.TrimSpace(viper.GetString("audio.openai_voice")),
		openAIModel:          viper.GetString("audio.openai_model"),
		openAIModelSet:       viper.IsSet("audio.openai_model"),
		openAISpeed:          viper.GetFloat64("audio.openai_speed"),
		openAISpeedSet:       viper.IsSet("audio.openai_speed"),
		openAIInstruction:    viper.GetString("audio.openai_instruction"),
		openAIInstructionSet: viper.IsSet("audio.openai_instruction"),

		imageProvider:               strings.ToLower(strings.TrimSpace(viper.GetString("image.provider"))),
		imageOpenAIModel:            viper.GetString("image.openai_model"),
		imageOpenAIModelSet:         viper.IsSet("image.openai_model"),
		imageOpenAISize:             viper.GetString("image.openai_size"),
		imageOpenAISizeSet:          viper.IsSet("image.openai_size"),
		imageOpenAIQuality:          viper.GetString("image.openai_quality"),
		imageOpenAIQualitySet:       viper.IsSet("image.openai_quality"),
		imageOpenAIStyle:            viper.GetString("image.openai_style"),
		imageOpenAIStyleSet:         viper.IsSet("image.openai_style"),
		imageNanoBananaModel:        strings.TrimSpace(viper.GetString("image.nanobanana_model")),
		imageNanoBananaModelSet:     viper.IsSet("image.nanobanana_model"),
		imageNanoBananaTextModel:    strings.TrimSpace(viper.GetString("image.nanobanana_text_model")),
		imageNanoBananaTextModelSet: viper.IsSet("image.nanobanana_text_model"),
	}
}

// Processor handles the main word processing logic.
// Audio coordination is in audio_coordinator.go, card directory management is
// in card_store.go, and image downloading is in image_downloader.go.
// The factory fields (newOpenAIImageClient, newNanoBananaImageClient, newAudioProvider)
// are injected at construction time so tests can swap them without mutating global state.
type Processor struct {
	flags            *cli.Flags
	translator       *translation.Translator
	translationCache *translation.TranslationCache
	phoneticFetcher  *phonetic.Fetcher
	randomIntn       func(n int) int
	// viperCfg holds all config-file values read once at construction time,
	// so individual methods never call Viper directly.
	viperCfg viperConfig

	// Factories — replaced by tests to inject fakes.
	newOpenAIImageClient     func(*image.OpenAIConfig) image.ImageClient
	newNanoBananaImageClient func(*image.NanoBananaConfig) image.ImageClient
	newAudioProvider         func(*audio.Config) (audio.Provider, error)
}

// NewProcessor creates a new word processor with default production factories.
// All Viper config values are read once here via newViperConfig() so that no
// method body ever calls Viper directly.
// Tests can replace the factory fields on the returned struct to inject fakes.
func NewProcessor(flags *cli.Flags) *Processor {
	cfg := newViperConfig()
	openAIKey := cli.GetOpenAIKey()
	googleAPIKey := cli.GetGoogleAPIKey()
	translationProvider := translation.Provider(cfg.translationProvider)
	phoneticProvider := phonetic.Provider(cfg.phoneticProvider)
	return &Processor{
		flags:            flags,
		viperCfg:         cfg,
		translator:       translation.NewTranslator(&translation.Config{Provider: translationProvider, OpenAIKey: openAIKey, GoogleAPIKey: googleAPIKey}),
		translationCache: translation.NewTranslationCache(),
		phoneticFetcher:  phonetic.NewFetcher(&phonetic.Config{Provider: phoneticProvider, OpenAIKey: openAIKey, GoogleAPIKey: googleAPIKey}),
		randomIntn:       rand.Intn,
		newOpenAIImageClient: func(config *image.OpenAIConfig) image.ImageClient {
			return image.NewOpenAIClient(config)
		},
		newNanoBananaImageClient: func(config *image.NanoBananaConfig) image.ImageClient {
			return image.NewNanoBananaClient(config)
		},
		newAudioProvider: audio.NewProvider,
	}
}

// ProcessBatch processes multiple words from a batch file.
// It first translates any entries that have English-to-Bulgarian translation
// needs, then validates all Bulgarian words, and finally processes each word
// with a per-word timeout to prevent a single hung API call from stalling the batch.
func (p *Processor) ProcessBatch() error {
	entries, err := batch.ReadBatchFile(p.flags.BatchFile)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(p.flags.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := p.translateBatchEntries(entries); err != nil {
		return err
	}

	if err := p.validateBatchEntries(entries); err != nil {
		return err
	}

	skipped, processed, errCount := p.processBatchEntries(entries)

	p.printBatchSummary(len(entries), processed, skipped, errCount)
	return nil
}

// translateBatchEntries runs the first pass over entries that need English→Bulgarian
// translation and mutates the slice in place with the result.
func (p *Processor) translateBatchEntries(entries []batch.WordEntry) error {
	for i, entry := range entries {
		if !entry.NeedsTranslation || entry.Translation == "" {
			continue
		}
		bulgarian, err := p.translator.TranslateEnglishToBulgarian(entry.Translation)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error translating '%s' to Bulgarian: %v\n", entry.Translation, err)
			continue
		}
		entries[i].Bulgarian = bulgarian
		fmt.Printf("Translated '%s' to Bulgarian: %s\n", entry.Translation, bulgarian)
	}
	return nil
}

// validateBatchEntries checks that every entry with a Bulgarian word contains
// only valid Bulgarian text. Returns on the first validation failure.
func (p *Processor) validateBatchEntries(entries []batch.WordEntry) error {
	for _, entry := range entries {
		if entry.Bulgarian == "" {
			continue
		}
		if err := audio.ValidateBulgarianText(entry.Bulgarian); err != nil {
			return fmt.Errorf("invalid word '%s': %w", entry.Bulgarian, err)
		}
	}
	return nil
}

// processBatchEntries iterates the validated entries and processes each word,
// skipping words that are already fully processed. Returns skip, process, and
// error counts for the summary.
func (p *Processor) processBatchEntries(entries []batch.WordEntry) (skipped, processed, errCount int) {
	for i, entry := range entries {
		if entry.Bulgarian == "" {
			continue
		}

		fmt.Printf("\nProcessing %d/%d: %s\n", i+1, len(entries), entry.Bulgarian)

		if p.isWordFullyProcessed(entry.Bulgarian) {
			wordDir := p.findCardDirectory(entry.Bulgarian)
			fmt.Printf("  ✓ Skipping '%s' - already fully processed in %s\n", entry.Bulgarian, filepath.Base(wordDir))
			skipped++
			continue
		}

		// Per-word timeout so a single hung API call cannot stall the whole batch.
		// 5 minutes is generous for audio TTS + image download.
		wordCtx, wordCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		err := p.ProcessWordWithTranslationAndType(wordCtx, entry.Bulgarian, entry.Translation, entry.CardType)
		wordCancel() // release resources even on success
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing '%s': %v\n", entry.Bulgarian, err)
			errCount++
		} else {
			processed++
		}
	}
	return
}

// printBatchSummary prints a human-readable summary of the batch run.
func (p *Processor) printBatchSummary(total, processed, skipped, errCount int) {
	fmt.Printf("\n=== Batch Processing Summary ===\n")
	fmt.Printf("Total words: %d\n", total)
	fmt.Printf("Processed: %d\n", processed)
	fmt.Printf("Skipped (already complete): %d\n", skipped)
	if errCount > 0 {
		fmt.Printf("Errors: %d\n", errCount)
	}
	fmt.Printf("================================\n")
}

// ProcessSingleWord validates and processes a single word from the command line.
func (p *Processor) ProcessSingleWord(word string) error {
	if err := audio.ValidateBulgarianText(word); err != nil {
		return fmt.Errorf("invalid word '%s': %w", word, err)
	}

	if err := os.MkdirAll(p.flags.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Printf("\nProcessing: %s\n", word)
	return p.ProcessWordWithTranslation(word, "")
}

// ProcessWordWithTranslation processes a word with an optional provided English
// translation, using the default en-bg card type.
func (p *Processor) ProcessWordWithTranslation(word, providedTranslation string) error {
	return p.ProcessWordWithTranslationAndType(context.Background(), word, providedTranslation, internal.CardTypeEnBg)
}

// ProcessWordWithTranslationAndType processes a word with optional provided
// translation and card type. ctx is used for all downstream API calls (audio
// TTS, image generation) so the caller can cancel or time-out the operation.
// ProcessBatch passes a per-word deadline; callers without a deadline may pass
// context.Background().
func (p *Processor) ProcessWordWithTranslationAndType(ctx context.Context, word, providedTranslation string, cardType internal.CardType) error {
	translationText := p.resolveTranslation(ctx, word, providedTranslation, cardType)

	wordDir := p.findOrCreateWordDirectory(word)

	if err := internal.SaveCardType(wordDir, cardType); err != nil {
		fmt.Printf("  Warning: Failed to save card type: %v\n", err)
	}

	if err := p.saveTranslationIfNeeded(word, translationText, wordDir); err != nil {
		fmt.Printf("  Warning: Failed to save translation: %v\n", err)
	}

	fmt.Printf("  Fetching phonetic information...\n")
	if err := p.phoneticFetcher.FetchAndSave(word, wordDir); err != nil {
		fmt.Printf("  Warning: Failed to fetch phonetic info: %v\n", err)
	} else {
		fmt.Printf("  Saved phonetic information\n")
	}

	if !p.flags.SkipAudio {
		fmt.Printf("  Generating audio...\n")
		if err := p.generateAudioForCard(ctx, word, translationText, cardType); err != nil {
			return fmt.Errorf("audio generation failed: %w", err)
		}
	}

	if !p.flags.SkipImages {
		fmt.Printf("  Downloading images...\n")
		if err := p.downloadImagesWithTranslation(ctx, word, translationText); err != nil {
			return fmt.Errorf("image download failed: %w", err)
		}
	}

	return nil
}

// resolveTranslation determines the effective translation text for the word.
// For bg-bg cards it uses the provided definition; for en-bg cards it fetches
// an English translation when none was provided.
func (p *Processor) resolveTranslation(_ context.Context, word, providedTranslation string, cardType internal.CardType) string {
	if providedTranslation != "" {
		if cardType.IsBgBg() {
			fmt.Printf("  Using provided definition: %s\n", providedTranslation)
		} else {
			fmt.Printf("  Using provided translation: %s\n", providedTranslation)
		}
		return providedTranslation
	}

	if cardType.IsBgBg() {
		// bg-bg cards do not need an English translation.
		return ""
	}

	fmt.Printf("  Translating to English...\n")
	translationText, err := p.translator.TranslateWord(word)
	if err != nil {
		fmt.Printf("  Warning: Translation failed: %v\n", err)
		return ""
	}
	fmt.Printf("  Translation: %s\n", translationText)
	return translationText
}

// saveTranslationIfNeeded stores the translation in the in-memory cache and
// writes translation.txt to wordDir if the file does not already exist.
func (p *Processor) saveTranslationIfNeeded(word, translationText, wordDir string) error {
	if translationText == "" {
		return nil
	}

	p.translationCache.Add(word, translationText)

	translationFile := filepath.Join(wordDir, "translation.txt")
	if _, err := os.Stat(translationFile); os.IsNotExist(err) {
		return translation.SaveTranslation(wordDir, word, translationText)
	}

	fmt.Printf("  Translation file already exists\n")
	return nil
}

// generateAudioForCard dispatches audio generation to the appropriate helper
// based on card type. bg-bg cards need audio for both front and back sides.
func (p *Processor) generateAudioForCard(ctx context.Context, word, translationText string, cardType internal.CardType) error {
	if cardType.IsBgBg() {
		return p.generateAudioBgBg(ctx, word, translationText)
	}
	return p.generateAudio(ctx, word)
}

// GenerateAnkiFile generates the Anki import file and returns the output path.
// When --anki is specified the file is placed in the user's home directory;
// otherwise it goes into the configured output directory.
func (p *Processor) GenerateAnkiFile() (string, error) {
	outputDir, err := p.resolveAnkiOutputDir()
	if err != nil {
		return "", err
	}

	audioFormat := p.effectiveAudioFormat()
	gen := anki.NewGenerator(&anki.GeneratorOptions{
		OutputPath:     filepath.Join(outputDir, "anki_import.csv"),
		MediaFolder:    p.flags.OutputDir,
		IncludeHeaders: true,
		AudioFormat:    audioFormat,
	})

	if err := p.populateAnkiGenerator(gen, audioFormat); err != nil {
		return "", err
	}

	return p.writeAnkiOutput(gen, outputDir)
}

// resolveAnkiOutputDir returns the directory where the Anki file should be
// written. When --anki is set it resolves to the user's home directory.
func (p *Processor) resolveAnkiOutputDir() (string, error) {
	if p.flags.GenerateAnki {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return homeDir, nil
	}
	return p.flags.OutputDir, nil
}

// populateAnkiGenerator fills the generator with cards. When the translation
// cache is populated it is used as the authoritative source; otherwise the
// generator falls back to scanning the output directory for existing cards.
func (p *Processor) populateAnkiGenerator(gen *anki.Generator, audioFormat string) error {
	translations := p.translationCache.GetAll()
	if len(translations) == 0 {
		fmt.Println("  No translations found in cache, generating cards from directory...")
		if err := gen.GenerateFromDirectory(p.flags.OutputDir); err != nil {
			return fmt.Errorf("failed to generate cards from directory: %w", err)
		}
		return nil
	}

	fmt.Printf("  Generating cards from %d translations in cache...\n", len(translations))
	for bulgarian, english := range translations {
		card := p.buildAnkiCard(bulgarian, english, audioFormat)
		gen.AddCard(card)
	}
	return nil
}

// buildAnkiCard constructs an anki.Card for a word, resolving all associated
// media files (audio, image, phonetic) from the word's card directory.
func (p *Processor) buildAnkiCard(bulgarian, english, audioFormat string) anki.Card {
	card := anki.Card{
		Bulgarian:   bulgarian,
		Translation: english,
	}

	wordDir := p.findCardDirectory(bulgarian)
	if wordDir == "" {
		return card
	}

	cardType := internal.LoadCardType(wordDir)
	if cardType.IsBgBg() {
		card.AudioFile = anki.ResolveAudioFile(wordDir, "audio_front", audioFormat)
		card.AudioFileBack = anki.ResolveAudioFile(wordDir, "audio_back", audioFormat)
	} else {
		card.AudioFile = anki.ResolveAudioFile(wordDir, "audio", audioFormat)
	}

	// Image file (prefer .jpg; the downloader may use other extensions).
	imageFile := filepath.Join(wordDir, "image.jpg")
	if _, err := os.Stat(imageFile); err == nil {
		card.ImageFile = imageFile
	}

	// Phonetic notes (newlines replaced with HTML line breaks for Anki).
	phoneticFile := filepath.Join(wordDir, "phonetic.txt")
	if data, err := os.ReadFile(phoneticFile); err == nil {
		notes := strings.TrimSpace(string(data))
		card.Notes = strings.ReplaceAll(notes, "\n", "<br>")
	}

	return card
}

// writeAnkiOutput generates either a CSV or APKG file depending on the
// --anki-csv flag and returns the output path.
func (p *Processor) writeAnkiOutput(gen *anki.Generator, outputDir string) (string, error) {
	if p.flags.AnkiCSV {
		outputPath := filepath.Join(outputDir, "anki_import.csv")
		if err := gen.GenerateCSV(); err != nil {
			return "", fmt.Errorf("failed to generate CSV: %w", err)
		}
		p.printAnkiStats(gen)
		return outputPath, nil
	}

	outputPath := filepath.Join(outputDir, fmt.Sprintf("%s.apkg", internal.SanitizeFilename(p.flags.DeckName)))
	if err := gen.GenerateAPKG(outputPath, p.flags.DeckName); err != nil {
		return "", fmt.Errorf("failed to generate APKG: %w", err)
	}
	p.printAnkiStats(gen)
	return outputPath, nil
}

// printAnkiStats logs the card generation statistics to stdout.
func (p *Processor) printAnkiStats(gen *anki.Generator) {
	total, withAudio, withImages := gen.Stats()
	fmt.Printf("  Generated %d cards (%d with audio, %d with images)\n", total, withAudio, withImages)
}

// GUIConfig returns a gui.Config populated from the processor's flags and
// Viper settings. Callers (typically cmd/main.go) use this to construct the
// GUI application so that gui.New() lives outside the processor package and
// the processor→gui dependency is limited to the Config type only.
func (p *Processor) GUIConfig() *gui.Config {
	imageProvider := p.flags.ImageAPI
	if !p.flags.ImageAPISpecified {
		imageProvider = gui.DefaultConfig().ImageProvider
	}

	openAIKey := cli.GetOpenAIKey()
	googleAPIKey := cli.GetGoogleAPIKey()
	translationProvider := translation.Provider(p.viperCfg.translationProvider)
	phoneticProvider := phonetic.Provider(p.viperCfg.phoneticProvider)

	// Construct and inject phonetic/translation dependencies at the composition
	// root so gui.New() receives ready-to-use instances rather than raw config strings.
	phoneticFetcher := phonetic.NewFetcher(&phonetic.Config{
		Provider:     phoneticProvider,
		OpenAIKey:    openAIKey,
		GoogleAPIKey: googleAPIKey,
	})
	translator := translation.NewTranslator(&translation.Config{
		Provider:    translationProvider,
		OpenAIKey:   openAIKey,
		GeminiModel: p.viperCfg.translationGeminiModel,
	})

	return &gui.Config{
		AudioFormat:         p.effectiveAudioFormat(),
		AudioProvider:       p.audioProviderName(),
		ImageProvider:       imageProvider,
		OpenAIKey:           openAIKey,
		GoogleAPIKey:        googleAPIKey,
		NanoBananaModel:     p.nanoBananaModelForRunMode(),
		NanoBananaTextModel: p.nanoBananaTextModelForRunMode(),
		GeminiTTSModel:      p.geminiTTSModel(),
		GeminiVoice:         p.geminiVoice(),
		TranslationProvider: translationProvider,
		PhoneticProvider:    phoneticProvider,
		AutoPlay:            !p.flags.NoAutoPlay, // Invert the flag (--no-auto-play disables auto-play)
		PhoneticFetcher:     phoneticFetcher,
		Translator:          translator,
	}
}

// nanoBananaModelForRunMode resolves the NanoBanana image model, preferring
// the explicit CLI flag value when set, then the viper config value, then the
// package default.
func (p *Processor) nanoBananaModelForRunMode() string {
	if p != nil && p.flags != nil && p.flags.NanoBananaModelSpecified {
		if model := strings.TrimSpace(p.flags.NanoBananaModel); model != "" {
			return model
		}
	}
	if p.viperCfg.imageNanoBananaModel != "" {
		return p.viperCfg.imageNanoBananaModel
	}
	if p != nil && p.flags != nil {
		if model := strings.TrimSpace(p.flags.NanoBananaModel); model != "" {
			return model
		}
	}
	return image.DefaultNanoBananaModel
}

// nanoBananaTextModelForRunMode resolves the NanoBanana text (prompt) model
// using the same precedence as nanoBananaModelForRunMode.
func (p *Processor) nanoBananaTextModelForRunMode() string {
	if p != nil && p.flags != nil && p.flags.NanoBananaTextModelSpecified {
		if model := strings.TrimSpace(p.flags.NanoBananaTextModel); model != "" {
			return model
		}
	}
	if p.viperCfg.imageNanoBananaTextModel != "" {
		return p.viperCfg.imageNanoBananaTextModel
	}
	if p != nil && p.flags != nil {
		if model := strings.TrimSpace(p.flags.NanoBananaTextModel); model != "" {
			return model
		}
	}
	return image.DefaultNanoBananaTextModel
}
