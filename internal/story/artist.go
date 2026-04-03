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
	// storyPageCount is the number of 9-panel story pages (excluding cover/back).
	storyPageCount = 3

	// comicPageAspectRatio: portrait (3:4) matches a standard comic book page
	// and gives the model room for a 3×3 panel grid.
	comicPageAspectRatio = "3:4"

	// comicPromptMaxChars caps each page's story excerpt in the NanoBanana prompt.
	comicPromptMaxChars = 900

	// bibleModel is the Gemini text model used to generate the character bible.
	bibleModel = "gemini-2.5-flash"

	// bibleTimeout gives Gemini up to 90 s to produce the character bible.
	bibleTimeout = 90 * time.Second

	// bibleMaxTokens matches the story generator's proven budget.
	// No ThinkingConfig is set — the model manages token allocation itself,
	// which is the same approach used by the working story generator.
	bibleMaxTokens = int32(8192)
)

// comicStyles is the pool from which the page style is drawn each run.
// Ultra realistic is selected 90% of the time.
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

// characterBiblePrompt instructs Gemini to produce a strict visual reference
// prepended verbatim to every panel, cover, and back-cover prompt.
const characterBiblePrompt = `You are a comic-book art director. Read the Bulgarian story below and write a
CHARACTER CONSISTENCY GUIDE in English for an illustrator.

For every named character provide: name, age estimate, hair (colour + style), eye colour,
skin tone, build, and the EXACT clothing they wear — specify garment, colour, pattern, and fit.
Clothing must NOT change between panels unless the story explicitly describes a change;
if no change is described, list the same outfit for all appearances.

Also describe: the setting (location, time of day, weather, key props) and overall
lighting / colour mood.

Be extremely specific — this guide will be copy-pasted into every panel prompt to lock visual
consistency. Maximum 220 words. No headers, just dense descriptive prose.

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

// DrawComicPages generates 5 images total:
//   - comic_cover.png   — full-bleed cover
//   - comic_page_1.png … comic_page_3.png — 9-panel (3×3) story pages
//   - comic_back.png    — back cover
//
// A character bible is built first and injected into every prompt so
// characters, clothing, and setting stay consistent across all pages.
// Returns the list of saved image paths in order.
func (a *Artist) DrawComicPages(storyText string) ([]string, error) {
	style := a.style
	if style == "" {
		style = pickStyle()
	}
	fmt.Printf("  Comic style: %s\n", style)

	bible, err := a.buildCharacterBible(storyText)
	if err != nil {
		fmt.Printf("  Warning: character bible failed (%v); panels may vary\n", err)
		bible = ""
	} else {
		fmt.Printf("  Character bible ready (%d chars)\n", len(bible))
	}

	var paths []string

	// 1. Cover
	fmt.Println("  Generating cover page...")
	if p, err := a.generateSinglePage(buildCoverPrompt(storyText, style, bible), "comic_cover"); err != nil {
		fmt.Printf("  Warning: cover generation failed: %v\n", err)
	} else {
		paths = append(paths, p)
	}

	// 2. Story pages (9-panel grids)
	sections := splitIntoSections(storyText, storyPageCount)
	for i, section := range sections {
		pageNum := i + 1
		fmt.Printf("  Generating story page %d/%d...\n", pageNum, storyPageCount)
		p, err := a.generateSinglePage(
			buildStoryPagePrompt(section, pageNum, storyPageCount, style, bible),
			fmt.Sprintf("comic_page_%d", pageNum),
		)
		if err != nil {
			return paths, fmt.Errorf("story page %d failed: %w", pageNum, err)
		}
		paths = append(paths, p)
	}

	// 3. Back cover
	fmt.Println("  Generating back cover...")
	if p, err := a.generateSinglePage(buildBackCoverPrompt(storyText, style, bible), "comic_back"); err != nil {
		fmt.Printf("  Warning: back cover generation failed: %v\n", err)
	} else {
		paths = append(paths, p)
	}

	return paths, nil
}

// buildCharacterBible calls Gemini to produce a strict visual reference card.
// No ThinkingConfig is set — same pattern as the working story generator.
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

	ctx, cancel := context.WithTimeout(context.Background(), bibleTimeout)
	defer cancel()

	resp, err := client.Models.GenerateContent(ctx, bibleModel,
		[]*genai.Content{genai.NewContentFromText(characterBiblePrompt+storyText, genai.RoleUser)},
		&genai.GenerateContentConfig{
			MaxOutputTokens: bibleMaxTokens,
		},
	)
	if err != nil {
		return "", fmt.Errorf("gemini bible call: %w", err)
	}

	bible := strings.TrimSpace(resp.Text())
	if bible == "" {
		reason := "unknown"
		if len(resp.Candidates) > 0 {
			reason = string(resp.Candidates[0].FinishReason)
		}
		return "", fmt.Errorf("empty response (finish reason: %s)", reason)
	}

	return bible, nil
}

// generateSinglePage downloads and saves one image for the given prompt.
func (a *Artist) generateSinglePage(prompt, fileNamePattern string) (string, error) {
	opts := image.DefaultSearchOptions("vocabulary story")
	opts.CustomPrompt = prompt
	opts.AspectRatio = comicPageAspectRatio

	downloader := image.NewDownloader(a.nbClient, &image.DownloadOptions{
		OutputDir:         a.outputDir,
		OverwriteExisting: true,
		CreateDir:         true,
		FileNamePattern:   fileNamePattern,
		MaxSizeBytes:      20 * 1024 * 1024,
	})

	_, savedPath, err := downloader.DownloadBestMatchWithOptions(context.Background(), opts)
	return savedPath, err
}

// buildCoverPrompt constructs the front-cover image prompt.
func buildCoverPrompt(storyText, style, bible string) string {
	// Use a short excerpt as a teaser on the cover prompt.
	teaser := strings.TrimSpace(storyText)
	if len(teaser) > 300 {
		teaser = teaser[:300]
		if idx := strings.LastIndex(teaser, " "); idx > 0 {
			teaser = teaser[:idx]
		}
		teaser += "…"
	}

	bibleBlock := bibleSection(bible, "cover")
	return fmt.Sprintf(
		"Art style: %s.%s\n"+
			"COMIC BOOK FRONT COVER — single full-bleed illustration, no panel grid. "+
			"Large bold title text at the top: \"BULGARIAN VOCABULARY ADVENTURE\". "+
			"Show the main character(s) in a dynamic, eye-catching pose with the story setting "+
			"behind them. Dramatic, inviting, professional comic cover composition. "+
			"Story teaser:\n\n%s",
		style, bibleBlock, teaser,
	)
}

// buildStoryPagePrompt constructs a 9-panel grid page prompt.
func buildStoryPagePrompt(section string, pageNum, totalPages int, style, bible string) string {
	excerpt := strings.TrimSpace(section)
	if len(excerpt) > comicPromptMaxChars {
		excerpt = excerpt[:comicPromptMaxChars]
		if idx := strings.LastIndex(excerpt, " "); idx > 0 {
			excerpt = excerpt[:idx]
		}
		excerpt += "…"
	}

	bibleBlock := bibleSection(bible, fmt.Sprintf("story page %d of %d", pageNum, totalPages))
	return fmt.Sprintf(
		"Art style: %s.%s\n"+
			"Comic book story page %d of %d. Layout: a 3×3 grid of 9 panels filling the page, "+
			"each panel showing a distinct moment from the excerpt below. "+
			"All characters MUST look identical across every panel — same face, hair, and clothing "+
			"as described in the reference above. Story excerpt:\n\n%s",
		style, bibleBlock, pageNum, totalPages, excerpt,
	)
}

// buildBackCoverPrompt constructs the back-cover image prompt.
func buildBackCoverPrompt(storyText, style, bible string) string {
	// Use the last ~200 chars of the story as the resolution hint.
	ending := strings.TrimSpace(storyText)
	if len(ending) > 200 {
		ending = ending[len(ending)-200:]
		if idx := strings.Index(ending, " "); idx > 0 {
			ending = ending[idx+1:]
		}
	}

	bibleBlock := bibleSection(bible, "back cover")
	return fmt.Sprintf(
		"Art style: %s.%s\n"+
			"COMIC BOOK BACK COVER — single full-bleed illustration, no panel grid. "+
			"A calm, warm, conclusive scene from the story's ending. "+
			"Small text area at the bottom for a short blurb (leave space). "+
			"Story ending hint:\n\n%s",
		style, bibleBlock, ending,
	)
}

// bibleSection formats the character bible as a labelled block for the prompt.
// Returns empty string when bible is empty.
func bibleSection(bible, context string) string {
	if bible == "" {
		return ""
	}
	return fmt.Sprintf(
		"\nCHARACTER & SETTING REFERENCE (%s — follow exactly, do NOT change clothing):\n%s\n",
		context, bible,
	)
}

func pickStyle() string {
	if rand.Float64() < 0.9 {
		return comicStyles[0]
	}
	return comicStyles[1+rand.IntN(len(comicStyles)-1)]
}

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
