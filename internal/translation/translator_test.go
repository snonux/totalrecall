package translation

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNewTranslator(t *testing.T) {
	translator := NewTranslator("test-api-key")

	if translator == nil {
		t.Fatal("NewTranslator returned nil")
	}

	if translator.apiKey != "test-api-key" {
		t.Errorf("Expected API key 'test-api-key', got '%s'", translator.apiKey)
	}

	if translator.client == nil {
		t.Error("OpenAI client not initialized")
	}
}

func TestTranslateWord_NoAPIKey(t *testing.T) {
	translator := NewTranslator("")

	_, err := translator.TranslateWord("ябълка")
	if err == nil {
		t.Error("Expected error for missing API key")
	}

	if err.Error() != "OpenAI API key not found" {
		t.Errorf("Expected 'OpenAI API key not found' error, got: %v", err)
	}
}

func TestTranslateWord_Integration(t *testing.T) {
	// Skip if no API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	translator := NewTranslator(apiKey)

	// Test with a simple word
	translation, err := translator.TranslateWord("ябълка")
	if err != nil {
		t.Errorf("TranslateWord failed: %v", err)
	}

	// Check that we got a reasonable translation
	// The exact translation might vary, but it should contain "apple"
	if translation == "" {
		t.Error("Got empty translation")
	}

	t.Logf("Translation of 'ябълка': %s", translation)
}

func TestSaveTranslation(t *testing.T) {
	tmpDir := t.TempDir()

	err := SaveTranslation(tmpDir, "ябълка", "apple")
	if err != nil {
		t.Errorf("SaveTranslation failed: %v", err)
	}

	// Check file was created
	translationFile := filepath.Join(tmpDir, "translation.txt")
	content, err := os.ReadFile(translationFile)
	if err != nil {
		t.Errorf("Failed to read translation file: %v", err)
	}

	expected := "ябълка = apple\n"
	if string(content) != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, string(content))
	}
}

func TestSaveTranslation_InvalidPath(t *testing.T) {
	err := SaveTranslation("/nonexistent/path", "ябълка", "apple")
	if err == nil {
		t.Error("Expected error for invalid path")
	}
}

func TestTranslationCache(t *testing.T) {
	cache := NewTranslationCache()

	// Test empty cache
	_, found := cache.Get("ябълка")
	if found {
		t.Error("Expected not found in empty cache")
	}

	// Test adding and retrieving
	cache.Add("ябълка", "apple")
	cache.Add("котка", "cat")

	translation, found := cache.Get("ябълка")
	if !found {
		t.Error("Expected to find 'ябълка' in cache")
	}
	if translation != "apple" {
		t.Errorf("Expected 'apple', got '%s'", translation)
	}

	// Test overwriting
	cache.Add("ябълка", "apple (fruit)")
	translation, found = cache.Get("ябълка")
	if !found || translation != "apple (fruit)" {
		t.Errorf("Expected 'apple (fruit)', got '%s'", translation)
	}
}

func TestTranslationCache_GetAll(t *testing.T) {
	cache := NewTranslationCache()

	// Add some translations
	cache.Add("ябълка", "apple")
	cache.Add("котка", "cat")
	cache.Add("куче", "dog")

	all := cache.GetAll()

	expected := map[string]string{
		"ябълка": "apple",
		"котка":  "cat",
		"куче":   "dog",
	}

	if !reflect.DeepEqual(all, expected) {
		t.Errorf("GetAll() = %v, want %v", all, expected)
	}

	// Test that modifying returned map doesn't affect cache
	all["ябълка"] = "modified"

	translation, _ := cache.Get("ябълка")
	if translation != "apple" {
		t.Error("Cache was modified through returned map")
	}
}

func TestTranslationCache_EmptyCache(t *testing.T) {
	cache := NewTranslationCache()

	all := cache.GetAll()
	if len(all) != 0 {
		t.Errorf("Expected empty map, got %v", all)
	}
}
