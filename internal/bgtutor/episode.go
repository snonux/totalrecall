// Package bgtutor implements the Bulgarian Podcast Tutor: an episode library of
// prepared English podcast transcripts and a vocabulary notebook, both stored as
// plain JSON files on disk. The MCP server (internal/bgtutor/mcpserver) and the
// preparation pipeline (internal/bgtutor/prepare) are thin layers on top.
//
// The on-disk format is documented in bgtutor/FORMAT.md.
package bgtutor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// FormatVersion is the current version of meta.json, paragraphs.json and
// vocabulary/saved.json.
const FormatVersion = 1

// ReferenceNote accompanies every bulgarian_reference so the voice AI treats it
// as a fallback rather than a script to read out.
const ReferenceNote = "Reference translation. Prefer your own live translation; " +
	"use this if unsure or if the learner asks."

// episodeIDPattern restricts folder names to safe, URL-friendly ids. It also
// rules out path traversal, since ids are joined onto the episodes directory.
var episodeIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,79}$`)

// Speaker describes one voice in an episode; Label is what paragraphs refer to.
type Speaker struct {
	Label string `json:"label"`
	Name  string `json:"name,omitempty"`
	Role  string `json:"role,omitempty"`
}

// Meta is the content of an episode's meta.json.
type Meta struct {
	FormatVersion   int            `json:"format_version"`
	Title           string         `json:"title"`
	Topic           string         `json:"topic,omitempty"`
	Description     string         `json:"description,omitempty"`
	Difficulty      string         `json:"difficulty,omitempty"`
	DurationMinutes float64        `json:"duration_minutes,omitempty"`
	Speakers        []Speaker      `json:"speakers,omitempty"`
	Source          map[string]any `json:"source,omitempty"`
	Status          string         `json:"status"`
	PreparedAt      string         `json:"prepared_at,omitempty"`
	PreparedBy      string         `json:"prepared_by,omitempty"`
}

// VocabNote is one vocabulary hint attached to a paragraph.
type VocabNote struct {
	BG   string `json:"bg"`
	EN   string `json:"en"`
	Note string `json:"note"`
}

// Paragraph is one record of paragraphs.json.
type Paragraph struct {
	Index              int         `json:"index"`
	Speaker            string      `json:"speaker"`
	English            string      `json:"english"`
	BulgarianReference string      `json:"bulgarian_reference"`
	GrammarNotes       []string    `json:"grammar_notes"`
	VocabularyNotes    []VocabNote `json:"vocabulary_notes"`
	Background         *string     `json:"background"`
}

// Episode is a loaded episode folder. Problem is empty when it is playable.
type Episode struct {
	ID         string
	Dir        string
	Meta       Meta
	Paragraphs []Paragraph
	Problem    string
}

// Ready reports whether the episode can be played.
func (e *Episode) Ready() bool { return e.Problem == "" }

// UserError is an error meant to be shown to the voice AI (and so the learner):
// unknown episode, index out of range, episode not prepared, bad input.
type UserError struct{ Msg string }

func (e *UserError) Error() string { return e.Msg }

func userErrorf(format string, args ...any) error {
	return &UserError{Msg: fmt.Sprintf(format, args...)}
}

// ValidEpisodeID reports whether id is a well-formed episode folder name.
func ValidEpisodeID(id string) bool { return episodeIDPattern.MatchString(id) }

// LoadEpisode reads and validates one episode folder. It never returns an
// error for bad content; problems are recorded in Episode.Problem instead so a
// broken folder still shows up in list_episodes with a reason.
func LoadEpisode(dir string) *Episode {
	ep := &Episode{ID: filepath.Base(dir), Dir: dir}
	if err := readJSON(filepath.Join(dir, "meta.json"), &ep.Meta); err != nil {
		ep.Problem = describeReadError("meta.json", err)
		return ep
	}
	problems := ValidateMeta(&ep.Meta)
	if len(problems) == 0 {
		if err := readJSON(filepath.Join(dir, "paragraphs.json"), &ep.Paragraphs); err != nil {
			problems = append(problems, describeReadError("paragraphs.json", err))
		} else {
			problems = append(problems, ValidateParagraphs(ep.Paragraphs, speakerLabels(ep.Meta))...)
		}
	}
	switch {
	case len(problems) > 0:
		ep.Problem = joinProblems(problems)
	case ep.Meta.Status != "ready":
		ep.Problem = fmt.Sprintf("episode status is %q, not \"ready\"", ep.Meta.Status)
	}
	return ep
}

// ValidateMeta returns a list of problems with meta.json; empty means valid.
func ValidateMeta(m *Meta) []string {
	var problems []string
	if m.FormatVersion != FormatVersion {
		problems = append(problems, fmt.Sprintf("meta.format_version must be %d", FormatVersion))
	}
	if strings.TrimSpace(m.Title) == "" {
		problems = append(problems, "meta.title is required")
	}
	if m.Status != "ready" && m.Status != "draft" {
		problems = append(problems, `meta.status must be "ready" or "draft"`)
	}
	for i, s := range m.Speakers {
		if strings.TrimSpace(s.Label) == "" {
			problems = append(problems, fmt.Sprintf("meta.speakers[%d].label is required", i))
		}
	}
	return problems
}

// ValidateParagraphs checks ordering, required fields and speaker labels.
// labels may be nil, in which case any speaker label is accepted.
func ValidateParagraphs(ps []Paragraph, labels map[string]bool) []string {
	if len(ps) == 0 {
		return []string{"paragraphs.json must be a non-empty array"}
	}
	var problems []string
	for i, p := range ps {
		where := fmt.Sprintf("paragraph %d", i+1)
		if p.Index != i+1 {
			problems = append(problems, fmt.Sprintf("%s: index is %d, expected %d", where, p.Index, i+1))
		}
		for name, v := range map[string]string{
			"speaker": p.Speaker, "english": p.English, "bulgarian_reference": p.BulgarianReference,
		} {
			if strings.TrimSpace(v) == "" {
				problems = append(problems, fmt.Sprintf("%s: %s must not be empty", where, name))
			}
		}
		if labels != nil && p.Speaker != "" && !labels[p.Speaker] {
			problems = append(problems, fmt.Sprintf("%s: speaker %q is not in meta.speakers", where, p.Speaker))
		}
		for j, v := range p.VocabularyNotes {
			if strings.TrimSpace(v.BG) == "" || strings.TrimSpace(v.EN) == "" {
				problems = append(problems, fmt.Sprintf("%s: vocabulary_notes[%d] needs bg and en", where, j))
			}
		}
	}
	sort.Strings(problems) // map iteration above is unordered; keep output stable
	return problems
}

func speakerLabels(m Meta) map[string]bool {
	if len(m.Speakers) == 0 {
		return nil
	}
	labels := make(map[string]bool, len(m.Speakers))
	for _, s := range m.Speakers {
		labels[s.Label] = true
	}
	return labels
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func describeReadError(name string, err error) string {
	if errors.Is(err, fs.ErrNotExist) {
		if name == "paragraphs.json" {
			return "paragraphs.json is missing (episode not prepared yet)"
		}
		return name + " is missing"
	}
	return fmt.Sprintf("%s is not valid: %v", name, err)
}

func joinProblems(problems []string) string {
	const maxShown = 5
	if len(problems) <= maxShown {
		return strings.Join(problems, "; ")
	}
	return strings.Join(problems[:maxShown], "; ") + fmt.Sprintf("; and %d more", len(problems)-maxShown)
}
