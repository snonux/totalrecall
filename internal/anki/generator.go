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
	ImageFiles  []string // Paths to image files
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
			g.formatImageField(card.ImageFiles),
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

// formatImageField formats image file references for Anki
func (g *Generator) formatImageField(imageFiles []string) string {
	if len(imageFiles) == 0 {
		return ""
	}
	
	// For multiple images, we'll use HTML to display them
	if len(imageFiles) == 1 {
		filename := filepath.Base(imageFiles[0])
		return fmt.Sprintf(`<img src="%s">`, filename)
	}
	
	// Multiple images - create a simple layout
	var html strings.Builder
	html.WriteString(`<div style="display: flex; flex-wrap: wrap; gap: 10px;">`)
	
	for _, imageFile := range imageFiles {
		filename := filepath.Base(imageFile)
		html.WriteString(fmt.Sprintf(`<img src="%s" style="max-width: 200px; height: auto;">`, filename))
	}
	
	html.WriteString(`</div>`)
	return html.String()
}

// GenerateFromDirectory creates cards from a directory of materials
func (g *Generator) GenerateFromDirectory(dir string) error {
	// Map to group files by word
	wordFiles := make(map[string]*Card)
	
	// Walk the directory
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip directories
		if info.IsDir() {
			return nil
		}
		
		// Get filename without extension
		filename := info.Name()
		ext := filepath.Ext(filename)
		base := strings.TrimSuffix(filename, ext)
		
		// Skip attribution files
		if strings.HasSuffix(base, "_attribution") {
			return nil
		}
		
		// Extract word from filename (assumes format: word_type.ext or word_index.ext)
		parts := strings.Split(base, "_")
		if len(parts) == 0 {
			return nil
		}
		
		word := parts[0]
		
		// Get or create card for this word
		card, exists := wordFiles[word]
		if !exists {
			card = &Card{
				Bulgarian:  word,
				ImageFiles: make([]string, 0),
			}
			wordFiles[word] = card
		}
		
		// Add file to appropriate field
		switch strings.ToLower(ext) {
		case ".mp3", ".wav":
			if card.AudioFile == "" { // Use first audio file found
				card.AudioFile = path
			}
		case ".jpg", ".jpeg", ".png", ".gif":
			card.ImageFiles = append(card.ImageFiles, path)
		}
		
		return nil
	})
	
	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}
	
	// Add all cards to generator
	for _, card := range wordFiles {
		g.AddCard(*card)
	}
	
	return nil
}

// GeneratePackage creates a complete Anki package with media files
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
		
		// Copy image files
		newImagePaths := make([]string, 0, len(card.ImageFiles))
		for _, imagePath := range card.ImageFiles {
			newPath, err := g.copyMediaFile(imagePath, mediaDir)
			if err != nil {
				return fmt.Errorf("failed to copy image file: %w", err)
			}
			newImagePaths = append(newImagePaths, newPath)
		}
		g.cards[i].ImageFiles = newImagePaths
	}
	
	// Update output path to package directory
	g.options.OutputPath = filepath.Join(outputDir, "import.csv")
	
	// Generate CSV
	return g.GenerateCSV()
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
		if len(card.ImageFiles) > 0 {
			withImages++
		}
	}
	
	return
}