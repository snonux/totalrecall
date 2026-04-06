// Package video_test provides unit tests for the Veo video generator.
// All tests are mock-based — no real API calls are made.
package video

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/genai"
)

// TestNewVeoGenerator_EmptyKey verifies that an empty API key is rejected.
func TestNewVeoGenerator_EmptyKey(t *testing.T) {
	t.Parallel()

	_, err := NewVeoGenerator("")
	if err == nil {
		t.Fatal("expected error for empty API key, got nil")
	}
}

// TestNewVeoGenerator_WhitespaceKey verifies that a whitespace-only API key is
// treated the same as an empty key.
func TestNewVeoGenerator_WhitespaceKey(t *testing.T) {
	t.Parallel()

	_, err := NewVeoGenerator("   ")
	if err == nil {
		t.Fatal("expected error for whitespace API key, got nil")
	}
}

// TestNewVeoGenerator_ClientInitFailure verifies that a genai client
// initialisation error propagates as a wrapped error.
func TestNewVeoGenerator_ClientInitFailure(t *testing.T) {
	t.Parallel()

	// Temporarily replace the genai client constructor with one that always fails.
	orig := newGenaiClient
	newGenaiClient = func(_ context.Context, _ *genai.ClientConfig) (*genai.Client, error) {
		return nil, errors.New("injected init error")
	}
	t.Cleanup(func() { newGenaiClient = orig })

	_, err := NewVeoGenerator("test-api-key")
	if err == nil {
		t.Fatal("expected error from client init failure, got nil")
	}
	if !strings.Contains(err.Error(), "injected init error") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

// TestLoadGalleryImage_Missing verifies that loadGalleryImage returns an error
// when no matching file exists in the given directory.
func TestLoadGalleryImage_Missing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, _, err := loadGalleryImage(dir, 1)
	if err == nil {
		t.Fatal("expected error for missing gallery image, got nil")
	}
}

// TestLoadGalleryImage_Found verifies that loadGalleryImage returns the correct
// path and bytes when the expected file exists.
func TestLoadGalleryImage_Found(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	imgFile := filepath.Join(dir, "ябълка_gallery_2.png")
	wantBytes := []byte("fake-png-data")
	if err := os.WriteFile(imgFile, wantBytes, 0o644); err != nil {
		t.Fatalf("setup: write test image: %v", err)
	}

	gotPath, gotBytes, err := loadGalleryImage(dir, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != imgFile {
		t.Errorf("path: got %q, want %q", gotPath, imgFile)
	}
	if string(gotBytes) != string(wantBytes) {
		t.Errorf("bytes: got %q, want %q", gotBytes, wantBytes)
	}
}

// TestBuildVeoPrompt verifies that the prompt is non-empty and contains the
// key terms that shape Veo's output style.
func TestBuildVeoPrompt(t *testing.T) {
	t.Parallel()

	prompt := buildVeoPrompt()
	if prompt == "" {
		t.Fatal("buildVeoPrompt returned empty string")
	}

	keywords := []string{"comic", "Bulgarian", "educational", "8-second"}
	for _, kw := range keywords {
		if !strings.Contains(prompt, kw) {
			t.Errorf("expected prompt to contain %q", kw)
		}
	}
}

// TestSaveMP4_WritesFile verifies that saveMP4 creates the expected MP4 file and
// returns its absolute path.
func TestSaveMP4_WritesFile(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	fakeVideo := []byte{0x00, 0x00, 0x00, 0x20, 0x66, 0x74, 0x79, 0x70} // minimal ftyp box bytes

	// Simulate source path like the real gallery image would produce.
	srcPath := "/stories/ябълка/ябълка_gallery_3.png"

	got, err := saveMP4(fakeVideo, outDir, srcPath, 3)
	if err != nil {
		t.Fatalf("saveMP4 failed: %v", err)
	}

	if !strings.HasSuffix(got, ".mp4") {
		t.Errorf("expected .mp4 suffix, got %q", got)
	}

	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("reading saved MP4: %v", err)
	}
	if string(data) != string(fakeVideo) {
		t.Errorf("file contents mismatch")
	}
}

// TestSaveMP4_CreatesOutputDir verifies that saveMP4 creates the output directory
// when it does not already exist.
func TestSaveMP4_CreatesOutputDir(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	outDir := filepath.Join(base, "nested", "output")
	fakeVideo := []byte("video-data")

	_, err := saveMP4(fakeVideo, outDir, "word_gallery_1.png", 1)
	if err != nil {
		t.Fatalf("saveMP4 failed: %v", err)
	}

	if _, statErr := os.Stat(outDir); os.IsNotExist(statErr) {
		t.Error("expected output directory to be created")
	}
}
