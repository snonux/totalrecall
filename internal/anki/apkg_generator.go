package anki

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// APKGGenerator creates Anki package files (.apkg)
type APKGGenerator struct {
	deckName     string
	deckID       int64
	modelID      int64
	modelIDBgBg  int64 // Separate model for bg-bg cards
	cards        []Card
	mediaFiles   map[string]int // maps original filename to media number
	mediaCounter int

	schemer   *SQLiteSchemer
	packager  *ZipPackager
	templates *CardTemplate
}

// NewAPKGGenerator creates a new APKG generator.
// Deck and model IDs are derived deterministically from deckName so that
// re-exporting produces the same IDs and Anki merges cards instead of
// creating duplicate notetypes/decks.
func NewAPKGGenerator(deckName string) *APKGGenerator {
	deckID := stableID(deckName)
	return &APKGGenerator{
		deckName:     deckName,
		deckID:       deckID,
		modelID:      stableID(deckName + "/model/en-bg"),
		modelIDBgBg:  stableID(deckName + "/model/bg-bg"),
		cards:        make([]Card, 0),
		mediaFiles:   make(map[string]int),
		mediaCounter: 0,
		schemer:      NewSQLiteSchemer(),
		packager:     NewZipPackager(),
		templates:    MustCardTemplate(),
	}
}

// stableID produces a stable positive int64 from a seed string.
// The value looks like a millisecond timestamp (13 digits) so Anki treats it
// the same way it treats its own generated IDs.
func stableID(seed string) int64 {
	h := sha1.Sum([]byte(seed))
	v := int64(binary.BigEndian.Uint64(h[:8]))
	if v < 0 {
		v = -v
	}
	// Clamp to 13 digits (1e12..9.999e12) so it resembles a millis timestamp
	const lo, hi = 1_000_000_000_000, 9_999_999_999_999
	return lo + v%(hi-lo+1)
}

// AddCard adds a card to the generator
func (g *APKGGenerator) AddCard(card Card) {
	g.cards = append(g.cards, card)
}

// GenerateAPKG creates an .apkg file
func (g *APKGGenerator) GenerateAPKG(outputPath string) error {
	// Create temporary directory for building the package
	tempDir, err := os.MkdirTemp("", "anki_export_*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	// Copy media files FIRST (this populates g.mediaFiles map)
	if err := g.copyMediaFiles(tempDir); err != nil {
		return fmt.Errorf("failed to copy media files: %w", err)
	}

	// Create media mapping file
	if err := g.createMediaMapping(tempDir); err != nil {
		return fmt.Errorf("failed to create media mapping: %w", err)
	}

	// Create SQLite database (this uses g.mediaFiles map)
	dbPath := filepath.Join(tempDir, "collection.anki2")
	if err := g.createDatabase(dbPath); err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	// Create the .apkg zip file with a timestamped name
	timestamp := time.Now().Format("2006-01-02-15:04:05")
	safeDeckName := strings.ReplaceAll(g.deckName, " ", "_")
	safeDeckName = strings.ReplaceAll(safeDeckName, "/", "-")
	numberOfCards := len(g.cards)
	outputDir := filepath.Dir(outputPath)
	finalName := fmt.Sprintf("%s-%s-%d.apkg", safeDeckName, timestamp, numberOfCards)
	finalPath := filepath.Join(outputDir, finalName)

	if err := g.createZipPackage(tempDir, finalPath); err != nil {
		return fmt.Errorf("failed to create zip package: %w", err)
	}

	return nil
}

func (g *APKGGenerator) createDatabase(dbPath string) error {
	return g.schemer.CreateDatabase(dbPath, g, g.templates)
}

func (g *APKGGenerator) createZipPackage(tempDir, outputPath string) error {
	return g.packager.CreatePackage(tempDir, outputPath)
}

// copyMediaFiles copies media files and assigns them numbers
func (g *APKGGenerator) copyMediaFiles(tempDir string) error {
	for _, card := range g.cards {
		// Copy audio file (front audio for bg-bg, only audio for en-bg)
		if card.AudioFile != "" && fileExists(card.AudioFile) {
			cardDirID := filepath.Base(filepath.Dir(card.AudioFile))
			originalFilename := filepath.Base(card.AudioFile)
			uniqueFilename := fmt.Sprintf("%s_%s", cardDirID, originalFilename)

			if _, exists := g.mediaFiles[uniqueFilename]; !exists {
				targetPath := filepath.Join(tempDir, fmt.Sprintf("%d", g.mediaCounter))
				if err := copyFile(card.AudioFile, targetPath); err != nil {
					return fmt.Errorf("failed to copy audio file %s: %w", card.AudioFile, err)
				}
				g.mediaFiles[uniqueFilename] = g.mediaCounter
				g.mediaCounter++
			}
		}

		// Copy back audio file (only for bg-bg cards)
		if card.AudioFileBack != "" && fileExists(card.AudioFileBack) {
			cardDirID := filepath.Base(filepath.Dir(card.AudioFileBack))
			originalFilename := filepath.Base(card.AudioFileBack)
			uniqueFilename := fmt.Sprintf("%s_%s", cardDirID, originalFilename)

			if _, exists := g.mediaFiles[uniqueFilename]; !exists {
				targetPath := filepath.Join(tempDir, fmt.Sprintf("%d", g.mediaCounter))
				if err := copyFile(card.AudioFileBack, targetPath); err != nil {
					return fmt.Errorf("failed to copy back audio file %s: %w", card.AudioFileBack, err)
				}
				g.mediaFiles[uniqueFilename] = g.mediaCounter
				g.mediaCounter++
			}
		}

		// Copy image file
		if card.ImageFile != "" && fileExists(card.ImageFile) {
			cardDirID := filepath.Base(filepath.Dir(card.ImageFile))
			originalFilename := filepath.Base(card.ImageFile)
			uniqueFilename := fmt.Sprintf("%s_%s", cardDirID, originalFilename)

			if _, exists := g.mediaFiles[uniqueFilename]; !exists {
				targetPath := filepath.Join(tempDir, fmt.Sprintf("%d", g.mediaCounter))
				if err := copyFile(card.ImageFile, targetPath); err != nil {
					return fmt.Errorf("failed to copy image file %s: %w", card.ImageFile, err)
				}
				g.mediaFiles[uniqueFilename] = g.mediaCounter
				g.mediaCounter++
			}
		}
	}

	return nil
}

// createMediaMapping creates the media mapping JSON file
func (g *APKGGenerator) createMediaMapping(tempDir string) error {
	// Create reverse mapping (number -> filename)
	mapping := make(map[string]string)
	for filename, num := range g.mediaFiles {
		mapping[fmt.Sprintf("%d", num)] = filename
	}

	data, err := marshalJSON("media", mapping)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(tempDir, "media"), data, 0644)
}

// Helper functions

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		_ = srcFile.Close()
	}()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		_ = dstFile.Close()
	}()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
