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
