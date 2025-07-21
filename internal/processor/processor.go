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

// NewProcessor creates a new word processor
func NewProcessor(flags *cli.Flags) *Processor {
	apiKey := cli.GetOpenAIKey()
	return &Processor{
		flags:            flags,
		translator:       translation.NewTranslator(apiKey),
		translationCache: translation.NewTranslationCache(),
		phoneticFetcher:  phonetic.NewFetcher(apiKey),
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

		if err := p.ProcessWordWithTranslation(entry.Bulgarian, entry.Translation); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing '%s': %v\n", entry.Bulgarian, err)
			errorCount++
			// Continue with next word
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

// ProcessWordWithTranslation processes a word with optional provided translation
func (p *Processor) ProcessWordWithTranslation(word, providedTranslation string) error {
	var translationText string

	// Use provided translation if available, otherwise translate
	if providedTranslation != "" {
		translationText = providedTranslation
		fmt.Printf("  Using provided translation: %s\n", translationText)
	} else {
		// Translate the word first
		fmt.Printf("  Translating to English...\n")
		var err error
		translationText, err = p.translator.TranslateWord(word)
		if err != nil {
			fmt.Printf("  Warning: Translation failed: %v\n", err)
			translationText = "" // Continue without translation
		} else {
			fmt.Printf("  Translation: %s\n", translationText)
		}
	}

	// Store translation for Anki export
	if translationText != "" {
		p.translationCache.Add(word, translationText)

		// Find or create word directory
		wordDir := p.findOrCreateWordDirectory(word)

		// Check if translation file already exists
		translationFile := filepath.Join(wordDir, "translation.txt")
		if _, err := os.Stat(translationFile); os.IsNotExist(err) {
			// Save translation to file
			if err := translation.SaveTranslation(wordDir, word, translationText); err != nil {
				fmt.Printf("  Warning: Failed to save translation: %v\n", err)
			}
		} else {
			fmt.Printf("  Translation file already exists\n")
		}
	}

	// Fetch phonetic information
	fmt.Printf("  Fetching phonetic information...\n")
	wordDir := p.findOrCreateWordDirectory(word)
	if err := p.phoneticFetcher.FetchAndSave(word, wordDir); err != nil {
		// Don't fail the whole process if phonetic info fails
		fmt.Printf("  Warning: Failed to fetch phonetic info: %v\n", err)
	} else {
		fmt.Printf("  Saved phonetic information\n")
	}

	// Generate audio
	if !p.flags.SkipAudio {
		fmt.Printf("  Generating audio...\n")
		if err := p.generateAudio(word); err != nil {
			return fmt.Errorf("audio generation failed: %w", err)
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

// generateAudio generates audio files for a word
func (p *Processor) generateAudio(word string) error {
	allVoicesList := []string{"alloy", "ash", "ballad", "coral", "echo", "fable", "onyx", "nova", "sage", "shimmer", "verse"}

	// Get list of voices to use
	var voices []string
	if p.flags.AllVoices {
		voices = allVoicesList
	} else if p.flags.OpenAIVoice != "" {
		// Use explicitly specified voice
		voices = []string{p.flags.OpenAIVoice}
		fmt.Printf("  Using specified voice: %s\n", p.flags.OpenAIVoice)
	} else {
		// Select a random voice
		randomVoice := allVoicesList[rand.Intn(len(allVoicesList))]
		voices = []string{randomVoice}
		fmt.Printf("  Using random voice: %s\n", randomVoice)
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

// generateAudioWithVoice generates audio for a word with a specific voice
func (p *Processor) generateAudioWithVoice(word, voice string) error {
	// Generate random speed between 0.90 and 1.00 if not explicitly set
	speed := p.flags.OpenAISpeed
	if p.flags.OpenAISpeed == 0.9 && !viper.IsSet("audio.openai_speed") {
		// Default was used, generate random speed
		speed = 0.90 + rand.Float64()*0.10
	}

	// Create audio provider configuration
	providerConfig := &audio.Config{
		Provider:     "openai",
		OutputDir:    p.flags.OutputDir,
		OutputFormat: p.flags.AudioFormat,

		// OpenAI settings
		OpenAIKey:         cli.GetOpenAIKey(),
		OpenAIModel:       p.flags.OpenAIModel,
		OpenAIVoice:       voice,
		OpenAISpeed:       speed,
		OpenAIInstruction: p.flags.OpenAIInstruction,

		// Caching
		EnableCache: viper.GetBool("audio.enable_cache"),
		CacheDir:    viper.GetString("audio.cache_dir"),
	}

	// Set defaults
	if providerConfig.CacheDir == "" {
		providerConfig.CacheDir = "./.audio_cache"
	}

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

	// Create the audio provider
	provider, err := audio.NewProvider(providerConfig)
	if err != nil {
		return err
	}

	// Generate audio file
	ctx := context.Background()

	// Find existing card directory or create new one
	wordDir := p.findOrCreateWordDirectory(word)

	// Add voice name to filename if generating multiple voices
	var outputFile string
	if p.flags.AllVoices {
		outputFile = filepath.Join(wordDir, fmt.Sprintf("audio_%s.%s", voice, p.flags.AudioFormat))
	} else {
		outputFile = filepath.Join(wordDir, fmt.Sprintf("audio.%s", p.flags.AudioFormat))
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
	// Create image searcher based on provider
	var searcher image.ImageSearcher

	switch p.flags.ImageAPI {
	case "openai":
		// Create OpenAI image configuration
		openaiConfig := &image.OpenAIConfig{
			APIKey:  cli.GetOpenAIKey(),
			Model:   p.flags.OpenAIImageModel,
			Size:    p.flags.OpenAIImageSize,
			Quality: p.flags.OpenAIImageQuality,
			Style:   p.flags.OpenAIImageStyle,
		}

		// Use config file values if not overridden by flags
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

		searcher = image.NewOpenAIClient(openaiConfig)
		if openaiConfig.APIKey == "" {
			return fmt.Errorf("OpenAI API key is required for image generation")
		}

	default:
		return fmt.Errorf("unknown image provider: %s", p.flags.ImageAPI)
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

	// Download single image
	ctx := context.Background()
	_, path, err := downloader.DownloadBestMatchWithOptions(ctx, searchOpts)
	if err != nil {
		return err
	}
	fmt.Printf("    Downloaded: %s\n", path)

	// If using OpenAI, save the prompt
	if p.flags.ImageAPI == "openai" {
		if openaiClient, ok := searcher.(*image.OpenAIClient); ok {
			usedPrompt := openaiClient.GetLastPrompt()
			if usedPrompt != "" {
				promptFile := filepath.Join(wordDir, "image_prompt.txt")
				if err := os.WriteFile(promptFile, []byte(usedPrompt), 0644); err != nil {
					fmt.Printf("  Warning: Failed to save image prompt: %v\n", err)
				}
			}
		}
	}

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
	gen := anki.NewGenerator(&anki.GeneratorOptions{
		OutputPath:     filepath.Join(outputDir, "anki_import.csv"),
		MediaFolder:    p.flags.OutputDir,
		IncludeHeaders: true,
		AudioFormat:    p.flags.AudioFormat,
	})

	// Generate cards from output directory
	if err := gen.GenerateFromDirectory(p.flags.OutputDir); err != nil {
		return "", fmt.Errorf("failed to generate cards: %w", err)
	}

	// Add translations to cards
	translations := p.translationCache.GetAll()
	for i := range gen.GetCards() {
		if translation, ok := translations[gen.GetCards()[i].Bulgarian]; ok {
			gen.GetCards()[i].Translation = translation
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
	// Create GUI configuration from command line flags and viper config
	guiConfig := &gui.Config{
		AudioFormat:   p.flags.AudioFormat,
		ImageProvider: p.flags.ImageAPI,
		OpenAIKey:     cli.GetOpenAIKey(),
		AutoPlay:      !p.flags.NoAutoPlay, // Invert the flag (--no-auto-play disables auto-play)
	}

	// Only set OutputDir if it was explicitly provided via flag
	// Check if the outputDir is different from the default
	home, _ := os.UserHomeDir()
	defaultOutputDir := filepath.Join(home, "Downloads")
	if p.flags.OutputDir != defaultOutputDir {
		// User explicitly set a different output directory
		guiConfig.OutputDir = p.flags.OutputDir
	}
	// Otherwise, gui.New will use its own default (XDG state directory)

	// Create and run GUI application
	app := gui.New(guiConfig)
	app.Run()

	return nil
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
		// Add audio-related files to required list
		requiredFiles = append(requiredFiles,
			"audio_attribution.txt",
			"audio_metadata.txt",
		)

		// Check for audio file (without voice suffix for single voice mode)
		audioFile := filepath.Join(wordDir, fmt.Sprintf("audio.%s", p.flags.AudioFormat))
		if _, err := os.Stat(audioFile); os.IsNotExist(err) {
			// Also check for audio files with voice suffix (for all-voices mode)
			audioPattern := fmt.Sprintf("audio_*.%s", p.flags.AudioFormat)
			matches, _ := filepath.Glob(filepath.Join(wordDir, audioPattern))
			if len(matches) == 0 {
				if os.Getenv("DEBUG_BATCH") != "" {
					fmt.Printf("  [DEBUG] No audio file found: %s or pattern %s\n", audioFile, audioPattern)
				}
				return false // No audio file found
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
	// Create attribution text
	attribution := fmt.Sprintf("Audio generated by OpenAI TTS\n\n")
	attribution += fmt.Sprintf("Bulgarian word: %s\n", word)
	attribution += fmt.Sprintf("Model: %s\n", config.OpenAIModel)
	attribution += fmt.Sprintf("Voice: %s\n", config.OpenAIVoice)
	attribution += fmt.Sprintf("Speed: %.2f\n", config.OpenAISpeed)

	if config.OpenAIInstruction != "" {
		attribution += fmt.Sprintf("\nVoice instructions:\n%s\n", config.OpenAIInstruction)
	}

	// Add preprocessing information
	cleanedWord := strings.TrimSpace(word)
	punctuationToRemove := []string{"!", "?", ".", ",", ";", ":", "\"", "'", "(", ")", "[", "]", "{", "}", "-", "—", "–"}
	for _, punct := range punctuationToRemove {
		cleanedWord = strings.ReplaceAll(cleanedWord, punct, "")
	}
	processedText := fmt.Sprintf("%s...", strings.TrimSpace(cleanedWord))
	attribution += fmt.Sprintf("\nProcessed text sent to TTS: %s\n", processedText)

	attribution += fmt.Sprintf("\nGenerated at: %s\n", time.Now().Format("2006-01-02 15:04:05"))

	// Save to file
	attrPath := strings.TrimSuffix(audioFile, filepath.Ext(audioFile)) + "_attribution.txt"
	if err := os.WriteFile(attrPath, []byte(attribution), 0644); err != nil {
		return fmt.Errorf("failed to write audio attribution file: %w", err)
	}

	// Also save metadata for GUI display
	wordDir := filepath.Dir(audioFile)
	metadataFile := filepath.Join(wordDir, "audio_metadata.txt")
	metadata := fmt.Sprintf("voice=%s\nspeed=%.2f\n", config.OpenAIVoice, config.OpenAISpeed)
	if err := os.WriteFile(metadataFile, []byte(metadata), 0644); err != nil {
		// Non-fatal error, just log it
		fmt.Printf("Warning: Failed to save audio metadata: %v\n", err)
	}

	return nil
}
