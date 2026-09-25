package bgtutor

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Library reads episodes from a directory with one folder per episode. It
// re-scans on every call, so dropping a new folder in makes it available
// without restarting the server. Episodes are small, so this stays cheap.
type Library struct {
	Dir string
}

// NewLibrary returns a library rooted at episodesDir.
func NewLibrary(episodesDir string) *Library { return &Library{Dir: episodesDir} }

// EpisodeSummary is what list_episodes returns for each episode.
type EpisodeSummary struct {
	EpisodeID       string    `json:"episode_id"`
	Title           string    `json:"title"`
	Topic           string    `json:"topic,omitempty"`
	Description     string    `json:"description,omitempty"`
	Difficulty      string    `json:"difficulty,omitempty"`
	DurationMinutes float64   `json:"duration_minutes,omitempty"`
	Speakers        []Speaker `json:"speakers,omitempty"`
	ParagraphCount  int       `json:"paragraph_count"`
	Ready           bool      `json:"ready"`
	NotReadyReason  string    `json:"not_ready_reason,omitempty"`
}

// ParagraphPayload is what get_paragraph returns: one paragraph plus the
// position marker the voice AI uses to decide whether to continue.
type ParagraphPayload struct {
	EpisodeID              string      `json:"episode_id"`
	EpisodeTitle           string      `json:"episode_title"`
	Index                  int         `json:"index"`
	Position               string      `json:"position"`
	TotalParagraphs        int         `json:"total_paragraphs"`
	IsLast                 bool        `json:"is_last"`
	NextIndex              *int        `json:"next_index"`
	Speaker                string      `json:"speaker"`
	English                string      `json:"english"`
	BulgarianReference     string      `json:"bulgarian_reference"`
	BulgarianReferenceNote string      `json:"bulgarian_reference_note"`
	GrammarNotes           []string    `json:"grammar_notes"`
	VocabularyNotes        []VocabNote `json:"vocabulary_notes"`
	Background             *string     `json:"background"`
}

// Episodes loads every episode folder, sorted by id. Folders whose names are
// not valid episode ids (e.g. hidden or scratch folders) are skipped.
func (l *Library) Episodes() ([]*Episode, error) {
	entries, err := os.ReadDir(l.Dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var eps []*Episode
	for _, e := range entries {
		if e.IsDir() && ValidEpisodeID(e.Name()) {
			eps = append(eps, LoadEpisode(filepath.Join(l.Dir, e.Name())))
		}
	}
	sort.Slice(eps, func(i, j int) bool { return eps[i].ID < eps[j].ID })
	return eps, nil
}

// Episode loads one episode by id, or returns a UserError naming the
// available episodes when it doesn't exist.
func (l *Library) Episode(id string) (*Episode, error) {
	if !ValidEpisodeID(id) {
		return nil, userErrorf("Invalid episode id %q. Use an episode_id from list_episodes.", id)
	}
	dir := filepath.Join(l.Dir, id)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return nil, userErrorf("Unknown episode %q. Available episodes: %s.", id, l.readyIDs())
	}
	return LoadEpisode(dir), nil
}

func (l *Library) readyIDs() string {
	eps, _ := l.Episodes()
	var ids []string
	for _, e := range eps {
		if e.Ready() {
			ids = append(ids, e.ID)
		}
	}
	if len(ids) == 0 {
		return "none"
	}
	return strings.Join(ids, ", ")
}

// ListEpisodes returns a summary of every episode, including ones that are
// not ready (with the reason), so a half-prepared upload is visible.
func (l *Library) ListEpisodes() ([]EpisodeSummary, error) {
	eps, err := l.Episodes()
	if err != nil {
		return nil, err
	}
	out := make([]EpisodeSummary, 0, len(eps))
	for _, e := range eps {
		title := e.Meta.Title
		if title == "" {
			title = e.ID
		}
		out = append(out, EpisodeSummary{
			EpisodeID: e.ID, Title: title, Topic: e.Meta.Topic,
			Description: e.Meta.Description, Difficulty: e.Meta.Difficulty,
			DurationMinutes: e.Meta.DurationMinutes, Speakers: e.Meta.Speakers,
			ParagraphCount: len(e.Paragraphs), Ready: e.Ready(), NotReadyReason: e.Problem,
		})
	}
	return out, nil
}

// Paragraph returns one paragraph (1-based index) with its position marker.
// It is stateless: the caller passes the index every time.
func (l *Library) Paragraph(id string, index int) (*ParagraphPayload, error) {
	ep, err := l.Episode(id)
	if err != nil {
		return nil, err
	}
	if !ep.Ready() {
		return nil, userErrorf("Episode %q is not ready to play: %s", id, ep.Problem)
	}
	total := len(ep.Paragraphs)
	if index < 1 || index > total {
		return nil, userErrorf("Paragraph index %d is out of range for %q. Valid indexes are 1 to %d.",
			index, id, total)
	}
	p := ep.Paragraphs[index-1]
	payload := &ParagraphPayload{
		EpisodeID: ep.ID, EpisodeTitle: ep.Meta.Title, Index: index,
		Position: positionMarker(index, total), TotalParagraphs: total, IsLast: index == total,
		Speaker: p.Speaker, English: p.English, BulgarianReference: p.BulgarianReference,
		BulgarianReferenceNote: ReferenceNote, GrammarNotes: nonNil(p.GrammarNotes),
		VocabularyNotes: p.VocabularyNotes, Background: p.Background,
	}
	if payload.VocabularyNotes == nil {
		payload.VocabularyNotes = []VocabNote{}
	}
	if !payload.IsLast {
		next := index + 1
		payload.NextIndex = &next
	}
	return payload, nil
}

// positionMarker renders the human-readable position, e.g. "3 of 12".
func positionMarker(index, total int) string {
	return fmt.Sprintf("%d of %d", index, total)
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
