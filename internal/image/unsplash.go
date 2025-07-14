package image

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	unsplashAPIURL = "https://api.unsplash.com"
	unsplashTimeout = 30 * time.Second
)

// UnsplashClient implements ImageSearcher for Unsplash API
type UnsplashClient struct {
	accessKey   string
	httpClient  *http.Client
	rateLimit   *rateLimiter
}

// unsplashSearchResponse represents the search API response
type unsplashSearchResponse struct {
	Total      int             `json:"total"`
	TotalPages int             `json:"total_pages"`
	Results    []unsplashPhoto `json:"results"`
}

// unsplashPhoto represents a photo in the response
type unsplashPhoto struct {
	ID          string              `json:"id"`
	CreatedAt   string              `json:"created_at"`
	UpdatedAt   string              `json:"updated_at"`
	Width       int                 `json:"width"`
	Height      int                 `json:"height"`
	Color       string              `json:"color"`
	BlurHash    string              `json:"blur_hash"`
	Description string              `json:"description"`
	AltDesc     string              `json:"alt_description"`
	URLs        unsplashPhotoURLs   `json:"urls"`
	Links       unsplashPhotoLinks  `json:"links"`
	User        unsplashUser        `json:"user"`
}

// unsplashPhotoURLs contains various size URLs
type unsplashPhotoURLs struct {
	Raw     string `json:"raw"`
	Full    string `json:"full"`
	Regular string `json:"regular"`
	Small   string `json:"small"`
	Thumb   string `json:"thumb"`
}

// unsplashPhotoLinks contains photo-related links
type unsplashPhotoLinks struct {
	Self     string `json:"self"`
	HTML     string `json:"html"`
	Download string `json:"download"`
}

// unsplashUser represents the photo author
type unsplashUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

// NewUnsplashClient creates a new Unsplash API client
func NewUnsplashClient(accessKey string) (*UnsplashClient, error) {
	if accessKey == "" {
		return nil, fmt.Errorf("Unsplash access key is required")
	}
	
	return &UnsplashClient{
		accessKey: accessKey,
		httpClient: &http.Client{
			Timeout: unsplashTimeout,
		},
		rateLimit: newRateLimiter(50), // 50 requests per hour
	}, nil
}

// Search performs an image search on Unsplash
func (u *UnsplashClient) Search(ctx context.Context, opts *SearchOptions) ([]SearchResult, error) {
	// Apply rate limiting (50 per hour = ~0.83 per minute)
	u.rateLimit.wait()
	
	// Build query parameters
	params := url.Values{}
	params.Set("query", opts.Query)
	params.Set("per_page", fmt.Sprintf("%d", opts.PerPage))
	params.Set("page", fmt.Sprintf("%d", opts.Page))
	
	if opts.Orientation != "all" && opts.Orientation != "" {
		params.Set("orientation", mapOrientation(opts.Orientation))
	}
	
	// Make request
	reqURL := unsplashAPIURL + "/search/photos?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Add authorization header
	req.Header.Set("Authorization", "Client-ID "+u.accessKey)
	req.Header.Set("Accept-Version", "v1")
	
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	// Check status code
	if resp.StatusCode == http.StatusTooManyRequests {
		// Try to parse rate limit headers
		retryAfter := 3600 // Default to 1 hour
		if retryStr := resp.Header.Get("X-Ratelimit-Reset"); retryStr != "" {
			// Parse Unix timestamp and calculate seconds until reset
			// Implementation simplified for brevity
			retryAfter = 3600
		}
		
		return nil, &RateLimitError{
			Provider:     "unsplash",
			RetryAfter:   retryAfter,
			LimitPerHour: 50,
		}
	}
	
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, &SearchError{
			Provider: "unsplash",
			Code:     "401",
			Message:  "Invalid access key",
		}
	}
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &SearchError{
			Provider: "unsplash",
			Code:     fmt.Sprintf("%d", resp.StatusCode),
			Message:  string(body),
		}
	}
	
	// Parse response
	var searchResp unsplashSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	// Convert to SearchResult
	results := make([]SearchResult, 0, len(searchResp.Results))
	for _, photo := range searchResp.Results {
		description := photo.Description
		if description == "" {
			description = photo.AltDesc
		}
		
		results = append(results, SearchResult{
			ID:           photo.ID,
			URL:          photo.URLs.Regular,
			ThumbnailURL: photo.URLs.Thumb,
			Width:        photo.Width,
			Height:       photo.Height,
			Description:  description,
			Attribution:  u.formatAttribution(&photo),
			Source:       "unsplash",
		})
	}
	
	// Trigger download tracking as per Unsplash guidelines
	go u.trackDownloads(searchResp.Results)
	
	return results, nil
}

// Download downloads an image from the given URL
func (u *UnsplashClient) Download(ctx context.Context, imageURL string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}
	
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}
	
	return resp.Body, nil
}

// GetAttribution returns the required attribution text for an image
func (u *UnsplashClient) GetAttribution(result *SearchResult) string {
	// Unsplash always requires attribution
	return result.Attribution
}

// Name returns the name of the search provider
func (u *UnsplashClient) Name() string {
	return "unsplash"
}

// formatAttribution creates the proper attribution string as per Unsplash guidelines
func (u *UnsplashClient) formatAttribution(photo *unsplashPhoto) string {
	return fmt.Sprintf("Photo by %s on Unsplash", photo.User.Name)
}

// mapOrientation maps our orientation values to Unsplash API values
func mapOrientation(orientation string) string {
	switch orientation {
	case "horizontal":
		return "landscape"
	case "vertical":
		return "portrait"
	default:
		return ""
	}
}

// trackDownloads triggers download events as required by Unsplash API guidelines
func (u *UnsplashClient) trackDownloads(photos []unsplashPhoto) {
	// Unsplash requires triggering their download endpoint when images are used
	// This is done asynchronously to not block the search
	for _, photo := range photos {
		go func(downloadURL string) {
			req, _ := http.NewRequest("GET", downloadURL, nil)
			req.Header.Set("Authorization", "Client-ID "+u.accessKey)
			u.httpClient.Do(req)
		}(photo.Links.Download)
	}
}

// SearchWithTranslation performs a search with automatic translation
// Unsplash has better international support, so we'll try both queries
func (u *UnsplashClient) SearchWithTranslation(ctx context.Context, opts *SearchOptions) ([]SearchResult, error) {
	// First try with original Bulgarian query
	results, err := u.Search(ctx, opts)
	if err == nil && len(results) > 0 {
		return results, nil
	}
	
	// If no results, try with translated query
	translatedQuery := translateBulgarianQuery(opts.Query)
	if translatedQuery != opts.Query {
		translatedOpts := *opts
		translatedOpts.Query = translatedQuery
		return u.Search(ctx, &translatedOpts)
	}
	
	return results, err
}