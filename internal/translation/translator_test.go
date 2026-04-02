package translation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"google.golang.org/genai"
)

func TestNewTranslator_DefaultsToOpenAI(t *testing.T) {
	translator := NewTranslator(nil)

	if translator == nil {
		t.Fatal("NewTranslator returned nil")
	}
	if translator.provider != ProviderOpenAI {
		t.Fatalf("Expected default provider %q, got %q", ProviderOpenAI, translator.provider)
	}
	if translator.openAIModel != "gpt-4o-mini" {
		t.Fatalf("Expected OpenAI model gpt-4o-mini, got %q", translator.openAIModel)
	}
}

func TestNewTranslator_ExplicitGeminiProvider(t *testing.T) {
	translator := NewTranslator(&Config{
		Provider:     ProviderGemini,
		GoogleAPIKey: "test-google-key",
	})

	if translator == nil {
		t.Fatal("NewTranslator returned nil")
	}
	if translator.provider != ProviderGemini {
		t.Fatalf("Expected provider %q, got %q", ProviderGemini, translator.provider)
	}
	if translator.geminiClient == nil {
		t.Fatal("Gemini client not initialized")
	}
}

func TestNewTranslator_EmptyProviderDefaultsToOpenAI(t *testing.T) {
	translator := NewTranslator(&Config{})

	if translator == nil {
		t.Fatal("NewTranslator returned nil")
	}
	if translator.provider != ProviderOpenAI {
		t.Fatalf("Expected empty provider to default to %q, got %q", ProviderOpenAI, translator.provider)
	}
}

func TestNewTranslator_OpenAIProvider(t *testing.T) {
	translator := NewTranslator(&Config{
		Provider:  ProviderOpenAI,
		OpenAIKey: "test-api-key",
	})

	if translator == nil {
		t.Fatal("NewTranslator returned nil")
	}
	if translator.provider != ProviderOpenAI {
		t.Fatalf("Expected provider %q, got %q", ProviderOpenAI, translator.provider)
	}
	if translator.openAIClient == nil {
		t.Fatal("OpenAI client not initialized")
	}
}

func TestTranslateWord_DefaultProviderRequiresOpenAIKey(t *testing.T) {
	translator := NewTranslator(&Config{})

	_, err := translator.TranslateWord("ябълка")
	if err == nil {
		t.Fatal("Expected error for missing OpenAI API key")
	}
	if err.Error() != "OpenAI API key not found" {
		t.Fatalf("Expected 'OpenAI API key not found' error, got: %v", err)
	}
}

func TestTranslateWord_ExplicitGeminiRequiresGoogleAPIKey(t *testing.T) {
	translator := NewTranslator(&Config{Provider: ProviderGemini})

	_, err := translator.TranslateWord("ябълка")
	if err == nil {
		t.Fatal("Expected error for missing Google API key")
	}
	if err.Error() != "google API key not found" {
		t.Fatalf("Expected 'google API key not found' error, got: %v", err)
	}
}

func TestTranslateWord_GeminiClientInitError(t *testing.T) {
	originalNewGeminiClient := newGeminiClient
	defer func() {
		newGeminiClient = originalNewGeminiClient
	}()

	initErr := errors.New("boom")
	newGeminiClient = func(context.Context, *genai.ClientConfig) (*genai.Client, error) {
		return nil, initErr
	}

	translator := NewTranslator(&Config{
		Provider:     ProviderGemini,
		GoogleAPIKey: "test-google-key",
	})

	_, err := translator.TranslateWord("ябълка")
	if err == nil {
		t.Fatal("Expected Gemini init error")
	}
	if !errors.Is(err, initErr) {
		t.Fatalf("Expected wrapped init error, got: %v", err)
	}
}

func TestTranslateWord_IntegrationGemini(t *testing.T) {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: GOOGLE_API_KEY not set")
	}

	translator := NewTranslator(&Config{
		Provider:     ProviderGemini,
		GoogleAPIKey: apiKey,
	})

	translation, err := translator.TranslateWord("ябълка")
	if err != nil {
		t.Fatalf("TranslateWord failed: %v", err)
	}
	if translation == "" {
		t.Fatal("Got empty translation")
	}

	t.Logf("Translation of 'ябълка': %s", translation)
}

func TestTranslateEnglishToBulgarian_NoOpenAIKey(t *testing.T) {
	translator := NewTranslator(&Config{Provider: ProviderOpenAI})

	_, err := translator.TranslateEnglishToBulgarian("apple")
	if err == nil {
		t.Fatal("Expected error for missing OpenAI API key")
	}
	if err.Error() != "OpenAI API key not found" {
		t.Fatalf("Expected 'OpenAI API key not found' error, got: %v", err)
	}
}

func TestTranslateWithUnknownProvider(t *testing.T) {
	translator := NewTranslator(&Config{
		Provider: Provider("legacy"),
	})

	_, err := translator.TranslateWord("ябълка")
	if err == nil {
		t.Fatal("Expected error for unknown provider")
	}
	if err.Error() != "unknown translation provider: legacy" {
		t.Fatalf("Expected unknown provider error, got: %v", err)
	}
}

func TestTranslateEnglishToBulgarian_IntegrationOpenAI(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	translator := NewTranslator(&Config{
		Provider:  ProviderOpenAI,
		OpenAIKey: apiKey,
	})

	translation, err := translator.TranslateEnglishToBulgarian("apple")
	if err != nil {
		t.Fatalf("TranslateEnglishToBulgarian failed: %v", err)
	}
	if translation == "" {
		t.Fatal("Got empty translation")
	}

	hasCyrillic := false
	for _, r := range translation {
		if r >= 'А' && r <= 'я' {
			hasCyrillic = true
			break
		}
	}
	if !hasCyrillic {
		t.Fatalf("Expected Cyrillic translation, got: %s", translation)
	}

	t.Logf("Translation of 'apple': %s", translation)
}

func TestSaveTranslation(t *testing.T) {
	tmpDir := t.TempDir()

	if err := SaveTranslation(tmpDir, "ябълка", "apple"); err != nil {
		t.Fatalf("SaveTranslation failed: %v", err)
	}

	translationFile := filepath.Join(tmpDir, "translation.txt")
	content, err := os.ReadFile(translationFile)
	if err != nil {
		t.Fatalf("Failed to read translation file: %v", err)
	}

	expected := "ябълка = apple\n"
	if string(content) != expected {
		t.Fatalf("Expected content %q, got %q", expected, string(content))
	}
}

func TestSaveTranslation_InvalidPath(t *testing.T) {
	if err := SaveTranslation("/nonexistent/path", "ябълка", "apple"); err == nil {
		t.Fatal("Expected error for invalid path")
	}
}

func TestTranslationCache(t *testing.T) {
	cache := NewTranslationCache()

	if _, found := cache.Get("ябълка"); found {
		t.Fatal("Expected not found in empty cache")
	}

	cache.Add("ябълка", "apple")
	cache.Add("котка", "cat")

	translation, found := cache.Get("ябълка")
	if !found {
		t.Fatal("Expected to find 'ябълка' in cache")
	}
	if translation != "apple" {
		t.Fatalf("Expected 'apple', got %q", translation)
	}

	cache.Add("ябълка", "apple (fruit)")
	translation, found = cache.Get("ябълка")
	if !found || translation != "apple (fruit)" {
		t.Fatalf("Expected 'apple (fruit)', got %q", translation)
	}
}

func TestTranslationCache_GetAll(t *testing.T) {
	cache := NewTranslationCache()

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
		t.Fatalf("GetAll() = %v, want %v", all, expected)
	}

	all["ябълка"] = "modified"

	translation, _ := cache.Get("ябълка")
	if translation != "apple" {
		t.Fatal("Cache was modified through returned map")
	}
}

func TestTranslationCache_EmptyCache(t *testing.T) {
	cache := NewTranslationCache()

	all := cache.GetAll()
	if len(all) != 0 {
		t.Fatalf("Expected empty map, got %v", all)
	}
}
