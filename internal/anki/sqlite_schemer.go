package anki

import (
	"crypto/sha1"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteSchemer creates and populates the Anki SQLite database (collection.anki2).
type SQLiteSchemer struct{}

// NewSQLiteSchemer returns a SQLiteSchemer ready for use.
func NewSQLiteSchemer() *SQLiteSchemer {
	return &SQLiteSchemer{}
}

// CreateDatabase opens dbPath, applies schema, and inserts collection metadata plus notes/cards.
func (s *SQLiteSchemer) CreateDatabase(dbPath string, g *APKGGenerator, tmpl *CardTemplate) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
	}()

	if err := s.createTables(db); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	if err := s.insertCollection(db, g, tmpl); err != nil {
		return fmt.Errorf("failed to insert collection: %w", err)
	}

	if err := s.insertNotesAndCards(db, g); err != nil {
		return fmt.Errorf("failed to insert notes and cards: %w", err)
	}

	return nil
}

func (s *SQLiteSchemer) createTables(db *sql.DB) error {
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

func (s *SQLiteSchemer) insertCollection(db *sql.DB, g *APKGGenerator, tmpl *CardTemplate) error {
	now := time.Now().Unix()

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
	decksJSON, err := marshalJSON("decks", decks)
	if err != nil {
		return err
	}

	models := map[string]interface{}{
		fmt.Sprintf("%d", g.modelID):     tmpl.EnBgNoteTypeConfig(g.modelID, g.deckID),
		fmt.Sprintf("%d", g.modelIDBgBg): tmpl.BgBgNoteTypeConfig(g.modelIDBgBg, g.deckID),
	}
	modelsJSON, err := marshalJSON("models", models)
	if err != nil {
		return err
	}

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
	confJSON, err := marshalJSON("conf", conf)
	if err != nil {
		return err
	}

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
	dconfJSON, err := marshalJSON("dconf", dconf)
	if err != nil {
		return err
	}

	query := `INSERT INTO col VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = db.Exec(query,
		1,
		now,
		now*1000,
		now*1000,
		11,
		0,
		0,
		0,
		string(confJSON),
		string(modelsJSON),
		string(decksJSON),
		string(dconfJSON),
		"{}",
	)
	return err
}

func marshalJSON(name string, value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", name, err)
	}

	return data, nil
}

func (s *SQLiteSchemer) insertNotesAndCards(db *sql.DB, g *APKGGenerator) error {
	now := time.Now()

	for i, card := range g.cards {
		noteID := now.UnixMilli() + int64(i*3)
		cardID1 := noteID + 1
		cardID2 := noteID + 2

		isBgBg := card.CardType == "bg-bg"

		imageField := buildMediaField(card.ImageFile, g.mediaFiles, func(name string) string {
			return fmt.Sprintf(`<img src="%s">`, name)
		})
		audioField := buildMediaField(card.AudioFile, g.mediaFiles, func(name string) string {
			return fmt.Sprintf("[sound:%s]", name)
		})
		audioFieldBack := buildMediaField(card.AudioFileBack, g.mediaFiles, func(name string) string {
			return fmt.Sprintf("[sound:%s]", name)
		})

		var fields string
		var modelID int64
		var guid string

		if isBgBg {
			fields = strings.Join([]string{
				card.Bulgarian,
				card.Translation,
				imageField,
				audioField,
				audioFieldBack,
				card.Notes,
			}, "\x1f")
			modelID = g.modelIDBgBg
			guid = ankiGUID(fmt.Sprintf("tr_bgbg_%s", card.Bulgarian))
		} else {
			english := card.Translation
			if english == "" {
				english = "Translation needed"
			}
			fields = strings.Join([]string{
				english,
				card.Bulgarian,
				imageField,
				audioField,
				card.Notes,
			}, "\x1f")
			modelID = g.modelID
			guid = ankiGUID(fmt.Sprintf("tr_%s", card.Bulgarian))
		}

		csum := fieldChecksum(card.Bulgarian)

		noteQuery := `INSERT INTO notes VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		_, err := db.Exec(noteQuery,
			noteID,
			guid,
			modelID,
			now.Unix(),
			-1,
			"",
			fields,
			card.Bulgarian,
			csum,
			0,
			"",
		)
		if err != nil {
			return fmt.Errorf("failed to insert note: %w", err)
		}

		// For new cards (type=0), due is the position in the new-card queue
		dueForward := i * 2
		dueReverse := i*2 + 1

		cardQuery := `INSERT INTO cards VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		_, err = db.Exec(cardQuery,
			cardID1, noteID, g.deckID,
			0,            // ord
			now.Unix(),   // mod
			-1,           // usn
			0,            // type (new)
			0,            // queue (new)
			dueForward,   // due (position in new queue)
			0, 0, 0, 0, 0, 0, 0, 0, "",
		)
		if err != nil {
			return fmt.Errorf("failed to insert forward card: %w", err)
		}

		_, err = db.Exec(cardQuery,
			cardID2, noteID, g.deckID,
			1,            // ord
			now.Unix(),   // mod
			-1,           // usn
			0,            // type (new)
			0,            // queue (new)
			dueReverse,   // due (position in new queue)
			0, 0, 0, 0, 0, 0, 0, 0, "",
		)
		if err != nil {
			return fmt.Errorf("failed to insert reverse card: %w", err)
		}
	}

	return nil
}

// buildMediaField resolves a media file path to an Anki field string using the formatter.
func buildMediaField(filePath string, mediaFiles map[string]int, formatter func(string) string) string {
	if filePath == "" || !fileExists(filePath) {
		return ""
	}
	cardDirID := filepath.Base(filepath.Dir(filePath))
	originalFilename := filepath.Base(filePath)
	uniqueFilename := fmt.Sprintf("%s_%s", cardDirID, originalFilename)
	if _, ok := mediaFiles[uniqueFilename]; ok {
		return formatter(uniqueFilename)
	}
	return ""
}

// fieldChecksum computes Anki's csum: first 4 bytes of SHA-1 of the sort field, as uint32.
func fieldChecksum(sortField string) uint32 {
	h := sha1.Sum([]byte(sortField))
	return binary.BigEndian.Uint32(h[:4])
}

// ankiGUID generates a short, stable, base91-style GUID from a seed string.
// Anki expects GUIDs to be ~10 characters. We hash the seed for stability
// (same word always produces the same GUID) and encode as base91.
func ankiGUID(seed string) string {
	h := sha1.Sum([]byte(seed))
	return base91Encode(h[:8])
}

// base91Encode encodes bytes into Anki's base91 character set (same as Anki's Python implementation).
func base91Encode(data []byte) string {
	const table = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789" +
		"!#$%&()*+,-./:;<=>?@[]^_`{|}~"
	// Use a simple encoding: convert to a big random-ish number via the hash bytes
	// and repeatedly divide by 91
	var result []byte
	// Treat data as a big-endian unsigned integer
	val := uint64(0)
	for _, b := range data {
		val = val*256 + uint64(b)
	}
	for val > 0 {
		result = append(result, table[val%91])
		val /= 91
	}
	if len(result) == 0 {
		// Fallback: generate a random GUID
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for i := 0; i < 10; i++ {
			result = append(result, table[r.Intn(91)])
		}
	}
	return string(result)
}
