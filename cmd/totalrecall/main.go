package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	
	"codeberg.org/snonux/totalrecall/internal"
	"codeberg.org/snonux/totalrecall/internal/anki"
	"codeberg.org/snonux/totalrecall/internal/audio"
	"codeberg.org/snonux/totalrecall/internal/gui"
	"codeberg.org/snonux/totalrecall/internal/image"
)

var (
	// Flags
	cfgFile      string
	// voice removed - was only for espeak
	outputDir    string
	audioFormat  string
	imageAPI     string
	batchFile    string
	skipAudio    bool
	skipImages   bool
	generateAnki bool
	ankiCSV      bool
	deckName     string
	listModels   bool
	allVoices    bool
	guiMode      bool
	// Audio provider flags removed - now only OpenAI
	// OpenAI flags
	openAIModel       string
	openAIVoice       string
	openAISpeed       float64
	openAIInstruction string
	// OpenAI Image flags
	openAIImageModel   string
	openAIImageSize    string
	openAIImageQuality string
	openAIImageStyle   string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "totalrecall [word]",
	Short: "Bulgarian Anki Flashcard Generator",
	Long: `totalrecall generates Anki flashcard materials from Bulgarian words.

It creates audio pronunciation files using OpenAI TTS and downloads
representative images from web search APIs.

Example:
  totalrecall ябълка              # Generate materials for "apple"
  totalrecall --batch words.txt   # Process multiple words from file`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCommand,
	Version: internal.Version,
}

func init() {
	cobra.OnInitialize(initConfig)
	
	// Initialize random number generator
	rand.Seed(time.Now().UnixNano())
	
	// Set default output directory to Downloads
	home, _ := os.UserHomeDir()
	defaultOutputDir := filepath.Join(home, "Downloads")
	
	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.totalrecall.yaml)")
	
	// Local flags
	rootCmd.Flags().StringVarP(&outputDir, "output", "o", defaultOutputDir, "Output directory")
	rootCmd.Flags().StringVarP(&audioFormat, "format", "f", "mp3", "Audio format (wav or mp3)")
	rootCmd.Flags().StringVar(&imageAPI, "image-api", "openai", "Image source (only openai supported)")
	rootCmd.Flags().StringVar(&batchFile, "batch", "", "Process words from file (one per line)")
	rootCmd.Flags().BoolVar(&skipAudio, "skip-audio", false, "Skip audio generation")
	rootCmd.Flags().BoolVar(&skipImages, "skip-images", false, "Skip image download")
	rootCmd.Flags().BoolVar(&generateAnki, "anki", false, "Generate Anki import file (APKG format by default, use --anki-csv for legacy CSV)")
	rootCmd.Flags().BoolVar(&ankiCSV, "anki-csv", false, "Generate legacy CSV format instead of APKG when using --anki")
	rootCmd.Flags().StringVar(&deckName, "deck-name", "Bulgarian Vocabulary", "Deck name for APKG export")
	rootCmd.Flags().BoolVar(&listModels, "list-models", false, "List available OpenAI models for the current API key")
	rootCmd.Flags().BoolVar(&allVoices, "all-voices", false, "Generate audio in all available voices (creates multiple files)")
	rootCmd.Flags().BoolVar(&guiMode, "gui", false, "Launch interactive GUI mode")
	
	// Audio provider removed - now only OpenAI
	
	// OpenAI flags
	rootCmd.Flags().StringVar(&openAIModel, "openai-model", "gpt-4o-mini-tts", "OpenAI TTS model: tts-1, tts-1-hd, gpt-4o-mini-tts")
	rootCmd.Flags().StringVar(&openAIVoice, "openai-voice", "", "OpenAI voice: alloy, ash, ballad, coral, echo, fable, onyx, nova, sage, shimmer, verse (default: random)")
	rootCmd.Flags().Float64Var(&openAISpeed, "openai-speed", 0.9, "OpenAI speech speed (0.25 to 4.0, may be ignored by gpt-4o-mini-tts)")
	rootCmd.Flags().StringVar(&openAIInstruction, "openai-instruction", "", "Voice instructions for gpt-4o-mini-tts model (e.g., 'speak slowly with a Bulgarian accent')")
	
	// OpenAI Image Generation flags
	rootCmd.Flags().StringVar(&openAIImageModel, "openai-image-model", "dall-e-3", "OpenAI image model: dall-e-2 or dall-e-3")
	rootCmd.Flags().StringVar(&openAIImageSize, "openai-image-size", "1024x1024", "Image size: 256x256, 512x512, 1024x1024 (dall-e-3: also 1024x1792, 1792x1024)")
	rootCmd.Flags().StringVar(&openAIImageQuality, "openai-image-quality", "standard", "Image quality: standard or hd (dall-e-3 only)")
	rootCmd.Flags().StringVar(&openAIImageStyle, "openai-image-style", "natural", "Image style: natural or vivid (dall-e-3 only)")
	
	// Bind flags to viper
	viper.BindPFlag("audio.provider", rootCmd.Flags().Lookup("audio-provider"))
	viper.BindPFlag("audio.voice", rootCmd.Flags().Lookup("voice"))
	viper.BindPFlag("audio.format", rootCmd.Flags().Lookup("format"))
	viper.BindPFlag("audio.pitch", rootCmd.Flags().Lookup("pitch"))
	viper.BindPFlag("audio.amplitude", rootCmd.Flags().Lookup("amplitude"))
	viper.BindPFlag("audio.word_gap", rootCmd.Flags().Lookup("word-gap"))
	viper.BindPFlag("audio.openai_model", rootCmd.Flags().Lookup("openai-model"))
	viper.BindPFlag("audio.openai_voice", rootCmd.Flags().Lookup("openai-voice"))
	viper.BindPFlag("audio.openai_speed", rootCmd.Flags().Lookup("openai-speed"))
	viper.BindPFlag("audio.openai_instruction", rootCmd.Flags().Lookup("openai-instruction"))
	viper.BindPFlag("output.directory", rootCmd.Flags().Lookup("output"))
	viper.BindPFlag("image.provider", rootCmd.Flags().Lookup("image-api"))
	// Bind OpenAI image flags
	viper.BindPFlag("image.openai_model", rootCmd.Flags().Lookup("openai-image-model"))
	viper.BindPFlag("image.openai_size", rootCmd.Flags().Lookup("openai-image-size"))
	viper.BindPFlag("image.openai_quality", rootCmd.Flags().Lookup("openai-image-quality"))
	viper.BindPFlag("image.openai_style", rootCmd.Flags().Lookup("openai-image-style"))
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		
		// Search config in home directory with name ".totalrecall" (without extension)
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".totalrecall")
	}
	
	// Environment variables
	viper.SetEnvPrefix("TOTALRECALL")
	viper.AutomaticEnv()
	
	// Read config file
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

func runCommand(cmd *cobra.Command, args []string) error {
	// Check if output directory was set in config file
	if !cmd.Flags().Changed("output") && viper.IsSet("output.directory") {
		outputDir = viper.GetString("output.directory")
	}
	
	// Handle --list-models flag
	if listModels {
		return listAvailableModels()
	}
	
	// Handle --gui flag
	if guiMode {
		return runGUIMode()
	}
	
	// Structure to hold word and optional translation
	type wordEntry struct {
		bulgarian   string
		translation string
	}
	
	// Determine words to process
	var entries []wordEntry
	
	if batchFile != "" {
		// Read words from file
		content, err := os.ReadFile(batchFile)
		if err != nil {
			return fmt.Errorf("failed to read batch file: %w", err)
		}
		// Split by newlines and filter empty lines
		lines := string(content)
		for _, line := range splitLines(lines) {
			if line = trimSpace(line); line != "" {
				// Check if line contains '=' for bulgarian = english format
				if strings.Contains(line, "=") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						bulgarian := strings.TrimSpace(parts[0])
						english := strings.TrimSpace(parts[1])
						if bulgarian != "" {
							entries = append(entries, wordEntry{
								bulgarian:   bulgarian,
								translation: english,
							})
						}
					}
				} else {
					// Just a bulgarian word
					entries = append(entries, wordEntry{
						bulgarian:   line,
						translation: "",
					})
				}
			}
		}
	} else if len(args) > 0 {
		// Single word from command line
		entries = []wordEntry{{bulgarian: args[0], translation: ""}}
	} else {
		// No input provided
		return fmt.Errorf("please provide a Bulgarian word or use --batch flag")
	}
	
	// Validate words
	for _, entry := range entries {
		if err := audio.ValidateBulgarianText(entry.bulgarian); err != nil {
			return fmt.Errorf("invalid word '%s': %w", entry.bulgarian, err)
		}
	}
	
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	
	// Process each entry
	for i, entry := range entries {
		fmt.Printf("\nProcessing %d/%d: %s\n", i+1, len(entries), entry.bulgarian)
		
		if err := processWordWithTranslation(entry.bulgarian, entry.translation); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing '%s': %v\n", entry.bulgarian, err)
			// Continue with next word
		}
	}
	
	// Generate Anki file if requested
	if generateAnki {
		fmt.Printf("\nGenerating Anki import file...\n")
		if err := generateAnkiFile(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to generate Anki file: %v\n", err)
		} else {
			if ankiCSV {
				fmt.Println("Anki import file created: anki_import.csv")
			} else {
				fmt.Printf("Anki package created: %s.apkg\n", deckName)
			}
		}
	}
	
	fmt.Println("\nDone! Materials saved to:", outputDir)
	return nil
}

func processWord(word string) error {
	return processWordWithTranslation(word, "")
}

func processWordWithTranslation(word, providedTranslation string) error {
	var translation string
	
	// Use provided translation if available, otherwise translate
	if providedTranslation != "" {
		translation = providedTranslation
		fmt.Printf("  Using provided translation: %s\n", translation)
	} else {
		// Translate the word first
		fmt.Printf("  Translating to English...\n")
		var err error
		translation, err = translateWord(word)
		if err != nil {
			fmt.Printf("  Warning: Translation failed: %v\n", err)
			translation = "" // Continue without translation
		} else {
			fmt.Printf("  Translation: %s\n", translation)
		}
	}
	
	// Store translation for Anki export
	if translation != "" {
		wordTranslations[word] = translation
		// Save translation to file
		if err := saveTranslation(word, translation); err != nil {
			fmt.Printf("  Warning: Failed to save translation: %v\n", err)
		}
	}
	
	// Generate audio
	if !skipAudio {
		fmt.Printf("  Generating audio...\n")
		if err := generateAudio(word); err != nil {
			return fmt.Errorf("audio generation failed: %w", err)
		}
	}
	
	// Download images - pass the translation for better image generation
	if !skipImages {
		fmt.Printf("  Downloading images...\n")
		if err := downloadImagesWithTranslation(word, translation); err != nil {
			return fmt.Errorf("image download failed: %w", err)
		}
	}
	
	// Fetch phonetic information
	fmt.Printf("  Fetching phonetic information...\n")
	if err := fetchAndSavePhoneticInfo(word); err != nil {
		// Don't fail the whole process if phonetic info fails
		fmt.Printf("  Warning: Failed to fetch phonetic info: %v\n", err)
	}
	
	return nil
}

func generateAudio(word string) error {
	allVoicesList := []string{"alloy", "ash", "ballad", "coral", "echo", "fable", "onyx", "nova", "sage", "shimmer", "verse"}
	
	// Get list of voices to use
	var voices []string
	if allVoices {
		voices = allVoicesList
	} else if openAIVoice != "" {
		// Use explicitly specified voice
		voices = []string{openAIVoice}
		fmt.Printf("  Using specified voice: %s\n", openAIVoice)
	} else {
		// Select a random voice
		randomVoice := allVoicesList[rand.Intn(len(allVoicesList))]
		voices = []string{randomVoice}
		fmt.Printf("  Using random voice: %s\n", randomVoice)
	}
	
	// Generate audio for each voice
	for i, voice := range voices {
		if allVoices {
			fmt.Printf("  Generating audio %d/%d (voice: %s)...\n", i+1, len(voices), voice)
		}
		if err := generateAudioWithVoice(word, voice); err != nil {
			return fmt.Errorf("failed to generate audio with voice %s: %w", voice, err)
		}
	}
	
	return nil
}

func generateAudioWithVoice(word, voice string) error {
	// Generate random speed between 0.90 and 1.00 if not explicitly set
	speed := openAISpeed
	if openAISpeed == 0.9 && !viper.IsSet("audio.openai_speed") {
		// Default was used, generate random speed
		speed = 0.90 + rand.Float64()*0.10
	}
	
	// Create audio provider configuration
	providerConfig := &audio.Config{
		Provider:     "openai",
		OutputDir:    outputDir,
		OutputFormat: audioFormat,
		
		// OpenAI settings
		OpenAIKey:         getOpenAIKey(),
		OpenAIModel:       openAIModel,
		OpenAIVoice:       voice,
		OpenAISpeed:       speed,
		OpenAIInstruction: openAIInstruction,
		
		// Caching
		EnableCache: viper.GetBool("audio.enable_cache"),
		CacheDir:    viper.GetString("audio.cache_dir"),
	}
	
	// Set defaults
	if providerConfig.CacheDir == "" {
		providerConfig.CacheDir = "./.audio_cache"
	}
	
	// Use config file values if not overridden by flags
	if openAIModel == "gpt-4o-mini-tts" && viper.IsSet("audio.openai_model") {
		providerConfig.OpenAIModel = viper.GetString("audio.openai_model")
	}
	// Voice is always random or specified via command line, not from config
	if openAISpeed == 0.9 && viper.IsSet("audio.openai_speed") {
		providerConfig.OpenAISpeed = viper.GetFloat64("audio.openai_speed")
	}
	if openAIInstruction == "" && viper.IsSet("audio.openai_instruction") {
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
	wordDir := findCardDirectory(word)
	if wordDir == "" {
		// No existing directory, create new one with card ID
		cardID := internal.GenerateCardID(word)
		wordDir = filepath.Join(outputDir, cardID)
		if err := os.MkdirAll(wordDir, 0755); err != nil {
			return fmt.Errorf("failed to create word directory: %w", err)
		}
		
		// Save word metadata
		metadataFile := filepath.Join(wordDir, "word.txt")
		if err := os.WriteFile(metadataFile, []byte(word), 0644); err != nil {
			return fmt.Errorf("failed to save word metadata: %w", err)
		}
	}
	
	// Add voice name to filename if generating multiple voices
	var outputFile string
	if allVoices {
		outputFile = filepath.Join(wordDir, fmt.Sprintf("audio_%s.%s", voice, audioFormat))
	} else {
		outputFile = filepath.Join(wordDir, fmt.Sprintf("audio.%s", audioFormat))
	}
	
	// Generate the audio
	err = provider.GenerateAudio(ctx, word, outputFile)
	if err != nil {
		return err
	}
	
	// Save audio attribution
	if err := saveAudioAttribution(word, outputFile, providerConfig); err != nil {
		fmt.Printf("  Warning: Failed to save audio attribution: %v\n", err)
	}
	
	return nil
}

func downloadImages(word string) error {
	return downloadImagesWithTranslation(word, "")
}

func downloadImagesWithTranslation(word, translation string) error {
	// Create image searcher based on provider
	var searcher image.ImageSearcher
	var err error
	
	switch imageAPI {
	case "openai":
		// Create OpenAI image configuration
		openaiConfig := &image.OpenAIConfig{
			APIKey:      getOpenAIKey(),
			Model:       openAIImageModel,
			Size:        openAIImageSize,
			Quality:     openAIImageQuality,
			Style:       openAIImageStyle,
		}
		
		// Use config file values if not overridden by flags
		if openAIImageModel == "dall-e-3" && viper.IsSet("image.openai_model") {
			openaiConfig.Model = viper.GetString("image.openai_model")
		}
		if openAIImageSize == "1024x1024" && viper.IsSet("image.openai_size") {
			openaiConfig.Size = viper.GetString("image.openai_size")
		}
		if openAIImageQuality == "standard" && viper.IsSet("image.openai_quality") {
			openaiConfig.Quality = viper.GetString("image.openai_quality")
		}
		if openAIImageStyle == "natural" && viper.IsSet("image.openai_style") {
			openaiConfig.Style = viper.GetString("image.openai_style")
		}
		
		
		searcher = image.NewOpenAIClient(openaiConfig)
		if openaiConfig.APIKey == "" {
			return fmt.Errorf("OpenAI API key is required for image generation")
		}
		
	default:
		return fmt.Errorf("unknown image provider: %s", imageAPI)
	}
	
	// Find existing card directory or create new one
	wordDir := findCardDirectory(word)
	if wordDir == "" {
		// No existing directory, create new one with card ID
		cardID := internal.GenerateCardID(word)
		wordDir = filepath.Join(outputDir, cardID)
		if err := os.MkdirAll(wordDir, 0755); err != nil {
			return fmt.Errorf("failed to create word directory: %w", err)
		}
		
		// Save word metadata
		metadataFile := filepath.Join(wordDir, "word.txt")
		if err := os.WriteFile(metadataFile, []byte(word), 0644); err != nil {
			return fmt.Errorf("failed to save word metadata: %w", err)
		}
	}
	
	// Create downloader
	downloadOpts := &image.DownloadOptions{
		OutputDir:         wordDir,
		OverwriteExisting: true,  // Allow overwriting existing files
		CreateDir:         true,
		FileNamePattern:   "image",
		MaxSizeBytes:      5 * 1024 * 1024, // 5MB
	}
	
	downloader := image.NewDownloader(searcher, downloadOpts)
	
	// Create search options with translation if provided
	searchOpts := image.DefaultSearchOptions(word)
	if translation != "" {
		searchOpts.Translation = translation
	}
	
	// Download single image
	ctx := context.Background()
	_, path, err := downloader.DownloadBestMatchWithOptions(ctx, searchOpts)
	if err != nil {
		return err
	}
	fmt.Printf("    Downloaded: %s\n", path)
	
	// If using OpenAI, save the prompt
	if imageAPI == "openai" {
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


func splitLines(s string) []string {
	// Simple line splitter
	var lines []string
	current := ""
	for _, r := range s {
		if r == '\n' {
			lines = append(lines, current)
			current = ""
		} else if r != '\r' {
			current += string(r)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func trimSpace(s string) string {
	// Simple trim implementation
	start := 0
	end := len(s)
	
	// Trim from start
	for start < end && isSpace(rune(s[start])) {
		start++
	}
	
	// Trim from end
	for end > start && isSpace(rune(s[end-1])) {
		end--
	}
	
	return s[start:end]
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

func generateAnkiFile() error {
	// Create Anki generator
	gen := anki.NewGenerator(&anki.GeneratorOptions{
		OutputPath:     filepath.Join(outputDir, "anki_import.csv"),
		MediaFolder:    outputDir,
		IncludeHeaders: true,
		AudioFormat:    audioFormat,
	})
	
	// Generate cards from output directory
	if err := gen.GenerateFromDirectory(outputDir); err != nil {
		return fmt.Errorf("failed to generate cards: %w", err)
	}
	
	// Add translations to cards
	for i := range gen.GetCards() {
		if translation, ok := wordTranslations[gen.GetCards()[i].Bulgarian]; ok {
			gen.GetCards()[i].Translation = translation
		}
	}
	
	if ankiCSV {
		// Generate CSV
		if err := gen.GenerateCSV(); err != nil {
			return fmt.Errorf("failed to generate CSV: %w", err)
		}
	} else {
		// Generate APKG
		outputPath := filepath.Join(outputDir, fmt.Sprintf("%s.apkg", internal.SanitizeFilename(deckName)))
		if err := gen.GenerateAPKG(outputPath, deckName); err != nil {
			return fmt.Errorf("failed to generate APKG: %w", err)
		}
	}
	
	// Print stats
	total, withAudio, withImages := gen.Stats()
	fmt.Printf("  Generated %d cards (%d with audio, %d with images)\n", 
		total, withAudio, withImages)
	
	return nil
}

func getOpenAIKey() string {
	// First check environment variable
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return key
	}
	
	// Then check config file
	return viper.GetString("audio.openai_key")
}

func listAvailableModels() error {
	// Get OpenAI API key
	apiKey := getOpenAIKey()
	if apiKey == "" {
		return fmt.Errorf("OpenAI API key not found. Set OPENAI_API_KEY environment variable or configure in .totalrecall.yaml")
	}
	
	// Create OpenAI client
	client := openai.NewClient(apiKey)
	
	// List models
	ctx := context.Background()
	models, err := client.ListModels(ctx)
	if err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}
	
	// Categorize models
	ttsModels := []string{}
	imageModels := []string{}
	chatModels := []string{}
	
	for _, model := range models.Models {
		modelID := model.ID
		if strings.Contains(modelID, "tts") || strings.Contains(modelID, "audio") {
			ttsModels = append(ttsModels, modelID)
		} else if strings.Contains(modelID, "dall-e") {
			imageModels = append(imageModels, modelID)
		} else if strings.Contains(modelID, "gpt") || strings.Contains(modelID, "chat") {
			chatModels = append(chatModels, modelID)
		}
	}
	
	// Sort models
	sort.Strings(ttsModels)
	sort.Strings(imageModels)
	sort.Strings(chatModels)
	
	// Print models
	fmt.Println("Available OpenAI Models:")
	fmt.Println("\nText-to-Speech (TTS) Models:")
	if len(ttsModels) == 0 {
		fmt.Println("  No TTS models found")
	} else {
		for _, model := range ttsModels {
			fmt.Printf("  %s\n", model)
		}
	}
	
	fmt.Println("\nImage Generation Models:")
	if len(imageModels) == 0 {
		fmt.Println("  No image models found")
	} else {
		for _, model := range imageModels {
			fmt.Printf("  %s\n", model)
		}
	}
	
	fmt.Println("\nChat/Translation Models (for Bulgarian translation):")
	if len(chatModels) > 10 {
		// Show only relevant models
		relevantModels := []string{}
		for _, model := range chatModels {
			if strings.Contains(model, "gpt-4") || strings.Contains(model, "gpt-3.5") {
				relevantModels = append(relevantModels, model)
			}
		}
		for _, model := range relevantModels {
			fmt.Printf("  %s\n", model)
		}
		fmt.Printf("  ... and %d more models\n", len(chatModels)-len(relevantModels))
	} else {
		for _, model := range chatModels {
			fmt.Printf("  %s\n", model)
		}
	}
	
	return nil
}

func translateWord(word string) (string, error) {
	// Use OpenAI to translate Bulgarian to English
	apiKey := getOpenAIKey()
	if apiKey == "" {
		return "", fmt.Errorf("OpenAI API key not found")
	}
	
	client := openai.NewClient(apiKey)
	ctx := context.Background()
	
	req := openai.ChatCompletionRequest{
		Model: openai.GPT4oMini,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: fmt.Sprintf("Translate the Bulgarian word '%s' to English. Respond with only the English translation, nothing else.", word),
			},
		},
		MaxTokens:   50,
		Temperature: 0.3,
	}
	
	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("OpenAI API error: %w", err)
	}
	
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no translation returned")
	}
	
	translation := strings.TrimSpace(resp.Choices[0].Message.Content)
	return translation, nil
}

func saveTranslation(word, translation string) error {
	// Find existing card directory or create new one
	wordDir := findCardDirectory(word)
	if wordDir == "" {
		// No existing directory, create new one with card ID
		cardID := internal.GenerateCardID(word)
		wordDir = filepath.Join(outputDir, cardID)
		
		// Ensure directory exists
		if err := os.MkdirAll(wordDir, 0755); err != nil {
			return fmt.Errorf("failed to create word directory: %w", err)
		}
		
		// Save word metadata
		metadataFile := filepath.Join(wordDir, "word.txt")
		if err := os.WriteFile(metadataFile, []byte(word), 0644); err != nil {
			return fmt.Errorf("failed to save word metadata: %w", err)
		}
	}
	
	outputFile := filepath.Join(wordDir, "translation.txt")
	
	content := fmt.Sprintf("%s = %s\n", word, translation)
	
	if err := os.WriteFile(outputFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write translation file: %w", err)
	}
	
	return nil
}

// Global map to store translations for Anki export
var wordTranslations = make(map[string]string)

// findCardDirectory finds the directory for a given Bulgarian word
func findCardDirectory(word string) string {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return ""
	}
	
	// Look through all directories to find one with matching word.txt
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		
		dirPath := filepath.Join(outputDir, entry.Name())
		wordFile := filepath.Join(dirPath, "word.txt")
		
		// Read the word file to check if it matches
		if data, err := os.ReadFile(wordFile); err == nil {
			storedWord := strings.TrimSpace(string(data))
			if storedWord == word {
				return dirPath
			}
		} else {
			// Try old format with underscore for backward compatibility
			wordFile = filepath.Join(dirPath, "_word.txt")
			if data, err := os.ReadFile(wordFile); err == nil {
				storedWord := strings.TrimSpace(string(data))
				if storedWord == word {
					return dirPath
				}
			}
		}
	}
	
	return ""
}

func saveAudioAttribution(word, audioFile string, config *audio.Config) error {
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

func fetchAndSavePhoneticInfo(word string) error {
	// Check if OpenAI key is available
	apiKey := getOpenAIKey()
	if apiKey == "" {
		return fmt.Errorf("OpenAI API key not configured")
	}

	client := openai.NewClient(apiKey)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := openai.ChatCompletionRequest{
		Model: openai.GPT4o,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a Bulgarian language expert helping language learners understand pronunciation. Provide detailed phonetic information using the International Phonetic Alphabet (IPA). For each IPA symbol used, give concrete examples of how it sounds using familiar English words or sounds when possible.",
			},
			{
				Role: openai.ChatMessageRoleUser,
				Content: fmt.Sprintf(`For the Bulgarian word '%s':
1. Provide the complete IPA transcription
2. Break down EACH phonetic symbol used in the transcription
3. For EVERY symbol, explain how it's pronounced with examples:
   - If similar to an English sound, give English word examples
   - If not in English, describe tongue/mouth position or compare to similar sounds
   - Include stress marks and explain which syllable is stressed

Example format:
Word: [IPA transcription]
• /p/ - like 'p' in English 'pot'
• /a/ - like 'a' in 'father'
• /ˈ/ - stress mark (following syllable is stressed)`, word),
			},
		},
		Temperature: 0.3,
		MaxTokens:   500,
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return fmt.Errorf("no response from OpenAI")
	}

	phoneticInfo := strings.TrimSpace(resp.Choices[0].Message.Content)
	
	// Find the card directory for this word
	wordDir := findCardDirectory(word)
	if wordDir == "" {
		return fmt.Errorf("card directory not found for word: %s", word)
	}
	
	// Save phonetic info to file
	phoneticFile := filepath.Join(wordDir, "phonetic.txt")
	if err := os.WriteFile(phoneticFile, []byte(phoneticInfo), 0644); err != nil {
		return fmt.Errorf("failed to write phonetic file: %w", err)
	}
	
	fmt.Printf("  Saved phonetic information\n")
	return nil
}

func runGUIMode() error {
	// Create GUI configuration from command line flags and viper config
	guiConfig := &gui.Config{
		OutputDir:     outputDir,
		AudioFormat:   audioFormat,
		ImageProvider: imageAPI,
		OpenAIKey:     getOpenAIKey(),
	}
	
	// Create and run GUI application
	app := gui.New(guiConfig)
	app.Run()
	
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}