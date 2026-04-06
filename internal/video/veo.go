// Package video provides video generation capabilities using Google's Veo model.
// It reads existing gallery images (comic-style flashcard panels) and animates
// them into short MP4 clips via the Veo API's long-running operation pattern.
package video

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/genai"
)

const (
	// DefaultVeoModel is the Veo model used for video generation.
	// veo-2.0-generate-001 is the current stable Gemini-API-accessible model.
	DefaultVeoModel = "veo-2.0-generate-001"

	// videoDurationSeconds is the clip length requested from Veo.
	// 8 seconds is the minimum duration supported by the Veo API and produces
	// clips long enough to convey the flashcard content without excess.
	videoDurationSeconds = int32(8)

	// videoAspectRatio is the target aspect ratio for generated clips.
	// 16:9 matches the landscape orientation of the comic-style gallery panels.
	videoAspectRatio = "16:9"

	// pollInterval is the time to wait between operation status checks.
	// Veo generation typically takes 1–3 minutes; 15 s keeps polling overhead low.
	pollInterval = 15 * time.Second

	// maxPollAttempts caps the number of polling iterations so that a hung or
	// stalled Veo operation does not block the process indefinitely.
	// At 15 s per attempt, 40 attempts ≈ 10 minutes — well above the observed
	// worst-case generation time of ~3 minutes.
	maxPollAttempts = 40
)

// VeoGenerator wraps the Google GenAI client for Veo video generation.
type VeoGenerator struct {
	client *genai.Client
	model  string
}

// newGenaiClient is the constructor used in production and can be replaced in
// unit tests to inject a mock transport.
var newGenaiClient = genai.NewClient

// NewVeoGenerator creates a new VeoGenerator backed by the Gemini API.
// It returns an error if the API key is empty or the SDK client cannot be
// initialised (e.g. due to network or credential issues).
func NewVeoGenerator(apiKey string) (*VeoGenerator, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("veo: API key is required")
	}

	client, err := newGenaiClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("veo: failed to create genai client: %w", err)
	}

	return &VeoGenerator{
		client: client,
		model:  DefaultVeoModel,
	}, nil
}

// GenerateVideoFromGallery reads the gallery PNG for pageNum, calls the Veo API,
// polls until the operation completes, then writes the resulting MP4 to outputDir.
// It returns the absolute path of the saved MP4 file, or an error.
//
// galleryPath is the directory containing files named
// "<slug>_gallery_<N>.png" (e.g. /stories/ябълка/ябълка_gallery_1.png).
// outputDir is where the output MP4 will be written.
// pageNum selects which gallery page to animate (1-based).
func (g *VeoGenerator) GenerateVideoFromGallery(ctx context.Context, galleryPath string, outputDir string, pageNum int) (string, error) {
	imgPath, imgBytes, err := loadGalleryImage(galleryPath, pageNum)
	if err != nil {
		return "", err
	}

	prompt := buildVeoPrompt()

	log.Printf("veo: generating video from %s (page %d)", imgPath, pageNum)

	mp4Path, err := g.generateAndSave(ctx, imgBytes, prompt, outputDir, imgPath, pageNum)
	if err != nil {
		return "", err
	}

	return mp4Path, nil
}

// GenerateVideoFromPath reads the gallery PNG at the given absolute (or
// relative) imgPath, calls the Veo API, and writes the resulting MP4 to the
// same directory that contains imgPath.  It returns the absolute path of the
// saved MP4 or an error.
//
// This variant is preferred over GenerateVideoFromGallery when the caller
// already knows the exact image path (e.g. from a recursive directory walk),
// because it avoids a second glob search and always writes the video next to
// its source image.
func (g *VeoGenerator) GenerateVideoFromPath(ctx context.Context, imgPath string) (string, error) {
	imgBytes, err := os.ReadFile(imgPath)
	if err != nil {
		return "", fmt.Errorf("veo: reading gallery image %s: %w", imgPath, err)
	}

	// Derive the page number from the file name for saveMP4 naming purposes.
	pageNum := pageNumFromPath(imgPath)

	prompt := buildVeoPrompt()

	log.Printf("veo: generating video from %s", imgPath)

	// Write the MP4 next to the source image so gallery + video stay together.
	outputDir := filepath.Dir(imgPath)

	return g.generateAndSave(ctx, imgBytes, prompt, outputDir, imgPath, pageNum)
}

// pageNumFromPath extracts the gallery page number from a file name of the
// form "<slug>_gallery_<N>.png".  Returns 0 when the name does not match.
func pageNumFromPath(imgPath string) int {
	base := filepath.Base(imgPath)
	name := strings.TrimSuffix(base, ".png")
	const marker = "_gallery_"
	idx := strings.LastIndex(name, marker)
	if idx < 0 {
		return 0
	}
	numStr := name[idx+len(marker):]
	var n int
	if _, err := fmt.Sscanf(numStr, "%d", &n); err != nil || n <= 0 {
		return 0
	}
	return n
}

// loadGalleryImage finds the gallery PNG for the given page number and returns
// its path and raw bytes. It searches galleryPath for any file whose name
// matches the pattern "*_gallery_<N>.png".
func loadGalleryImage(galleryPath string, pageNum int) (string, []byte, error) {
	pattern := filepath.Join(galleryPath, fmt.Sprintf("*_gallery_%d.png", pageNum))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", nil, fmt.Errorf("veo: glob for gallery image: %w", err)
	}
	if len(matches) == 0 {
		return "", nil, fmt.Errorf("veo: no gallery image found for page %d in %s", pageNum, galleryPath)
	}

	imgPath := matches[0]
	imgBytes, err := os.ReadFile(imgPath)
	if err != nil {
		return "", nil, fmt.Errorf("veo: reading gallery image %s: %w", imgPath, err)
	}

	return imgPath, imgBytes, nil
}

// buildVeoPrompt returns the text prompt sent alongside the gallery image.
// The prompt asks Veo to animate the comic panel while preserving the style
// and characters so that the result fits naturally into a flashcard context.
func buildVeoPrompt() string {
	return "Animate this comic-style flashcard illustration as a short, loopable 8-second clip. " +
		"Preserve the hand-drawn comic art style exactly — bold outlines, flat colours, speech bubbles. " +
		"Add subtle motion: characters breathe or gesture gently, the Bulgarian word label pulses softly, " +
		"and background elements drift slowly. Keep the mood educational and friendly. " +
		"No scene cuts, no camera moves — a single steady wide shot throughout. " +
		"Do not change the characters, layout, or colour palette."
}

// generateAndSave calls the Veo API, polls the long-running operation, downloads
// the resulting video bytes, and writes them to an MP4 file in outputDir.
func (g *VeoGenerator) generateAndSave(ctx context.Context, imgBytes []byte, prompt, outputDir, srcPath string, pageNum int) (string, error) {
	op, err := g.startOperation(ctx, imgBytes, prompt)
	if err != nil {
		return "", err
	}

	op, err = g.pollUntilDone(ctx, op)
	if err != nil {
		return "", err
	}

	videoBytes, err := g.downloadVideo(ctx, op)
	if err != nil {
		return "", err
	}

	return saveMP4(videoBytes, outputDir, srcPath, pageNum)
}

// startOperation submits the image + prompt to the Veo API and returns the
// initial operation descriptor (which will have Done == false).
func (g *VeoGenerator) startOperation(ctx context.Context, imgBytes []byte, prompt string) (*genai.GenerateVideosOperation, error) {
	dur := videoDurationSeconds
	cfg := &genai.GenerateVideosConfig{
		AspectRatio:     videoAspectRatio,
		DurationSeconds: &dur,
		NumberOfVideos:  1,
	}

	source := &genai.GenerateVideosSource{
		Prompt: prompt,
		Image: &genai.Image{
			ImageBytes: imgBytes,
			MIMEType:   "image/png",
		},
	}

	op, err := g.client.Models.GenerateVideosFromSource(ctx, g.model, source, cfg)
	if err != nil {
		return nil, fmt.Errorf("veo: failed to start video generation: %w", err)
	}

	log.Printf("veo: operation started (done=%v)", op.Done)
	return op, nil
}

// pollUntilDone repeatedly calls GetVideosOperation until the operation reports
// completion, the context is cancelled, or maxPollAttempts is reached.
// It sleeps pollInterval between checks to avoid hammering the API.
func (g *VeoGenerator) pollUntilDone(ctx context.Context, op *genai.GenerateVideosOperation) (*genai.GenerateVideosOperation, error) {
	for attempt := 0; !op.Done; attempt++ {
		if attempt >= maxPollAttempts {
			return nil, fmt.Errorf("veo: operation did not complete after %d attempts (%s each)", maxPollAttempts, pollInterval)
		}

		log.Printf("veo: operation in progress, waiting %s (attempt %d/%d)...", pollInterval, attempt+1, maxPollAttempts)

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("veo: context cancelled while polling: %w", ctx.Err())
		case <-time.After(pollInterval):
		}

		var err error
		op, err = g.client.Operations.GetVideosOperation(ctx, op, nil)
		if err != nil {
			return nil, fmt.Errorf("veo: polling operation failed: %w", err)
		}
	}

	log.Printf("veo: operation completed")
	return op, nil
}

// downloadVideo extracts the video from a completed operation, downloading bytes
// via the Files API when the response contains only a URI reference.
func (g *VeoGenerator) downloadVideo(ctx context.Context, op *genai.GenerateVideosOperation) ([]byte, error) {
	// Surface any API-level error (e.g. content policy or geographic restriction)
	// before checking for videos, so the caller gets a meaningful message.
	if len(op.Error) > 0 {
		msg, _ := op.Error["message"].(string)
		if msg == "" {
			msg = fmt.Sprintf("%v", op.Error)
		}
		return nil, fmt.Errorf("veo: %s", msg)
	}
	if op.Response == nil || len(op.Response.GeneratedVideos) == 0 {
		return nil, fmt.Errorf("veo: operation completed but no videos in response")
	}

	gv := op.Response.GeneratedVideos[0]
	if gv == nil || gv.Video == nil {
		return nil, fmt.Errorf("veo: generated video entry is empty")
	}

	// When the Gemini API returns a URI, download bytes via the Files API.
	if gv.Video.URI != "" {
		log.Printf("veo: downloading video from URI %s", gv.Video.URI)
		data, err := g.client.Files.Download(ctx, genai.NewDownloadURIFromGeneratedVideo(gv), nil)
		if err != nil {
			return nil, fmt.Errorf("veo: downloading video: %w", err)
		}
		return data, nil
	}

	// Inline bytes path (used in some Vertex AI configurations).
	if len(gv.Video.VideoBytes) > 0 {
		return gv.Video.VideoBytes, nil
	}

	return nil, fmt.Errorf("veo: no video bytes or URI available in response")
}

// saveMP4 writes videoBytes to a file in outputDir, deriving the file name from
// the source gallery image path and the page number.
// It returns the absolute path of the written file.
func saveMP4(videoBytes []byte, outputDir, srcPath string, pageNum int) (string, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("veo: creating output dir %s: %w", outputDir, err)
	}

	// Derive base name from the source image, e.g. "ябълка_gallery_1.png" → "ябълка_gallery_1.mp4"
	base := strings.TrimSuffix(filepath.Base(srcPath), ".png")
	if base == "" || base == srcPath {
		// Fallback when the source name is unexpected.
		base = fmt.Sprintf("gallery_%d", pageNum)
	}

	outPath := filepath.Join(outputDir, base+".mp4")
	if err := os.WriteFile(outPath, videoBytes, 0o644); err != nil {
		return "", fmt.Errorf("veo: writing mp4 to %s: %w", outPath, err)
	}

	log.Printf("veo: saved MP4 to %s (%d bytes)", outPath, len(videoBytes))
	return outPath, nil
}
