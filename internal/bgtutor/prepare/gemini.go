package prepare

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/genai"

	"github.com/snonux/totalrecall/internal/httpctx"
)

// DefaultGeminiModel is used for preparation. Translation and teaching notes
// are done once per episode, offline, so the stronger model is worth it.
const DefaultGeminiModel = "gemini-2.5-pro"

// chunkTimeout bounds one LLM call; a chunk is ~1200 words in and a few
// thousand tokens out.
const chunkTimeout = 5 * time.Minute

// GeminiGenerator implements Generator with Google Gemini, the same provider
// the rest of totalrecall uses by default (GOOGLE_API_KEY).
type GeminiGenerator struct {
	client *genai.Client
	model  string
}

// NewGeminiGenerator creates a Gemini-backed generator.
func NewGeminiGenerator(ctx context.Context, apiKey, model string) (*GeminiGenerator, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("GOOGLE_API_KEY is not set")
	}
	if model == "" {
		model = DefaultGeminiModel
	}
	client, err := httpctx.NewGenAIClient(ctx, &genai.ClientConfig{APIKey: apiKey, Backend: genai.BackendGeminiAPI})
	if err != nil {
		return nil, fmt.Errorf("create gemini client: %w", err)
	}
	return &GeminiGenerator{client: client, model: model}, nil
}

// Name returns the model id.
func (g *GeminiGenerator) Name() string { return g.model }

// GenerateJSON asks Gemini for a JSON response constrained by schema.
func (g *GeminiGenerator) GenerateJSON(ctx context.Context, system, user string, schema map[string]any) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, chunkTimeout)
	defer cancel()
	temp := float32(0.3)
	resp, err := g.client.Models.GenerateContent(ctx, g.model,
		[]*genai.Content{genai.NewContentFromText(user, genai.RoleUser)},
		&genai.GenerateContentConfig{
			SystemInstruction:  genai.NewContentFromText(system, genai.RoleUser),
			Temperature:        &temp,
			ResponseMIMEType:   "application/json",
			ResponseJsonSchema: schema,
		})
	if err != nil {
		return "", fmt.Errorf("gemini API error: %w", err)
	}
	text := strings.TrimSpace(resp.Text())
	if text == "" {
		return "", fmt.Errorf("gemini returned an empty response")
	}
	return text, nil
}
