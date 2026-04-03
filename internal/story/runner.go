package story

import (
	"fmt"
	"os"
	"path/filepath"

	"codeberg.org/snonux/totalrecall/internal/batch"
)

// ttsTodoContent is written to story_tts_todo.txt as a fallback when Gemini TTS
// narration fails or no API key is available.  It documents the original
// ElevenLabs integration placeholder for reference.
const ttsTodoContent = `# Story Narration — Fallback Placeholder
#
# Gemini TTS narration was not produced (missing API key or generation error).
#
# To generate narration manually, run again with GOOGLE_API_KEY set, or
# use the ElevenLabs TTS API as an alternative:
#   POST https://api.elevenlabs.io/v1/text-to-speech/{voice_id}
#   with the contents of story.txt and save the result as story_narration.mp3
#
# Reference: https://elevenlabs.io/docs/api-reference/text-to-speech
`

// RunnerConfig holds all settings required to orchestrate story generation.
type RunnerConfig struct {
	APIKey         string // Google API key (Gemini text + NanoBanana image + Gemini TTS)
	TextModel      string // Gemini text model for Generator (empty → default)
	ImageModel     string // NanoBanana image model for Artist
	ImageTextModel string // NanoBanana text model for Artist
	OutputDir      string // directory for output files; defaults to "."
	// Style overrides the random art-style pick when non-empty.
	// Accepts any free-form description; it is passed verbatim to the image model.
	Style string
	// NarratorVoice picks a specific Gemini cinematic voice for narration.
	// Empty → random pick from the curated cinematic pool each run.
	NarratorVoice string
}

// Runner orchestrates the full pipeline: text → image → narration.
type Runner struct {
	config    *RunnerConfig
	generator *Generator
	artist    *Artist
	narrator  *Narrator // nil when API key is absent or init fails
}

// NewRunner wires together a Generator, Artist, and Narrator from the given config.
func NewRunner(config *RunnerConfig) *Runner {
	dir := "."
	if config != nil && config.OutputDir != "" {
		dir = config.OutputDir
	}

	var apiKey, textModel, imageModel, imageTextModel, style, narratorVoice string
	if config != nil {
		apiKey = config.APIKey
		textModel = config.TextModel
		imageModel = config.ImageModel
		imageTextModel = config.ImageTextModel
		style = config.Style
		narratorVoice = config.NarratorVoice
	}

	// Narrator init failure (e.g. missing key) is non-fatal — handleNarration
	// falls back to writing story_tts_todo.txt when narrator is nil.
	narrator, err := NewNarrator(&NarratorConfig{
		APIKey: apiKey,
		Voice:  narratorVoice,
	})
	if err != nil {
		narrator = nil
	}

	return &Runner{
		config: config,
		generator: NewGenerator(&Config{
			APIKey:    apiKey,
			TextModel: textModel,
		}),
		artist: NewArtist(&ArtistConfig{
			APIKey:    apiKey,
			Model:     imageModel,
			TextModel: imageTextModel,
			OutputDir: dir,
			Style:     style,
		}),
		narrator: narrator,
	}
}

// Run reads the batch file, generates a story, draws comic pages, narrates the
// story, and writes all output files to the configured OutputDir.
func (r *Runner) Run(batchFile string) error {
	dir := "."
	if r.config != nil && r.config.OutputDir != "" {
		dir = r.config.OutputDir
	}

	entries, err := batch.ReadBatchFile(batchFile)
	if err != nil {
		return fmt.Errorf("failed to read batch file: %w", err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("batch file %q contains no words", batchFile)
	}

	fmt.Printf("Generating story for %d words...\n", len(entries))
	storyText, err := r.generator.Generate(entries)
	if err != nil {
		return fmt.Errorf("story generation failed: %w", err)
	}

	if err := r.saveStoryText(storyText, dir); err != nil {
		return err
	}

	r.drawComicStrip(storyText)

	return r.handleNarration(storyText, dir)
}

// drawComicStrip generates a single tall comic strip image; errors are
// non-fatal so story.txt is always accessible even when image generation fails.
func (r *Runner) drawComicStrip(storyText string) {
	fmt.Println("Generating comic strip...")
	imagePath, err := r.artist.DrawComicStrip(storyText)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: comic strip generation failed: %v\n", err)
		return
	}
	fmt.Printf("Comic strip saved: %s\n", imagePath)
}

// handleNarration generates a cinematic MP3 via Gemini TTS when a narrator is
// available, or falls back to writing story_tts_todo.txt.  Narration failure is
// non-fatal: the placeholder is written instead so the pipeline always finishes.
func (r *Runner) handleNarration(storyText, dir string) error {
	if r.narrator == nil {
		return r.saveTTSPlaceholder(dir)
	}

	mp3Path := filepath.Join(dir, "story_narration.mp3")
	fmt.Printf("Generating cinematic narration (voice: %s)...\n", r.narrator.voice)
	if err := r.narrator.Narrate(storyText, mp3Path); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: narration failed: %v\n", err)
		return r.saveTTSPlaceholder(dir)
	}

	fmt.Printf("Narration saved: %s\n", mp3Path)
	return nil
}

// saveStoryText writes the generated story to story.txt in dir.
func (r *Runner) saveStoryText(text, dir string) error {
	path := filepath.Join(dir, "story.txt")
	if err := os.WriteFile(path, []byte(text+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write story.txt: %w", err)
	}
	fmt.Printf("Story saved: %s\n", path)
	return nil
}

// saveTTSPlaceholder writes story_tts_todo.txt as a fallback when narration
// is unavailable or fails.
func (r *Runner) saveTTSPlaceholder(dir string) error {
	path := filepath.Join(dir, "story_tts_todo.txt")
	if err := os.WriteFile(path, []byte(ttsTodoContent), 0644); err != nil {
		return fmt.Errorf("failed to write story_tts_todo.txt: %w", err)
	}
	fmt.Printf("TTS placeholder saved: %s\n", path)
	return nil
}
