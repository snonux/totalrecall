package anki

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultGeneratorOptions(t *testing.T) {
	opts := DefaultGeneratorOptions()

	if opts.OutputPath != "anki_import.csv" {
		t.Errorf("Expected output path 'anki_import.csv', got '%s'", opts.OutputPath)
	}

	if opts.MediaFolder != "." {
		t.Errorf("Expected media folder '.', got '%s'", opts.MediaFolder)
	}

	if !opts.IncludeHeaders {
		t.Error("Expected IncludeHeaders to be true")
	}

	if opts.AudioFormat != "mp3" {
		t.Errorf("Expected audio format 'mp3', got '%s'", opts.AudioFormat)
	}

	if opts.ImageFormat != "jpg" {
		t.Errorf("Expected image format 'jpg', got '%s'", opts.ImageFormat)
	}
}

func TestNewGenerator(t *testing.T) {
	// Test with nil options
	gen := NewGenerator(nil)
	if gen == nil {
		t.Fatal("NewGenerator returned nil")
	}
	if gen.options == nil {
		t.Error("Generator options should not be nil")
	}

	// Test with custom options
	opts := &GeneratorOptions{
		OutputPath: "custom.csv",
	}
	gen = NewGenerator(opts)
	if gen.options.OutputPath != "custom.csv" {
		t.Errorf("Expected custom output path, got '%s'", gen.options.OutputPath)
	}
}

func TestAddCard(t *testing.T) {
	gen := NewGenerator(nil)

	card := Card{
		Bulgarian:   "ябълка",
		AudioFile:   "audio.mp3",
		ImageFile:   "image.jpg",
		Translation: "apple",
		Notes:       "test note",
	}

	gen.AddCard(card)

	if len(gen.cards) != 1 {
		t.Errorf("Expected 1 card, got %d", len(gen.cards))
	}

	if gen.cards[0].Bulgarian != "ябълка" {
		t.Errorf("Expected Bulgarian 'ябълка', got '%s'", gen.cards[0].Bulgarian)
	}
}

func TestGetCards(t *testing.T) {
	gen := NewGenerator(nil)

	card1 := Card{Bulgarian: "ябълка"}
	card2 := Card{Bulgarian: "котка"}

	gen.AddCard(card1)
	gen.AddCard(card2)

	cards := gen.GetCards()
	if len(cards) != 2 {
		t.Errorf("Expected 2 cards, got %d", len(cards))
	}

	// Test that we can modify the returned slice
	cards[0].Translation = "apple"
	if gen.cards[0].Translation != "apple" {
		t.Error("GetCards should return the actual slice, not a copy")
	}
}

func TestFormatAudioField(t *testing.T) {
	gen := NewGenerator(nil)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty path",
			input:    "",
			expected: "",
		},
		{
			name:     "simple audio file",
			input:    "/path/to/word123/audio.mp3",
			expected: "[sound:word123_audio.mp3]",
		},
		{
			name:     "audio file with complex path",
			input:    "/home/user/totalrecall/ябълка/audio.mp3",
			expected: "[sound:ябълка_audio.mp3]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gen.formatAudioField(tt.input)
			if result != tt.expected {
				t.Errorf("formatAudioField(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatImageField(t *testing.T) {
	gen := NewGenerator(nil)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty path",
			input:    "",
			expected: "",
		},
		{
			name:     "simple image file",
			input:    "/path/to/word123/image.jpg",
			expected: `<img src="word123_image.jpg">`,
		},
		{
			name:     "image file with complex path",
			input:    "/home/user/totalrecall/котка/image.png",
			expected: `<img src="котка_image.png">`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gen.formatImageField(tt.input)
			if result != tt.expected {
				t.Errorf("formatImageField(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateCSV(t *testing.T) {
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "test.csv")

	gen := NewGenerator(&GeneratorOptions{
		OutputPath:     outputPath,
		IncludeHeaders: true,
	})

	// Add test cards
	gen.AddCard(Card{
		Bulgarian:   "ябълка",
		AudioFile:   "/path/to/apple/audio.mp3",
		ImageFile:   "/path/to/apple/image.jpg",
		Translation: "apple",
		Notes:       "A fruit",
	})

	gen.AddCard(Card{
		Bulgarian:   "котка",
		AudioFile:   "/path/to/cat/audio.mp3",
		ImageFile:   "/path/to/cat/image.jpg",
		Translation: "cat",
		Notes:       "An animal",
	})

	// Generate CSV
	err := gen.GenerateCSV()
	if err != nil {
		t.Fatalf("GenerateCSV() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatal("CSV file was not created")
	}

	// Read and verify content
	file, err := os.Open(outputPath)
	if err != nil {
		t.Fatalf("Failed to open CSV file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read CSV: %v", err)
	}

	// Check headers
	if len(records) < 1 {
		t.Fatal("CSV file is empty")
	}

	expectedHeaders := []string{"Bulgarian", "Audio", "Image", "Translation", "Notes"}
	if len(records[0]) != len(expectedHeaders) {
		t.Errorf("Expected %d columns, got %d", len(expectedHeaders), len(records[0]))
	}

	for i, header := range expectedHeaders {
		if records[0][i] != header {
			t.Errorf("Expected header '%s' at position %d, got '%s'", header, i, records[0][i])
		}
	}

	// Check first data row
	if len(records) < 2 {
		t.Fatal("CSV file has no data rows")
	}

	if records[1][0] != "ябълка" {
		t.Errorf("Expected Bulgarian 'ябълка', got '%s'", records[1][0])
	}

	if records[1][1] != "[sound:apple_audio.mp3]" {
		t.Errorf("Expected audio field '[sound:apple_audio.mp3]', got '%s'", records[1][1])
	}

	if records[1][2] != `<img src="apple_image.jpg">` {
		t.Errorf("Expected image field '<img src=\"apple_image.jpg\">', got '%s'", records[1][2])
	}

	if records[1][3] != "apple" {
		t.Errorf("Expected translation 'apple', got '%s'", records[1][3])
	}
}

func TestGenerateCSVWithoutHeaders(t *testing.T) {
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "test.csv")

	gen := NewGenerator(&GeneratorOptions{
		OutputPath:     outputPath,
		IncludeHeaders: false,
	})

	gen.AddCard(Card{
		Bulgarian: "ябълка",
	})

	err := gen.GenerateCSV()
	if err != nil {
		t.Fatalf("GenerateCSV() error = %v", err)
	}

	// Read and verify no headers
	file, err := os.Open(outputPath)
	if err != nil {
		t.Fatalf("Failed to open CSV file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read CSV: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record (no headers), got %d", len(records))
	}

	if records[0][0] != "ябълка" {
		t.Errorf("First field should be 'ябълка', got '%s'", records[0][0])
	}
}

func TestGenerateFromDirectory(t *testing.T) {
	// Create test directory structure
	tempDir := t.TempDir()

	// Create word directories
	word1Dir := filepath.Join(tempDir, "ябълка")
	os.MkdirAll(word1Dir, 0755)

	word2Dir := filepath.Join(tempDir, "котка")
	os.MkdirAll(word2Dir, 0755)

	// Create hidden directory (should be skipped)
	hiddenDir := filepath.Join(tempDir, ".hidden")
	os.MkdirAll(hiddenDir, 0755)

	// Create word files
	os.WriteFile(filepath.Join(word1Dir, "word.txt"), []byte("ябълка"), 0644)
	os.WriteFile(filepath.Join(word1Dir, "translation.txt"), []byte("ябълка = apple"), 0644)
	os.WriteFile(filepath.Join(word1Dir, "audio.mp3"), []byte("audio data"), 0644)
	os.WriteFile(filepath.Join(word1Dir, "image.jpg"), []byte("image data"), 0644)
	os.WriteFile(filepath.Join(word1Dir, "phonetic.txt"), []byte("YA-bul-ka\nStress on first syllable"), 0644)

	// Word 2 with old format
	os.WriteFile(filepath.Join(word2Dir, "_word.txt"), []byte("котка"), 0644)
	os.WriteFile(filepath.Join(word2Dir, "audio.wav"), []byte("audio data"), 0644)

	// Hidden directory files (should be ignored)
	os.WriteFile(filepath.Join(hiddenDir, "word.txt"), []byte("hidden"), 0644)

	gen := NewGenerator(nil)
	err := gen.GenerateFromDirectory(tempDir)
	if err != nil {
		t.Fatalf("GenerateFromDirectory() error = %v", err)
	}

	// Check results
	if len(gen.cards) != 2 {
		t.Errorf("Expected 2 cards, got %d", len(gen.cards))
	}

	// Find and check first card
	var appleCard *Card
	for i := range gen.cards {
		if gen.cards[i].Bulgarian == "ябълка" {
			appleCard = &gen.cards[i]
			break
		}
	}

	if appleCard == nil {
		t.Fatal("Could not find apple card")
	}

	if appleCard.Translation != "apple" {
		t.Errorf("Expected translation 'apple', got '%s'", appleCard.Translation)
	}

	if !strings.HasSuffix(appleCard.AudioFile, "audio.mp3") {
		t.Errorf("Expected audio file to end with 'audio.mp3', got '%s'", appleCard.AudioFile)
	}

	if !strings.HasSuffix(appleCard.ImageFile, "image.jpg") {
		t.Errorf("Expected image file to end with 'image.jpg', got '%s'", appleCard.ImageFile)
	}

	if !strings.Contains(appleCard.Notes, "YA-bul-ka<br>Stress on first syllable") {
		t.Errorf("Expected phonetic notes with HTML breaks, got '%s'", appleCard.Notes)
	}
}

func TestCopyMediaFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create source file structure
	srcDir := filepath.Join(tempDir, "src", "word123")
	os.MkdirAll(srcDir, 0755)

	srcFile := filepath.Join(srcDir, "audio.mp3")
	os.WriteFile(srcFile, []byte("test audio"), 0644)

	// Create destination directory
	destDir := filepath.Join(tempDir, "dest")
	os.MkdirAll(destDir, 0755)

	gen := NewGenerator(nil)

	// Test copying file
	newPath, err := gen.copyMediaFile(srcFile, destDir)
	if err != nil {
		t.Fatalf("copyMediaFile() error = %v", err)
	}

	expectedName := "word123_audio.mp3"
	if newPath != expectedName {
		t.Errorf("Expected filename '%s', got '%s'", expectedName, newPath)
	}

	// Verify file was copied
	destFile := filepath.Join(destDir, newPath)
	if _, err := os.Stat(destFile); os.IsNotExist(err) {
		t.Error("Destination file was not created")
	}

	// Verify content
	content, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if string(content) != "test audio" {
		t.Errorf("File content mismatch: got '%s', want 'test audio'", string(content))
	}

	// Test copying same file again (should create unique name)
	newPath2, err := gen.copyMediaFile(srcFile, destDir)
	if err != nil {
		t.Fatalf("copyMediaFile() second call error = %v", err)
	}

	if newPath2 == newPath {
		t.Error("Second copy should have unique name")
	}

	expectedName2 := "word123_audio_1.mp3"
	if newPath2 != expectedName2 {
		t.Errorf("Expected filename '%s', got '%s'", expectedName2, newPath2)
	}
}

func TestStats(t *testing.T) {
	gen := NewGenerator(nil)

	// Empty stats
	total, audio, images := gen.Stats()
	if total != 0 || audio != 0 || images != 0 {
		t.Errorf("Expected empty stats, got total=%d, audio=%d, images=%d", total, audio, images)
	}

	// Add cards with different media
	gen.AddCard(Card{
		Bulgarian: "ябълка",
		AudioFile: "audio1.mp3",
		ImageFile: "image1.jpg",
	})

	gen.AddCard(Card{
		Bulgarian: "котка",
		AudioFile: "audio2.mp3",
	})

	gen.AddCard(Card{
		Bulgarian: "куче",
		ImageFile: "image3.jpg",
	})

	gen.AddCard(Card{
		Bulgarian:   "хляб",
		Translation: "bread",
	})

	total, audio, images = gen.Stats()
	if total != 4 {
		t.Errorf("Expected 4 total cards, got %d", total)
	}

	if audio != 2 {
		t.Errorf("Expected 2 cards with audio, got %d", audio)
	}

	if images != 2 {
		t.Errorf("Expected 2 cards with images, got %d", images)
	}
}

func TestGeneratePackage(t *testing.T) {
	tempDir := t.TempDir()

	// Create source files
	srcDir := filepath.Join(tempDir, "src", "word1")
	os.MkdirAll(srcDir, 0755)

	audioFile := filepath.Join(srcDir, "audio.mp3")
	os.WriteFile(audioFile, []byte("audio data"), 0644)

	imageFile := filepath.Join(srcDir, "image.jpg")
	os.WriteFile(imageFile, []byte("image data"), 0644)

	// Create generator with card
	gen := NewGenerator(nil)
	gen.AddCard(Card{
		Bulgarian: "ябълка",
		AudioFile: audioFile,
		ImageFile: imageFile,
	})

	// Generate package
	outputDir := filepath.Join(tempDir, "output")
	err := gen.GeneratePackage(outputDir)
	if err != nil {
		t.Fatalf("GeneratePackage() error = %v", err)
	}

	// Verify structure
	mediaDir := filepath.Join(outputDir, "collection.media")
	if _, err := os.Stat(mediaDir); os.IsNotExist(err) {
		t.Error("Media directory was not created")
	}

	csvFile := filepath.Join(outputDir, "import.csv")
	if _, err := os.Stat(csvFile); os.IsNotExist(err) {
		t.Error("CSV file was not created")
	}

	// Verify media files were copied
	copiedAudio := filepath.Join(mediaDir, "word1_audio.mp3")
	if _, err := os.Stat(copiedAudio); os.IsNotExist(err) {
		t.Error("Audio file was not copied")
	}

	copiedImage := filepath.Join(mediaDir, "word1_image.jpg")
	if _, err := os.Stat(copiedImage); os.IsNotExist(err) {
		t.Error("Image file was not copied")
	}
}
