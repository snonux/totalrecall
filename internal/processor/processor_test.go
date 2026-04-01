package processor

import (
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/snonux/totalrecall/internal/cli"
	"codeberg.org/snonux/totalrecall/internal/gui"
	"codeberg.org/snonux/totalrecall/internal/phonetic"
	"github.com/spf13/viper"
)

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
