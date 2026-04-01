package image

import "testing"

func TestDownloaderGenerateFileName_DataURIUsesPNG(t *testing.T) {
	t.Parallel()

	d := NewDownloader(&mockSearcher{name: nanoBananaSource}, &DownloadOptions{
		FileNamePattern: "{word}_{source}",
	})
	result := &SearchResult{
		URL:    "data:image/png;base64,AAAA",
		Source: nanoBananaSource,
	}

	if got := d.generateFileName("ябълка", result, 0); got != "ябълка_nanobanana.png" {
		t.Fatalf("generateFileName() = %q, want %q", got, "ябълка_nanobanana.png")
	}
}
