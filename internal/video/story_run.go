package video

import (
	"fmt"
	"os"
)

// RunStoryVideos runs the post-story gallery prompt and Veo generation when
// videoEnabled is true. When false, it returns immediately (e.g. --video=false).
//
// Video generation failures are intentionally non-fatal: the comic, PDF, and
// narration are already on disk, so a Veo API error should not invalidate those
// outputs. Errors are printed as warnings and the function returns nil.
func RunStoryVideos(videoEnabled bool, outputDir string, apiKey string) error {
	if !videoEnabled {
		return nil
	}

	selectedPaths, err := PromptForGalleryVideos(outputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: video prompt failed: %v\n", err)
		return nil
	}

	if err := GenerateSelectedVideos(apiKey, selectedPaths); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: video generation failed: %v\n", err)
	}

	return nil
}
