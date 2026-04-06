package cli

import (
	"testing"
)

// TestGenerateSelectedVideos_EmptyPaths verifies that GenerateSelectedVideos
// returns nil immediately when no paths are provided, without attempting any
// API calls.
func TestGenerateSelectedVideos_EmptyPaths(t *testing.T) {
	err := GenerateSelectedVideos("any-api-key", []string{})
	if err != nil {
		t.Fatalf("expected nil for empty paths, got: %v", err)
	}
}

// TestGenerateSelectedVideos_NilPaths verifies that GenerateSelectedVideos
// handles a nil slice the same way as an empty slice.
func TestGenerateSelectedVideos_NilPaths(t *testing.T) {
	err := GenerateSelectedVideos("any-api-key", nil)
	if err != nil {
		t.Fatalf("expected nil for nil paths, got: %v", err)
	}
}

// TestGenerateSelectedVideos_EmptyAPIKey verifies that GenerateSelectedVideos
// returns an error when a non-empty path list is provided but the API key is
// empty.  The error originates from video.NewVeoGenerator, so we just check
// that some error is returned without making any real API calls.
func TestGenerateSelectedVideos_EmptyAPIKey(t *testing.T) {
	err := GenerateSelectedVideos("", []string{"/some/word_gallery_1.png"})
	if err == nil {
		t.Fatal("expected error for empty API key with non-empty paths, got nil")
	}
}

// TestGenerateSelectedVideos_WhitespaceAPIKey verifies that a whitespace-only
// API key is treated equivalently to an empty key when paths are supplied.
func TestGenerateSelectedVideos_WhitespaceAPIKey(t *testing.T) {
	err := GenerateSelectedVideos("   ", []string{"/some/word_gallery_1.png"})
	if err == nil {
		t.Fatal("expected error for whitespace API key with non-empty paths, got nil")
	}
}
