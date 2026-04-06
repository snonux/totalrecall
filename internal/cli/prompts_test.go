package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// ---------------------------------------------------------------------------
// parseSelection
// ---------------------------------------------------------------------------

func TestParseSelection_All(t *testing.T) {
	got, err := parseSelection("all", 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []int{1, 2, 3, 4}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSelection(\"all\", 4) = %v, want %v", got, want)
	}
}

func TestParseSelection_AllCaseInsensitive(t *testing.T) {
	got, err := parseSelection("ALL", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []int{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSelection(\"ALL\", 3) = %v, want %v", got, want)
	}
}

func TestParseSelection_CommaSeparated(t *testing.T) {
	got, err := parseSelection("1,3,5", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []int{1, 3, 5}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSelection(\"1,3,5\", 10) = %v, want %v", got, want)
	}
}

func TestParseSelection_SortsOutput(t *testing.T) {
	// Input order is reversed; output must be sorted.
	got, err := parseSelection("5,2,1", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []int{1, 2, 5}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSelection(\"5,2,1\", 10) = %v, want %v", got, want)
	}
}

func TestParseSelection_DuplicatesDeduped(t *testing.T) {
	got, err := parseSelection("2,2,3", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []int{2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSelection(\"2,2,3\", 5) = %v, want %v", got, want)
	}
}

func TestParseSelection_SinglePage(t *testing.T) {
	got, err := parseSelection("7", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []int{7}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSelection(\"7\", 10) = %v, want %v", got, want)
	}
}

func TestParseSelection_WhitespaceTrimmed(t *testing.T) {
	got, err := parseSelection(" 1 , 3 , 5 ", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []int{1, 3, 5}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSelection with spaces = %v, want %v", got, want)
	}
}

func TestParseSelection_InvalidToken(t *testing.T) {
	_, err := parseSelection("1,abc,3", 10)
	if err == nil {
		t.Fatal("expected error for non-numeric token, got nil")
	}
}

func TestParseSelection_ZeroPage(t *testing.T) {
	_, err := parseSelection("0,1", 5)
	if err == nil {
		t.Fatal("expected error for zero page number, got nil")
	}
}

func TestParseSelection_NegativePage(t *testing.T) {
	_, err := parseSelection("-1", 5)
	if err == nil {
		t.Fatal("expected error for negative page number, got nil")
	}
}

func TestParseSelection_EmptyTokensIgnored(t *testing.T) {
	// Trailing comma should not produce an error; the empty token is skipped.
	got, err := parseSelection("1,2,", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []int{1, 2}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSelection(\"1,2,\", 5) = %v, want %v", got, want)
	}
}

// ---------------------------------------------------------------------------
// extractPageNumber
// ---------------------------------------------------------------------------

func TestExtractPageNumber(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"story_gallery_1.png", 1},
		{"my_story_gallery_10.png", 10},
		{"no_match.png", 0},
		{"_gallery_.png", 0},  // missing number after marker
		{"gallery_0.png", 0},  // zero is invalid
		{"gallery_-1.png", 0}, // negative is invalid
	}

	for _, tc := range cases {
		got := extractPageNumber(tc.input)
		if got != tc.want {
			t.Errorf("extractPageNumber(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// findGalleryPages
// ---------------------------------------------------------------------------

func TestFindGalleryPages_NoFiles(t *testing.T) {
	dir := t.TempDir()
	pages, paths, err := findGalleryPages(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pages) != 0 || len(paths) != 0 {
		t.Errorf("expected empty results for empty dir, got pages=%v paths=%v", pages, paths)
	}
}

func TestFindGalleryPages_WithFiles(t *testing.T) {
	dir := t.TempDir()

	// Create dummy gallery PNGs.
	for _, name := range []string{
		"story_gallery_1.png",
		"story_gallery_3.png",
		"story_gallery_2.png",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0644); err != nil {
			t.Fatalf("creating test file: %v", err)
		}
	}

	pages, paths, err := findGalleryPages(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantPages := []int{1, 2, 3}
	if !reflect.DeepEqual(pages, wantPages) {
		t.Errorf("pages = %v, want %v", pages, wantPages)
	}

	if len(paths) != 3 {
		t.Errorf("expected 3 paths, got %d", len(paths))
	}
}

// TestFindGalleryPages_Recursive verifies that gallery PNGs placed in
// subdirectories (as the story runner writes them into comics/<slug>/) are
// found by the recursive walk.
func TestFindGalleryPages_Recursive(t *testing.T) {
	root := t.TempDir()

	// Simulate comics/<slug>/ layout.
	subDir := filepath.Join(root, "comics", "my_story")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}

	for _, name := range []string{
		"my_story_gallery_1.png",
		"my_story_gallery_2.png",
	} {
		if err := os.WriteFile(filepath.Join(subDir, name), []byte(""), 0644); err != nil {
			t.Fatalf("creating test file: %v", err)
		}
	}

	pages, paths, err := findGalleryPages(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantPages := []int{1, 2}
	if !reflect.DeepEqual(pages, wantPages) {
		t.Errorf("pages = %v, want %v", pages, wantPages)
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got %d: %v", len(paths), paths)
	}
}

// ---------------------------------------------------------------------------
// filterPathsByPages
// ---------------------------------------------------------------------------

func TestFilterPathsByPages(t *testing.T) {
	paths := []string{
		"/comics/slug/slug_gallery_1.png",
		"/comics/slug/slug_gallery_2.png",
		"/comics/slug/slug_gallery_3.png",
	}

	got := filterPathsByPages(paths, []int{1, 3})
	want := []string{
		"/comics/slug/slug_gallery_1.png",
		"/comics/slug/slug_gallery_3.png",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("filterPathsByPages = %v, want %v", got, want)
	}
}

func TestFilterPathsByPages_All(t *testing.T) {
	paths := []string{
		"/comics/slug/slug_gallery_1.png",
		"/comics/slug/slug_gallery_2.png",
	}

	got := filterPathsByPages(paths, []int{1, 2})
	if !reflect.DeepEqual(got, paths) {
		t.Errorf("filterPathsByPages all = %v, want %v", got, paths)
	}
}

func TestFilterPathsByPages_Empty(t *testing.T) {
	paths := []string{"/comics/slug/slug_gallery_1.png"}
	got := filterPathsByPages(paths, []int{})
	if len(got) != 0 {
		t.Errorf("expected empty result, got %v", got)
	}
}
