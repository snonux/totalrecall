package bgtutor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPublishKeepsMetaReadable(t *testing.T) {
	dir := t.TempDir()
	writeEpisode(t, dir, "001-test", "draft", 2)
	meta := filepath.Join(dir, "001-test", "meta.json")
	if err := os.Chmod(meta, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Publish(filepath.Join(dir, "001-test")); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(meta)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o644 {
		t.Errorf("meta.json mode after publish = %v, want 0644", st.Mode().Perm())
	}
}

func TestWriteJSONAtomicNewFileIsWorldReadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.json")
	mustWriteJSON(t, path, map[string]int{"a": 1})
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o644 {
		t.Errorf("new file mode = %v, want 0644", st.Mode().Perm())
	}
}

func TestValidateParagraphsOrdersByParagraph(t *testing.T) {
	ps := make([]Paragraph, 12)
	for i := range ps {
		ps[i] = Paragraph{Index: i + 1, Speaker: "Host", English: "x", BulgarianReference: "y"}
	}
	ps[1].English = ""  // paragraph 2
	ps[9].English = ""  // paragraph 10
	ps[1].Speaker = " " // paragraph 2, reported before english
	got := ValidateParagraphs(ps, nil)
	want := []string{
		"paragraph 2: speaker must not be empty",
		"paragraph 2: english must not be empty",
		"paragraph 10: english must not be empty",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestListNewestFirstWithinSameSecond(t *testing.T) {
	nb := NewNotebook(filepath.Join(t.TempDir(), "saved.json"), nil)
	fixed := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	nb.now = func() time.Time { return fixed }
	for _, term := range []string{"едно", "две", "три"} {
		if _, err := nb.Save(SaveRequest{Term: term}); err != nil {
			t.Fatal(err)
		}
	}
	res, err := nb.List(ListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Items[0].Term != "три" || res.Items[2].Term != "едно" {
		t.Errorf("order = %v, want newest first", []string{res.Items[0].Term, res.Items[1].Term, res.Items[2].Term})
	}
}

func TestKindIsNormalised(t *testing.T) {
	nb := NewNotebook(filepath.Join(t.TempDir(), "saved.json"), nil)
	if _, err := nb.Save(SaveRequest{Term: "ще", Kind: " Rule "}); err != nil {
		t.Fatalf("save with kind %q: %v", " Rule ", err)
	}
	res, err := nb.List(ListRequest{Kind: "RULE"})
	if err != nil || res.TotalMatching != 1 {
		t.Fatalf("list by kind: %v, %+v", err, res)
	}
	del, err := nb.Delete(DeleteRequest{Term: "ще", Kind: "Rule"})
	if err != nil || del.Deleted != 1 {
		t.Fatalf("delete by kind: %v, %+v", err, del)
	}
}

func TestSaveRejectsBadParagraphIndex(t *testing.T) {
	nb := NewNotebook(filepath.Join(t.TempDir(), "saved.json"), nil)
	for _, req := range []SaveRequest{
		{Term: "a", EpisodeID: "001-test", ParagraphIndex: -1},
		{Term: "a", ParagraphIndex: 3},
	} {
		if _, err := nb.Save(req); err == nil {
			t.Errorf("Save(%+v) succeeded, want error", req)
		}
	}
}

func TestResaveAddsMissingSource(t *testing.T) {
	dir := t.TempDir()
	writeEpisode(t, filepath.Join(dir, "episodes"), "001-test", "ready", 2)
	lib := NewLibrary(filepath.Join(dir, "episodes"))
	nb := NewNotebook(filepath.Join(dir, "saved.json"), lib)
	if _, err := nb.Save(SaveRequest{Term: "здравей"}); err != nil {
		t.Fatal(err)
	}
	res, err := nb.Save(SaveRequest{Term: "здравей", EpisodeID: "001-test", ParagraphIndex: 2})
	if err != nil {
		t.Fatal(err)
	}
	if res.Item.EpisodeID != "001-test" || res.Item.ParagraphIndex != 2 || res.Item.Context == "" {
		t.Errorf("source not merged: %+v", res.Item)
	}
}

func TestSymlinkedEpisodeIsListed(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	writeEpisode(t, real, "001-test", "ready", 1)
	eps := filepath.Join(dir, "episodes")
	if err := os.MkdirAll(eps, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(real, "001-test"), filepath.Join(eps, "001-test")); err != nil {
		t.Skip("symlinks unsupported:", err)
	}
	got, err := NewLibrary(eps).ListEpisodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].Ready {
		t.Errorf("symlinked episode not listed: %+v", got)
	}
}
