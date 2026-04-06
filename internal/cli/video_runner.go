package cli

import (
	"context"
	"fmt"

	"codeberg.org/snonux/totalrecall/internal/video"
)

// GenerateSelectedVideos is the CLI runner that animates gallery PNG files
// into MP4 clips using Google's Veo model. It processes pages sequentially
// (Veo generation is slow and API quotas make parallelism impractical).
//
// apiKey is the Google/Gemini API key passed by the caller.
// selected is the list of gallery page numbers to process (from PromptForGalleryVideos).
// outputDir is both the directory that contains the gallery PNGs and the
// destination for the resulting MP4 files (written next to the PNGs).
//
// Each page prints a "Generating…" line before the API call and a "Video saved:"
// line with the output path on success. The function stops and returns on the
// first error so callers can log it without silently skipping pages.
func GenerateSelectedVideos(apiKey string, selected []int, outputDir string) error {
	if len(selected) == 0 {
		return nil
	}

	gen, err := video.NewVeoGenerator(apiKey)
	if err != nil {
		return fmt.Errorf("cli: initialising Veo generator: %w", err)
	}

	ctx := context.Background()

	for _, pageNum := range selected {
		fmt.Printf("Generating video for gallery page %d...\n", pageNum)

		mp4Path, err := gen.GenerateVideoFromGallery(ctx, outputDir, outputDir, pageNum)
		if err != nil {
			return fmt.Errorf("cli: generating video for page %d: %w", pageNum, err)
		}

		fmt.Printf("Video saved: %s\n", mp4Path)
	}

	return nil
}
