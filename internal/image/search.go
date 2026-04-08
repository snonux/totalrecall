package image

import (
	"context"
	"io"
)

// SearchResult represents a single image search result
type SearchResult struct {
	ID           string // Unique identifier
	URL          string // Direct URL to the image
	ThumbnailURL string // URL to thumbnail version
	Width        int    // Image width in pixels
	Height       int    // Image height in pixels
	Description  string // Image description or tags
	Attribution  string // Attribution text if required
	Source       string // Source provider (e.g., "pixabay", "unsplash")
}

// SearchOptions configures the image search
type SearchOptions struct {
	Query        string // Search query (Bulgarian word)
	Translation  string // English translation (if already available)
	Language     string // Language code (default: "bg")
	SafeSearch   bool   // Enable safe search filtering
	PerPage      int    // Number of results per page
	Page         int    // Page number (1-based)
	ImageType    string // Type: "photo", "illustration", "vector", "all"
	Orientation  string // Orientation: "horizontal", "vertical", "all"
	CustomPrompt string // Custom prompt for AI image generation
	AspectRatio  string // Override aspect ratio (e.g. "9:16"); empty = provider default
	// ReferenceImages holds raw PNG bytes of previously generated images.
	// When non-empty, the NanoBanana client sends them as multimodal content
	// alongside the text prompt so the model can match character appearance
	// across pages (iterative chaining technique).
	ReferenceImages [][]byte
}

// DefaultSearchOptions returns sensible defaults for Bulgarian word searches
func DefaultSearchOptions(query string) *SearchOptions {
	return &SearchOptions{
		Query:       query,
		Language:    "bg",
		SafeSearch:  true,
		PerPage:     10,
		Page:        1,
		ImageType:   "photo",
		Orientation: "all",
	}
}

// AttributionProvider returns required attribution text for a search result.
// Kept separate from ImageSearcher so callers that only need attribution
// do not depend on Search/Download/Name.
type AttributionProvider interface {
	GetAttribution(result *SearchResult) string
}

// ImageSearcher defines the interface for image search providers.
// Providers that also carry attribution text implement AttributionProvider
// in addition to this interface.
type ImageSearcher interface {
	// Search performs an image search with the given options.
	Search(ctx context.Context, opts *SearchOptions) ([]SearchResult, error)

	// Download downloads an image from the given URL.
	Download(ctx context.Context, url string) (io.ReadCloser, error)

	// Name returns the name of the search provider.
	Name() string
}

// ImageClient combines ImageSearcher and AttributionProvider for callers
// that need full provider capabilities (search, download, attribution).
type ImageClient interface {
	ImageSearcher
	AttributionProvider
}

// Image-generation provider names (AI backends). Use these keys when
// registering GUI/processor image client factories so string literals are not
// scattered across packages.
const (
	ImageProviderOpenAI     = "openai"
	ImageProviderNanoBanana = "nanobanana"
)

// PromptAwareClient extends ImageClient with a callback for receiving the
// generated image prompt before the actual image download begins. Both
// OpenAIClient and NanoBananaClient implement this interface. It is the
// preferred return type for factory functions so callers (processor, gui) can
// register a prompt callback without a type-assertion.
type PromptAwareClient interface {
	ImageClient
	// SetPromptCallback registers a function that is called with the generated
	// prompt text before the image download begins.
	SetPromptCallback(func(prompt string))
}

// OpenAIClientFactory is the canonical type for functions that construct an
// OpenAI image client from config. Using a named type avoids duplicating the
// raw function signature in every package that needs to inject or replace the
// factory (processor, gui).
type OpenAIClientFactory func(*OpenAIConfig) PromptAwareClient

// NanoBananaClientFactory is the canonical type for functions that construct a
// NanoBanana (Gemini) image client from config. Using a named type avoids
// duplicating the raw function signature in every package that needs to inject
// or replace the factory (processor, gui).
type NanoBananaClientFactory func(*NanoBananaConfig) PromptAwareClient

// ClientFactories groups the two image-provider construction functions.
// Embedding or holding a ClientFactories value is the single source of truth
// for the image-factory test seams; packages no longer redeclare the same
// fields independently. Audio factory injection is kept separate (audio.ProviderFactory)
// to avoid an import cycle between the image and audio packages.
type ClientFactories struct {
	// NewOpenAIClient constructs a PromptAwareClient from an OpenAI config.
	// Production code uses the real constructor; tests replace it with a fake.
	NewOpenAIClient OpenAIClientFactory

	// NewNanoBananaClient constructs a PromptAwareClient from a NanoBanana config.
	// Production code uses the real constructor; tests replace it with a fake.
	NewNanoBananaClient NanoBananaClientFactory
}

// DefaultClientFactories returns a ClientFactories wired to the real production
// constructors. Callers that need test doubles replace individual fields before
// passing the value to a constructor.
func DefaultClientFactories() ClientFactories {
	return ClientFactories{
		NewOpenAIClient: func(c *OpenAIConfig) PromptAwareClient {
			// NewOpenAIClient returns *OpenAIClient which implements PromptAwareClient.
			return NewOpenAIClient(c)
		},
		NewNanoBananaClient: func(c *NanoBananaConfig) PromptAwareClient {
			// NewNanoBananaClient returns *NanoBananaClient which implements PromptAwareClient.
			return NewNanoBananaClient(c)
		},
	}
}

// SearchError represents an error from an image search provider
type SearchError struct {
	Provider string
	Code     string
	Message  string
}

func (e *SearchError) Error() string {
	return e.Provider + ": " + e.Message
}

// RateLimitError indicates that the API rate limit has been exceeded
type RateLimitError struct {
	Provider     string
	RetryAfter   int // Seconds to wait before retry
	LimitPerHour int
	LimitPerDay  int
}

func (e *RateLimitError) Error() string {
	return e.Provider + ": rate limit exceeded"
}
