package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/snonux/totalrecall/internal/store"
)

// TestFindCardDirectory verifies that FindCardDirectory locates a directory by
// its word.txt content and returns an empty string when no match exists.
func TestFindCardDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a card directory with a word file.
	cardDir := filepath.Join(tmpDir, "someCardID")
	if err := os.MkdirAll(cardDir, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cardDir, "word.txt"), []byte("котка"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	tests := []struct {
		name    string
		word    string
		wantDir string // empty means expect ""
		wantHit bool
	}{
		{
			name:    "existing word found",
			word:    "котка",
			wantDir: cardDir,
			wantHit: true,
		},
		{
			name:    "unknown word returns empty string",
			word:    "куче",
			wantDir: "",
			wantHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := store.FindCardDirectory(tmpDir, tt.word)
			if tt.wantHit && got != tt.wantDir {
				t.Errorf("FindCardDirectory(%q) = %q; want %q", tt.word, got, tt.wantDir)
			}
			if !tt.wantHit && got != "" {
				t.Errorf("FindCardDirectory(%q) = %q; want empty string", tt.word, got)
			}
		})
	}
}

// TestFindCardDirectoryLegacyFallback checks that the legacy _word.txt naming
// convention is still supported for backward compatibility.
func TestFindCardDirectoryLegacyFallback(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a card directory with the old _word.txt naming.
	cardDir := filepath.Join(tmpDir, "legacyCardID")
	if err := os.MkdirAll(cardDir, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cardDir, "_word.txt"), []byte("ябълка"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got := store.FindCardDirectory(tmpDir, "ябълка")
	if got != cardDir {
		t.Errorf("FindCardDirectory (legacy) = %q; want %q", got, cardDir)
	}
}

// TestFindOrCreateCardDirectory verifies that a new directory is created when
// no matching one exists, and that the same directory is returned on a second
// call for the same word.
func TestFindOrCreateCardDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// First call: directory does not exist yet.
	dir1 := store.FindOrCreateCardDirectory(tmpDir, "хляб")
	if dir1 == "" || dir1 == tmpDir {
		t.Fatalf("expected a new card directory, got %q", dir1)
	}

	// word.txt must have been written.
	data, err := os.ReadFile(filepath.Join(dir1, "word.txt"))
	if err != nil {
		t.Fatalf("word.txt not created: %v", err)
	}
	if string(data) != "хляб" {
		t.Errorf("word.txt content = %q; want %q", string(data), "хляб")
	}

	// Second call: must return the same directory.
	dir2 := store.FindOrCreateCardDirectory(tmpDir, "хляб")
	if dir2 != dir1 {
		t.Errorf("second call returned %q; want %q", dir2, dir1)
	}
}

// TestCardStoreScanWords verifies that ScanWords returns only words from
// directories that pass the predicate and ignores hidden directories.
func TestCardStoreScanWords(t *testing.T) {
	tmpDir := t.TempDir()

	// Helper: create a card directory with word.txt.
	makeCard := func(id, word string) string {
		cardDir := filepath.Join(tmpDir, id)
		_ = os.MkdirAll(cardDir, 0755)
		_ = os.WriteFile(filepath.Join(cardDir, "word.txt"), []byte(word), 0644)
		return cardDir
	}

	dir1 := makeCard("card1", "котка")
	makeCard("card2", "куче")

	// Hidden directory must be skipped.
	makeCard(".hidden", "hidden")

	// ScanWords with a predicate that only passes dir1.
	cs := store.New(tmpDir)
	words := cs.ScanWords(func(wordDir string) bool {
		return wordDir == dir1
	})

	if len(words) != 1 || words[0] != "котка" {
		t.Errorf("ScanWords = %v; want [котка]", words)
	}

	// ScanWords with nil predicate must return all non-hidden words.
	allWords := cs.ScanWords(nil)
	if len(allWords) != 2 {
		t.Errorf("ScanWords(nil) = %v; want 2 words", allWords)
	}
}

func TestCardStoreListCardDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	makeCard := func(id, word string) string {
		cardDir := filepath.Join(tmpDir, id)
		_ = os.MkdirAll(cardDir, 0755)
		_ = os.WriteFile(filepath.Join(cardDir, "word.txt"), []byte(word), 0644)
		return cardDir
	}

	card2 := makeCard("card2", "куче")
	makeCard("card1", "котка")
	makeCard(".hidden", "hidden")

	cs := store.New(tmpDir)
	cards := cs.ListCardDirectories(func(wordDir string) bool {
		return wordDir != card2
	})

	if len(cards) != 1 {
		t.Fatalf("ListCardDirectories() returned %d cards, want 1", len(cards))
	}
	if cards[0].Word != "котка" {
		t.Fatalf("ListCardDirectories()[0].Word = %q, want %q", cards[0].Word, "котка")
	}
	if filepath.Base(cards[0].Path) != "card1" {
		t.Fatalf("ListCardDirectories()[0].Path = %q, want base %q", cards[0].Path, "card1")
	}
}
