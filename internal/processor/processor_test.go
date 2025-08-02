package processor

import (
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/snonux/totalrecall/internal/cli"
)

func TestNewProcessor(t *testing.T) {
	// Set up test environment
	os.Setenv("OPENAI_API_KEY", "test-key")
	defer os.Unsetenv("OPENAI_API_KEY")

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
	// Skip if no API key
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("Skipping test: OPENAI_API_KEY not set")
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

	// Skip if no API key
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("Skipping test: OPENAI_API_KEY not set")
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
	os.Remove(csvFile) // Clean up
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
	os.WriteFile(filepath.Join(word1Dir, "audio.mp3"), []byte("audio1"), 0644)
	os.WriteFile(filepath.Join(word2Dir, "audio.mp3"), []byte("audio2"), 0644)
	os.WriteFile(filepath.Join(word1Dir, "image.jpg"), []byte("image1"), 0644)

	_, err := p.GenerateAnkiFile()
	if err != nil {
		t.Errorf("GenerateAnkiFile (APKG) failed: %v", err)
	}

	// Check APKG file was created in home directory
	homeDir, _ := os.UserHomeDir()
	apkgFile := filepath.Join(homeDir, "Test_Deck.apkg")
	if _, err := os.Stat(apkgFile); os.IsNotExist(err) {
		t.Error("APKG file was not created in home directory")
	}
	os.Remove(apkgFile) // Clean up
}
