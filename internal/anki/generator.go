package anki

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Card represents a single Anki flashcard
type Card struct {
	Bulgarian   string   // The Bulgarian word/phrase
	AudioFile   string   // Path to audio file
	ImageFile   string   // Path to image file
	Translation string   // Optional translation
	Notes       string   // Optional notes
}

// GeneratorOptions configures the Anki export
type GeneratorOptions struct {
	OutputPath      string // Output CSV file path
	MediaFolder     string // Folder containing media files
	IncludeHeaders  bool   // Include CSV headers
	AudioFormat     string // Audio file format (mp3, wav)
	ImageFormat     string // Image file format (jpg, png)
}

// DefaultGeneratorOptions returns sensible defaults
func DefaultGeneratorOptions() *GeneratorOptions {
	return &GeneratorOptions{
		OutputPath:     "anki_import.csv",
		MediaFolder:    ".",
		IncludeHeaders: true,
		AudioFormat:    "mp3",
		ImageFormat:    "jpg",
	}
}

// Generator creates Anki-compatible import files
type Generator struct {
	options *GeneratorOptions
	cards   []Card
}

// NewGenerator creates a new Anki generator
func NewGenerator(options *GeneratorOptions) *Generator {
	if options == nil {
		options = DefaultGeneratorOptions()
	}
	return &Generator{
		options: options,
		cards:   make([]Card, 0),
	}
}

// AddCard adds a card to the collection
func (g *Generator) AddCard(card Card) {
	g.cards = append(g.cards, card)
}

// GetCards returns a slice of all cards for modification
func (g *Generator) GetCards() []Card {
	return g.cards
}

// GenerateCSV creates a CSV file for Anki import
func (g *Generator) GenerateCSV() error {
	// Create output file
	file, err := os.Create(g.options.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()
	
	// Create CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()
	
	// Write headers if requested
	if g.options.IncludeHeaders {
		headers := []string{"Bulgarian", "Audio", "Image", "Translation", "Notes"}
		if err := writer.Write(headers); err != nil {
			return fmt.Errorf("failed to write headers: %w", err)
		}
	}
	
	// Write cards
	for _, card := range g.cards {
		record := []string{
			card.Bulgarian,
			g.formatAudioField(card.AudioFile),
			g.formatImageField(card.ImageFile),
			card.Translation,
			card.Notes,
		}
		
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write card: %w", err)
		}
	}
	
	return nil
}

// formatAudioField formats the audio file reference for Anki
func (g *Generator) formatAudioField(audioFile string) string {
	if audioFile == "" {
		return ""
	}
	
	// Get just the filename
	filename := filepath.Base(audioFile)
	
	// Anki audio format: [sound:filename.mp3]
	return fmt.Sprintf("[sound:%s]", filename)
}

// formatImageField formats image file reference for Anki
func (g *Generator) formatImageField(imageFile string) string {
	if imageFile == "" {
		return ""
	}
	
	// Get just the filename
	filename := filepath.Base(imageFile)
	return fmt.Sprintf(`<img src="%s">`, filename)
}

// GenerateFromDirectory creates cards from a directory of materials
func (g *Generator) GenerateFromDirectory(dir string) error {
	// Read all subdirectories
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}
	
	// Process each subdirectory as a word
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		
		// Skip hidden directories like .trashbin
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		
		wordDir := filepath.Join(dir, entry.Name())
		sanitizedWord := entry.Name()
		
		// Create card for this word
		card := Card{}
		
		// Try to load translation and get original word
		translationFile := filepath.Join(wordDir, fmt.Sprintf("%s_translation.txt", sanitizedWord))
		if data, err := os.ReadFile(translationFile); err == nil {
			content := string(data)
			if parts := strings.Split(content, "="); len(parts) >= 2 {
				card.Bulgarian = strings.TrimSpace(parts[0])
				card.Translation = strings.TrimSpace(parts[1])
			}
		}
		
		// If no Bulgarian word found from translation, use directory name
		if card.Bulgarian == "" {
			card.Bulgarian = sanitizedWord
		}
		
		// Look for audio file
		audioFormats := []string{"mp3", "wav"}
		for _, format := range audioFormats {
			audioFile := filepath.Join(wordDir, fmt.Sprintf("%s.%s", sanitizedWord, format))
			if _, err := os.Stat(audioFile); err == nil {
				card.AudioFile = audioFile
				break
			}
		}
		
		// Look for image files
		imagePatterns := []string{
			fmt.Sprintf("%s.jpg", sanitizedWord),
			fmt.Sprintf("%s.png", sanitizedWord),
			fmt.Sprintf("%s_1.jpg", sanitizedWord),
			fmt.Sprintf("%s_1.png", sanitizedWord),
		}
		for _, pattern := range imagePatterns {
			imageFile := filepath.Join(wordDir, pattern)
			if _, err := os.Stat(imageFile); err == nil {
				card.ImageFile = imageFile
				break
			}
		}
		
		// Only add card if it has at least some content
		if card.AudioFile != "" || card.ImageFile != "" || card.Translation != "" {
			g.AddCard(card)
		}
	}
	
	return nil
}

// GeneratePackage creates a complete Anki package with media files
// Deprecated: Use GenerateAPKG for proper .apkg format
func (g *Generator) GeneratePackage(outputDir string) error {
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	
	// Create media directory
	mediaDir := filepath.Join(outputDir, "collection.media")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		return fmt.Errorf("failed to create media directory: %w", err)
	}
	
	// Copy media files and update paths
	for i, card := range g.cards {
		// Copy audio file
		if card.AudioFile != "" {
			newPath, err := g.copyMediaFile(card.AudioFile, mediaDir)
			if err != nil {
				return fmt.Errorf("failed to copy audio file: %w", err)
			}
			g.cards[i].AudioFile = newPath
		}
		
		// Copy image file
		if card.ImageFile != "" {
			newPath, err := g.copyMediaFile(card.ImageFile, mediaDir)
			if err != nil {
				return fmt.Errorf("failed to copy image file: %w", err)
			}
			g.cards[i].ImageFile = newPath
		}
	}
	
	// Update output path to package directory
	g.options.OutputPath = filepath.Join(outputDir, "import.csv")
	
	// Generate CSV
	return g.GenerateCSV()
}

// GenerateAPKG creates a proper .apkg file for Anki import
func (g *Generator) GenerateAPKG(outputPath, deckName string) error {
	// Create APKG generator
	apkgGen := NewAPKGGenerator(deckName)
	
	// Add all cards
	for _, card := range g.cards {
		apkgGen.AddCard(card)
	}
	
	// Generate the .apkg file
	return apkgGen.GenerateAPKG(outputPath)
}

// copyMediaFile copies a media file to the destination directory
func (g *Generator) copyMediaFile(src, destDir string) (string, error) {
	// Get source file info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	
	// Create destination path
	filename := filepath.Base(src)
	destPath := filepath.Join(destDir, filename)
	
	// Check if file already exists
	if _, err := os.Stat(destPath); err == nil {
		// File exists, generate unique name
		ext := filepath.Ext(filename)
		base := strings.TrimSuffix(filename, ext)
		for i := 1; ; i++ {
			filename = fmt.Sprintf("%s_%d%s", base, i, ext)
			destPath = filepath.Join(destDir, filename)
			if _, err := os.Stat(destPath); os.IsNotExist(err) {
				break
			}
		}
	}
	
	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer srcFile.Close()
	
	// Create destination file
	destFile, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer destFile.Close()
	
	// Copy content
	if _, err := destFile.ReadFrom(srcFile); err != nil {
		return "", err
	}
	
	// Preserve file mode
	if err := os.Chmod(destPath, srcInfo.Mode()); err != nil {
		return "", err
	}
	
	return filename, nil
}

// Stats returns statistics about the card collection
func (g *Generator) Stats() (totalCards, withAudio, withImages int) {
	totalCards = len(g.cards)
	
	for _, card := range g.cards {
		if card.AudioFile != "" {
			withAudio++
		}
		if card.ImageFile != "" {
			withImages++
		}
	}
	
	return
}