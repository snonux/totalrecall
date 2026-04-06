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
// selectedPaths contains the absolute (or relative) paths of the gallery PNGs
// to animate — typically returned by PromptForGalleryVideos.
//
// Each page prints a "Generating…" line before the API call and a "Video saved:"
// line with the output path on success. The MP4 is written next to its source
// PNG so that gallery images and their videos stay in the same directory.
// The function stops and returns on the first error so the caller can log it.
func GenerateSelectedVideos(apiKey string, selectedPaths []string) error {
	if len(selectedPaths) == 0 {
		return nil
	}

	gen, err := video.NewVeoGenerator(apiKey)
	if err != nil {
		return fmt.Errorf("cli: initialising Veo generator: %w", err)
	}

	ctx := context.Background()

	for _, imgPath := range selectedPaths {
		fmt.Printf("Generating video for: %s\n", imgPath)

		mp4Path, err := gen.GenerateVideoFromPath(ctx, imgPath)
		if err != nil {
			return fmt.Errorf("cli: generating video for %s: %w", imgPath, err)
		}

		fmt.Printf("Video saved: %s\n", mp4Path)
	}

	return nil
}
