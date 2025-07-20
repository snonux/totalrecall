package audio

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewOpenAIProvider(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "missing API key",
			config: &Config{
				OpenAIKey: "",
			},
			wantErr: true,
			errMsg:  "OpenAI API key is required",
		},
		{
			name: "valid config with cache",
			config: &Config{
				OpenAIKey:   "test-key",
				EnableCache: true,
				CacheDir:    "./test_cache",
			},
			wantErr: false,
		},
		{
			name: "valid config without cache",
			config: &Config{
				OpenAIKey:   "test-key",
				EnableCache: false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewOpenAIProvider(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewOpenAIProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && err.Error() != tt.errMsg {
				t.Errorf("NewOpenAIProvider() error = %v, want %v", err.Error(), tt.errMsg)
			}

			// Cleanup cache dir if created
			if !tt.wantErr && tt.config.EnableCache && tt.config.CacheDir != "" {
				os.RemoveAll(tt.config.CacheDir)
			}

			// Check provider properties
			if !tt.wantErr && provider != nil {
				if provider.Name() != "openai" {
					t.Errorf("Name() = %v, want %v", provider.Name(), "openai")
				}
			}
		})
	}
}

func TestOpenAIProviderIsAvailable(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "with API key",
			config: &Config{
				OpenAIKey: "test-key",
			},
			wantErr: false,
		},
		{
			name: "without API key",
			config: &Config{
				OpenAIKey: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &OpenAIProvider{
				config: tt.config,
			}
			err := provider.IsAvailable()
			if (err != nil) != tt.wantErr {
				t.Errorf("IsAvailable() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPreprocessBulgarianText(t *testing.T) {
	provider := &OpenAIProvider{
		config: &Config{},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple word",
			input:    "ябълка",
			expected: "ябълка",
		},
		{
			name:     "word with punctuation",
			input:    "ябълка!",
			expected: "ябълка",
		},
		{
			name:     "word with multiple punctuation",
			input:    "\"ябълка?\"",
			expected: "ябълка",
		},
		{
			name:     "word with spaces",
			input:    "  ябълка  ",
			expected: "ябълка",
		},
		{
			name:     "word with dashes",
			input:    "ябълка-круша",
			expected: "ябълкакруша",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.preprocessBulgarianText(tt.input)
			if result != tt.expected {
				t.Errorf("preprocessBulgarianText(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetCacheFilePath(t *testing.T) {
	provider := &OpenAIProvider{
		config: &Config{
			OpenAIModel: "tts-1",
			OpenAIVoice: "alloy",
			OpenAISpeed: 1.0,
		},
		cacheDir: "./test_cache",
	}

	// Test basic cache path generation
	path1 := provider.getCacheFilePath("ябълка")
	if !strings.HasPrefix(path1, "test_cache/") {
		t.Errorf("Cache path should start with cache dir, got %s", path1)
	}
	if !strings.HasSuffix(path1, ".mp3") {
		t.Errorf("Cache path should end with .mp3, got %s", path1)
	}

	// Test that same input produces same path
	path2 := provider.getCacheFilePath("ябълка")
	if path1 != path2 {
		t.Errorf("Same input should produce same cache path, got %s and %s", path1, path2)
	}

	// Test that different input produces different path
	path3 := provider.getCacheFilePath("котка")
	if path1 == path3 {
		t.Errorf("Different input should produce different cache path")
	}

	// Test that different settings produce different paths
	provider.config.OpenAIVoice = "nova"
	path4 := provider.getCacheFilePath("ябълка")
	if path1 == path4 {
		t.Errorf("Different voice should produce different cache path")
	}

	// Test with instruction for gpt-4o-mini-tts
	provider.config.OpenAIModel = "gpt-4o-mini-tts"
	provider.config.OpenAIInstruction = "Test instruction"
	path5 := provider.getCacheFilePath("ябълка")

	provider.config.OpenAIInstruction = "Different instruction"
	path6 := provider.getCacheFilePath("ябълка")
	if path5 == path6 {
		t.Errorf("Different instruction should produce different cache path for gpt-4o-mini-tts")
	}
}

func TestCopyFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	provider := &OpenAIProvider{}

	// Create source file
	srcPath := filepath.Join(tempDir, "source.txt")
	srcContent := []byte("test content")
	if err := os.WriteFile(srcPath, srcContent, 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Test copying to new file
	dstPath := filepath.Join(tempDir, "dest.txt")
	err := provider.copyFile(srcPath, dstPath)
	if err != nil {
		t.Errorf("copyFile() error = %v", err)
	}

	// Verify content
	dstContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}
	if string(dstContent) != string(srcContent) {
		t.Errorf("Copied content doesn't match: got %q, want %q", dstContent, srcContent)
	}

	// Test copying to subdirectory
	dstPath2 := filepath.Join(tempDir, "subdir", "dest2.txt")
	err = provider.copyFile(srcPath, dstPath2)
	if err != nil {
		t.Errorf("copyFile() to subdirectory error = %v", err)
	}

	// Test copying non-existent file
	err = provider.copyFile(filepath.Join(tempDir, "nonexistent.txt"), dstPath)
	if err == nil {
		t.Error("copyFile() expected error for non-existent source")
	}
}

func TestClearCache(t *testing.T) {
	tempDir := t.TempDir()

	provider := &OpenAIProvider{
		cacheDir: filepath.Join(tempDir, "cache"),
	}

	// Create cache directory with some files
	os.MkdirAll(filepath.Join(provider.cacheDir, "ab"), 0755)
	os.WriteFile(filepath.Join(provider.cacheDir, "ab", "test1.mp3"), []byte("data1"), 0644)
	os.WriteFile(filepath.Join(provider.cacheDir, "ab", "test2.mp3"), []byte("data2"), 0644)

	// Clear cache
	err := provider.ClearCache()
	if err != nil {
		t.Errorf("ClearCache() error = %v", err)
	}

	// Verify cache directory is gone
	if _, err := os.Stat(provider.cacheDir); !os.IsNotExist(err) {
		t.Error("Cache directory should be removed")
	}

	// Test clearing with empty cache dir
	provider.cacheDir = ""
	err = provider.ClearCache()
	if err != nil {
		t.Errorf("ClearCache() with empty dir should not error: %v", err)
	}
}

func TestGetCacheStats(t *testing.T) {
	tempDir := t.TempDir()

	provider := &OpenAIProvider{
		enableCache: true,
		cacheDir:    filepath.Join(tempDir, "cache"),
	}

	// Create the cache directory first
	os.MkdirAll(provider.cacheDir, 0755)

	// Test with no cache files
	count, size, err := provider.GetCacheStats()
	if err != nil {
		t.Errorf("GetCacheStats() error = %v", err)
	}
	if count != 0 || size != 0 {
		t.Errorf("Expected empty cache stats, got count=%d, size=%d", count, size)
	}
	// Create cache files
	os.MkdirAll(filepath.Join(provider.cacheDir, "ab"), 0755)
	data1 := []byte("test data 1")
	data2 := []byte("test data 22")
	os.WriteFile(filepath.Join(provider.cacheDir, "ab", "test1.mp3"), data1, 0644)
	os.WriteFile(filepath.Join(provider.cacheDir, "ab", "test2.mp3"), data2, 0644)

	// Get stats
	count, size, err = provider.GetCacheStats()
	if err != nil {
		t.Errorf("GetCacheStats() error = %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 files, got %d", count)
	}
	expectedSize := int64(len(data1) + len(data2))
	if size != expectedSize {
		t.Errorf("Expected size %d, got %d", expectedSize, size)
	}

	// Test with cache disabled
	provider.enableCache = false
	count, size, err = provider.GetCacheStats()
	if err != nil {
		t.Errorf("GetCacheStats() with cache disabled error = %v", err)
	}
	if count != 0 || size != 0 {
		t.Errorf("Expected zero stats with cache disabled, got count=%d, size=%d", count, size)
	}
}

func TestGenerateAudioValidation(t *testing.T) {
	provider := &OpenAIProvider{
		config: &Config{
			OpenAIKey: "test-key",
		},
	}

	ctx := context.Background()

	// Test with non-Bulgarian text
	err := provider.GenerateAudio(ctx, "hello", "output.mp3")
	if err == nil {
		t.Error("Expected error for non-Bulgarian text")
	}
	if !strings.Contains(err.Error(), "must contain Cyrillic characters") {
		t.Errorf("Expected Bulgarian validation error, got: %v", err)
	}

	// Test with empty text
	err = provider.GenerateAudio(ctx, "", "output.mp3")
	if err == nil {
		t.Error("Expected error for empty text")
	}
}
