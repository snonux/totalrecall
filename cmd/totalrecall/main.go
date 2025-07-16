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
	
	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.totalrecall.yaml)")
	
	// Local flags
	rootCmd.Flags().StringVarP(&outputDir, "output", "o", "./anki_cards", "Output directory")
	rootCmd.Flags().StringVarP(&audioFormat, "format", "f", "mp3", "Audio format (wav or mp3)")
	rootCmd.Flags().StringVar(&imageAPI, "image-api", "openai", "Image source (pixabay, unsplash, or openai)")
	rootCmd.Flags().StringVar(&batchFile, "batch", "", "Process words from file (one per line)")
	rootCmd.Flags().BoolVar(&skipAudio, "skip-audio", false, "Skip audio generation")
	rootCmd.Flags().BoolVar(&skipImages, "skip-images", false, "Skip image download")
	rootCmd.Flags().BoolVar(&generateAnki, "anki", false, "Generate Anki import CSV file")
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
	// Handle --list-models flag
	if listModels {
		return listAvailableModels()
	}
	
	// Handle --gui flag
	if guiMode {
		return runGUIMode()
	}
	
	// Determine words to process
	var words []string
	
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
				words = append(words, line)
			}
		}
	} else if len(args) > 0 {
		// Single word from command line
		words = []string{args[0]}
	} else {
		// No input provided
		return fmt.Errorf("please provide a Bulgarian word or use --batch flag")
	}
	
	// Validate words
	for _, word := range words {
		if err := audio.ValidateBulgarianText(word); err != nil {
			return fmt.Errorf("invalid word '%s': %w", word, err)
		}
	}
	
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	
	// Process each word
	for i, word := range words {
		fmt.Printf("\nProcessing %d/%d: %s\n", i+1, len(words), word)
		
		if err := processWord(word); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing '%s': %v\n", word, err)
			// Continue with next word
		}
	}
	
	// Generate Anki CSV if requested
	if generateAnki {
		fmt.Printf("\nGenerating Anki import file...\n")
		if err := generateAnkiCSV(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to generate Anki CSV: %v\n", err)
		} else {
			fmt.Println("Anki import file created: anki_import.csv")
		}
	}
	
	fmt.Println("\nDone! Materials saved to:", outputDir)
	return nil
}

func processWord(word string) error {
	// Translate the word first
	fmt.Printf("  Translating to English...\n")
	translation, err := translateWord(word)
	if err != nil {
		fmt.Printf("  Warning: Translation failed: %v\n", err)
		translation = "" // Continue without translation
	} else {
		fmt.Printf("  Translation: %s\n", translation)
		// Store translation for Anki export
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
	
	// Download images
	if !skipImages {
		fmt.Printf("  Downloading images...\n")
		if err := downloadImages(word); err != nil {
			return fmt.Errorf("image download failed: %w", err)
		}
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
	// Create audio provider configuration
	providerConfig := &audio.Config{
		Provider:     "openai",
		OutputDir:    outputDir,
		OutputFormat: audioFormat,
		
		// OpenAI settings
		OpenAIKey:         getOpenAIKey(),
		OpenAIModel:       openAIModel,
		OpenAIVoice:       voice,
		OpenAISpeed:       openAISpeed,
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
	if openAIVoice == "nova" && viper.IsSet("audio.openai_voice") {
		providerConfig.OpenAIVoice = viper.GetString("audio.openai_voice")
	}
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
	filename := sanitizeFilename(word)
	
	// Add voice name to filename if generating multiple voices
	var outputFile string
	if allVoices {
		outputFile = filepath.Join(outputDir, fmt.Sprintf("%s_%s.%s", filename, voice, audioFormat))
	} else {
		outputFile = filepath.Join(outputDir, fmt.Sprintf("%s.%s", filename, audioFormat))
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
	// Create image searcher based on provider
	var searcher image.ImageSearcher
	var err error
	
	switch imageAPI {
	case "pixabay":
		apiKey := viper.GetString("image.pixabay_key")
		searcher = image.NewPixabayClient(apiKey)
		
	case "unsplash":
		apiKey := viper.GetString("image.unsplash_key")
		if apiKey == "" {
			return fmt.Errorf("Unsplash API key is required in config")
		}
		searcher, err = image.NewUnsplashClient(apiKey)
		if err != nil {
			return err
		}
		
	case "openai":
		// Create OpenAI image configuration
		openaiConfig := &image.OpenAIConfig{
			APIKey:      getOpenAIKey(),
			Model:       openAIImageModel,
			Size:        openAIImageSize,
			Quality:     openAIImageQuality,
			Style:       openAIImageStyle,
			CacheDir:    viper.GetString("image.cache_dir"),
			EnableCache: viper.GetBool("image.enable_cache"),
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
		
		// Set defaults
		if openaiConfig.CacheDir == "" {
			openaiConfig.CacheDir = "./.image_cache"
		}
		if !viper.IsSet("image.enable_cache") {
			openaiConfig.EnableCache = true
		}
		
		searcher = image.NewOpenAIClient(openaiConfig)
		if openaiConfig.APIKey == "" {
			fmt.Printf("Warning: OpenAI API key not found, falling back to Pixabay for images\n")
			imageAPI = "pixabay"
			searcher = image.NewPixabayClient("")
		}
		
	default:
		return fmt.Errorf("unknown image provider: %s", imageAPI)
	}
	
	// Create downloader
	downloadOpts := &image.DownloadOptions{
		OutputDir:         outputDir,
		OverwriteExisting: true,  // Allow overwriting existing files
		CreateDir:         true,
		FileNamePattern:   "{word}_{index}",
		MaxSizeBytes:      5 * 1024 * 1024, // 5MB
	}
	
	downloader := image.NewDownloader(searcher, downloadOpts)
	
	// Download single image
	ctx := context.Background()
	_, path, err := downloader.DownloadBestMatch(ctx, word)
	if err != nil {
		return err
	}
	fmt.Printf("    Downloaded: %s\n", path)
	
	return nil
}

func sanitizeFilename(s string) string {
	// Simple filename sanitization
	result := ""
	for _, r := range s {
		if isAlphaNumeric(r) || r == '-' || r == '_' {
			result += string(r)
		} else {
			result += "_"
		}
	}
	return result
}

func isAlphaNumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || 
	       (r >= '0' && r <= '9') || (r >= 'а' && r <= 'я') || 
	       (r >= 'А' && r <= 'Я')
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

func generateAnkiCSV() error {
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
	
	// Generate CSV
	if err := gen.GenerateCSV(); err != nil {
		return fmt.Errorf("failed to generate CSV: %w", err)
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
	// Save translation to a text file
	filename := sanitizeFilename(word)
	outputFile := filepath.Join(outputDir, fmt.Sprintf("%s_translation.txt", filename))
	
	content := fmt.Sprintf("%s = %s\n", word, translation)
	
	if err := os.WriteFile(outputFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write translation file: %w", err)
	}
	
	return nil
}

// Global map to store translations for Anki export
var wordTranslations = make(map[string]string)

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
	
	return nil
}

func runGUIMode() error {
	// Create GUI configuration from command line flags and viper config
	guiConfig := &gui.Config{
		OutputDir:     outputDir,
		AudioFormat:   audioFormat,
		ImageProvider: imageAPI,
		EnableCache:   viper.GetBool("cache.enable"),
		OpenAIKey:     getOpenAIKey(),
		PixabayKey:    viper.GetString("image.pixabay_key"),
		UnsplashKey:   viper.GetString("image.unsplash_key"),
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