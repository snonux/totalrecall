package bgtutor

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeEpisode creates an episode folder with n paragraphs under dir.
func writeEpisode(t *testing.T, dir, id, status string, n int) {
	t.Helper()
	meta := Meta{FormatVersion: 1, Title: "Test " + id, Status: status,
		Speakers: []Speaker{{Label: "Host"}, {Label: "Guest"}}}
	ps := make([]Paragraph, n)
	for i := range ps {
		ps[i] = Paragraph{Index: i + 1, Speaker: "Host", English: "Hello number " + string(rune('A'+i)),
			BulgarianReference: "Здравей", GrammarNotes: []string{"note"},
			VocabularyNotes: []VocabNote{{BG: "здравей", EN: "hello"}}}
	}
	mustWriteJSON(t, filepath.Join(dir, id, "meta.json"), meta)
	mustWriteJSON(t, filepath.Join(dir, id, "paragraphs.json"), ps)
}

func mustWriteJSON(t *testing.T, path string, v any) {
	t.Helper()
	if err := WriteJSONAtomic(path, v); err != nil {
		t.Fatal(err)
	}
}

func TestParagraphPositionAndIsLast(t *testing.T) {
	dir := t.TempDir()
	writeEpisode(t, dir, "001-test", "ready", 3)
	lib := NewLibrary(dir)

	p, err := lib.Paragraph("001-test", 1)
	if err != nil {
		t.Fatal(err)
	}
	if p.Position != "1 of 3" || p.IsLast || p.NextIndex == nil || *p.NextIndex != 2 {
		t.Errorf("first paragraph: got position %q is_last %v next %v", p.Position, p.IsLast, p.NextIndex)
	}
	if p.BulgarianReferenceNote != ReferenceNote {
		t.Errorf("missing reference note")
	}
	last, err := lib.Paragraph("001-test", 3)
	if err != nil {
		t.Fatal(err)
	}
	if last.Position != "3 of 3" || !last.IsLast || last.NextIndex != nil {
		t.Errorf("last paragraph: got position %q is_last %v next %v", last.Position, last.IsLast, last.NextIndex)
	}
}

func TestParagraphErrors(t *testing.T) {
	dir := t.TempDir()
	writeEpisode(t, dir, "001-test", "ready", 2)
	writeEpisode(t, dir, "002-draft", "draft", 2)
	if err := os.MkdirAll(filepath.Join(dir, "003-unprepared"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteJSON(t, filepath.Join(dir, "003-unprepared", "meta.json"),
		Meta{FormatVersion: 1, Title: "x", Status: "ready"})
	lib := NewLibrary(dir)

	cases := []struct {
		id    string
		index int
		want  string
	}{
		{"999-missing", 1, "Available episodes: 001-test"},
		{"../etc", 1, "Invalid episode id"},
		{"001-test", 0, "Valid indexes are 1 to 2"},
		{"001-test", 3, "Valid indexes are 1 to 2"},
		{"002-draft", 1, `status is "draft"`},
		{"003-unprepared", 1, "not prepared yet"},
	}
	for _, c := range cases {
		_, err := lib.Paragraph(c.id, c.index)
		var ue *UserError
		if !errors.As(err, &ue) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("Paragraph(%q, %d) = %v, want UserError containing %q", c.id, c.index, err, c.want)
		}
	}
}

func TestListEpisodesShowsNotReady(t *testing.T) {
	dir := t.TempDir()
	writeEpisode(t, dir, "001-test", "ready", 2)
	writeEpisode(t, dir, "002-draft", "draft", 1)
	if err := os.MkdirAll(filepath.Join(dir, ".hidden"), 0o755); err != nil {
		t.Fatal(err)
	}
	eps, err := NewLibrary(dir).ListEpisodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 2 {
		t.Fatalf("got %d episodes, want 2", len(eps))
	}
	if !eps[0].Ready || eps[0].ParagraphCount != 2 {
		t.Errorf("001: %+v", eps[0])
	}
	if eps[1].Ready || eps[1].NotReadyReason == "" {
		t.Errorf("002 should be not ready with a reason: %+v", eps[1])
	}
}

func TestValidateParagraphs(t *testing.T) {
	ps := []Paragraph{
		{Index: 1, Speaker: "Host", English: "a", BulgarianReference: "б"},
		{Index: 3, Speaker: "Nobody", English: "", BulgarianReference: "б",
			VocabularyNotes: []VocabNote{{BG: "x"}}},
	}
	got := strings.Join(ValidateParagraphs(ps, map[string]bool{"Host": true}), "\n")
	for _, want := range []string{"index is 3, expected 2", `speaker "Nobody"`, "english must not be empty", "needs bg and en"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// TestSampleEpisode keeps the bundled test episode valid.
func TestSampleEpisode(t *testing.T) {
	ep := LoadEpisode(filepath.Join("..", "..", "bgtutor", "data", "episodes", "001-cooking-basics"))
	if !ep.Ready() {
		t.Fatalf("sample episode not ready: %s", ep.Problem)
	}
}

func TestNotebookSaveDedupAndList(t *testing.T) {
	dir := t.TempDir()
	writeEpisode(t, filepath.Join(dir, "episodes"), "001-test", "ready", 2)
	nb := NewNotebook(filepath.Join(dir, "vocabulary", "saved.json"), NewLibrary(filepath.Join(dir, "episodes")))
	clock := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	nb.now = func() time.Time { clock = clock.Add(time.Minute); return clock }

	r1, err := nb.Save(SaveRequest{Term: "тесто", Translation: "dough", EpisodeID: "001-test", ParagraphIndex: 2})
	if err != nil || r1.Status != "saved" || r1.Item.ID != 1 || r1.Item.Kind != "word" {
		t.Fatalf("first save: %+v %v", r1, err)
	}
	if r1.Item.Context != "Hello number B" {
		t.Errorf("context = %q, want the paragraph's English", r1.Item.Context)
	}
	r2, err := nb.Save(SaveRequest{Term: "Тесто", Note: "neuter"})
	if err != nil || r2.Status != "already_saved" || r2.Item.TimesSaved != 2 || r2.Item.Note != "neuter" {
		t.Fatalf("second save: %+v %v", r2, err)
	}
	if _, err := nb.Save(SaveRequest{Term: "ще + present", Kind: "rule", Note: "future tense"}); err != nil {
		t.Fatal(err)
	}
	if _, err := nb.Save(SaveRequest{Term: "x", Kind: "bogus"}); err == nil {
		t.Error("expected error for bad kind")
	}

	all, _ := nb.List(ListRequest{})
	if all.TotalMatching != 2 || all.Items[0].Kind != "rule" {
		t.Errorf("list all (newest first): %+v", all)
	}
	rules, _ := nb.List(ListRequest{Kind: "rule"})
	byQuery, _ := nb.List(ListRequest{Query: "DOUGH"})
	byEpisode, _ := nb.List(ListRequest{EpisodeID: "001-test"})
	if rules.TotalMatching != 1 || byQuery.TotalMatching != 1 || byEpisode.TotalMatching != 1 {
		t.Errorf("filters: rules=%d query=%d episode=%d", rules.TotalMatching, byQuery.TotalMatching, byEpisode.TotalMatching)
	}

	var onDisk vocabFile
	data, _ := os.ReadFile(nb.Path)
	if err := json.Unmarshal(data, &onDisk); err != nil || len(onDisk.Items) != 2 || onDisk.FormatVersion != 1 {
		t.Errorf("saved.json: %v %+v", err, onDisk)
	}
}

func TestPublish(t *testing.T) {
	dir := t.TempDir()
	writeEpisode(t, dir, "001-draft", "draft", 2)
	ep, err := Publish(filepath.Join(dir, "001-draft"))
	if err != nil || !ep.Ready() {
		t.Fatalf("publish valid draft: ready=%v err=%v", ep.Ready(), err)
	}
	if again := LoadEpisode(filepath.Join(dir, "001-draft")); !again.Ready() {
		t.Errorf("status not persisted: %s", again.Problem)
	}

	writeEpisode(t, dir, "002-broken", "draft", 1)
	mustWriteJSON(t, filepath.Join(dir, "002-broken", "paragraphs.json"), []Paragraph{{Index: 5}})
	ep, err = Publish(filepath.Join(dir, "002-broken"))
	if err == nil || len(ep.Errors) == 0 || ep.Meta.Status != "draft" {
		t.Errorf("publish invalid draft should fail and stay draft: err=%v errors=%v", err, ep.Errors)
	}
}
