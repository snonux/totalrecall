package story

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"google.golang.org/genai"

	"codeberg.org/snonux/totalrecall/internal/batch"
)

const (
	storyGeminiModel = "gemini-2.5-flash"
	storyTimeout     = 120 * time.Second
	// 8192 tokens for story-only generation (thinking + ~650-word visible story).
	storyMaxTokens = int32(8192)
	// 16384 total for the combined story+bible call, with thinking capped at 8192
	// (see storyFullThinkingBudget). This guarantees ~8192 tokens for the visible
	// output (story ~650 words + bible ~280 words ≈ 1300 tokens — well within budget).
	storyFullMaxTokens = int32(16384)
	// storyFullThinkingBudget caps the internal chain-of-thought so the model
	// cannot consume all MaxOutputTokens with thinking and produce no visible text.
	// Without this cap, gemini-2.5-flash exhausts all tokens on reasoning for the
	// complex combined prompt, making resp.Text() return an empty string.
	storyFullThinkingBudget = int32(8192)
	storySystemPrompt       = "You are a creative Bulgarian language teacher. Write engaging stories that naturally incorporate vocabulary words to help students learn."

	// storyBibleSeparator is the exact line the model must output between the
	// story and the character bible. Parsing splits the response on this marker.
	storyBibleSeparator = "---CHARACTER GUIDE---"

	// storyTitleSeparator is the exact line the model outputs after the bible to
	// deliver a short English comic title. parseGenerateResult extracts it for
	// use as the output directory name and file prefix.
	storyTitleSeparator = "---COMIC TITLE---"

	// storyPanelSeparator marks the start of the 20-line panel visual script.
	// The script lists one sentence per panel (P1-A … P5-D) so the artist can
	// draw each panel from an explicit description rather than guessing from
	// raw prose — this is the primary mechanism for narrative coherence.
	storyPanelSeparator = "---PANEL SCRIPT---"

	// storyPageCount duplicated here so generator.go can reference it without
	// importing artist.go (both are in package story; used in buildStoryPromptFull).
	storyPagesInScript = 5
	storyPanelsPerPage = 4
)

// storyGenres is the pool of genres picked randomly each run to keep stories
// varied — not always fairy tales. Realistic/slice-of-life is weighted at 40%
// (picked when index 0 or 1 is chosen) and the rest appear equally.
var storyGenres = []string{
	"a warm realistic slice-of-life story",
	"a heartfelt family drama",
	"an exciting science-fiction adventure",
	"a thrilling action-adventure story",
	"a mystery with a surprising twist",
	"a funny comedy with silly misunderstandings",
	"a fantasy quest in a magical world",
	"a spooky but kid-friendly horror story",
	"a space exploration adventure",
	"a superhero origin story",
}

// pickStoryGenre returns a random genre from the pool.
// Realistic genres (indices 0–1) appear 40% of the time; the rest 60%.
func pickStoryGenre() string {
	if rand.Float64() < 0.4 {
		return storyGenres[rand.IntN(2)]
	}
	return storyGenres[2+rand.IntN(len(storyGenres)-2)]
}

// resolveGenre returns theme if non-empty, otherwise picks a random genre.
// This lets the caller override the genre via --story-theme without changing
// the random pick logic.
func resolveGenre(theme string) string {
	if theme != "" {
		return theme
	}
	return pickStoryGenre()
}

// GenerateResult holds the story text, character bible, comic title, and panel
// script from a single combined Gemini call. All four are produced in the same
// request — no second API call, no rate limits.
type GenerateResult struct {
	StoryText   string     // Bulgarian vocabulary story
	Bible       string     // English character consistency guide for illustrators
	Title       string     // Short English comic title (2-4 words), used as filename slug
	PanelScript [][]string // [page][panel] explicit visual description, 5 pages × 4 panels
}

// Config holds generator settings and API credentials.
type Config struct {
	APIKey    string
	TextModel string // defaults to storyGeminiModel
	// Theme overrides the random genre pick when non-empty.
	// Passed verbatim as the genre phrase in the story prompt.
	Theme string
}

// Generator uses Gemini to produce vocabulary-based stories.
type Generator struct {
	client    *genai.Client
	initErr   error
	textModel string
	theme     string // overrides random genre pick when non-empty
}

// var seam for test injection, mirrors the phonetic/fetcher.go pattern.
var generateStoryText = func(ctx context.Context, client *genai.Client, model, prompt string) (string, error) {
	resp, err := client.Models.GenerateContent(ctx, model, []*genai.Content{
		genai.NewContentFromText(prompt, genai.RoleUser),
	}, &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: storySystemPrompt}},
		},
		MaxOutputTokens: storyMaxTokens,
	})
	if err != nil {
		return "", fmt.Errorf("gemini API error: %w", err)
	}

	text := strings.TrimSpace(resp.Text())
	if text == "" {
		return "", fmt.Errorf("no story content returned from Gemini")
	}

	return text, nil
}

// NewGenerator creates a Generator that calls Gemini with the given API key.
// If the API key is empty, Generate will return an error.
func NewGenerator(config *Config) *Generator {
	g := &Generator{
		textModel: storyGeminiModel,
		theme:     config.Theme,
	}

	if config == nil || config.APIKey == "" {
		g.initErr = fmt.Errorf("Google API key is required for story generation")
		return g
	}

	if config.TextModel != "" {
		g.textModel = config.TextModel
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey: config.APIKey,
	})
	if err != nil {
		g.initErr = fmt.Errorf("failed to create Gemini client: %w", err)
		return g
	}

	g.client = client
	return g
}

// Generate builds a ~500-word story that uses every word in entries naturally
// and returns the raw story text.
func (g *Generator) Generate(entries []batch.WordEntry) (string, error) {
	if g.initErr != nil {
		return "", g.initErr
	}

	ctx, cancel := context.WithTimeout(context.Background(), storyTimeout)
	defer cancel()

	prompt := buildStoryPrompt(entries, g.theme)
	return generateStoryText(ctx, g.client, g.textModel, prompt)
}

// GenerateFull generates the Bulgarian story and the character consistency bible
// in a single Gemini call, eliminating the separate bible API call that was
// failing due to thinking-token exhaustion. Uses storyFullMaxTokens (65536) —
// the model maximum — because the combined task requires more thinking budget
// than story generation alone.
func (g *Generator) GenerateFull(entries []batch.WordEntry) (GenerateResult, error) {
	if g.initErr != nil {
		return GenerateResult{}, g.initErr
	}

	ctx, cancel := context.WithTimeout(context.Background(), storyTimeout)
	defer cancel()

	// Call Gemini directly rather than through the generateStoryText seam so we
	// can apply a ThinkingBudget cap. Without it, gemini-2.5-flash uses all
	// available tokens on internal chain-of-thought for the complex combined
	// prompt, leaving nothing for visible text (resp.Text() returns "").
	thinkingBudget := storyFullThinkingBudget
	resp, err := g.client.Models.GenerateContent(ctx, g.textModel,
		[]*genai.Content{genai.NewContentFromText(buildStoryPromptFull(entries, g.theme), genai.RoleUser)},
		&genai.GenerateContentConfig{
			SystemInstruction: &genai.Content{
				Parts: []*genai.Part{{Text: storySystemPrompt}},
			},
			MaxOutputTokens: storyFullMaxTokens,
			ThinkingConfig:  &genai.ThinkingConfig{ThinkingBudget: &thinkingBudget},
		},
	)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("gemini API error: %w", err)
	}

	combined := strings.TrimSpace(resp.Text())
	if combined == "" {
		return GenerateResult{}, fmt.Errorf("no content returned from Gemini")
	}

	return parseGenerateResult(combined), nil
}

// parseGenerateResult splits the combined model output on the three separators.
// Format expected:
//
//	<story text>
//	---CHARACTER GUIDE---
//	<bible>
//	---COMIC TITLE---
//	<title>
//	---PANEL SCRIPT---
//	P1-A: ...
//	...
//	P5-D: ...
//
// Missing separators are tolerated — the pipeline continues with empty fields.
func parseGenerateResult(combined string) GenerateResult {
	bibleIdx := strings.Index(combined, storyBibleSeparator)
	if bibleIdx < 0 {
		return GenerateResult{StoryText: strings.TrimSpace(combined)}
	}

	story := strings.TrimSpace(combined[:bibleIdx])
	afterBible := strings.TrimSpace(combined[bibleIdx+len(storyBibleSeparator):])

	titleIdx := strings.Index(afterBible, storyTitleSeparator)
	if titleIdx < 0 {
		return GenerateResult{StoryText: story, Bible: strings.TrimSpace(afterBible)}
	}

	bible := strings.TrimSpace(afterBible[:titleIdx])
	afterTitle := strings.TrimSpace(afterBible[titleIdx+len(storyTitleSeparator):])

	panelIdx := strings.Index(afterTitle, storyPanelSeparator)
	var title, panelText string
	if panelIdx < 0 {
		title = afterTitle
	} else {
		title = afterTitle[:panelIdx]
		panelText = afterTitle[panelIdx+len(storyPanelSeparator):]
	}
	// Keep only the first non-empty line of the title.
	if nl := strings.IndexByte(title, '\n'); nl >= 0 {
		title = title[:nl]
	}
	title = strings.TrimSpace(title)

	return GenerateResult{
		StoryText:   story,
		Bible:       bible,
		Title:       title,
		PanelScript: parsePanelScript(panelText),
	}
}

// parsePanelScript parses the panel script block into a [page][panel] slice.
// Lines must match the format "P{1-5}-{A-D}: description".
// Missing or malformed lines produce empty strings so callers can fall back
// to raw story excerpts for those panels.
func parsePanelScript(text string) [][]string {
	script := make([][]string, storyPagesInScript)
	for i := range script {
		script[i] = make([]string, storyPanelsPerPage)
	}

	panelIndex := map[byte]int{'A': 0, 'B': 1, 'C': 2, 'D': 3}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		// Expected prefix: "P1-A: " through "P5-D: "
		if len(line) < 7 || line[0] != 'P' || line[2] != '-' || line[4] != ':' {
			continue
		}
		page := int(line[1] - '1') // '1'→0 … '5'→4
		panel, ok := panelIndex[line[3]]
		if !ok || page < 0 || page >= storyPagesInScript {
			continue
		}
		script[page][panel] = strings.TrimSpace(line[5:])
	}
	return script
}

// buildStoryPrompt creates the simple story-only prompt used by Generate.
// theme overrides the random genre when non-empty.
func buildStoryPrompt(entries []batch.WordEntry, theme string) string {
	genre := resolveGenre(theme)
	header := fmt.Sprintf(
		"Write a ~250-word story in Bulgarian that naturally uses all of the following words.\n"+
			"The story must be %s — do NOT write a generic fairy tale.\n"+
			"Number each word as shown below. Return ONLY the story text — no title, no header, no explanation.\n\n",
		genre,
	)
	return buildWordList(entries, header)
}

// buildStoryPromptFull creates the extended prompt used by GenerateFull that
// requests both the Bulgarian story and the character bible in one response.
// theme overrides the random genre when non-empty.
// The separator line lets parseGenerateResult split them reliably.
func buildStoryPromptFull(entries []batch.WordEntry, theme string) string {
	genre := resolveGenre(theme)
	var sb strings.Builder
	sb.WriteString("Write a ~250-word story in Bulgarian that naturally uses all of the following words.\n")
	sb.WriteString(fmt.Sprintf("The story must be %s — do NOT write a generic fairy tale.\n", genre))
	sb.WriteString("Number each word as shown below.\n\n")
	sb.WriteString(buildWordList(entries, ""))

	sb.WriteString("\nAfter the story text, write exactly this line by itself (nothing else on that line):\n")
	sb.WriteString(storyBibleSeparator)
	sb.WriteString("\n\nThen write a CHARACTER CONSISTENCY GUIDE in English for an illustrator.\n")
	sb.WriteString("IMPORTANT: all human characters must be adults (18 years or older). ")
	sb.WriteString("Do NOT describe any character as a child, teenager, or minor.\n")
	sb.WriteString("For every named HUMAN character: name, apparent age as a young adult or older ")
	sb.WriteString("(e.g. young adult, adult, middle-aged, elderly), hair (colour + style), eye colour, skin tone, ")
	sb.WriteString("build, and EXACT clothing (garment, colour, pattern, fit). ")
	sb.WriteString("Apparent age and clothing must NOT change — list the same for all appearances.\n")
	sb.WriteString("For every named ANIMAL character: name, species, exact breed, fur colour ")
	sb.WriteString("and pattern, eye colour, size, distinctive markings, body posture. Must ")
	sb.WriteString("look IDENTICAL on every page — same breed, same markings, same eye colour.\n")
	sb.WriteString("Also describe: setting (location, time of day, weather, key props) and ")
	sb.WriteString("overall lighting/colour mood.\n")
	sb.WriteString("Maximum 280 words for the guide. No headers, just dense descriptive prose.\n")

	// Request a short title after the bible for use as the output folder/file name.
	sb.WriteString("\nAfter the character guide, write exactly this line by itself:\n")
	sb.WriteString(storyTitleSeparator)
	sb.WriteString("\n\nThen write a short comic book title in English: 2-4 words that capture the ")
	sb.WriteString("story's theme and characters (e.g. 'Stardust Explorers', 'The Clockwork Dragon', ")
	sb.WriteString("'Mystery at Midnight'). Output only the title — no quotes, no punctuation, no explanation.\n")

	sb.WriteString("\nAfter the title, write exactly this line by itself:\n")
	sb.WriteString(storyPanelSeparator)
	sb.WriteString(buildPanelScriptPrompt())

	return sb.String()
}

// buildPanelScriptPrompt returns the instructions for the 20-panel visual script
// section appended after ---PANEL SCRIPT---. Kept separate so buildStoryPromptFull
// stays under 50 lines.
func buildPanelScriptPrompt() string {
	return "\n\nWrite exactly 20 visual panel descriptions for the comic illustrator, " +
		"one per line, in strict chronological story order.\n" +
		"Format each line exactly as: P{page}-{panel}: {description}\n" +
		"where page is 1–5 and panel is A (top-left), B (top-right), C (bottom-left), D (bottom-right).\n" +
		"Each description is 1–2 sentences covering: WHO is in the panel, WHAT they are doing, " +
		"WHERE they are, their expression or body language, and — REQUIRED for most panels — " +
		"the exact Bulgarian dialogue or thought text the character says/thinks, written in quotes.\n" +
		"At least 3 of the 4 panels on every page MUST include Bulgarian dialogue in a speech bubble " +
		"or thought bubble. The dialogue must come directly from the story and be in Bulgarian Cyrillic.\n" +
		"The 20 panels must retell the story from beginning to end — " +
		"each page covers one story beat, each panel advances the action.\n" +
		"Do NOT repeat the same scene or camera angle on consecutive panels.\n" +
		"Example format (replace with actual story content):\n" +
		"P1-A: Maria walks out her front door into morning sunlight, laptop bag on shoulder, saying \"Най-накрая!\" in a speech bubble.\n" +
		"P1-B: She strides down a busy city street past parked cars, headphones in, thinking \"Толкова хубав ден.\" in a thought bubble.\n" +
		"P1-C: Close-up of her hand gripping a large steaming coffee cup, the barista handing it over saying \"Заповядайте!\" in a speech bubble.\n" +
		"P1-D: Maria pauses at a park entrance, gazing at green trees ahead, saying \"Точно това ми трябваше.\" in a speech bubble.\n"
}

// slugify converts a comic title into a safe directory/file name component.
// It lowercases, replaces whitespace with hyphens, and removes everything that
// is not an ASCII letter, digit, or hyphen. Falls back to "comic" if the result
// would be empty.
func slugify(title string) string {
	title = strings.ToLower(strings.TrimSpace(title))
	var b strings.Builder
	prevHyphen := false
	for _, r := range title {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevHyphen = false
		case r == ' ' || r == '-' || r == '_':
			if !prevHyphen && b.Len() > 0 {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	slug := strings.TrimRight(b.String(), "-")
	if slug == "" {
		return "comic"
	}
	return slug
}

// buildWordList formats the vocabulary entries as a numbered list.
func buildWordList(entries []batch.WordEntry, header string) string {
	var sb strings.Builder
	sb.WriteString(header)
	if header != "" {
		sb.WriteString("Words to include:\n")
	}
	for i, e := range entries {
		if e.Translation != "" {
			sb.WriteString(fmt.Sprintf("%d. %s (%s)\n", i+1, e.Bulgarian, e.Translation))
		} else {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, e.Bulgarian))
		}
	}
	return sb.String()
}
