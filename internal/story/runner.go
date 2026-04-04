package story

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	// Theme overrides the random story genre when non-empty (e.g. "a thrilling space
	// adventure with aliens and spaceships"). Passed verbatim as the genre line in the
	// Gemini story prompt so the model writes in that genre instead of a random one.
	Theme string
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

	var apiKey, textModel, imageModel, imageTextModel, style, theme, narratorVoice string
	if config != nil {
		apiKey = config.APIKey
		textModel = config.TextModel
		imageModel = config.ImageModel
		imageTextModel = config.ImageTextModel
		style = config.Style
		theme = config.Theme
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
			Theme:     theme,
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

	// GenerateFull produces the story AND character bible in one Gemini call so the
	// bible is always available without a second API round-trip.
	fmt.Printf("Generating story for %d words...\n", len(entries))
	result, err := r.generator.GenerateFull(entries)
	if err != nil {
		return fmt.Errorf("story generation failed: %w", err)
	}

	if result.Bible != "" {
		fmt.Printf("  Character bible ready from story generation (%d chars)\n", len(result.Bible))
	}

	// Derive a slug from the generated title and create the comics subfolder.
	// All output files (images, PDF, story text, narration) go into comics/<slug>/.
	slug := slugify(result.Title)
	if result.Title != "" {
		fmt.Printf("  Comic title: %q (slug: %s)\n", result.Title, slug)
	}
	comicsDir := filepath.Join(dir, "comics", slug)
	if err := os.MkdirAll(comicsDir, 0755); err != nil {
		return fmt.Errorf("failed to create comics dir %s: %w", comicsDir, err)
	}
	// Point the artist at the per-comic subfolder for image output.
	r.artist.outputDir = comicsDir

	if err := r.saveStoryText(result.StoryText, slug, comicsDir); err != nil {
		return err
	}

	if err := r.saveVocabularyFile(result.StoryText, entries, slug, comicsDir); err != nil {
		// Non-fatal: vocabulary file is a learning aid, not required for the comic.
		fmt.Fprintf(os.Stderr, "Warning: could not write vocabulary file: %v\n", err)
	}

	if err := r.saveThemeFile(slug, comicsDir); err != nil {
		// Non-fatal: theme file is a convenience record for reproduction.
		fmt.Fprintf(os.Stderr, "Warning: could not write theme file: %v\n", err)
	}

	r.drawComicPages(result.StoryText, result.Bible, slug, entries)

	return r.handleNarration(result.StoryText, slug, comicsDir)
}

// drawComicPages generates the 5 comic images and assembles them into a PDF.
// entries carries the vocabulary words so panels can visually feature and label them.
// Errors are non-fatal — story.txt is always accessible regardless of image failures.
func (r *Runner) drawComicPages(storyText, bible, titleSlug string, entries []batch.WordEntry) {
	fmt.Printf("Generating %d comic pages...\n", storyPageCount+galleryPageCount+2) // cover + story + gallery + back
	paths, err := r.artist.DrawComicPages(storyText, bible, titleSlug, entries)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: comic page generation failed: %v\n", err)
	}
	for _, p := range paths {
		fmt.Printf("Comic page saved: %s\n", p)
	}

	if len(paths) == 0 {
		return
	}

	// Assemble all generated pages into a single PDF named after the comic title.
	pdfPath, err := AssembleComicPDF(r.artist.outputDir, titleSlug, paths)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: PDF assembly failed: %v\n", err)
		return
	}
	fmt.Printf("Comic PDF saved: %s\n", pdfPath)
}

// handleNarration generates a cinematic MP3 via Gemini TTS when a narrator is
// available, or falls back to writing <slug>_tts_todo.txt.  Narration failure is
// non-fatal: the placeholder is written instead so the pipeline always finishes.
func (r *Runner) handleNarration(storyText, titleSlug, dir string) error {
	if r.narrator == nil {
		return r.saveTTSPlaceholder(titleSlug, dir)
	}

	mp3Path := filepath.Join(dir, titleSlug+"_narration.mp3")
	fmt.Printf("Generating cinematic narration (voice: %s)...\n", r.narrator.voice)
	if err := r.narrator.Narrate(storyText, mp3Path); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: narration failed: %v\n", err)
		return r.saveTTSPlaceholder(titleSlug, dir)
	}

	fmt.Printf("Narration saved: %s\n", mp3Path)
	return nil
}

// saveStoryText writes the generated story to <slug>_story.txt in dir.
func (r *Runner) saveStoryText(text, titleSlug, dir string) error {
	path := filepath.Join(dir, titleSlug+"_story.txt")
	if err := os.WriteFile(path, []byte(text+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write story file: %w", err)
	}
	fmt.Printf("Story saved: %s\n", path)
	return nil
}

// saveVocabularyFile writes <slug>_comic_vocabulary.txt — a learning aid that
// lists the vocabulary words with translations followed by the full story text,
// so the reader can study the words in context.
func (r *Runner) saveVocabularyFile(storyText string, entries []batch.WordEntry, titleSlug, dir string) error {
	path := filepath.Join(dir, titleSlug+"_comic_vocabulary.txt")
	var sb strings.Builder

	sb.WriteString("# Vocabulary Words\n\n")
	for _, e := range entries {
		if e.Translation != "" {
			sb.WriteString(fmt.Sprintf("  %s — %s\n", e.Bulgarian, e.Translation))
		} else {
			sb.WriteString(fmt.Sprintf("  %s\n", e.Bulgarian))
		}
	}

	sb.WriteString("\n# Story Text\n\n")
	sb.WriteString(strings.TrimSpace(storyText))
	sb.WriteString("\n")

	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write vocabulary file: %w", err)
	}
	fmt.Printf("Vocabulary saved: %s\n", path)
	return nil
}

// saveThemeFile writes <slug>_theme.txt containing the --story-theme value used
// for this run, so the comic can be reproduced by passing the same theme again.
func (r *Runner) saveThemeFile(titleSlug, dir string) error {
	theme := ""
	if r.config != nil {
		theme = r.config.Theme
	}
	path := filepath.Join(dir, titleSlug+"_theme.txt")
	content := theme + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write theme file: %w", err)
	}
	fmt.Printf("Theme saved: %s\n", path)
	return nil
}

// saveTTSPlaceholder writes <slug>_tts_todo.txt as a fallback when narration
// is unavailable or fails.
func (r *Runner) saveTTSPlaceholder(titleSlug, dir string) error {
	path := filepath.Join(dir, titleSlug+"_tts_todo.txt")
	if err := os.WriteFile(path, []byte(ttsTodoContent), 0644); err != nil {
		return fmt.Errorf("failed to write TTS placeholder: %w", err)
	}
	fmt.Printf("TTS placeholder saved: %s\n", path)
	return nil
}
