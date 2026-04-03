package story

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"

	"codeberg.org/snonux/totalrecall/internal/image"
)

const (
	// comicPageCount is the number of comic pages generated per story.
	comicPageCount = 3

	// comicPromptMaxChars caps each page's story excerpt in the NanoBanana prompt
	// so it stays well within the model's context window.
	comicPromptMaxChars = 800
)

// comicStyles is the pool from which page styles are drawn without replacement.
// "Ultra realistic" is always included; the remaining slots are randomised so
// each run produces a different visual mix.
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

// ArtistConfig holds settings for comic-book image generation via NanoBanana.
type ArtistConfig struct {
	APIKey    string // Google API key
	Model     string // NanoBanana image model
	TextModel string // NanoBanana text/prompt model
	OutputDir string // target directory; defaults to "."
	// Style overrides the random art-style pick when non-empty.
	Style string
}

// Artist generates comic-book-style images that illustrate the story.
type Artist struct {
	nbClient  image.ImageClient
	outputDir string
	style     string // empty = pick randomly each run
}

// NewArtist creates an Artist backed by the NanoBanana image generator.
func NewArtist(config *ArtistConfig) *Artist {
	dir := "."
	if config != nil && config.OutputDir != "" {
		dir = config.OutputDir
	}

	var nbConfig *image.NanoBananaConfig
	if config != nil {
		nbConfig = &image.NanoBananaConfig{
			APIKey:    config.APIKey,
			Model:     config.Model,
			TextModel: config.TextModel,
		}
	}

	var style string
	if config != nil {
		style = config.Style
	}

	return &Artist{
		nbClient:  image.NewNanoBananaClient(nbConfig),
		outputDir: dir,
		style:     style,
	}
}

// DrawComicPages splits the story into comicPageCount sections and generates
// one image per section. A single art style is chosen at random for the whole
// comic so all pages look visually consistent. Files are saved as
// comic_page_1.png … comic_page_N.png; attribution files are auto-written by
// the Downloader. Returns the list of saved image paths.
func (a *Artist) DrawComicPages(storyText string) ([]string, error) {
	sections := splitIntoSections(storyText, comicPageCount)
	// Use the configured style override, or pick one at random.
	style := a.style
	if style == "" {
		style = pickStyle()
	}
	var paths []string

	fmt.Printf("  Comic style: %s\n", style)

	for i, section := range sections {
		pageNum := i + 1
		fmt.Printf("  Generating comic page %d/%d...\n", pageNum, comicPageCount)

		path, err := a.drawPage(section, pageNum, len(sections), style)
		if err != nil {
			return paths, fmt.Errorf("comic page %d failed: %w", pageNum, err)
		}
		paths = append(paths, path)
	}

	return paths, nil
}

// drawPage generates a single comic page with the given style.
// fileNamePattern comic_page_N → comic_page_N.png + comic_page_N_attribution.txt.
func (a *Artist) drawPage(section string, pageNum, totalPages int, style string) (string, error) {
	opts := image.DefaultSearchOptions("vocabulary story")
	opts.CustomPrompt = buildComicPrompt(section, pageNum, totalPages, style)

	downloader := image.NewDownloader(a.nbClient, &image.DownloadOptions{
		OutputDir:         a.outputDir,
		OverwriteExisting: true,
		CreateDir:         true,
		FileNamePattern:   fmt.Sprintf("comic_page_%d", pageNum),
		MaxSizeBytes:      20 * 1024 * 1024,
	})

	ctx := context.Background()
	_, savedPath, err := downloader.DownloadBestMatchWithOptions(ctx, opts)
	if err != nil {
		return "", err
	}

	return savedPath, nil
}

// buildComicPrompt constructs the NanoBanana prompt for one comic page.
// The story excerpt is capped at comicPromptMaxChars.
func buildComicPrompt(section string, pageNum, totalPages int, style string) string {
	excerpt := strings.TrimSpace(section)
	if len(excerpt) > comicPromptMaxChars {
		excerpt = excerpt[:comicPromptMaxChars]
		if idx := strings.LastIndex(excerpt, " "); idx > 0 {
			excerpt = excerpt[:idx]
		}
		excerpt += "…"
	}

	return fmt.Sprintf(
		"Art style: %s.\n"+
			"This is page %d of %d of a comic strip. "+
			"Scene based on this part of a Bulgarian vocabulary story:\n\n%s",
		style, pageNum, totalPages, excerpt,
	)
}

// pickStyle returns a randomly chosen art style.
// Ultra realistic is selected 90% of the time; one of the other styles fills
// the remaining 10% to provide occasional visual variety.
func pickStyle() string {
	if rand.Float64() < 0.9 {
		return comicStyles[0] // ultra realistic
	}
	// Pick from the non-ultra-realistic styles (index 1 onwards).
	return comicStyles[1+rand.IntN(len(comicStyles)-1)]
}

// splitIntoSections divides text into n roughly equal parts on paragraph
// boundaries where possible, falling back to equal character splits.
func splitIntoSections(text string, n int) []string {
	paragraphs := splitParagraphs(text)

	// If there are enough paragraphs, distribute them evenly across pages.
	if len(paragraphs) >= n {
		return distributeParagraphs(paragraphs, n)
	}

	// Fallback: split by characters when the text has fewer paragraphs than pages.
	return splitByChars(text, n)
}

// splitParagraphs splits text on blank lines, discarding empty entries.
func splitParagraphs(text string) []string {
	var out []string
	for _, p := range strings.Split(text, "\n\n") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// distributeParagraphs assigns paragraphs to n buckets as evenly as possible.
func distributeParagraphs(paragraphs []string, n int) []string {
	sections := make([]string, n)
	size := len(paragraphs) / n
	rem := len(paragraphs) % n
	idx := 0

	for i := range n {
		count := size
		if i < rem {
			count++ // distribute remainder one-per-bucket from the front
		}
		sections[i] = strings.Join(paragraphs[idx:idx+count], "\n\n")
		idx += count
	}

	return sections
}

// splitByChars splits text into n roughly equal character-based sections.
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
