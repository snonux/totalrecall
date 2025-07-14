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
	pixabayAPIURL = "https://pixabay.com/api/"
	pixabayTimeout = 30 * time.Second
)

// PixabayClient implements ImageSearcher for Pixabay API
type PixabayClient struct {
	apiKey     string
	httpClient *http.Client
	rateLimit  *rateLimiter
}

// pixabayResponse represents the API response structure
type pixabayResponse struct {
	Total     int            `json:"total"`
	TotalHits int            `json:"totalHits"`
	Hits      []pixabayImage `json:"hits"`
}

// pixabayImage represents a single image in the response
type pixabayImage struct {
	ID               int    `json:"id"`
	PageURL          string `json:"pageURL"`
	Type             string `json:"type"`
	Tags             string `json:"tags"`
	PreviewURL       string `json:"previewURL"`
	PreviewWidth     int    `json:"previewWidth"`
	PreviewHeight    int    `json:"previewHeight"`
	WebformatURL     string `json:"webformatURL"`
	WebformatWidth   int    `json:"webformatWidth"`
	WebformatHeight  int    `json:"webformatHeight"`
	LargeImageURL    string `json:"largeImageURL"`
	ImageWidth       int    `json:"imageWidth"`
	ImageHeight      int    `json:"imageHeight"`
	Views            int    `json:"views"`
	Downloads        int    `json:"downloads"`
	Collections      int    `json:"collections"`
	Likes            int    `json:"likes"`
	Comments         int    `json:"comments"`
	UserID           int    `json:"user_id"`
	User             string `json:"user"`
	UserImageURL     string `json:"userImageURL"`
}

// rateLimiter implements simple rate limiting
type rateLimiter struct {
	requestsPerMinute int
	requests         []time.Time
}

func newRateLimiter(rpm int) *rateLimiter {
	return &rateLimiter{
		requestsPerMinute: rpm,
		requests:         make([]time.Time, 0, rpm),
	}
}

func (rl *rateLimiter) wait() {
	now := time.Now()
	
	// Remove requests older than 1 minute
	cutoff := now.Add(-1 * time.Minute)
	i := 0
	for i < len(rl.requests) && rl.requests[i].Before(cutoff) {
		i++
	}
	rl.requests = rl.requests[i:]
	
	// If we're at the limit, wait
	if len(rl.requests) >= rl.requestsPerMinute {
		oldestRequest := rl.requests[0]
		waitDuration := oldestRequest.Add(1 * time.Minute).Sub(now)
		if waitDuration > 0 {
			time.Sleep(waitDuration)
		}
	}
	
	// Record this request
	rl.requests = append(rl.requests, now)
}

// NewPixabayClient creates a new Pixabay API client
func NewPixabayClient(apiKey string) *PixabayClient {
	return &PixabayClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: pixabayTimeout,
		},
		rateLimit: newRateLimiter(100), // 100 requests per minute
	}
}

// Search performs an image search on Pixabay
func (p *PixabayClient) Search(ctx context.Context, opts *SearchOptions) ([]SearchResult, error) {
	// Apply rate limiting
	p.rateLimit.wait()
	
	// Build query parameters
	params := url.Values{}
	if p.apiKey != "" {
		params.Set("key", p.apiKey)
	}
	params.Set("q", opts.Query)
	params.Set("lang", opts.Language)
	params.Set("image_type", opts.ImageType)
	params.Set("safesearch", fmt.Sprintf("%t", opts.SafeSearch))
	params.Set("per_page", fmt.Sprintf("%d", opts.PerPage))
	params.Set("page", fmt.Sprintf("%d", opts.Page))
	
	if opts.Orientation != "all" && opts.Orientation != "" {
		params.Set("orientation", opts.Orientation)
	}
	
	// Make request
	reqURL := pixabayAPIURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	// Check status code
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, &RateLimitError{
			Provider:     "pixabay",
			RetryAfter:   60,
			LimitPerHour: 5000,
		}
	}
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &SearchError{
			Provider: "pixabay",
			Code:     fmt.Sprintf("%d", resp.StatusCode),
			Message:  string(body),
		}
	}
	
	// Parse response
	var pixResp pixabayResponse
	if err := json.NewDecoder(resp.Body).Decode(&pixResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	// Convert to SearchResult
	results := make([]SearchResult, 0, len(pixResp.Hits))
	for _, hit := range pixResp.Hits {
		results = append(results, SearchResult{
			ID:           fmt.Sprintf("%d", hit.ID),
			URL:          hit.WebformatURL,
			ThumbnailURL: hit.PreviewURL,
			Width:        hit.WebformatWidth,
			Height:       hit.WebformatHeight,
			Description:  hit.Tags,
			Attribution:  fmt.Sprintf("Image by %s from Pixabay", hit.User),
			Source:       "pixabay",
		})
	}
	
	return results, nil
}

// Download downloads an image from the given URL
func (p *PixabayClient) Download(ctx context.Context, imageURL string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}
	
	resp, err := p.httpClient.Do(req)
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
func (p *PixabayClient) GetAttribution(result *SearchResult) string {
	if p.apiKey == "" {
		// Without API key, attribution is required
		return result.Attribution
	}
	// With API key, attribution is optional but recommended
	return ""
}

// Name returns the name of the search provider
func (p *PixabayClient) Name() string {
	return "pixabay"
}


// SearchWithTranslation performs a search with automatic translation
func (p *PixabayClient) SearchWithTranslation(ctx context.Context, opts *SearchOptions) ([]SearchResult, error) {
	// Try with translated query first
	translatedQuery := translateBulgarianQuery(opts.Query)
	translatedOpts := *opts
	translatedOpts.Query = translatedQuery
	
	results, err := p.Search(ctx, &translatedOpts)
	if err != nil || len(results) == 0 {
		// Fall back to original query
		return p.Search(ctx, opts)
	}
	
	return results, nil
}