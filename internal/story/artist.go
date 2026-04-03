package story

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"google.golang.org/genai"

	"codeberg.org/snonux/totalrecall/internal/image"
)

const (
	comicPageCount = 3

	// comicPromptMaxChars caps each panel's story excerpt in the NanoBanana prompt.
	comicPromptMaxChars = 800

	// bibleModel is the Gemini text model used to generate the character bible.
	bibleModel = "gemini-2.5-flash"

	// bibleTimeout gives Gemini up to 60 s to produce the character bible.
	bibleTimeout = 60 * time.Second

	// bibleMaxTokens must be large enough to cover Gemini 2.5 Flash's internal
	// thinking tokens plus the ~180-word visible bible output.  A small budget
	// (e.g. 512) is silently consumed by thinking before any text is emitted.
	bibleMaxTokens = int32(2048)
)

// comicStyles is the pool from which the strip style is drawn each run.
// Ultra realistic is selected 90% of the time; the remaining 10% comes from
// the other styles to provide occasional visual variety.
var comicStyles = []string{
	"ultra realistic comic strip with photographic detail and dramatic lighting",
	"classic American comic book with bold ink outlines, halftone dots, and primary colors",
	"Japanese manga with clean linework, expressive eyes, and speed lines",
	"retro 1960s pop art in the style of Roy Lichtenstein with thick outlines and Ben-Day dots",
	"watercolor illustration with soft washes, delicate linework, and pastel tones",
	"European bande dessinée with detailed backgrounds, clear lines, and rich flat colors",
	"noir black-and-white graphic novel with heavy shadows and high contrast",
	"children's picture book with bright, friendly illustrations and thick outlines",
	"painterly oil-on-canvas comic with loose brushwork and vivid impressionist colors",
	"cyberpunk neon art with glowing outlines, dark backgrounds, and electric accent colors",
}

// characterBiblePrompt instructs Gemini to produce a concise visual reference
// that will be prepended verbatim to every panel prompt.
const characterBiblePrompt = `You are a comic-book art director. Read the Bulgarian story below and write a
CHARACTER CONSISTENCY GUIDE in English for an illustrator. Cover every character that appears:
name, age estimate, hair (colour + style), eye colour, skin tone, build, and exact clothing worn
throughout the story. Then describe the setting (location, time of day, weather, key props) and
the overall lighting / colour mood. Be very specific — this guide will be copy-pasted into every
panel prompt to lock visual consistency. Maximum 180 words. No headers, just dense prose.

Story:
`

// ArtistConfig holds settings for comic-book image generation via NanoBanana.
type ArtistConfig struct {
	APIKey    string // Google API key (NanoBanana image + Gemini bible generation)
	Model     string // NanoBanana image model
	TextModel string // NanoBanana text/prompt model
	OutputDir string // target directory; defaults to "."
	Style     string // overrides the random art-style pick when non-empty
}

// Artist generates comic-book pages that illustrate the story.
type Artist struct {
	nbClient  image.ImageClient
	apiKey    string // used for the character-bible Gemini call
	outputDir string
	style     string
}

// NewArtist creates an Artist backed by the NanoBanana image generator.
func NewArtist(config *ArtistConfig) *Artist {
	dir := "."
	var apiKey, style string
	var nbConfig *image.NanoBananaConfig

	if config != nil {
		dir = orDefault(config.OutputDir, ".")
		apiKey = config.APIKey
		style = config.Style
		nbConfig = &image.NanoBananaConfig{
			APIKey:    config.APIKey,
			Model:     config.Model,
			TextModel: config.TextModel,
		}
	}

	return &Artist{
		nbClient:  image.NewNanoBananaClient(nbConfig),
		apiKey:    apiKey,
		outputDir: dir,
		style:     style,
	}
}

// DrawComicPages generates one image per story section (comicPageCount total).
// A character bible is produced first and embedded in every panel prompt so
// characters, clothes, and setting stay visually consistent across all pages.
// Files are saved as comic_page_1.png … comic_page_N.png; attribution files
// are auto-written by the Downloader. Returns the list of saved image paths.
func (a *Artist) DrawComicPages(storyText string) ([]string, error) {
	style := a.style
	if style == "" {
		style = pickStyle()
	}
	fmt.Printf("  Comic style: %s\n", style)

	bible, err := a.buildCharacterBible(storyText)
	if err != nil {
		// Non-fatal: warn and continue without the bible rather than aborting.
		fmt.Printf("  Warning: character bible generation failed (%v); panels may vary\n", err)
		bible = ""
	} else {
		fmt.Printf("  Character bible ready (%d chars)\n", len(bible))
	}

	sections := splitIntoSections(storyText, comicPageCount)
	var paths []string

	for i, section := range sections {
		pageNum := i + 1
		fmt.Printf("  Generating comic page %d/%d...\n", pageNum, comicPageCount)

		path, err := a.drawPage(section, pageNum, comicPageCount, style, bible)
		if err != nil {
			return paths, fmt.Errorf("comic page %d failed: %w", pageNum, err)
		}
		paths = append(paths, path)
	}

	return paths, nil
}

// buildCharacterBible calls Gemini to produce a concise visual reference card
// describing every character and the setting. This is prepended to each panel
// prompt to lock character appearance across all generated images.
func (a *Artist) buildCharacterBible(storyText string) (string, error) {
	if a.apiKey == "" {
		return "", fmt.Errorf("no API key for character bible generation")
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey: a.apiKey,
	})
	if err != nil {
		return "", fmt.Errorf("create genai client: %w", err)
	}

	thinkingBudget := int32(0) // disable thinking — short factual extraction, no reasoning needed
	ctx, cancel := context.WithTimeout(context.Background(), bibleTimeout)
	defer cancel()

	resp, err := client.Models.GenerateContent(ctx, bibleModel,
		[]*genai.Content{genai.NewContentFromText(characterBiblePrompt+storyText, genai.RoleUser)},
		&genai.GenerateContentConfig{
			MaxOutputTokens: bibleMaxTokens,
			ThinkingConfig:  &genai.ThinkingConfig{ThinkingBudget: &thinkingBudget},
		},
	)
	if err != nil {
		return "", fmt.Errorf("gemini bible call: %w", err)
	}

	bible := strings.TrimSpace(resp.Text())
	if bible == "" {
		return "", fmt.Errorf("gemini returned empty character bible")
	}

	return bible, nil
}

// drawPage generates a single comic page with the given style and bible.
func (a *Artist) drawPage(section string, pageNum, totalPages int, style, bible string) (string, error) {
	opts := image.DefaultSearchOptions("vocabulary story")
	opts.CustomPrompt = buildPagePrompt(section, pageNum, totalPages, style, bible)

	downloader := image.NewDownloader(a.nbClient, &image.DownloadOptions{
		OutputDir:         a.outputDir,
		OverwriteExisting: true,
		CreateDir:         true,
		FileNamePattern:   fmt.Sprintf("comic_page_%d", pageNum),
		MaxSizeBytes:      20 * 1024 * 1024,
	})

	_, savedPath, err := downloader.DownloadBestMatchWithOptions(context.Background(), opts)
	if err != nil {
		return "", err
	}

	return savedPath, nil
}

// buildPagePrompt constructs the NanoBanana prompt for one panel.
// The character bible is injected between the style directive and the scene
// excerpt so the model has the visual reference before reading the scene.
func buildPagePrompt(section string, pageNum, totalPages int, style, bible string) string {
	excerpt := strings.TrimSpace(section)
	if len(excerpt) > comicPromptMaxChars {
		excerpt = excerpt[:comicPromptMaxChars]
		if idx := strings.LastIndex(excerpt, " "); idx > 0 {
			excerpt = excerpt[:idx]
		}
		excerpt += "…"
	}

	bibleBlock := ""
	if bible != "" {
		bibleBlock = fmt.Sprintf("\nCHARACTER & SETTING REFERENCE (follow exactly — this is panel %d of %d):\n%s\n", pageNum, totalPages, bible)
	}

	return fmt.Sprintf(
		"Art style: %s.%s\nPanel %d of %d. Scene from a Bulgarian vocabulary story:\n\n%s",
		style, bibleBlock, pageNum, totalPages, excerpt,
	)
}

// pickStyle returns a randomly chosen art style.
// Ultra realistic is selected 90% of the time.
func pickStyle() string {
	if rand.Float64() < 0.9 {
		return comicStyles[0]
	}
	return comicStyles[1+rand.IntN(len(comicStyles)-1)]
}

// splitIntoSections divides text into n roughly equal parts on paragraph
// boundaries where possible, falling back to equal character splits.
func splitIntoSections(text string, n int) []string {
	paragraphs := splitParagraphs(text)
	if len(paragraphs) >= n {
		return distributeParagraphs(paragraphs, n)
	}
	return splitByChars(text, n)
}

func splitParagraphs(text string) []string {
	var out []string
	for _, p := range strings.Split(text, "\n\n") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func distributeParagraphs(paragraphs []string, n int) []string {
	sections := make([]string, n)
	size, rem, idx := len(paragraphs)/n, len(paragraphs)%n, 0
	for i := range n {
		count := size
		if i < rem {
			count++
		}
		sections[i] = strings.Join(paragraphs[idx:idx+count], "\n\n")
		idx += count
	}
	return sections
}

func splitByChars(text string, n int) []string {
	size := len(text) / n
	sections := make([]string, n)
	for i := range n {
		start := i * size
		end := start + size
		if i == n-1 {
			end = len(text)
		}
		sections[i] = strings.TrimSpace(text[start:end])
	}
	return sections
}

func orDefault(s, def string) string {
	if s != "" {
		return s
	}
	return def
}
