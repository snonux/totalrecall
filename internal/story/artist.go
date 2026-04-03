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

	// helperModel matches the story generator's proven model (gemini-2.5-flash).
	// Both the bible and blurb use the same SystemInstruction + user-content pattern
	// that the story generator uses successfully.
	helperModel = "gemini-2.5-flash"

	// helperTimeout gives Gemini up to 90 s per helper call; thinking tokens
	// within gemini-2.5-flash need more time than a plain text model.
	helperTimeout = 90 * time.Second

	// helperMaxTokens must be large enough to cover internal thinking tokens
	// (gemini-2.5-flash) plus the visible output. 8192 matches the story generator.
	helperMaxTokens = int32(8192)

	// helperRetryPause waits before retrying when the model returns an empty
	// response — typically caused by free-tier RPM exhaustion between rapid calls.
	helperRetryPause = 15 * time.Second
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
// bibleSystemInstruction is the SystemInstruction role for the character bible call.
// Matching the story generator's proven SystemInstruction + user-content split ensures
// gemini-2.5-flash allocates its thinking budget correctly instead of returning empty.
const bibleSystemInstruction = `You are a comic-book art director producing a CHARACTER CONSISTENCY GUIDE in English for an illustrator.

For every named HUMAN character provide: name, age estimate, hair (colour + style), eye colour,
skin tone, build, and the EXACT clothing they wear — specify garment, colour, pattern, and fit.
Clothing must NOT change between panels unless the story explicitly describes a change;
if no change is described, list the same outfit for all appearances.

For every named ANIMAL character provide: name, species, exact breed, fur/feather/scale colour
and pattern, eye colour, size, any distinctive markings, and typical body posture.
The animal must look IDENTICAL on every page — same breed, same markings, same eye colour.
Do NOT substitute a generic animal; if the story says Persian cat, every panel must show a
Persian cat with the exact described colouring.

Also describe: the setting (location, time of day, weather, key props) and overall
lighting / colour mood.

Be extremely specific — this guide will be copy-pasted into every panel prompt to lock visual
consistency. Maximum 280 words. No headers, just dense descriptive prose.`

// blurbSystemInstruction is the SystemInstruction role for the back-cover blurb call.
const blurbSystemInstruction = `You are a comic-book editor writing back-cover marketing copy.
Rules: write exactly 2–3 sentences in English; exciting and enticing; do NOT spoil the ending;
use present-tense second-person (e.g. "Join Eli as she discovers…").
Output only the blurb text — no quotes, no labels, no extra commentary.`

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

	bible, blurb := a.buildHelperTexts(storyText)

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
	if p, err := a.generateSinglePage(buildBackCoverPrompt(storyText, style, bible, blurb), "comic_back"); err != nil {
		fmt.Printf("  Warning: back cover generation failed: %v\n", err)
	} else {
		paths = append(paths, p)
	}

	return paths, nil
}

// buildHelperTexts generates the character bible and back-cover blurb in sequence.
// Both use gemini-2.0-flash with a single retry on empty response (rate-limit recovery).
// Returns ("", "") on total failure — callers degrade gracefully without these.
func (a *Artist) buildHelperTexts(storyText string) (bible, blurb string) {
	if a.apiKey == "" {
		fmt.Println("  Warning: no API key — skipping character bible and blurb")
		return "", ""
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: a.apiKey})
	if err != nil {
		fmt.Printf("  Warning: Gemini client failed (%v); panels may vary\n", err)
		return "", ""
	}

	bible = a.callGeminiHelper(client, bibleSystemInstruction, storyText, "character bible")
	if bible != "" {
		fmt.Printf("  Character bible ready (%d chars)\n", len(bible))
	}

	blurb = a.callGeminiHelper(client, blurbSystemInstruction, storyText, "back-cover blurb")
	if blurb != "" {
		fmt.Printf("  Back-cover blurb ready (%d chars)\n", len(blurb))
	}

	return bible, blurb
}

// callGeminiHelper sends one text prompt to helperModel and returns the trimmed response.
// Uses the same SystemInstruction + user-content pattern as the story generator,
// which is the proven approach for gemini-2.5-flash. Retries once after
// helperRetryPause when the model returns an empty string (free-tier RPM recovery).
func (a *Artist) callGeminiHelper(client *genai.Client, systemInstruction, userPrompt, label string) string {
	for attempt := 1; attempt <= 2; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), helperTimeout)
		resp, err := client.Models.GenerateContent(ctx, helperModel,
			[]*genai.Content{genai.NewContentFromText(userPrompt, genai.RoleUser)},
			&genai.GenerateContentConfig{
				SystemInstruction: &genai.Content{
					Parts: []*genai.Part{{Text: systemInstruction}},
				},
				MaxOutputTokens: helperMaxTokens,
			},
		)
		cancel()

		if err != nil {
			fmt.Printf("  Warning: %s attempt %d failed: %v\n", label, attempt, err)
		} else if text := strings.TrimSpace(resp.Text()); text != "" {
			return text
		} else {
			fmt.Printf("  Warning: %s attempt %d returned empty response\n", label, attempt)
		}

		if attempt < 2 {
			fmt.Printf("  Retrying %s in %s...\n", label, helperRetryPause)
			time.Sleep(helperRetryPause)
		}
	}
	return ""
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
			"TRADITIONAL COMIC BOOK FRONT COVER — portrait orientation, single full-bleed illustration.\n"+
			"NO panel grid. NO speech bubbles.\n"+
			"MANDATORY TITLE — the most important element on this cover:\n"+
			"  The title 'BULGARIAN VOCABULARY ADVENTURE' MUST appear in HUGE, dominant lettering "+
			"across the very top of the cover. Use a bold, colourful comic-book masthead font — "+
			"thick outlines, high contrast against the background, taking up the top 20%% of the image. "+
			"This title MUST be legible and unmissable.\n"+
			"Remaining layout rules:\n"+
			"  • MAIN ART: below the title, a single dramatic illustration of the main character(s) "+
			"and any animals in a dynamic pose, richly detailed story setting behind them.\n"+
			"  • COVER LINES: 2–3 short teaser phrases in bold display type (e.g. 'A Summer Adventure!').\n"+
			"  • BOTTOM STRIP: price box bottom-left, issue number bottom-right — "+
			"classic Silver-Age / Bronze-Age comic production design.\n"+
			"Characters and animals MUST match the reference exactly. Story teaser:\n\n%s",
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
			"each panel showing a distinct moment from the excerpt below.\n"+
			"STRICT CONSISTENCY RULES — apply to every single panel:\n"+
			"  • Human characters: identical face, hair colour/style, and clothing to the reference.\n"+
			"  • Animal characters: identical breed, fur colour/pattern, markings, and eye colour — "+
			"NEVER substitute a different animal or a generic version of the species.\n"+
			"  • Clothing changes only if this page's excerpt explicitly describes a change.\n"+
			"Story excerpt:\n\n%s",
		style, bibleBlock, pageNum, totalPages, excerpt,
	)
}

// buildBackCoverPrompt constructs the back-cover image prompt.
// blurb is an English marketing summary generated by Gemini; when non-empty it is
// embedded verbatim in the blurb-box instruction so the image model renders it.
func buildBackCoverPrompt(storyText, style, bible, blurb string) string {
	// Use the last ~200 chars of the story as a visual hint for the scene.
	ending := strings.TrimSpace(storyText)
	if len(ending) > 200 {
		ending = ending[len(ending)-200:]
		if idx := strings.Index(ending, " "); idx > 0 {
			ending = ending[idx+1:]
		}
	}

	// Build the blurb box instruction: use the generated blurb if available,
	// otherwise ask the model to leave a styled empty box.
	blurbBoxInstruction := "a rectangular text box (white or cream background, thin black border) " +
		"near the bottom — styled like a classic back-cover synopsis box, box shape required."
	if blurb != "" {
		blurbBoxInstruction = fmt.Sprintf(
			"a rectangular text box (white or cream background, thin black border) "+
				"near the bottom displaying this blurb text in italic type:\n"+
				"    \"%s\"", blurb)
	}

	bibleBlock := bibleSection(bible, "back cover")
	return fmt.Sprintf(
		"Art style: %s.%s\n"+
			"TRADITIONAL COMIC BOOK BACK COVER — portrait orientation, single full-bleed illustration.\n"+
			"NO panel grid. NO speech bubbles.\n"+
			"Layout rules (must follow exactly):\n"+
			"  • MAIN ART: a calm, warm, resolved scene filling the upper 60%% of the cover — "+
			"the main character(s) and any animals in a peaceful or triumphant ending moment, "+
			"with the full story setting behind them.\n"+
			"  • BLURB BOX: %s\n"+
			"  • BOTTOM STRIP: barcode box bottom-left (black-and-white barcode graphic), "+
			"series title 'BULGARIAN VOCABULARY ADVENTURE' bottom-right — "+
			"classic comic book back-cover production design.\n"+
			"Characters and animals MUST match the reference above exactly. "+
			"Story ending hint:\n\n%s",
		style, bibleBlock, blurbBoxInstruction, ending,
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
