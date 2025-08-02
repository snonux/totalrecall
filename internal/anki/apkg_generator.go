package anki

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// APKGGenerator creates Anki package files (.apkg)
type APKGGenerator struct {
	deckName     string
	deckID       int64
	modelID      int64
	cards        []Card
	mediaFiles   map[string]int // maps original filename to media number
	mediaCounter int
}

// NewAPKGGenerator creates a new APKG generator
func NewAPKGGenerator(deckName string) *APKGGenerator {
	// Generate IDs based on timestamp to ensure uniqueness
	now := time.Now().UnixMilli()
	return &APKGGenerator{
		deckName:     deckName,
		deckID:       now,
		modelID:      now + 1,
		cards:        make([]Card, 0),
		mediaFiles:   make(map[string]int),
		mediaCounter: 0,
	}
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
	defer os.RemoveAll(tempDir)

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

	// Create the .apkg zip file
	if err := g.createZipPackage(tempDir, outputPath); err != nil {
		return fmt.Errorf("failed to create zip package: %w", err)
	}

	return nil
}

// createDatabase creates the Anki SQLite database
func (g *APKGGenerator) createDatabase(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// Create tables
	if err := g.createTables(db); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	// Insert collection metadata
	if err := g.insertCollection(db); err != nil {
		return fmt.Errorf("failed to insert collection: %w", err)
	}

	// Insert notes and cards
	if err := g.insertNotesAndCards(db); err != nil {
		return fmt.Errorf("failed to insert notes and cards: %w", err)
	}

	return nil
}

// createTables creates the required Anki database tables
func (g *APKGGenerator) createTables(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE col (
			id integer PRIMARY KEY,
			crt integer NOT NULL,
			mod integer NOT NULL,
			scm integer NOT NULL,
			ver integer NOT NULL,
			dty integer NOT NULL,
			usn integer NOT NULL,
			ls integer NOT NULL,
			conf text NOT NULL,
			models text NOT NULL,
			decks text NOT NULL,
			dconf text NOT NULL,
			tags text NOT NULL
		)`,
		`CREATE TABLE notes (
			id integer PRIMARY KEY,
			guid text NOT NULL,
			mid integer NOT NULL,
			mod integer NOT NULL,
			usn integer NOT NULL,
			tags text NOT NULL,
			flds text NOT NULL,
			sfld text NOT NULL,
			csum integer NOT NULL,
			flags integer NOT NULL,
			data text NOT NULL
		)`,
		`CREATE TABLE cards (
			id integer PRIMARY KEY,
			nid integer NOT NULL,
			did integer NOT NULL,
			ord integer NOT NULL,
			mod integer NOT NULL,
			usn integer NOT NULL,
			type integer NOT NULL,
			queue integer NOT NULL,
			due integer NOT NULL,
			ivl integer NOT NULL,
			factor integer NOT NULL,
			reps integer NOT NULL,
			lapses integer NOT NULL,
			left integer NOT NULL,
			odue integer NOT NULL,
			odid integer NOT NULL,
			flags integer NOT NULL,
			data text NOT NULL
		)`,
		`CREATE TABLE revlog (
			id integer PRIMARY KEY,
			cid integer NOT NULL,
			usn integer NOT NULL,
			ease integer NOT NULL,
			ivl integer NOT NULL,
			lastIvl integer NOT NULL,
			factor integer NOT NULL,
			time integer NOT NULL,
			type integer NOT NULL
		)`,
		`CREATE TABLE graves (
			usn integer NOT NULL,
			oid integer NOT NULL,
			type integer NOT NULL
		)`,
		// Create indexes
		`CREATE INDEX ix_notes_csum ON notes (csum)`,
		`CREATE INDEX ix_notes_usn ON notes (usn)`,
		`CREATE INDEX ix_cards_usn ON cards (usn)`,
		`CREATE INDEX ix_cards_nid ON cards (nid)`,
		`CREATE INDEX ix_cards_sched ON cards (did, queue, due)`,
		`CREATE INDEX ix_revlog_usn ON revlog (usn)`,
		`CREATE INDEX ix_revlog_cid ON revlog (cid)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
	}

	return nil
}

// insertCollection inserts the collection metadata
func (g *APKGGenerator) insertCollection(db *sql.DB) error {
	now := time.Now().Unix()

	// Create deck configuration
	// The arrays are [learningCount, reviewCount] for today's stats
	decks := map[string]interface{}{
		"1": map[string]interface{}{
			"id":               1,
			"name":             "Default",
			"mod":              now,
			"desc":             "",
			"collapsed":        false,
			"dyn":              0,
			"conf":             1,
			"usn":              0,
			"newToday":         []int{0, 0},
			"revToday":         []int{0, 0},
			"lrnToday":         []int{0, 0},
			"timeToday":        []int{0, 0},
			"browserCollapsed": false,
			"extendNew":        10,
			"extendRev":        50,
		},
		fmt.Sprintf("%d", g.deckID): map[string]interface{}{
			"id":               g.deckID,
			"name":             g.deckName,
			"mod":              now,
			"desc":             "Bulgarian vocabulary cards created by TotalRecall",
			"collapsed":        false,
			"dyn":              0,
			"conf":             1,
			"usn":              0,
			"newToday":         []int{0, 0},
			"revToday":         []int{0, 0},
			"lrnToday":         []int{0, 0},
			"timeToday":        []int{0, 0},
			"browserCollapsed": false,
			"extendNew":        10,
			"extendRev":        50,
		},
	}
	decksJSON, _ := json.Marshal(decks)

	// Create model (note type) configuration
	models := map[string]interface{}{
		fmt.Sprintf("%d", g.modelID): g.createNoteTypeConfig(),
	}
	modelsJSON, _ := json.Marshal(models)

	// Default configuration
	conf := map[string]interface{}{
		"nextPos":       1,
		"estTimes":      true,
		"activeDecks":   []int64{1},
		"sortType":      "noteFld",
		"sortBackwards": false,
		"addToCur":      true,
		"curDeck":       1,
		"newSpread":     0,
		"dueCounts":     true,
		"collapseTime":  1200,
		"timeLim":       0,
		"schedVer":      1,
		"curModel":      fmt.Sprintf("%d", g.modelID),
		"dayLearnFirst": false,
	}
	confJSON, _ := json.Marshal(conf)

	// Deck options
	dconf := map[string]interface{}{
		"1": map[string]interface{}{
			"id":   1,
			"name": "Default",
			"dyn":  0,
			"new": map[string]interface{}{
				"delays":        []int{1, 10},
				"ints":          []int{1, 4, 7},
				"initialFactor": 2500,
				"perDay":        20,
				"order":         1,
				"bury":          true,
				"separate":      true,
			},
			"lapse": map[string]interface{}{
				"delays":      []int{10},
				"mult":        0,
				"minInt":      1,
				"leechFails":  8,
				"leechAction": 0,
			},
			"rev": map[string]interface{}{
				"perDay":   100,
				"ease4":    1.3,
				"fuzz":     0.05,
				"maxIvl":   36500,
				"ivlFct":   1,
				"bury":     true,
				"minSpace": 1,
			},
			"timer":    0,
			"maxTaken": 60,
			"usn":      0,
			"mod":      now,
			"autoplay": true,
			"replayq":  true,
		},
	}
	dconfJSON, _ := json.Marshal(dconf)

	query := `INSERT INTO col VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := db.Exec(query,
		1,        // id
		now,      // crt
		now*1000, // mod
		now*1000, // scm
		11,       // ver (schema version)
		0,        // dty
		0,        // usn
		0,        // ls
		string(confJSON),
		string(modelsJSON),
		string(decksJSON),
		string(dconfJSON),
		"{}", // tags
	)
	return err
}

// createNoteTypeConfig creates the note type configuration
func (g *APKGGenerator) createNoteTypeConfig() map[string]interface{} {
	return map[string]interface{}{
		"id":    g.modelID,
		"name":  "Vocabulary from TotalRecall (Basic + Reverse)",
		"type":  0,
		"mod":   time.Now().Unix(),
		"usn":   -1,
		"sortf": 0,
		"did":   g.deckID,
		"req":   [][]interface{}{[]interface{}{0, "all", []int{0}}, []interface{}{1, "all", []int{1}}},
		"vers":  []int{},
		"tags":  []string{},
		"latexPre": `\documentclass[12pt]{article}
\special{papersize=3in,5in}
\usepackage[utf8]{inputenc}
\usepackage{amssymb,amsmath}
\pagestyle{empty}
\setlength{\parindent}{0in}
\begin{document}`,
		"latexPost": `\end{document}`,
		"flds": []map[string]interface{}{
			{
				"name":   "English",
				"ord":    0,
				"sticky": false,
				"rtl":    false,
				"font":   "Arial",
				"size":   20,
				"media":  []string{},
			},
			{
				"name":   "Bulgarian",
				"ord":    1,
				"sticky": false,
				"rtl":    false,
				"font":   "Arial",
				"size":   20,
				"media":  []string{},
			},
			{
				"name":   "Image",
				"ord":    2,
				"sticky": false,
				"rtl":    false,
				"font":   "Arial",
				"size":   20,
				"media":  []string{},
			},
			{
				"name":   "Audio",
				"ord":    3,
				"sticky": false,
				"rtl":    false,
				"font":   "Arial",
				"size":   20,
				"media":  []string{},
			},
			{
				"name":   "Notes",
				"ord":    4,
				"sticky": false,
				"rtl":    false,
				"font":   "Arial",
				"size":   16,
				"media":  []string{},
			},
		},
		"tmpls": []map[string]interface{}{
			{
				"name":  "Forward",
				"ord":   0,
				"qfmt":  g.getFrontTemplate(),
				"afmt":  g.getBackTemplate(),
				"did":   nil,
				"bqfmt": "",
				"bafmt": "",
			},
			{
				"name":  "Reverse",
				"ord":   1,
				"qfmt":  g.getReverseFrontTemplate(),
				"afmt":  g.getReverseBackTemplate(),
				"did":   nil,
				"bqfmt": "",
				"bafmt": "",
			},
		},
		"css": g.getCSS(),
	}
}

// getFrontTemplate returns the question template
func (g *APKGGenerator) getFrontTemplate() string {
	return `<div class="front">
{{#Image}}
<div class="image-container">
{{Image}}
</div>
{{/Image}}
<div class="english">{{English}}</div>
</div>`
}

// getBackTemplate returns the answer template
func (g *APKGGenerator) getBackTemplate() string {
	return `{{FrontSide}}

<hr id="answer">

<div class="back">
<div class="bulgarian">{{Bulgarian}}</div>
{{#Audio}}
<div class="audio">{{Audio}}</div>
{{/Audio}}
{{#Notes}}
<div class="notes">{{Notes}}</div>
{{/Notes}}
</div>`
}

// getReverseFrontTemplate returns the question template for the reverse card
func (g *APKGGenerator) getReverseFrontTemplate() string {
	return `<div class="front">
<div class="bulgarian">{{Bulgarian}}</div>
</div>`
}

// getReverseBackTemplate returns the answer template for the reverse card
func (g *APKGGenerator) getReverseBackTemplate() string {
	return `{{FrontSide}}

<hr id="answer">

<div class="back">
<div class="english">{{English}}</div>
{{#Image}}
<div class="image-container">
{{Image}}
</div>
{{/Image}}
{{#Audio}}
<div class="audio">{{Audio}}</div>
{{/Audio}}
{{#Notes}}
<div class="notes">{{Notes}}</div>
{{/Notes}}
</div>`
}

// getCSS returns the card styling
func (g *APKGGenerator) getCSS() string {
	return `.card {
  font-family: Arial, sans-serif;
  font-size: 20px;
  text-align: center;
  color: #333;
  background-color: white;
}

.front, .back {
  padding: 20px;
}

.image-container {
  margin: 20px auto;
  max-width: 400px;
}

.image-container img {
  max-width: 100%;
  height: auto;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.1);
}

.english {
  font-size: 28px;
  font-weight: bold;
  color: #2c3e50;
  margin: 20px 0;
}

.bulgarian {
  font-size: 32px;
  font-weight: bold;
  color: #c0392b;
  margin: 20px 0;
}

.audio {
  margin: 15px 0;
}

.notes {
  font-size: 16px;
  color: #7f8c8d;
  margin-top: 20px;
  font-style: italic;
}

hr#answer {
  margin: 30px 0;
  border: 0;
  border-top: 1px solid #ecf0f1;
}`
}

// insertNotesAndCards inserts all notes and cards into the database
func (g *APKGGenerator) insertNotesAndCards(db *sql.DB) error {
	now := time.Now()

	for i, card := range g.cards {
		// Generate unique IDs, leaving space for 2 cards per note
		noteID := now.UnixMilli() + int64(i*3)
		cardID1 := noteID + 1
		cardID2 := noteID + 2

		// Prepare field values
		english := card.Translation
		if english == "" {
			english = "Translation needed"
		}

		imageField := ""
		if card.ImageFile != "" && fileExists(card.ImageFile) {
			// Get card ID from the source path (parent directory name)
			cardID := filepath.Base(filepath.Dir(card.ImageFile))
			originalFilename := filepath.Base(card.ImageFile)
			// Create unique filename with card ID prefix
			uniqueFilename := fmt.Sprintf("%s_%s", cardID, originalFilename)

			if _, ok := g.mediaFiles[uniqueFilename]; ok {
				// Use the unique filename in the card content
				imageField = fmt.Sprintf(`<img src="%s">`, uniqueFilename)
			}
		}

		audioField := ""
		if card.AudioFile != "" && fileExists(card.AudioFile) {
			// Get card ID from the source path (parent directory name)
			cardID := filepath.Base(filepath.Dir(card.AudioFile))
			originalFilename := filepath.Base(card.AudioFile)
			// Create unique filename with card ID prefix
			uniqueFilename := fmt.Sprintf("%s_%s", cardID, originalFilename)

			if _, ok := g.mediaFiles[uniqueFilename]; ok {
				// Use the unique filename in the card content
				audioField = fmt.Sprintf("[sound:%s]", uniqueFilename)
			}
		}

		// Join fields with field separator (ASCII 31)
		fields := strings.Join([]string{
			english,
			card.Bulgarian,
			imageField,
			audioField,
			card.Notes,
		}, "\x1f")

		// Generate GUID
		guid := fmt.Sprintf("tr_%d_%s", now.Unix(), card.Bulgarian)

		// Insert note
		noteQuery := `INSERT INTO notes VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		_, err := db.Exec(noteQuery,
			noteID,         // id
			guid,           // guid
			g.modelID,      // mid
			now.Unix(),     // mod
			-1,             // usn
			"",             // tags
			fields,         // flds
			card.Bulgarian, // sfld (sort field)
			0,              // csum
			0,              // flags
			"",             // data
		)
		if err != nil {
			return fmt.Errorf("failed to insert note: %w", err)
		}

		// Insert card 1 (Forward)
		cardQuery := `INSERT INTO cards VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		_, err = db.Exec(cardQuery,
			cardID1,    // id
			noteID,     // nid
			g.deckID,   // did
			0,          // ord (template 0)
			now.Unix(), // mod
			-1,         // usn
			0,          // type (0=new)
			0,          // queue (0=new)
			noteID,     // due (for new cards, this is position)
			0,          // ivl
			0,          // factor
			0,          // reps
			0,          // lapses
			0,          // left
			0,          // odue
			0,          // odid
			0,          // flags
			"",         // data
		)
		if err != nil {
			return fmt.Errorf("failed to insert forward card: %w", err)
		}

		// Insert card 2 (Reverse)
		_, err = db.Exec(cardQuery,
			cardID2,    // id
			noteID,     // nid
			g.deckID,   // did
			1,          // ord (template 1)
			now.Unix(), // mod
			-1,         // usn
			0,          // type (0=new)
			0,          // queue (0=new)
			noteID+1,   // due (for new cards, this is position, should be unique)
			0,          // ivl
			0,          // factor
			0,          // reps
			0,          // lapses
			0,          // left
			0,          // odue
			0,          // odid
			0,          // flags
			"",         // data
		)
		if err != nil {
			return fmt.Errorf("failed to insert reverse card: %w", err)
		}
	}

	return nil
}

// copyMediaFiles copies media files and assigns them numbers
func (g *APKGGenerator) copyMediaFiles(tempDir string) error {
	// Media files go directly in the temp directory with numeric names

	for _, card := range g.cards {
		// Copy audio file
		if card.AudioFile != "" && fileExists(card.AudioFile) {
			// Get card ID from the source path (parent directory name)
			cardID := filepath.Base(filepath.Dir(card.AudioFile))
			originalFilename := filepath.Base(card.AudioFile)
			// Create unique filename with card ID prefix
			uniqueFilename := fmt.Sprintf("%s_%s", cardID, originalFilename)

			if _, exists := g.mediaFiles[uniqueFilename]; !exists {
				targetPath := filepath.Join(tempDir, fmt.Sprintf("%d", g.mediaCounter))
				if err := copyFile(card.AudioFile, targetPath); err != nil {
					return fmt.Errorf("failed to copy audio file %s: %w", card.AudioFile, err)
				}
				g.mediaFiles[uniqueFilename] = g.mediaCounter
				g.mediaCounter++
			}
		}

		// Copy image file
		if card.ImageFile != "" && fileExists(card.ImageFile) {
			// Get card ID from the source path (parent directory name)
			cardID := filepath.Base(filepath.Dir(card.ImageFile))
			originalFilename := filepath.Base(card.ImageFile)
			// Create unique filename with card ID prefix
			uniqueFilename := fmt.Sprintf("%s_%s", cardID, originalFilename)

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

	data, err := json.Marshal(mapping)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(tempDir, "media"), data, 0644)
}

// createZipPackage creates the final .apkg zip file
func (g *APKGGenerator) createZipPackage(tempDir, outputPath string) error {
	// Create the zip file
	zipFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	// Walk the temp directory and add all files to the zip
	return filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(tempDir, path)
		if err != nil {
			return err
		}

		// Create zip entry
		writer, err := archive.Create(relPath)
		if err != nil {
			return err
		}

		// Open and copy file
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
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
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
