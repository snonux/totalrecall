package phonetic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewFetcher(t *testing.T) {
	fetcher := NewFetcher("test-api-key")

	if fetcher == nil {
		t.Fatal("NewFetcher returned nil")
	}

	if fetcher.apiKey != "test-api-key" {
		t.Errorf("Expected API key 'test-api-key', got '%s'", fetcher.apiKey)
	}

	if fetcher.client == nil {
		t.Error("OpenAI client not initialized")
	}
}

func TestFetchAndSave_NoAPIKey(t *testing.T) {
	fetcher := NewFetcher("")
	tmpDir := t.TempDir()

	err := fetcher.FetchAndSave("ябълка", tmpDir)
	if err == nil {
		t.Error("Expected error for missing API key")
	}

	if err.Error() != "OpenAI API key not configured" {
		t.Errorf("Expected 'OpenAI API key not configured' error, got: %v", err)
	}
}

func TestFetchAndSave_Integration(t *testing.T) {
	// Skip if no API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	fetcher := NewFetcher(apiKey)
	tmpDir := t.TempDir()

	// Test with a simple word
	err := fetcher.FetchAndSave("ябълка", tmpDir)
	if err != nil {
		t.Errorf("FetchAndSave failed: %v", err)
	}

	// Check file was created
	phoneticFile := filepath.Join(tmpDir, "phonetic.txt")
	content, err := os.ReadFile(phoneticFile)
	if err != nil {
		t.Errorf("Failed to read phonetic file: %v", err)
	}

	// Check content is reasonable
	if len(content) < 50 {
		t.Error("Phonetic content seems too short")
	}

	// Should contain IPA symbols or phonetic information
	contentStr := string(content)
	if !strings.Contains(contentStr, "/") && !strings.Contains(contentStr, "[") {
		t.Error("Content doesn't appear to contain IPA transcription")
	}

	t.Logf("Phonetic info for 'ябълка':\n%s", contentStr)
}

func TestFetchAndSave_InvalidDirectory(t *testing.T) {
	// Skip if no API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping test: OPENAI_API_KEY not set")
	}

	fetcher := NewFetcher(apiKey)

	// Try to save to a non-existent directory
	err := fetcher.FetchAndSave("ябълка", "/nonexistent/path")
	if err == nil {
		t.Error("Expected error for invalid directory")
	}
}
