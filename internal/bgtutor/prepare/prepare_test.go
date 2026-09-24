package prepare

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/snonux/totalrecall/internal/bgtutor"
)

// fakeGenerator returns one paragraph per transcript line in the user message
// and records the prompts it saw.
type fakeGenerator struct {
	users []string
	fail  bool
}

func (f *fakeGenerator) Name() string { return "fake" }

func (f *fakeGenerator) GenerateJSON(_ context.Context, system, user string, _ map[string]any) (string, error) {
	if f.fail {
		return "", fmt.Errorf("boom")
	}
	f.users = append(f.users, user)
	body := user[strings.Index(user, "<transcript>\n")+len("<transcript>\n") : strings.Index(user, "\n</transcript>")]
	var ps []map[string]any
	for _, line := range strings.Split(body, "\n") {
		speaker, text, _ := strings.Cut(line, ": ")
		ps = append(ps, map[string]any{
			"speaker": speaker, "english": text, "bulgarian_reference": "превод",
			"grammar_notes":    []string{"note"},
			"vocabulary_notes": []map[string]string{{"bg": "дума" + fmt.Sprint(len(f.users)), "en": "word", "note": ""}},
			"background":       "",
		})
	}
	out, _ := json.Marshal(map[string]any{"paragraphs": ps})
	return string(out), nil
}

func writeTranscript(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t.txt")
	text := "Host: one two three four five.\nGuest: six seven eight nine ten.\n\nHost: eleven twelve thirteen."
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func testOptions(t *testing.T, transcript string) Options {
	return Options{
		TranscriptPath: transcript, OutDir: t.TempDir(), EpisodeID: "001-test", ChunkWords: 6,
		Meta: bgtutor.Meta{Title: "Test", Speakers: []bgtutor.Speaker{{Label: "Host"}, {Label: "Guest"}}},
	}
}

func TestRunWritesReadyEpisode(t *testing.T) {
	gen := &fakeGenerator{}
	opts := testOptions(t, writeTranscript(t))
	dir, err := Run(context.Background(), gen, opts)
	if err != nil {
		t.Fatal(err)
	}
	ep := bgtutor.LoadEpisode(dir)
	if !ep.Ready() || len(ep.Paragraphs) != 3 || ep.Paragraphs[2].Index != 3 {
		t.Fatalf("episode: ready=%v problem=%q paragraphs=%d", ep.Ready(), ep.Problem, len(ep.Paragraphs))
	}
	if ep.Paragraphs[0].Background != nil {
		t.Error("empty background should be stored as null")
	}
	if len(gen.users) != 3 {
		t.Fatalf("expected 3 chunks with 6 words each, got %d", len(gen.users))
	}
	if !strings.Contains(gen.users[1], "Already covered vocabulary: дума1") ||
		!strings.Contains(gen.users[1], "<previous>") {
		t.Errorf("second chunk lacks continuity context:\n%s", gen.users[1])
	}
	if _, err := Run(context.Background(), gen, opts); err == nil {
		t.Error("second run without Force should refuse to overwrite")
	}
}

func TestRunFailureLeavesDraft(t *testing.T) {
	opts := testOptions(t, writeTranscript(t))
	dir, err := Run(context.Background(), &fakeGenerator{fail: true}, opts)
	if err == nil {
		t.Fatal("expected error")
	}
	if ep := bgtutor.LoadEpisode(dir); ep.Ready() {
		t.Error("failed preparation must not produce a ready episode")
	}
}

func TestRenderPrompt(t *testing.T) {
	p := renderPrompt(bgtutor.Meta{Title: "T", Difficulty: "A2",
		Speakers: []bgtutor.Speaker{{Label: "Host", Name: "Anna", Role: "host"}}}, "Lesson 5: aspect")
	for _, want := range []string{"Level: A2", "Host (Anna, host)", "<course_reference>\nLesson 5: aspect"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if strings.Contains(p, "{{") {
		t.Error("unreplaced placeholder in prompt")
	}
}

func TestSplitTranscript(t *testing.T) {
	long := strings.Repeat("word ", 9) + "end. " + strings.Repeat("more ", 9) + "stop."
	chunks := SplitTranscript("a b\nc d\n"+long, 10)
	if len(chunks) != 3 || chunks[0] != "a b\nc d" {
		t.Fatalf("chunks = %q", chunks)
	}
	for _, c := range chunks {
		if n := len(strings.Fields(c)); n > 10 {
			t.Errorf("chunk has %d words: %q", n, c)
		}
	}
}
