package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	
	"codeberg.org/snonux/totalrecall/internal"
	"codeberg.org/snonux/totalrecall/internal/anki"
	"codeberg.org/snonux/totalrecall/internal/audio"
	"codeberg.org/snonux/totalrecall/internal/image"
)

var (
	// Flags
	cfgFile      string
	voice        string
	outputDir    string
	audioFormat  string
	imageAPI     string
	batchFile    string
	skipAudio    bool
	skipImages   bool
	imagesPerWord int
	generateAnki bool
	listModels   bool
	// Audio provider flags
	audioProvider  string
	// Audio tuning flags (espeak)
	audioPitch     int
	audioAmplitude int
	audioWordGap   int
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

It creates audio pronunciation files using espeak-ng and downloads
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
	
	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.totalrecall.yaml)")
	
	// Local flags
	rootCmd.Flags().StringVarP(&voice, "voice", "v", "bg+f1", "Voice variant (bg, bg+m1, bg+f1, etc.)")
	rootCmd.Flags().StringVarP(&outputDir, "output", "o", "./anki_cards", "Output directory")
	rootCmd.Flags().StringVarP(&audioFormat, "format", "f", "mp3", "Audio format (wav or mp3)")
	rootCmd.Flags().StringVar(&imageAPI, "image-api", "openai", "Image source (pixabay, unsplash, or openai)")
	rootCmd.Flags().StringVar(&batchFile, "batch", "", "Process words from file (one per line)")
	rootCmd.Flags().BoolVar(&skipAudio, "skip-audio", false, "Skip audio generation")
	rootCmd.Flags().BoolVar(&skipImages, "skip-images", false, "Skip image download")
	rootCmd.Flags().IntVar(&imagesPerWord, "images-per-word", 1, "Number of images to download per word")
	rootCmd.Flags().BoolVar(&generateAnki, "anki", false, "Generate Anki import CSV file")
	rootCmd.Flags().BoolVar(&listModels, "list-models", false, "List available OpenAI models for the current API key")
	
	// Audio provider selection
	rootCmd.Flags().StringVar(&audioProvider, "audio-provider", "openai", "Audio provider: espeak or openai")
	
	// Audio tuning flags (espeak)
	rootCmd.Flags().IntVar(&audioPitch, "pitch", 50, "Audio pitch adjustment (0-99, default 50, espeak only)")
	rootCmd.Flags().IntVar(&audioAmplitude, "amplitude", 100, "Audio volume (0-200, default 100, espeak only)")
	rootCmd.Flags().IntVar(&audioWordGap, "word-gap", 0, "Gap between words in 10ms units (default 0, espeak only)")
	
	// OpenAI flags
	rootCmd.Flags().StringVar(&openAIModel, "openai-model", "gpt-4o-mini-tts", "OpenAI TTS model: tts-1, tts-1-hd, gpt-4o-mini-tts")
	rootCmd.Flags().StringVar(&openAIVoice, "openai-voice", "nova", "OpenAI voice: alloy, ash, ballad, coral, echo, fable, onyx, nova, sage, shimmer, verse")
	rootCmd.Flags().Float64Var(&openAISpeed, "openai-speed", 0.8, "OpenAI speech speed (0.25 to 4.0, may be ignored by gpt-4o-mini-tts)")
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
	// Create audio provider configuration
	providerConfig := &audio.Config{
		Provider:     audioProvider,
		OutputDir:    outputDir,
		OutputFormat: audioFormat,
		
		// ESpeak settings
		ESpeakVoice:     voice,
		ESpeakSpeed:     viper.GetInt("audio.speed"),
		ESpeakPitch:     audioPitch,
		ESpeakAmplitude: audioAmplitude,
		ESpeakWordGap:   audioWordGap,
		
		// OpenAI settings
		OpenAIKey:         getOpenAIKey(),
		OpenAIModel:       openAIModel,
		OpenAIVoice:       openAIVoice,
		OpenAISpeed:       openAISpeed,
		OpenAIInstruction: openAIInstruction,
		
		// Caching
		EnableCache: viper.GetBool("audio.enable_cache"),
		CacheDir:    viper.GetString("audio.cache_dir"),
	}
	
	// Set defaults
	if providerConfig.ESpeakSpeed == 0 {
		providerConfig.ESpeakSpeed = 150
	}
	if providerConfig.CacheDir == "" {
		providerConfig.CacheDir = "./.audio_cache"
	}
	
	// Use config file values if not overridden by flags
	if audioProvider == "openai" && viper.IsSet("audio.provider") {
		providerConfig.Provider = viper.GetString("audio.provider")
	}
	if audioPitch == 50 && viper.IsSet("audio.pitch") {
		providerConfig.ESpeakPitch = viper.GetInt("audio.pitch")
	}
	if audioAmplitude == 100 && viper.IsSet("audio.amplitude") {
		providerConfig.ESpeakAmplitude = viper.GetInt("audio.amplitude")
	}
	if audioWordGap == 0 && viper.IsSet("audio.word_gap") {
		providerConfig.ESpeakWordGap = viper.GetInt("audio.word_gap")
	}
	if openAIModel == "gpt-4o-mini-tts" && viper.IsSet("audio.openai_model") {
		providerConfig.OpenAIModel = viper.GetString("audio.openai_model")
	}
	if openAIVoice == "nova" && viper.IsSet("audio.openai_voice") {
		providerConfig.OpenAIVoice = viper.GetString("audio.openai_voice")
	}
	if openAISpeed == 0.8 && viper.IsSet("audio.openai_speed") {
		providerConfig.OpenAISpeed = viper.GetFloat64("audio.openai_speed")
	}
	if openAIInstruction == "" && viper.IsSet("audio.openai_instruction") {
		providerConfig.OpenAIInstruction = viper.GetString("audio.openai_instruction")
	}
	
	// Create the audio provider
	provider, err := audio.NewProvider(providerConfig)
	if err != nil {
		// If OpenAI fails, try to create a fallback to espeak
		if providerConfig.Provider == "openai" {
			fmt.Printf("Warning: OpenAI audio provider failed (%v), falling back to espeak-ng\n", err)
			providerConfig.Provider = "espeak"
			fallbackProvider, fallbackErr := audio.NewProvider(providerConfig)
			if fallbackErr != nil {
				return fmt.Errorf("both OpenAI and espeak-ng failed: %v", fallbackErr)
			}
			provider = fallbackProvider
		} else {
			return err
		}
	}
	
	// Generate audio file
	filename := sanitizeFilename(word)
	outputFile := filepath.Join(outputDir, fmt.Sprintf("%s.%s", filename, audioFormat))
	
	ctx := context.Background()
	return provider.GenerateAudio(ctx, word, outputFile)
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
		OverwriteExisting: false,
		CreateDir:         true,
		FileNamePattern:   "{word}_{index}",
		MaxSizeBytes:      5 * 1024 * 1024, // 5MB
	}
	
	downloader := image.NewDownloader(searcher, downloadOpts)
	
	// Download images
	ctx := context.Background()
	if imagesPerWord == 1 {
		_, path, err := downloader.DownloadBestMatch(ctx, word)
		if err != nil {
			return err
		}
		fmt.Printf("    Downloaded: %s\n", path)
	} else {
		paths, err := downloader.DownloadMultiple(ctx, word, imagesPerWord)
		if err != nil {
			return err
		}
		for _, path := range paths {
			fmt.Printf("    Downloaded: %s\n", path)
		}
	}
	
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

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}