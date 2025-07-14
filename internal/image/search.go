package image

import (
	"context"
	"io"
)

// SearchResult represents a single image search result
type SearchResult struct {
	ID           string   // Unique identifier
	URL          string   // Direct URL to the image
	ThumbnailURL string   // URL to thumbnail version
	Width        int      // Image width in pixels
	Height       int      // Image height in pixels
	Description  string   // Image description or tags
	Attribution  string   // Attribution text if required
	Source       string   // Source provider (e.g., "pixabay", "unsplash")
}

// SearchOptions configures the image search
type SearchOptions struct {
	Query       string   // Search query (Bulgarian word)
	Language    string   // Language code (default: "bg")
	SafeSearch  bool     // Enable safe search filtering
	PerPage     int      // Number of results per page
	Page        int      // Page number (1-based)
	ImageType   string   // Type: "photo", "illustration", "vector", "all"
	Orientation string   // Orientation: "horizontal", "vertical", "all"
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

// ImageSearcher defines the interface for image search providers
type ImageSearcher interface {
	// Search performs an image search with the given options
	Search(ctx context.Context, opts *SearchOptions) ([]SearchResult, error)
	
	// Download downloads an image from the given URL
	Download(ctx context.Context, url string) (io.ReadCloser, error)
	
	// GetAttribution returns the required attribution text for an image
	GetAttribution(result *SearchResult) string
	
	// Name returns the name of the search provider
	Name() string
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
	Provider      string
	RetryAfter    int // Seconds to wait before retry
	LimitPerHour  int
	LimitPerDay   int
}

func (e *RateLimitError) Error() string {
	return e.Provider + ": rate limit exceeded"
}

// DownloadImage is a utility function to download an image to a file
func DownloadImage(ctx context.Context, searcher ImageSearcher, url string, outputPath string) error {
	// Implementation will be in a separate download.go file
	// This is just the interface definition
	return nil
}