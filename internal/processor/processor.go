package processor

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"

	"codeberg.org/snonux/totalrecall/internal"
	"codeberg.org/snonux/totalrecall/internal/audio"
	"codeberg.org/snonux/totalrecall/internal/cli"
	"codeberg.org/snonux/totalrecall/internal/httpctx"
	"codeberg.org/snonux/totalrecall/internal/image"
	"codeberg.org/snonux/totalrecall/internal/phonetic"
	"codeberg.org/snonux/totalrecall/internal/store"
	"codeberg.org/snonux/totalrecall/internal/translation"
)

// Config holds all configuration-file values resolved once at startup by the
// composition root (cmd/totalrecall/main.go via cli.NewProcessorConfig).
// Keeping config in a dedicated struct means the processor package never
// imports or queries Viper directly, which improves testability and removes
// tight coupling to the global Viper singleton.
type Config struct {
	// Translation & phonetic settings
	TranslationProvider    string
	PhoneticProvider       string
	TranslationGeminiModel string

	// Audio settings
	AudioProvider        string
	AudioFormat          string
	AudioFormatSet       bool
	GeminiTTSModel       string
	GeminiVoice          string
	OpenAIVoice          string
	OpenAIModel          string
	OpenAIModelSet       bool
	OpenAISpeed          float64
	OpenAISpeedSet       bool
	OpenAIInstruction    string
	OpenAIInstructionSet bool

	// Image settings
	ImageProvider               string
	ImageOpenAIModel            string
	ImageOpenAIModelSet         bool
	ImageOpenAISize             string
	ImageOpenAISizeSet          bool
	ImageOpenAIQuality          string
	ImageOpenAIQualitySet       bool
	ImageOpenAIStyle            string
	ImageOpenAIStyleSet         bool
	ImageNanoBananaModel        string
	ImageNanoBananaModelSet     bool
	ImageNanoBananaTextModel    string
	ImageNanoBananaTextModelSet bool
}

// Processor handles the main word processing logic.
// Audio coordination is in audio_coordinator.go, card directory management is
// in card_store.go, and image downloading is in image_downloader.go.
// Batch orchestration lives in batch_processor.go; Anki export in anki_exporter.go;
// CLI vs file config precedence in cli_config_resolver.go.
// Factory functions for image and audio providers are grouped in image.ClientFactories
// and the audio.ProviderFactory type so the signatures are defined once and
// shared with the gui package — eliminating parallel field duplication.
type Processor struct {
	*CLIConfigResolver
	translator       *translation.Translator
	translationCache *translation.TranslationCache
	phoneticFetcher  *phonetic.Fetcher
	randomIntn       func(n int) int

	// cardStore is the shared CardStore for locating and creating on-disk
	// card directories. It is initialised from flags.OutputDir in NewProcessor
	// and used by all card_store.go helpers.
	cardStore *store.CardStore

	// imageFactories groups the two image-provider construction functions.
	// Production code uses image.DefaultClientFactories(); tests replace fields.
	imageFactories image.ClientFactories

	// newAudioProvider constructs an audio.Provider from a Config.
	// Production code uses audio.NewProvider; tests replace it with a fake.
	newAudioProvider audio.ProviderFactory

	batchProcessor *BatchProcessor
	ankiExporter   *AnkiExporter
}

// NewProcessor creates a new word processor with default production factories.
// cfg must be fully resolved before calling NewProcessor; the composition root
// (cmd/totalrecall/main.go) builds it via cli.NewProcessorConfig() so that
// the processor package never imports or queries Viper.
// Tests can replace the factory fields on the returned struct to inject fakes.
func NewProcessor(flags *cli.Flags, cfg *Config) *Processor {
	openAIKey := cli.GetOpenAIKey()
	googleAPIKey := cli.GetGoogleAPIKey()
	translationProvider := translation.Provider(cfg.TranslationProvider)
	phoneticProvider := phonetic.Provider(cfg.PhoneticProvider)
	p := &Processor{
		CLIConfigResolver: &CLIConfigResolver{Flags: flags, Config: cfg},
		translator:       translation.NewTranslator(&translation.Config{Provider: translationProvider, OpenAIKey: openAIKey, GoogleAPIKey: googleAPIKey}),
		translationCache: translation.NewTranslationCache(),
		phoneticFetcher:  phonetic.NewFetcher(&phonetic.Config{Provider: phoneticProvider, OpenAIKey: openAIKey, GoogleAPIKey: googleAPIKey}),
		randomIntn:       rand.Intn,
		cardStore:        store.New(flags.OutputDir),
		imageFactories:   image.DefaultClientFactories(),
		newAudioProvider: audio.NewProvider,
	}
	p.batchProcessor = &BatchProcessor{p: p}
	p.ankiExporter = &AnkiExporter{p: p}
	return p
}

// ProcessBatch processes multiple words from a batch file.
func (p *Processor) ProcessBatch() error {
	return p.batchProcessor.ProcessBatch()
}

// ProcessSingleWord validates and processes a single word from the command line.
func (p *Processor) ProcessSingleWord(word string) error {
	if err := audio.ValidateBulgarianText(word); err != nil {
		return fmt.Errorf("invalid word '%s': %w", word, err)
	}

	if err := os.MkdirAll(p.Flags.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Printf("\nProcessing: %s\n", word)
	return p.ProcessWordWithTranslation(word, "")
}

// ProcessWordWithTranslation processes a word with an optional provided English
// translation, using the default en-bg card type.
func (p *Processor) ProcessWordWithTranslation(word, providedTranslation string) error {
	ctx, cancel := context.WithTimeout(context.Background(), httpctx.SingleWordProcessTimeout)
	defer cancel()
	return p.ProcessWordWithTranslationAndType(ctx, word, providedTranslation, internal.CardTypeEnBg)
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
		return fmt.Errorf("failed to save card type: %w", err)
	}

	if err := p.saveTranslationIfNeeded(word, translationText, wordDir); err != nil {
		return fmt.Errorf("failed to save translation: %w", err)
	}

	fmt.Printf("  Fetching phonetic information...\n")
	if err := p.phoneticFetcher.FetchAndSave(word, wordDir); err != nil {
		return fmt.Errorf("failed to fetch phonetic info: %w", err)
	}
	fmt.Printf("  Saved phonetic information\n")

	if !p.Flags.SkipAudio {
		fmt.Printf("  Generating audio...\n")
		if err := p.generateAudioForCard(ctx, word, translationText, cardType); err != nil {
			return fmt.Errorf("audio generation failed: %w", err)
		}
	}

	if !p.Flags.SkipImages {
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
func (p *Processor) GenerateAnkiFile() (string, error) {
	return p.ankiExporter.GenerateAnkiFile()
}
