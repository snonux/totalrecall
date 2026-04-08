// Package httpctx provides HTTP client defaults and context helpers for
// outbound API calls. go-openai uses http.Client{} with zero timeout by
// default; google.golang.org/genai can also run without an explicit client
// deadline. This package sets consistent per-request HTTP timeouts and
// optional operation-level context deadlines when callers pass Background.
package httpctx

import (
	"context"
	"net/http"
	"time"

	"github.com/sashabaranov/go-openai"
	"google.golang.org/genai"
)

const (
	// OpenAIHTTPTimeout bounds each go-openai HTTP round-trip (TTS, chat,
	// images). Without this, the default client has no timeout.
	OpenAIHTTPTimeout = 15 * time.Minute

	// GenAIHTTPTimeout bounds each Google GenAI SDK HTTP request (Gemini text,
	// image, TTS, Veo polling, file download).
	GenAIHTTPTimeout = 30 * time.Minute

	// ImageDownloadTimeout limits fetches of remote image URLs (e.g. DALL-E
	// temporary URLs, HTTP image links).
	ImageDownloadTimeout = 60 * time.Second

	// OperationTimeoutDefault caps a full high-level operation (e.g. one image
	// Search including scene + generation) when the caller did not set a deadline.
	OperationTimeoutDefault = 15 * time.Minute

	// ListModelsTimeout bounds model-listing CLI calls.
	ListModelsTimeout = 3 * time.Minute

	// StoryPageImageTimeout bounds a single comic page image pipeline (search +
	// download) when no parent deadline exists.
	StoryPageImageTimeout = 25 * time.Minute

	// VeoCLIPerVideoTimeout bounds one gallery-to-MP4 Veo run (start + poll +
	// download) when the CLI passes Background.
	VeoCLIPerVideoTimeout = 25 * time.Minute

	// SingleWordProcessTimeout caps ProcessWordWithTranslation when the CLI
	// uses an unbounded context (batch processing already applies per-word
	// timeouts elsewhere).
	SingleWordProcessTimeout = 10 * time.Minute
)

// OpenAIHTTPClient returns an http.Client for go-openai DefaultConfig.
func OpenAIHTTPClient() *http.Client {
	return &http.Client{Timeout: OpenAIHTTPTimeout}
}

// GenAIHTTPClient returns an http.Client for google.golang.org/genai.
func GenAIHTTPClient() *http.Client {
	return &http.Client{Timeout: GenAIHTTPTimeout}
}

// ImageDownloadHTTPClient returns a client for generic image URL downloads.
func ImageDownloadHTTPClient() *http.Client {
	return &http.Client{Timeout: ImageDownloadTimeout}
}

// NewOpenAIClient creates a go-openai client whose HTTP transport has a deadline.
func NewOpenAIClient(token string) *openai.Client {
	cfg := openai.DefaultConfig(token)
	cfg.HTTPClient = OpenAIHTTPClient()
	return openai.NewClientWithConfig(cfg)
}

// NewGenAIClient wraps genai.NewClient, setting HTTPClient when the config does
// not supply one so outbound requests never rely on an unbounded default.
func NewGenAIClient(ctx context.Context, cfg *genai.ClientConfig) (*genai.Client, error) {
	if cfg == nil {
		cfg = &genai.ClientConfig{}
	}
	merged := *cfg
	if merged.HTTPClient == nil {
		merged.HTTPClient = GenAIHTTPClient()
	}
	return genai.NewClient(ctx, &merged)
}

// WithTimeoutUnlessSet returns a child context with timeout d when ctx has no
// deadline. If ctx already has a deadline, it returns ctx and a no-op cancel.
func WithTimeoutUnlessSet(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}
