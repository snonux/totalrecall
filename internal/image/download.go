package image

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// DownloadOptions configures image download behavior
type DownloadOptions struct {
	OutputDir       string   // Directory to save images
	OverwriteExisting bool   // Whether to overwrite existing files
	CreateDir       bool     // Create output directory if it doesn't exist
	FileNamePattern string   // Pattern for file naming (e.g., "{word}_{source}")
	MaxSizeBytes    int64    // Maximum file size to download (0 = no limit)
}

// DefaultDownloadOptions returns sensible defaults for image downloads
func DefaultDownloadOptions() *DownloadOptions {
	return &DownloadOptions{
		OutputDir:       "./images",
		OverwriteExisting: false,
		CreateDir:       true,
		FileNamePattern: "{word}_{source}",
		MaxSizeBytes:    10 * 1024 * 1024, // 10MB
	}
}

// Downloader handles image downloads from search results
type Downloader struct {
	searcher ImageSearcher
	options  *DownloadOptions
}

// NewDownloader creates a new image downloader
func NewDownloader(searcher ImageSearcher, options *DownloadOptions) *Downloader {
	if options == nil {
		options = DefaultDownloadOptions()
	}
	return &Downloader{
		searcher: searcher,
		options:  options,
	}
}

// DownloadImage downloads a single image to the specified path
func (d *Downloader) DownloadImage(ctx context.Context, result *SearchResult, outputPath string) error {
	// Ensure directory exists
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}
	
	// Check if file already exists
	if !d.options.OverwriteExisting {
		if _, err := os.Stat(outputPath); err == nil {
			return fmt.Errorf("file already exists: %s", outputPath)
		}
	}
	
	// Download the image
	reader, err := d.searcher.Download(ctx, result.URL)
	if err != nil {
		return fmt.Errorf("failed to download image: %w", err)
	}
	defer reader.Close()
	
	// Create output file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()
	
	// Copy with size limit if specified
	var written int64
	if d.options.MaxSizeBytes > 0 {
		written, err = io.CopyN(file, reader, d.options.MaxSizeBytes)
		if err != nil && err != io.EOF {
			os.Remove(outputPath) // Clean up on error
			return fmt.Errorf("failed to write file: %w", err)
		}
		
		// Check if we hit the size limit
		if written == d.options.MaxSizeBytes {
			// Try to read one more byte to see if file is larger
			if _, err := reader.Read(make([]byte, 1)); err != io.EOF {
				os.Remove(outputPath) // Clean up
				return fmt.Errorf("image exceeds maximum size of %d bytes", d.options.MaxSizeBytes)
			}
		}
	} else {
		written, err = io.Copy(file, reader)
		if err != nil {
			os.Remove(outputPath) // Clean up on error
			return fmt.Errorf("failed to write file: %w", err)
		}
	}
	
	// Save attribution if required
	if attribution := d.searcher.GetAttribution(result); attribution != "" {
		attrPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + "_attribution.txt"
		if err := os.WriteFile(attrPath, []byte(attribution), 0644); err != nil {
			// Non-fatal error - log but don't fail the download
			fmt.Fprintf(os.Stderr, "Warning: failed to save attribution: %v\n", err)
		}
	}
	
	return nil
}

// DownloadBestMatch downloads the best matching image for a query
func (d *Downloader) DownloadBestMatch(ctx context.Context, query string) (*SearchResult, string, error) {
	// Search for images
	opts := DefaultSearchOptions(query)
	opts.PerPage = 5 // Get top 5 results
	
	results, err := d.searcher.Search(ctx, opts)
	if err != nil {
		return nil, "", fmt.Errorf("search failed: %w", err)
	}
	
	if len(results) == 0 {
		return nil, "", fmt.Errorf("no images found for query: %s", query)
	}
	
	// Try to download the first available image
	for i, result := range results {
		// Generate filename
		filename := d.generateFileName(query, &result, i)
		outputPath := filepath.Join(d.options.OutputDir, filename)
		
		// Try to download
		err := d.DownloadImage(ctx, &result, outputPath)
		if err == nil {
			return &result, outputPath, nil
		}
		
		// Log error and try next
		fmt.Fprintf(os.Stderr, "Warning: failed to download image %d: %v\n", i+1, err)
	}
	
	return nil, "", fmt.Errorf("failed to download any images for query: %s", query)
}

// generateFileName creates a filename based on the pattern
func (d *Downloader) generateFileName(word string, result *SearchResult, index int) string {
	// Start with the pattern
	filename := d.options.FileNamePattern
	
	// Replace placeholders
	filename = strings.ReplaceAll(filename, "{word}", sanitizeFileName(word))
	filename = strings.ReplaceAll(filename, "{source}", result.Source)
	filename = strings.ReplaceAll(filename, "{id}", result.ID)
	filename = strings.ReplaceAll(filename, "{index}", fmt.Sprintf("%d", index))
	
	// Determine extension from URL
	ext := filepath.Ext(result.URL)
	if ext == "" || len(ext) > 5 { // Probably not a real extension
		ext = ".jpg" // Default to jpg
	}
	
	// Add extension if not present
	if filepath.Ext(filename) == "" {
		filename += ext
	}
	
	return filename
}

// sanitizeFileName removes or replaces characters that are problematic in filenames
func sanitizeFileName(name string) string {
	// Replace common problematic characters
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		" ", "_",
		".", "_",
	)
	
	sanitized := replacer.Replace(name)
	
	// Ensure the filename is not too long
	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
	}
	
	return sanitized
}


// DownloadBestMatchWithOptions downloads the best matching image for given search options
func (d *Downloader) DownloadBestMatchWithOptions(ctx context.Context, opts *SearchOptions) (*SearchResult, string, error) {
	// Search for images
	searchOpts := *opts // Copy to avoid modifying original
	searchOpts.PerPage = 5 // Get top 5 results
	
	results, err := d.searcher.Search(ctx, &searchOpts)
	if err != nil {
		return nil, "", fmt.Errorf("search failed: %w", err)
	}
	
	if len(results) == 0 {
		return nil, "", fmt.Errorf("no images found for query: %s", opts.Query)
	}
	
	// Try to download the first available image
	for i, result := range results {
		// Generate filename
		filename := d.generateFileName(opts.Query, &result, i)
		outputPath := filepath.Join(d.options.OutputDir, filename)
		
		// Try to download
		err := d.DownloadImage(ctx, &result, outputPath)
		if err == nil {
			return &result, outputPath, nil
		}
		
		// Log error and try next
		fmt.Fprintf(os.Stderr, "Warning: failed to download image %d: %v\n", i+1, err)
	}
	
	return nil, "", fmt.Errorf("failed to download any images for query: %s", opts.Query)
}

