package story

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/genai"

	"codeberg.org/snonux/totalrecall/internal/batch"
	"codeberg.org/snonux/totalrecall/internal/image"
)

const (
	// storyPageCount is the number of story pages (excluding cover/back/gallery).
	// Each page uses a 2×2 grid of 4 panels in landscape (16:9) format.
	// cover + 5 story pages + 3 gallery pages + back cover = 10 total.
	storyPageCount = 5

	// galleryPageCount is the number of text-free close-up character art pages
	// inserted between the story pages and the back cover. Each is a full-bleed
	// single illustration of the hero/heroine in a distinct dramatic pose.
	// cover + 5 story pages + 5 gallery pages + back cover = 12 total.
	galleryPageCount = 5

	// pageMaxRetries is the number of times a story page generation is retried
	// before being skipped. Gemini image generation occasionally returns no data
	// due to transient safety filter hits or API hiccups; a retry usually succeeds.
	pageMaxRetries = 3

	// pageRetryPause is the wait between story page retries.
	pageRetryPause = 10 * time.Second

	// comicPageAspectRatio: 16:9 is the closest supported widescreen ratio for
	// the ThinkPad X1 Gen 9 (2560×1600 / 16:10), filling the display with minimal
	// letterboxing. The API supports 16:9 but not 16:10.
	comicPageAspectRatio = "16:9"

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

	// renderingRequirement is appended to every image prompt (cover, story pages,
	// back cover, gallery) to push the model toward photorealistic output even
	// within a comic grid layout. Centralised here so it is easy to tune.
	// Omitted when Artist.ultraRealistic is false (--no-ultra-realistic flag).
	renderingRequirement = "RENDERING REQUIREMENT: every panel and illustration must look " +
		"like a real photograph — photorealistic skin texture, fabric detail, lighting, " +
		"and environment. NOT a drawing, painting, or illustration. Real-world photo quality.\n"
)

// realisticStyles is the style pool used when ultra-realistic mode is active.
// These descriptions avoid "comic strip" / "illustration" language so the image
// model produces photographic output rather than comic-book artwork.
var realisticStyles = []string{
	"ultra-realistic DSLR photography, cinematic 35mm lens, natural lighting, hyper-detailed textures",
	"cinematic still photography, golden-hour lighting, shallow depth of field, photojournalism quality",
	"hyper-realistic photography, studio-quality lighting, sharp focus, true-to-life colours and textures",
}

// comicStyles is the pool used when standard comic style is active.
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

For every named HUMAN character provide: name, apparent age category (young child, teenager,
young adult, middle-aged, elderly), hair (colour + style), eye colour, skin tone, build,
and the EXACT clothing they wear — specify garment, colour, pattern, and fit.
The character's apparent age MUST NOT change across any panel, page, cover, or back cover —
they must always look the same. Clothing must NOT change between panels unless the story
explicitly describes a change; if no change is described, list the same outfit for all appearances.

For every named ANIMAL character provide: name, species, exact breed, fur/feather/scale colour
and pattern, eye colour, size, any distinctive markings, and typical body posture.
The animal must look IDENTICAL on every page — same breed, same markings, same eye colour.
Do NOT substitute a generic animal; if the story says Persian cat, every panel must show a
Persian cat with the exact described colouring.

Also describe: the setting (location, time of day, weather, key props) and overall
lighting / colour mood.

Be extremely specific — this guide will be copy-pasted into every panel prompt to lock visual
consistency. Maximum 300 words. No headers, just dense descriptive prose.`

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
	// UltraRealistic controls whether renderingRequirement is injected into every
	// prompt. Default true (ultra-realistic). Set false via --no-ultra-realistic
	// to produce standard comic-book style output without the photo requirement.
	UltraRealistic bool
}

// Artist generates comic-book pages that illustrate the story.
type Artist struct {
	nbClient       image.ImageClient
	apiKey         string // used for the character-bible Gemini call
	outputDir      string
	style          string
	ultraRealistic bool // false → omit renderingRequirement from all prompts
}

// NewArtist creates an Artist backed by the NanoBanana image generator.
func NewArtist(config *ArtistConfig) *Artist {
	dir := "."
	var apiKey, style string
	ultraRealistic := true // default on
	var nbConfig *image.NanoBananaConfig

	if config != nil {
		dir = orDefault(config.OutputDir, ".")
		apiKey = config.APIKey
		style = config.Style
		ultraRealistic = config.UltraRealistic
		nbConfig = &image.NanoBananaConfig{
			APIKey:    config.APIKey,
			Model:     config.Model,
			TextModel: config.TextModel,
		}
	}

	return &Artist{
		nbClient:       image.NewNanoBananaClient(nbConfig),
		apiKey:         apiKey,
		outputDir:      dir,
		style:          style,
		ultraRealistic: ultraRealistic,
	}
}

// DrawComicPages generates 5 images total:
//   - <titleSlug>_cover.png          — full-bleed cover
//   - <titleSlug>_page_1.png … _3    — 4-panel (2×2 grid) landscape story pages
//   - <titleSlug>_back.png           — back cover
//
// A character bible injected into every prompt keeps characters, clothing, and
// setting consistent across all pages. The bible is produced by GenerateFull in
// the same Gemini call as the story; prebuiltBible is passed in from there.
// entries are the vocabulary words from input.txt — they are injected into every
// story page prompt so the image model visually features and labels them in panels.
// titleSlug is used as the file-name prefix; it must already be a safe slug.
// Returns the list of saved image paths in order.
func (a *Artist) DrawComicPages(storyText, prebuiltBible, titleSlug string, entries []batch.WordEntry) ([]string, error) {
	style := a.style
	if style == "" {
		// Ultra-realistic mode uses photography-only language so the image model
		// produces photographic output. The comicStyles pool contains "comic strip"
		// which dominates the model's style interpretation even when the
		// renderingRequirement const is present — hence a separate pool is needed.
		if a.ultraRealistic {
			style = realisticStyles[rand.IntN(len(realisticStyles))]
		} else {
			style = pickStyle()
		}
	}
	fmt.Printf("  Comic style: %s\n", style)
	if a.ultraRealistic {
		fmt.Println("  Rendering mode: ultra-realistic (photorealistic panels)")
	} else {
		fmt.Println("  Rendering mode: standard comic style")
	}

	bible, blurb := a.resolveHelperTexts(storyText, prebuiltBible)

	var paths []string
	// recentRefs holds image bytes from recently generated pages for iterative
	// chaining: each new page receives the cover + the previous page as visual
	// reference so the model can match character appearance directly from pixels
	// rather than relying on text descriptions alone.
	var recentRefs [][]byte

	// 1. Cover — generated without refs (it is the visual baseline).
	// Retried up to pageMaxRetries times; failure is non-fatal but the cover
	// is omitted from the PDF and no anchor reference is established.
	p, coverBytes := a.loadOrGenerate(titleSlug+"_cover", func() (string, []byte) {
		return a.generatePageWithRetry(buildCoverPrompt(storyText, style, bible, a.renderReq()), titleSlug+"_cover", nil, "cover page")
	})
	if p != "" {
		paths = append(paths, p)
		recentRefs = appendRef(recentRefs, coverBytes) // cover becomes the anchor reference
	}

	// 2. Story pages — each receives cover + previous page as refs.
	// Failures are non-fatal: up to pageMaxRetries attempts per page, then a
	// warning is logged and generation continues with the next page so the PDF
	// always contains as many pages as the API manages to produce.
	sections := splitIntoSections(storyText, storyPageCount)
	for i, section := range sections {
		pageNum := i + 1
		prompt := buildStoryPagePrompt(section, pageNum, storyPageCount, style, bible, entries, a.renderReq())
		fileName := fmt.Sprintf("%s_page_%d", titleSlug, pageNum)
		p, pageBytes := a.loadOrGenerate(fileName, func() (string, []byte) {
			return a.generateStoryPage(prompt, fileName, pageNum, recentRefs)
		})
		if p != "" {
			paths = append(paths, p)
			recentRefs = appendRef(recentRefs, pageBytes)
		}
	}

	// 3. Gallery pages — text-free close-up character art pages, one per pose.
	// Each is a full-bleed single illustration; no panels, no text, no speech bubbles.
	// They act as alternative covers and use the accumulated refs for consistency.
	for i := range galleryPageCount {
		galleryNum := i + 1
		prompt := buildGalleryPagePrompt(style, bible, galleryNum, a.renderReq())
		fileName := fmt.Sprintf("%s_gallery_%d", titleSlug, galleryNum)
		gp, galleryBytes := a.loadOrGenerate(fileName, func() (string, []byte) {
			return a.generatePageWithRetry(prompt, fileName, recentRefs,
				fmt.Sprintf("gallery page %d/%d", galleryNum, galleryPageCount))
		})
		if gp != "" {
			paths = append(paths, gp)
			recentRefs = appendRef(recentRefs, galleryBytes)
		}
	}

	// 4. Back cover — receives the same rolling refs as the last gallery page.
	// Retried up to pageMaxRetries times; failure is non-fatal.
	p, _ = a.loadOrGenerate(titleSlug+"_back", func() (string, []byte) {
		return a.generatePageWithRetry(buildBackCoverPrompt(storyText, style, bible, blurb, a.renderReq()), titleSlug+"_back", recentRefs, "back cover")
	})
	if p != "" {
		paths = append(paths, p)
	}

	return paths, nil
}

// generateStoryPage attempts to generate a single story page up to pageMaxRetries
// times. It returns the saved path and image bytes on success, or empty strings
// after all retries are exhausted (non-fatal — the caller continues with the next
// page so the PDF is never aborted by a single transient API failure).
func (a *Artist) generateStoryPage(prompt, fileName string, pageNum int, refs [][]byte) (string, []byte) {
	fmt.Printf("  Generating story page %d/%d...\n", pageNum, storyPageCount)
	for attempt := 1; attempt <= pageMaxRetries; attempt++ {
		p, pageBytes, err := a.generateSinglePage(prompt, fileName, refs)
		if err == nil {
			return p, pageBytes
		}
		if attempt < pageMaxRetries {
			fmt.Printf("  Story page %d attempt %d failed (%v), retrying in %s...\n",
				pageNum, attempt, err, pageRetryPause)
			time.Sleep(pageRetryPause)
		} else {
			fmt.Printf("  Warning: story page %d failed after %d attempts: %v\n",
				pageNum, pageMaxRetries, err)
		}
	}
	return "", nil
}

// generatePageWithRetry attempts to generate a single comic page (cover or back
// cover) up to pageMaxRetries times. Returns the saved path and image bytes on
// success, or ("", nil) after all retries are exhausted (non-fatal).
func (a *Artist) generatePageWithRetry(prompt, fileName string, refs [][]byte, label string) (string, []byte) {
	fmt.Printf("  Generating %s...\n", label)
	for attempt := 1; attempt <= pageMaxRetries; attempt++ {
		p, imgBytes, err := a.generateSinglePage(prompt, fileName, refs)
		if err == nil {
			return p, imgBytes
		}
		if attempt < pageMaxRetries {
			fmt.Printf("  Warning: %s attempt %d/%d failed (%v), retrying in %s...\n",
				label, attempt, pageMaxRetries, err, pageRetryPause)
			time.Sleep(pageRetryPause)
		} else {
			fmt.Printf("  Warning: %s failed after %d attempts: %v\n",
				label, pageMaxRetries, err)
		}
	}
	return "", nil
}

// loadOrGenerate returns the saved path and image bytes for fileName.
// If the PNG already exists on disk it is loaded and returned without an API
// call — skipping regeneration of pages that were produced in a previous run.
// If the file is missing, generateFn is called to produce it. This lets a
// re-run fill in only the pages that failed previously without wasting quota.
func (a *Artist) loadOrGenerate(fileName string, generateFn func() (string, []byte)) (string, []byte) {
	path := filepath.Join(a.outputDir, fileName+".png")
	if _, err := os.Stat(path); err == nil {
		// Page exists — load bytes for the reference chain and skip the API call.
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			fmt.Printf("  Warning: could not read existing %s for chaining: %v\n", fileName+".png", readErr)
			return path, nil
		}
		fmt.Printf("  Skipping %s (already exists)\n", fileName+".png")
		return path, b
	}
	return generateFn()
}

// appendRef adds imgBytes to refs and keeps at most 2 entries (cover anchor +
// the immediately preceding page). Larger windows inflate the multimodal
// payload significantly without proportional consistency gains.
func appendRef(refs [][]byte, imgBytes []byte) [][]byte {
	if len(imgBytes) == 0 {
		return refs
	}
	refs = append(refs, imgBytes)
	if len(refs) > 2 {
		// Keep the first entry (cover anchor) and the latest page only.
		refs = [][]byte{refs[0], refs[len(refs)-1]}
	}
	return refs
}

// resolveHelperTexts returns the character bible and back-cover blurb.
// The bible comes from prebuiltBible (produced by GenerateFull in the same
// Gemini call as the story — no extra API call, no rate-limiting). The blurb
// is still generated with a separate call since it is not part of story generation.
func (a *Artist) resolveHelperTexts(storyText, prebuiltBible string) (bible, blurb string) {
	bible = prebuiltBible
	if bible != "" {
		fmt.Printf("  Character bible ready (%d chars)\n", len(bible))
	} else {
		fmt.Println("  Warning: no character bible — characters may vary between pages")
	}

	if a.apiKey == "" {
		return bible, ""
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: a.apiKey})
	if err != nil {
		fmt.Printf("  Warning: Gemini client failed for blurb (%v)\n", err)
		return bible, ""
	}

	blurb = a.callGeminiHelper(client, blurbSystemInstruction, storyText, "back-cover blurb")
	if blurb != "" {
		fmt.Printf("  Back-cover blurb ready (%d chars)\n", len(blurb))
	}
	return bible, blurb
}

// callGeminiHelper sends one text prompt to helperModel and returns the trimmed response.
// Retries once after helperRetryPause on empty response.
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
// refs are optional previously generated page images passed as multimodal
// context to the image model for iterative chaining consistency.
// Returns the saved file path and raw PNG bytes (for use as ref in next page).
func (a *Artist) generateSinglePage(prompt, fileNamePattern string, refs [][]byte) (string, []byte, error) {
	opts := image.DefaultSearchOptions("vocabulary story")
	opts.CustomPrompt = prompt
	opts.AspectRatio = comicPageAspectRatio
	opts.ReferenceImages = refs

	downloader := image.NewDownloader(a.nbClient, &image.DownloadOptions{
		OutputDir:         a.outputDir,
		OverwriteExisting: true,
		CreateDir:         true,
		FileNamePattern:   fileNamePattern,
		MaxSizeBytes:      20 * 1024 * 1024,
	})

	_, savedPath, err := downloader.DownloadBestMatchWithOptions(context.Background(), opts)
	if err != nil {
		return "", nil, err
	}

	// Read back the saved PNG so callers can pass it as a reference image to
	// subsequent pages. Non-fatal if the read fails — we just skip the reference.
	imgBytes, readErr := os.ReadFile(savedPath)
	if readErr != nil {
		fmt.Printf("  Warning: could not read back %s for chaining: %v\n", savedPath, readErr)
		imgBytes = nil
	}

	return savedPath, imgBytes, nil
}

// renderReq returns the renderingRequirement string when ultraRealistic is true,
// or an empty string when --no-ultra-realistic is set. Used in all prompt builders.
func (a *Artist) renderReq() string {
	if a.ultraRealistic {
		return renderingRequirement
	}
	return ""
}

// buildCoverPrompt constructs the front-cover image prompt.
func buildCoverPrompt(storyText, style, bible, renderReq string) string {
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
		// Bulgarian language rule placed first so the model processes it before any other instruction.
		"ЗАДЪЛЖИТЕЛНО / MANDATORY LANGUAGE RULE: This is a BULGARIAN comic book. "+
			"All text on the cover (cover lines, banners, labels) MUST be in Bulgarian "+
			"Cyrillic script. The masthead title must also be rendered in a striking comic-book font.\n\n"+
			"Art style: %s.%s\n"+
			renderReq+
			"TRADITIONAL COMIC BOOK FRONT COVER — single full-bleed illustration, landscape 16:9 format.\n"+
			"NO panel grid. NO speech bubbles.\n"+
			"MANDATORY MASTHEAD — the most important visual element on this cover:\n"+
			"  • Invent a DRAMATIC, STORY-SPECIFIC comic book title that fits the characters and "+
			"theme of the story teaser below (e.g. for a space story: 'ГАЛАКТИЧЕСКИ ГЕРОИ', for "+
			"a mystery: 'ТАЙНАТА НА ГОРАТА'). The title must be in HUGE, dominant lettering "+
			"across the very top of the cover — bold comic-book masthead font, thick outlines, "+
			"bright contrasting colours (yellow, red, or white on dark), taking up the top 20%% "+
			"of the image. This title MUST be legible and unmissable.\n"+
			"  • Directly below the main title, add a smaller subtitle banner: "+
			"'BULGARIAN VOCABULARY ADVENTURE' in a contrasting accent colour.\n"+
			"  • Add a bold comic-book LOGO BUG (small circular or star-shaped badge) "+
			"in the top-left corner — e.g. a planet, rocket, magnifying glass, sword — "+
			"matching the story theme. The logo should feel like a real publisher imprint.\n"+
			"Remaining layout rules:\n"+
			"  • MAIN ART: below the masthead, a dramatic illustration of EXACTLY the named characters "+
			"from the story (as described in the reference above) — same faces, same ages, same "+
			"clothing, same animals. Do NOT invent new characters or use generic stand-ins.\n"+
			"  • COVER LINES: 2–3 short Bulgarian teaser phrases in bold display type "+
			"(e.g. 'НЕВЕРОЯТНО ПРИКЛЮЧЕНИЕ!' or 'СРЕЩА С НЕПОЗНАТОТО!')\n"+
			"  • BOTTOM STRIP: price box bottom-left, issue number bottom-right — "+
			"classic Silver-Age / Bronze-Age comic production design.\n"+
			"IMPORTANT: only the characters named in the reference may appear on this cover. "+
			"Same age, same face, same clothing as in the interior pages. "+
			"LANGUAGE REMINDER: all cover text in Bulgarian Cyrillic — see rule at top. "+
			"Story teaser:\n\n%s",
		style, bibleBlock, teaser,
	)
}

// buildStoryPagePrompt constructs a landscape comic page prompt.
// Layout uses a 2×2 grid of 4 panels optimised for the 16:9 aspect ratio.
// entries are injected as a vocabulary block so the image model features and
// labels each word visually inside the panels — making each page a learning tool.
// The Bulgarian language requirement is placed at the very top so it is processed
// before all other instructions.
func buildStoryPagePrompt(section string, pageNum, totalPages int, style, bible string, entries []batch.WordEntry, renderReq string) string {
	excerpt := strings.TrimSpace(section)
	if len(excerpt) > comicPromptMaxChars {
		excerpt = excerpt[:comicPromptMaxChars]
		if idx := strings.LastIndex(excerpt, " "); idx > 0 {
			excerpt = excerpt[:idx]
		}
		excerpt += "…"
	}

	bibleBlock := bibleSection(bible, fmt.Sprintf("story page %d of %d", pageNum, totalPages))
	vocabBlock := buildVocabBlock(entries)
	return fmt.Sprintf(
		// Lead with the hard language constraint so it is processed first.
		"ЗАДЪЛЖИТЕЛНО / MANDATORY LANGUAGE RULE: This is a BULGARIAN comic book. "+
			"Every word of text inside speech bubbles, thought bubbles, caption boxes, "+
			"and panel labels MUST be written in Bulgarian Cyrillic script "+
			"(например: Здравей! Какво правиш? Побързай!). "+
			"English text anywhere in the panels is STRICTLY FORBIDDEN — use ONLY Bulgarian.\n\n"+
			"%s"+ // vocabulary block — before art style so it is never truncated
			"Art style: %s.%s\n"+
			"Comic book story page %d of %d. "+
			"MANDATORY PANEL LAYOUT — divide the image into exactly 4 panels in a 2×2 grid:\n"+
			"  • TOP-LEFT panel: scene 1 from the excerpt\n"+
			"  • TOP-RIGHT panel: scene 2 from the excerpt\n"+
			"  • BOTTOM-LEFT panel: scene 3 from the excerpt\n"+
			"  • BOTTOM-RIGHT panel: scene 4 from the excerpt\n"+
			"Each panel is separated by a thin black gutter line. "+
			"All 4 panels must be clearly distinct scenes — NOT one continuous image. "+
			"The full image area must be covered by the 4 panels with no empty space.\n"+
			renderReq+
			"STRICT CONSISTENCY RULES — apply to every single panel:\n"+
			"  • Human characters: identical face, AGE APPEARANCE, hair colour/style, and clothing "+
			"to the reference — a child must never look older or younger than defined.\n"+
			"  • Animal characters: identical breed, fur colour/pattern, markings, and eye colour — "+
			"NEVER substitute a different animal or a generic version of the species.\n"+
			"  • Clothing changes only if this page's excerpt explicitly describes a change.\n"+
			"  • LANGUAGE: all speech, thought, and caption text — Bulgarian Cyrillic ONLY.\n"+
			"Story excerpt (ALL panels must illustrate THIS excerpt only — no other part of the story):\n\n%s",
		vocabBlock, style, bibleBlock, pageNum, totalPages, excerpt,
	)
}

// buildVocabBlock formats the vocabulary entries as a mandatory visual instruction
// block. Each word must appear as a clearly labelled object or element in at least
// one panel — making the comic page a vocabulary learning tool as well as a story page.
func buildVocabBlock(entries []batch.WordEntry) string {
	if len(entries) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("VOCABULARY WORDS — each word below MUST appear as a clearly visible, labelled\n")
	sb.WriteString("object or element in at least one panel. Show the object in the scene and add a\n")
	sb.WriteString("small Bulgarian label directly on it (bold text, contrasting colour, easy to read):\n")
	for _, e := range entries {
		if e.Translation != "" {
			sb.WriteString(fmt.Sprintf("  • %s (%s)\n", e.Bulgarian, e.Translation))
		} else {
			sb.WriteString(fmt.Sprintf("  • %s\n", e.Bulgarian))
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

// buildBackCoverPrompt constructs the back-cover image prompt.
// blurb is an English marketing summary generated by Gemini; when non-empty it is
// embedded verbatim in the blurb-box instruction so the image model renders it.
func buildBackCoverPrompt(storyText, style, bible, blurb, renderReq string) string {
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
		// Bulgarian language rule placed first for maximum model compliance.
		"ЗАДЪЛЖИТЕЛНО / MANDATORY LANGUAGE RULE: This is a BULGARIAN comic book. "+
			"All visible text (blurb box, labels, banners) MUST be in Bulgarian Cyrillic script. "+
			"English text anywhere on the back cover is STRICTLY FORBIDDEN.\n\n"+
			"Art style: %s.%s\n"+
			renderReq+
			"TRADITIONAL COMIC BOOK BACK COVER — single full-bleed illustration, landscape 16:9 format.\n"+
			"NO panel grid. NO speech bubbles.\n"+
			"Layout rules (must follow exactly):\n"+
			"  • MAIN ART: a calm, warm, resolved scene filling the upper 60%% of the cover — "+
			"EXACTLY the named characters from the story (as described in the reference above) "+
			"in a peaceful or triumphant ending moment, with the full story setting behind them. "+
			"Do NOT invent new characters or use generic stand-ins.\n"+
			"  • BLURB BOX: %s\n"+
			"  • BOTTOM STRIP: barcode box bottom-left (black-and-white barcode graphic), "+
			"series title 'BULGARIAN VOCABULARY ADVENTURE' bottom-right — "+
			"classic comic book back-cover production design.\n"+
			"IMPORTANT: only the characters named in the reference may appear on this back cover. "+
			"Same age, same face, same clothing, same animals as in the interior pages. "+
			"LANGUAGE REMINDER: all text in Bulgarian Cyrillic — see rule at top. "+
			"Story ending hint:\n\n%s",
		style, bibleBlock, blurbBoxInstruction, ending,
	)
}

// galleryPoses are the close-up compositions cycled across the 3 gallery pages.
// Each is a distinct dramatic framing so the pages feel like variant cover art.
var galleryPoses = []string{
	"extreme close-up portrait: face and shoulders filling the entire frame, dramatic three-quarter lighting, intense gaze directly at the viewer, fine detail on eyes and expression",
	"dynamic action pose: full body, low-angle shot looking up at the main character against the sky or setting backdrop, confident stance, hair and clothing caught in motion",
	"atmospheric mid-shot: waist-up, the main character silhouetted or lit by the ambient environment (bioluminescence, sunset, neon glow), looking off into the distance with a sense of wonder or resolve",
	"profile close-up: side view of face and upper body, soft rim lighting tracing the jawline and hair, contemplative expression, rich background bokeh",
	"power stance full-body: the main character seen from the front at eye level, arms relaxed but ready, environment filling the frame behind them, golden-hour or dramatic storm light",
}

// buildGalleryPagePrompt constructs a text-free close-up character art page prompt.
// galleryNum (1-based) selects the pose from galleryPoses so each page is distinct.
// No text, no panels, no speech bubbles — pure full-bleed illustration.
func buildGalleryPagePrompt(style, bible string, galleryNum int, renderReq string) string {
	pose := galleryPoses[(galleryNum-1)%len(galleryPoses)]
	bibleBlock := bibleSection(bible, fmt.Sprintf("gallery page %d", galleryNum))
	return fmt.Sprintf(
		"Art style: %s.%s\n"+
			renderReq+
			"FULL-BLEED SINGLE ILLUSTRATION — landscape 16:9 format, ONE image only, NO grid, NO panels.\n"+
			"DO NOT split the image into multiple panels or sections. The ENTIRE canvas is ONE single scene.\n"+
			"NO text of any kind. NO title. NO labels. NO speech bubbles. NO panel borders. NO UI elements.\n"+
			"This is a text-free character gallery page. Pure art only.\n\n"+
			"Composition: %s\n\n"+
			"The subject MUST be EXACTLY the main character(s) described in the reference above — "+
			"same faces, same genders, same ages, same clothing. Include the companion animal if naturally present. "+
			"Do NOT invent new characters or change any character's gender. Do NOT add any text overlays.\n"+
			"Background: the story's setting rendered with full cinematic atmosphere and colour mood.",
		style, bibleBlock, pose,
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

// pickUltraRealistic returns true (photorealistic) or false (comic style) with
// equal probability, giving each run a 50/50 chance of either look.
func pickUltraRealistic() bool {
	return rand.Float64() < 0.5
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
