package models

import (
	"os"
	"testing"
)

func TestNewLister(t *testing.T) {
	lister := NewLister("test-api-key")

	if lister == nil {
		t.Fatal("NewLister returned nil")
	}

	if lister.apiKey != "test-api-key" {
		t.Errorf("Expected API key 'test-api-key', got '%s'", lister.apiKey)
	}

	if lister.client == nil {
		t.Error("OpenAI client not initialized")
	}
}

func TestListAvailableModels_NoAPIKey(t *testing.T) {
	lister := NewLister("")

	err := lister.ListAvailableModels()
	if err == nil {
		t.Error("Expected error for missing API key")
	}

	expectedError := "OpenAI API key not found. Set OPENAI_API_KEY environment variable or configure in .totalrecall.yaml"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got: %v", expectedError, err)
	}
}

func TestListAvailableModels_Integration(t *testing.T) {
	// Skip if no API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	lister := NewLister(apiKey)

	// This test just verifies the method runs without error
	// The actual output goes to stdout which we don't capture in tests
	err := lister.ListAvailableModels()
	if err != nil {
		t.Errorf("ListAvailableModels failed: %v", err)
	}
}
