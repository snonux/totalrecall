package bgtutor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// VocabKinds lists what can be saved to the notebook.
var VocabKinds = []string{"word", "phrase", "rule"}

// VocabItem is one entry in vocabulary/saved.json.
type VocabItem struct {
	ID             int    `json:"id"`
	Term           string `json:"term"`
	Kind           string `json:"kind"`
	Translation    string `json:"translation,omitempty"`
	Note           string `json:"note,omitempty"`
	EpisodeID      string `json:"episode_id,omitempty"`
	ParagraphIndex int    `json:"paragraph_index,omitempty"`
	Context        string `json:"context,omitempty"`
	SavedAt        string `json:"saved_at"`
	LastSavedAt    string `json:"last_saved_at,omitempty"`
	TimesSaved     int    `json:"times_saved"`
}

type vocabFile struct {
	FormatVersion int         `json:"format_version"`
	Items         []VocabItem `json:"items"`
}

// SaveRequest is the input of save_vocabulary.
type SaveRequest struct {
	Term           string
	Kind           string
	Translation    string
	Note           string
	EpisodeID      string
	ParagraphIndex int
}

// SaveResult reports whether the term was new ("saved") or already in the
// notebook ("already_saved"), plus the stored item.
type SaveResult struct {
	Status string    `json:"status"`
	Item   VocabItem `json:"item"`
}

// ListRequest is the input of list_vocabulary; zero values mean "no filter".
type ListRequest struct {
	Query     string
	Kind      string
	EpisodeID string
	Limit     int
}

// ListResult is the output of list_vocabulary.
type ListResult struct {
	TotalMatching int         `json:"total_matching"`
	Returned      int         `json:"returned"`
	Items         []VocabItem `json:"items"`
}

// Notebook is the learner's vocabulary notebook: a single JSON file written
// atomically. A mutex serialises writers within this process, which is the
// only writer (the MCP server).
type Notebook struct {
	Path    string
	Library *Library // optional; used to attach the source sentence as context
	now     func() time.Time
	mu      sync.Mutex
}

// NewNotebook returns a notebook stored at path.
func NewNotebook(path string, lib *Library) *Notebook {
	return &Notebook{Path: path, Library: lib, now: time.Now}
}

// Save adds a term, or bumps times_saved and merges the note when the same
// term and kind are already present, so repeated saves become a "this keeps
// tripping me up" signal instead of duplicates.
func (n *Notebook) Save(req SaveRequest) (*SaveResult, error) {
	req.Term = strings.TrimSpace(req.Term)
	if req.Kind == "" {
		req.Kind = "word"
	}
	if err := validateSave(req); err != nil {
		return nil, err
	}
	context := n.sourceSentence(req.EpisodeID, req.ParagraphIndex)

	n.mu.Lock()
	defer n.mu.Unlock()
	f, err := n.read()
	if err != nil {
		return nil, err
	}
	stamp := n.now().UTC().Format(time.RFC3339)
	if i := findItem(f.Items, req.Term, req.Kind); i >= 0 {
		mergeItem(&f.Items[i], req, stamp)
		return &SaveResult{Status: "already_saved", Item: f.Items[i]}, n.write(f)
	}
	item := VocabItem{
		ID: nextID(f.Items), Term: req.Term, Kind: req.Kind, Translation: req.Translation,
		Note: req.Note, EpisodeID: req.EpisodeID, ParagraphIndex: req.ParagraphIndex,
		Context: context, SavedAt: stamp, TimesSaved: 1,
	}
	f.Items = append(f.Items, item)
	return &SaveResult{Status: "saved", Item: item}, n.write(f)
}

// List returns matching items, newest first.
func (n *Notebook) List(req ListRequest) (*ListResult, error) {
	if req.Kind != "" && !validKind(req.Kind) {
		return nil, userErrorf("kind must be one of %s.", strings.Join(VocabKinds, ", "))
	}
	n.mu.Lock()
	f, err := n.read()
	n.mu.Unlock()
	if err != nil {
		return nil, err
	}
	matches := make([]VocabItem, 0, len(f.Items))
	for _, it := range f.Items {
		if matchesFilter(it, req) {
			matches = append(matches, it)
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].SavedAt > matches[j].SavedAt })
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	limit = min(limit, 500, len(matches))
	return &ListResult{TotalMatching: len(matches), Returned: limit, Items: matches[:limit]}, nil
}

func validateSave(req SaveRequest) error {
	switch {
	case req.Term == "":
		return userErrorf("term must not be empty.")
	case len([]rune(req.Term)) > 500:
		return userErrorf("term is too long (max 500 characters).")
	case !validKind(req.Kind):
		return userErrorf("kind must be one of %s.", strings.Join(VocabKinds, ", "))
	case req.EpisodeID != "" && !ValidEpisodeID(req.EpisodeID):
		return userErrorf("Invalid episode id %q.", req.EpisodeID)
	}
	return nil
}

// sourceSentence looks up the English paragraph a term came from. It is a
// nicety: any failure just means the item is saved without context.
func (n *Notebook) sourceSentence(episodeID string, index int) string {
	if n.Library == nil || episodeID == "" || index < 1 {
		return ""
	}
	p, err := n.Library.Paragraph(episodeID, index)
	if err != nil {
		return ""
	}
	return p.English
}

func findItem(items []VocabItem, term, kind string) int {
	for i, it := range items {
		if it.Kind == kind && strings.EqualFold(it.Term, term) {
			return i
		}
	}
	return -1
}

func mergeItem(it *VocabItem, req SaveRequest, stamp string) {
	it.TimesSaved++
	it.LastSavedAt = stamp
	if it.Translation == "" {
		it.Translation = req.Translation
	}
	if req.Note != "" && !strings.Contains(it.Note, req.Note) {
		if it.Note == "" {
			it.Note = req.Note
		} else {
			it.Note += " | " + req.Note
		}
	}
}

func nextID(items []VocabItem) int {
	maxID := 0
	for _, it := range items {
		maxID = max(maxID, it.ID)
	}
	return maxID + 1
}

func matchesFilter(it VocabItem, req ListRequest) bool {
	if req.Kind != "" && it.Kind != req.Kind {
		return false
	}
	if req.EpisodeID != "" && it.EpisodeID != req.EpisodeID {
		return false
	}
	if req.Query == "" {
		return true
	}
	q := strings.ToLower(req.Query)
	for _, field := range []string{it.Term, it.Translation, it.Note} {
		if strings.Contains(strings.ToLower(field), q) {
			return true
		}
	}
	return false
}

func validKind(kind string) bool {
	for _, k := range VocabKinds {
		if k == kind {
			return true
		}
	}
	return false
}

func (n *Notebook) read() (*vocabFile, error) {
	f := &vocabFile{FormatVersion: FormatVersion, Items: []VocabItem{}}
	data, err := os.ReadFile(n.Path)
	if os.IsNotExist(err) {
		return f, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, f); err != nil {
		return nil, fmt.Errorf("%s is not a valid vocabulary file: %w", n.Path, err)
	}
	return f, nil
}

func (n *Notebook) write(f *vocabFile) error {
	return WriteJSONAtomic(n.Path, f)
}

// WriteJSONAtomic writes v as indented JSON via a temp file and rename, so a
// crash never leaves a half-written file behind.
func WriteJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // no-op after a successful rename
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
