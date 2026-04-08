package story

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyGalleryPNGsToComicsGallery(t *testing.T) {
	root := t.TempDir()
	comicDir := filepath.Join(root, "comics", "my-slug")
	if err := os.MkdirAll(comicDir, 0755); err != nil {
		t.Fatal(err)
	}
	g1 := filepath.Join(comicDir, "my-slug_gallery_1.png")
	if err := os.WriteFile(g1, []byte("png1"), 0644); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(comicDir, "my-slug_cover.png"), []byte("x"), 0644)

	if err := copyGalleryPNGsToComicsGallery(root, comicDir); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(root, "comics", "gallery", "my-slug_gallery_1.png")
	b, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "png1" {
		t.Fatalf("got %q", b)
	}
}
