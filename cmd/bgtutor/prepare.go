package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/snonux/totalrecall/internal/bgtutor"
	"github.com/snonux/totalrecall/internal/bgtutor/prepare"
)

type prepareFlags struct {
	id, title, topic, description, difficulty, reference, model, audio string
	speakers                                                           []string
	minutes                                                            float64
	chunkWords                                                         int
	force                                                              bool
}

func newPrepareCmd() *cobra.Command {
	var f prepareFlags
	cmd := &cobra.Command{
		Use:   "prepare TRANSCRIPT",
		Short: "Turn an English transcript into an episode folder using Gemini",
		Example: `  bgtutor prepare transcript.txt --id 002-morning-news --title "Morning News" \
    --speaker "Host:Maria:news anchor" --speaker "Guest:Peter:economist" --difficulty B1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dataDir, _ := cmd.Flags().GetString("data-dir")
			return runPrepare(cmd, dataDir, args[0], f)
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&f.id, "id", "", "episode id / folder name, e.g. 002-morning-news (required)")
	fl.StringVar(&f.title, "title", "", "episode title (required)")
	fl.StringArrayVar(&f.speakers, "speaker", nil, `speaker as "Label:Name:Role", repeatable (default "Narrator")`)
	fl.StringVar(&f.topic, "topic", "", "short topic label")
	fl.StringVar(&f.description, "description", "", "one or two sentence description")
	fl.StringVar(&f.difficulty, "difficulty", "B1", "learner CEFR level the notes are pitched at")
	fl.Float64Var(&f.minutes, "minutes", 0, "length of the original audio in minutes")
	fl.StringVar(&f.audio, "audio", "", "original audio file to copy into the episode folder (reference only)")
	fl.StringVar(&f.reference, "reference", "", "optional grammar reference / textbook notes (text file) to align terminology")
	fl.StringVar(&f.model, "model", prepare.DefaultGeminiModel, "Gemini model")
	fl.IntVar(&f.chunkWords, "chunk-words", 1200, "transcript words per LLM call")
	fl.BoolVar(&f.force, "force", false, "overwrite an existing prepared episode")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("title")
	return cmd
}

func runPrepare(cmd *cobra.Command, dataDir, transcript string, f prepareFlags) error {
	opts, err := prepareOptions(dataDir, transcript, f)
	if err != nil {
		return err
	}
	gen, err := prepare.NewGeminiGenerator(cmd.Context(), os.Getenv("GOOGLE_API_KEY"), f.model)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "bgtutor: preparing %s with %s ...\n", opts.EpisodeID, gen.Name())
	dir, err := prepare.Run(cmd.Context(), gen, opts)
	if err != nil {
		return err
	}
	if f.audio != "" {
		if err := copyFile(f.audio, filepath.Join(dir, "audio"+filepath.Ext(f.audio))); err != nil {
			return fmt.Errorf("episode prepared, but copying audio failed: %w", err)
		}
	}
	ep := bgtutor.LoadEpisode(dir)
	fmt.Printf("Prepared %s: %d paragraphs, ready=%v\n", dir, len(ep.Paragraphs), ep.Ready())
	return nil
}

func prepareOptions(dataDir, transcript string, f prepareFlags) (prepare.Options, error) {
	speakers, err := parseSpeakers(f.speakers)
	if err != nil {
		return prepare.Options{}, err
	}
	meta := bgtutor.Meta{
		Title: f.title, Topic: f.topic, Description: f.description, Difficulty: f.difficulty,
		DurationMinutes: f.minutes, Speakers: speakers,
	}
	if f.audio != "" {
		meta.Source = map[string]any{"audio_file": "audio" + filepath.Ext(f.audio)}
	}
	opts := prepare.Options{
		TranscriptPath: transcript, OutDir: filepath.Join(dataDir, "episodes"), EpisodeID: f.id,
		Meta: meta, ChunkWords: f.chunkWords, Force: f.force,
	}
	if f.reference != "" {
		ref, err := os.ReadFile(f.reference)
		if err != nil {
			return prepare.Options{}, fmt.Errorf("read reference: %w", err)
		}
		opts.ReferenceNotes = string(ref)
	}
	return opts, nil
}

// parseSpeakers reads "Label:Name:Role" values; Name and Role are optional.
func parseSpeakers(values []string) ([]bgtutor.Speaker, error) {
	if len(values) == 0 {
		return []bgtutor.Speaker{{Label: "Narrator"}}, nil
	}
	var out []bgtutor.Speaker
	for _, v := range values {
		parts := strings.SplitN(v, ":", 3)
		for len(parts) < 3 {
			parts = append(parts, "")
		}
		s := bgtutor.Speaker{Label: strings.TrimSpace(parts[0]), Name: strings.TrimSpace(parts[1]), Role: strings.TrimSpace(parts[2])}
		if s.Label == "" {
			return nil, fmt.Errorf("speaker %q has no label", v)
		}
		out = append(out, s)
	}
	return out, nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
