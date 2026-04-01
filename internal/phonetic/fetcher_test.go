package phonetic

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sashabaranov/go-openai"
	"google.golang.org/genai"
)

func TestNewFetcher_DefaultsToOpenAI(t *testing.T) {
	fetcher := NewFetcher(nil)

	if fetcher == nil {
		t.Fatal("NewFetcher returned nil")
	}

	if got := fetcher.Provider(); got != ProviderOpenAI {
		t.Fatalf("expected default provider %q, got %q", ProviderOpenAI, got)
	}
}

func TestFetchAndSave_NoOpenAIAPIKey(t *testing.T) {
	fetcher := NewFetcher(&Config{Provider: ProviderOpenAI})
	tmpDir := t.TempDir()

	err := fetcher.FetchAndSave("ябълка", tmpDir)
	if err == nil {
		t.Fatal("expected error for missing OpenAI API key")
	}

	if err.Error() != "OpenAI API key not configured" {
		t.Fatalf("expected OpenAI API key error, got %v", err)
	}
}

func TestFetchAndSave_NoGoogleAPIKey(t *testing.T) {
	fetcher := NewFetcher(&Config{Provider: ProviderGemini})
	tmpDir := t.TempDir()

	err := fetcher.FetchAndSave("ябълка", tmpDir)
	if err == nil {
		t.Fatal("expected error for missing Google API key")
	}

	if err.Error() != "Google API key not configured" {
		t.Fatalf("expected Google API key error, got %v", err)
	}
}

func TestFetchAndSave_UnknownProvider(t *testing.T) {
	fetcher := NewFetcher(&Config{Provider: Provider("mystery")})
	tmpDir := t.TempDir()

	err := fetcher.FetchAndSave("ябълка", tmpDir)
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}

	if err.Error() != "unknown phonetic provider: mystery" {
		t.Fatalf("expected unknown provider error, got %v", err)
	}
}

func TestFetchAndSave_OpenAIProvider_WritesFile(t *testing.T) {
	originalFetch := fetchOpenAIPhonetic
	fetchOpenAIPhonetic = func(context.Context, *openai.Client, string) (string, error) {
		return "[ˈjɤbɐlkɐ]", nil
	}
	t.Cleanup(func() {
		fetchOpenAIPhonetic = originalFetch
	})

	fetcher := NewFetcher(&Config{
		Provider:  ProviderOpenAI,
		OpenAIKey: "test-openai-key",
	})
	tmpDir := t.TempDir()

	if err := fetcher.FetchAndSave("ябълка", tmpDir); err != nil {
		t.Fatalf("FetchAndSave failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "phonetic.txt"))
	if err != nil {
		t.Fatalf("failed to read phonetic file: %v", err)
	}

	if got := string(content); got != "[ˈjɤbɐlkɐ]" {
		t.Fatalf("unexpected phonetic content %q", got)
	}
}

func TestFetchAndSave_GeminiProvider_WritesFile(t *testing.T) {
	originalFetch := fetchGeminiPhonetic
	fetchGeminiPhonetic = func(context.Context, *genai.Client, string) (string, error) {
		return "[ˈkotka]", nil
	}
	t.Cleanup(func() {
		fetchGeminiPhonetic = originalFetch
	})

	originalNewGeminiClient := newGeminiClient
	newGeminiClient = func(context.Context, *genai.ClientConfig) (*genai.Client, error) {
		return &genai.Client{}, nil
	}
	t.Cleanup(func() {
		newGeminiClient = originalNewGeminiClient
	})

	fetcher := NewFetcher(&Config{
		Provider:     ProviderGemini,
		GoogleAPIKey: "test-google-key",
	})
	tmpDir := t.TempDir()

	if err := fetcher.FetchAndSave("котка", tmpDir); err != nil {
		t.Fatalf("FetchAndSave failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "phonetic.txt"))
	if err != nil {
		t.Fatalf("failed to read phonetic file: %v", err)
	}

	if got := string(content); got != "[ˈkotka]" {
		t.Fatalf("unexpected phonetic content %q", got)
	}
}

func TestFetchAndSave_GeminiInitFailure(t *testing.T) {
	originalNewGeminiClient := newGeminiClient
	newGeminiClient = func(context.Context, *genai.ClientConfig) (*genai.Client, error) {
		return nil, context.Canceled
	}
	t.Cleanup(func() {
		newGeminiClient = originalNewGeminiClient
	})

	fetcher := NewFetcher(&Config{
		Provider:     ProviderGemini,
		GoogleAPIKey: "test-google-key",
	})

	err := fetcher.FetchAndSave("ябълка", t.TempDir())
	if err == nil {
		t.Fatal("expected Gemini init failure")
	}

	if err.Error() != "Gemini client initialization failed: context canceled" {
		t.Fatalf("unexpected Gemini init error: %v", err)
	}
}

func TestFetchAndSave_GeminiAPIFailure(t *testing.T) {
	originalFetch := fetchGeminiPhonetic
	fetchGeminiPhonetic = func(context.Context, *genai.Client, string) (string, error) {
		return "", context.DeadlineExceeded
	}
	t.Cleanup(func() {
		fetchGeminiPhonetic = originalFetch
	})

	originalNewGeminiClient := newGeminiClient
	newGeminiClient = func(context.Context, *genai.ClientConfig) (*genai.Client, error) {
		return &genai.Client{}, nil
	}
	t.Cleanup(func() {
		newGeminiClient = originalNewGeminiClient
	})

	fetcher := NewFetcher(&Config{
		Provider:     ProviderGemini,
		GoogleAPIKey: "test-google-key",
	})

	err := fetcher.FetchAndSave("ябълка", t.TempDir())
	if err == nil {
		t.Fatal("expected Gemini API failure")
	}

	if err.Error() != "context deadline exceeded" {
		t.Fatalf("unexpected Gemini API error: %v", err)
	}
}
