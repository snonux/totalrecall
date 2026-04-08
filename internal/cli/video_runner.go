package cli

import (
	"codeberg.org/snonux/totalrecall/internal/video"
)

// GenerateSelectedVideos is the CLI runner that animates gallery PNG files
// into MP4 clips using Google's Veo model. It delegates to the video package
// so GUI and tests can keep using the cli entry point without importing video
// directly.
//
// apiKey is the Google/Gemini API key passed by the caller.
// selectedPaths contains the absolute (or relative) paths of the gallery PNGs
// to animate — typically returned by video.PromptForGalleryVideos.
//
// Each page prints a "Generating…" line before the API call and a "Video saved:"
// line with the output path on success. The MP4 is written next to its source
// PNG so that gallery images and their videos stay in the same directory.
// The function stops and returns on the first error so the caller can log it.
func GenerateSelectedVideos(apiKey string, selectedPaths []string) error {
	return video.GenerateSelectedVideos(apiKey, selectedPaths)
}
