package anki

import (
	"database/sql"
	"encoding/json"
	"fmt"
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

		imageField := ""
		if card.ImageFile != "" && fileExists(card.ImageFile) {
			cardDirID := filepath.Base(filepath.Dir(card.ImageFile))
			originalFilename := filepath.Base(card.ImageFile)
			uniqueFilename := fmt.Sprintf("%s_%s", cardDirID, originalFilename)
			if _, ok := g.mediaFiles[uniqueFilename]; ok {
				imageField = fmt.Sprintf(`<img src="%s">`, uniqueFilename)
			}
		}

		audioField := ""
		if card.AudioFile != "" && fileExists(card.AudioFile) {
			cardDirID := filepath.Base(filepath.Dir(card.AudioFile))
			originalFilename := filepath.Base(card.AudioFile)
			uniqueFilename := fmt.Sprintf("%s_%s", cardDirID, originalFilename)
			if _, ok := g.mediaFiles[uniqueFilename]; ok {
				audioField = fmt.Sprintf("[sound:%s]", uniqueFilename)
			}
		}

		audioFieldBack := ""
		if card.AudioFileBack != "" && fileExists(card.AudioFileBack) {
			cardDirID := filepath.Base(filepath.Dir(card.AudioFileBack))
			originalFilename := filepath.Base(card.AudioFileBack)
			uniqueFilename := fmt.Sprintf("%s_%s", cardDirID, originalFilename)
			if _, ok := g.mediaFiles[uniqueFilename]; ok {
				audioFieldBack = fmt.Sprintf("[sound:%s]", uniqueFilename)
			}
		}

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
			guid = fmt.Sprintf("tr_bgbg_%d_%s", now.Unix(), card.Bulgarian)
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
			guid = fmt.Sprintf("tr_%d_%s", now.Unix(), card.Bulgarian)
		}

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
			0,
			0,
			"",
		)
		if err != nil {
			return fmt.Errorf("failed to insert note: %w", err)
		}

		cardQuery := `INSERT INTO cards VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		_, err = db.Exec(cardQuery,
			cardID1,
			noteID,
			g.deckID,
			0,
			now.Unix(),
			-1,
			0,
			0,
			noteID,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			"",
		)
		if err != nil {
			return fmt.Errorf("failed to insert forward card: %w", err)
		}

		_, err = db.Exec(cardQuery,
			cardID2,
			noteID,
			g.deckID,
			1,
			now.Unix(),
			-1,
			0,
			0,
			noteID+1,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			"",
		)
		if err != nil {
			return fmt.Errorf("failed to insert reverse card: %w", err)
		}
	}

	return nil
}
