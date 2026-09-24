// Package prepare turns an English podcast transcript into an episode folder
// (meta.json + paragraphs.json) using an LLM: it splits the transcript into
// teaching-sized paragraphs, labels speakers, translates each paragraph into
// Bulgarian and writes grammar, vocabulary and background notes.
//
// Long transcripts are processed in chunks so each LLM call stays small and a
// failure only costs one chunk. The LLM is behind the Generator interface so
// the pipeline can be tested without network access.
package prepare

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/snonux/totalrecall/internal/bgtutor"
)

//go:embed prompt.md
var promptTemplate string

// Generator produces a JSON document matching schema for the given prompts.
type Generator interface {
	GenerateJSON(ctx context.Context, system, user string, schema map[string]any) (string, error)
	// Name identifies the model, recorded in meta.json as prepared_by.
	Name() string
}

// Options describes one preparation run.
type Options struct {
	TranscriptPath string
	OutDir         string // episodes directory; the episode folder is created inside it
	EpisodeID      string
	Meta           bgtutor.Meta // Title, Speakers, Difficulty etc. supplied by the user
	ReferenceNotes string       // optional grammar reference / course terminology
	ChunkWords     int          // transcript words per LLM call; 0 means default
	Force          bool         // overwrite an existing paragraphs.json
}

const defaultChunkWords = 1200

// Run prepares one episode and returns the path of the episode folder.
// The folder is written with status "draft" first and flipped to "ready" only
// after every chunk succeeded and the result validated, so the MCP server
// never serves a half-prepared episode.
func Run(ctx context.Context, gen Generator, opts Options) (string, error) {
	if err := checkOptions(&opts); err != nil {
		return "", err
	}
	transcript, err := os.ReadFile(opts.TranscriptPath)
	if err != nil {
		return "", fmt.Errorf("read transcript: %w", err)
	}
	dir := filepath.Join(opts.OutDir, opts.EpisodeID)
	if _, err := os.Stat(filepath.Join(dir, "paragraphs.json")); err == nil && !opts.Force {
		return "", fmt.Errorf("%s already has paragraphs.json (use --force to overwrite)", dir)
	}

	meta := opts.Meta
	meta.FormatVersion, meta.Status = bgtutor.FormatVersion, "draft"
	meta.PreparedAt = time.Now().UTC().Format("2006-01-02")
	meta.PreparedBy = "totalrecall bgtutor prepare / " + gen.Name()
	if err := bgtutor.WriteJSONAtomic(filepath.Join(dir, "meta.json"), meta); err != nil {
		return "", err
	}

	paragraphs, err := generateParagraphs(ctx, gen, opts, meta, string(transcript))
	if err != nil {
		return dir, err
	}
	if problems := bgtutor.ValidateParagraphs(paragraphs, labelSet(meta.Speakers)); len(problems) > 0 {
		return dir, fmt.Errorf("generated paragraphs failed validation: %s", strings.Join(problems, "; "))
	}
	if err := bgtutor.WriteJSONAtomic(filepath.Join(dir, "paragraphs.json"), paragraphs); err != nil {
		return dir, err
	}
	meta.Status = "ready"
	return dir, bgtutor.WriteJSONAtomic(filepath.Join(dir, "meta.json"), meta)
}

func checkOptions(opts *Options) error {
	switch {
	case !bgtutor.ValidEpisodeID(opts.EpisodeID):
		return fmt.Errorf("episode id %q must be lowercase letters, digits and dashes, e.g. 001-cooking-basics", opts.EpisodeID)
	case strings.TrimSpace(opts.Meta.Title) == "":
		return fmt.Errorf("a title is required")
	case len(opts.Meta.Speakers) == 0:
		return fmt.Errorf("at least one speaker is required")
	}
	if opts.ChunkWords <= 0 {
		opts.ChunkWords = defaultChunkWords
	}
	if opts.Meta.Difficulty == "" {
		opts.Meta.Difficulty = "B1"
	}
	return nil
}

// generateParagraphs runs one LLM call per chunk and numbers the results.
// Vocabulary already explained in earlier chunks is passed along so notes
// don't repeat across the episode.
func generateParagraphs(ctx context.Context, gen Generator, opts Options, meta bgtutor.Meta, transcript string) ([]bgtutor.Paragraph, error) {
	system := renderPrompt(meta, opts.ReferenceNotes)
	chunks := SplitTranscript(transcript, opts.ChunkWords)
	var all []bgtutor.Paragraph
	covered := map[string]bool{}
	for i, chunk := range chunks {
		user := chunkMessage(i+1, len(chunks), chunk, all, covered)
		raw, err := gen.GenerateJSON(ctx, system, user, responseSchema(meta.Speakers))
		if err != nil {
			return nil, fmt.Errorf("chunk %d of %d: %w", i+1, len(chunks), err)
		}
		ps, err := parseChunk(raw)
		if err != nil {
			return nil, fmt.Errorf("chunk %d of %d: %w", i+1, len(chunks), err)
		}
		for _, p := range ps {
			p.Index = len(all) + 1
			all = append(all, p)
			for _, v := range p.VocabularyNotes {
				covered[v.BG] = true
			}
		}
	}
	return all, nil
}

func parseChunk(raw string) ([]bgtutor.Paragraph, error) {
	var out struct {
		Paragraphs []bgtutor.Paragraph `json:"paragraphs"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("model returned invalid JSON: %w", err)
	}
	if len(out.Paragraphs) == 0 {
		return nil, fmt.Errorf("model returned no paragraphs")
	}
	for i := range out.Paragraphs {
		p := &out.Paragraphs[i]
		if p.Background != nil && strings.TrimSpace(*p.Background) == "" {
			p.Background = nil
		}
	}
	return out.Paragraphs, nil
}

func labelSet(speakers []bgtutor.Speaker) map[string]bool {
	set := map[string]bool{}
	for _, s := range speakers {
		set[s.Label] = true
	}
	return set
}
