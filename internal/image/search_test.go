package image

import (
	"context"
	"io"
	"strings"
	"testing"
)

// mockSearcher implements ImageSearcher for testing
type mockSearcher struct {
	name          string
	searchResults []SearchResult
	searchErr     error
	downloadErr   error
}

func (m *mockSearcher) Search(ctx context.Context, opts *SearchOptions) ([]SearchResult, error) {
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	return m.searchResults, nil
}

func (m *mockSearcher) Download(ctx context.Context, url string) (io.ReadCloser, error) {
	if m.downloadErr != nil {
		return nil, m.downloadErr
	}
	return io.NopCloser(strings.NewReader("mock image data")), nil
}

func (m *mockSearcher) GetAttribution(result *SearchResult) string {
	return result.Attribution
}

func (m *mockSearcher) Name() string {
	return m.name
}

func TestDefaultSearchOptions(t *testing.T) {
	opts := DefaultSearchOptions("ябълка")
	
	if opts.Query != "ябълка" {
		t.Errorf("Expected query 'ябълка', got '%s'", opts.Query)
	}
	
	if opts.Language != "bg" {
		t.Errorf("Expected language 'bg', got '%s'", opts.Language)
	}
	
	if !opts.SafeSearch {
		t.Error("Expected SafeSearch to be true")
	}
	
	if opts.PerPage != 10 {
		t.Errorf("Expected PerPage 10, got %d", opts.PerPage)
	}
	
	if opts.Page != 1 {
		t.Errorf("Expected Page 1, got %d", opts.Page)
	}
	
	if opts.ImageType != "photo" {
		t.Errorf("Expected ImageType 'photo', got '%s'", opts.ImageType)
	}
}

func TestSearchError(t *testing.T) {
	err := &SearchError{
		Provider: "test",
		Code:     "404",
		Message:  "Not found",
	}
	
	expected := "test: Not found"
	if err.Error() != expected {
		t.Errorf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

func TestRateLimitError(t *testing.T) {
	err := &RateLimitError{
		Provider:     "test",
		RetryAfter:   60,
		LimitPerHour: 100,
	}
	
	expected := "test: rate limit exceeded"
	if err.Error() != expected {
		t.Errorf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

func TestMockSearcher(t *testing.T) {
	mockResults := []SearchResult{
		{
			ID:          "1",
			URL:         "https://example.com/image1.jpg",
			Width:       800,
			Height:      600,
			Description: "Test image",
			Source:      "mock",
		},
	}
	
	searcher := &mockSearcher{
		name:          "mock",
		searchResults: mockResults,
	}
	
	ctx := context.Background()
	opts := DefaultSearchOptions("test")
	
	results, err := searcher.Search(ctx, opts)
	if err != nil {
		t.Fatalf("Search() failed: %v", err)
	}
	
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	
	if results[0].ID != "1" {
		t.Errorf("Expected ID '1', got '%s'", results[0].ID)
	}
}

func TestDownloadOptions(t *testing.T) {
	opts := DefaultDownloadOptions()
	
	if opts.OutputDir != "./images" {
		t.Errorf("Expected output dir './images', got '%s'", opts.OutputDir)
	}
	
	if opts.OverwriteExisting {
		t.Error("Expected OverwriteExisting to be false")
	}
	
	if !opts.CreateDir {
		t.Error("Expected CreateDir to be true")
	}
	
	if opts.MaxSizeBytes != 10*1024*1024 {
		t.Errorf("Expected MaxSizeBytes 10MB, got %d", opts.MaxSizeBytes)
	}
}