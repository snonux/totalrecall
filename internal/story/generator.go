package story

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/genai"

	"codeberg.org/snonux/totalrecall/internal/batch"
)

const (
	storyGeminiModel  = "gemini-2.5-flash"
	storyTimeout      = 120 * time.Second
	// 8192 tokens gives plenty of room for both Gemini 2.5 Flash's internal
	// thinking tokens and the ~650 visible tokens of a 500-word Bulgarian story.
	// A small budget (e.g. 1024) is silently consumed by thinking before any
	// visible text is emitted, producing a truncated result.
	storyMaxTokens = int32(8192)
	storySystemPrompt = "You are a creative Bulgarian language teacher. Write engaging stories that naturally incorporate vocabulary words to help students learn."
)

// Config holds generator settings and API credentials.
type Config struct {
	APIKey    string
	TextModel string // defaults to storyGeminiModel
}

// Generator uses Gemini to produce vocabulary-based stories.
type Generator struct {
	client    *genai.Client
	initErr   error
	textModel string
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

	prompt := buildStoryPrompt(entries)
	return generateStoryText(ctx, g.client, g.textModel, prompt)
}

// buildStoryPrompt creates the Gemini prompt from the word list.
func buildStoryPrompt(entries []batch.WordEntry) string {
	var sb strings.Builder
	sb.WriteString("Write a ~500-word story in Bulgarian that naturally uses all of the following words.\n")
	sb.WriteString("Number each word as shown below. Return ONLY the story text — no title, no header, no explanation.\n\n")
	sb.WriteString("Words to include:\n")

	for i, e := range entries {
		word := e.Bulgarian
		if e.Translation != "" {
			sb.WriteString(fmt.Sprintf("%d. %s (%s)\n", i+1, word, e.Translation))
		} else {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, word))
		}
	}

	return sb.String()
}
