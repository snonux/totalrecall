package image

import (
	"context"
	"encoding/base64"
	"io"
	"os"
	"strings"
	"testing"
)

func TestNewNanoBananaClient(t *testing.T) {
	t.Parallel()

	client := NewNanoBananaClient(&NanoBananaConfig{APIKey: "test-key"})
	if client == nil {
		t.Fatal("expected client")
	}
	if client.config == nil {
		t.Fatal("expected normalized config")
	}
	if client.config.Model != DefaultNanoBananaModel {
		t.Fatalf("expected default model %q, got %q", DefaultNanoBananaModel, client.config.Model)
	}
	if client.config.TextModel != DefaultNanoBananaTextModel {
		t.Fatalf("expected default text model %q, got %q", DefaultNanoBananaTextModel, client.config.TextModel)
	}
	if client.Name() != nanoBananaSource {
		t.Fatalf("Name() = %q, want %q", client.Name(), nanoBananaSource)
	}
}

func TestNanoBananaClient_NoAPIKey(t *testing.T) {
	client := NewNanoBananaClient(&NanoBananaConfig{})

	_, err := client.Search(context.Background(), DefaultSearchOptions("ябълка"))
	if err == nil {
		t.Fatal("expected error for missing API key")
	}

	searchErr, ok := err.(*SearchError)
	if !ok {
		t.Fatalf("expected SearchError, got %T", err)
	}
	if searchErr.Code != "NO_API_KEY" {
		t.Fatalf("expected NO_API_KEY error, got %s", searchErr.Code)
	}
}

func TestNanoBananaClient_DownloadDataURI(t *testing.T) {
	client := &NanoBananaClient{}
	payload := []byte("png-bytes")
	url := "data:image/png;base64," + encodeBase64(payload)

	reader, err := client.Download(context.Background(), url)
	if err != nil {
		t.Fatalf("Download() unexpected error: %v", err)
	}
	t.Cleanup(func() {
		_ = reader.Close()
	})

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() unexpected error: %v", err)
	}
	if string(data) != string(payload) {
		t.Fatalf("Download() = %q, want %q", data, payload)
	}
}

func TestNanoBananaClient_GetAttribution(t *testing.T) {
	client := &NanoBananaClient{
		config: &NanoBananaConfig{
			Model:     "gemini-test-image",
			TextModel: "gemini-test-text",
		},
		lastPrompt: "Generate a simple illustration of an apple.",
	}

	attr := client.GetAttribution(&SearchResult{Description: "Generated educational image for ябълка (apple)"})
	for _, want := range []string{
		"Google Gemini Nano Banana",
		"gemini-test-image",
		"gemini-test-text",
		"Prompt used:",
		"Generated educational image for ябълка (apple)",
	} {
		if !strings.Contains(attr, want) {
			t.Fatalf("GetAttribution() = %q, missing %q", attr, want)
		}
	}
}

func TestNanoBananaClient_Integration(t *testing.T) {
	if os.Getenv("TOTALRECALL_IMAGE_INTEGRATION") == "" {
		t.Skip("TOTALRECALL_IMAGE_INTEGRATION not set, skipping integration test")
	}
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		t.Skip("GOOGLE_API_KEY not set, skipping integration test")
	}

	client := NewNanoBananaClient(&NanoBananaConfig{
		APIKey:    apiKey,
		Model:     DefaultNanoBananaModel,
		TextModel: DefaultNanoBananaTextModel,
	})

	results, err := client.Search(context.Background(), &SearchOptions{
		Query:       "ябълка",
		Translation: "apple",
	})
	if err != nil {
		t.Fatalf("Search() failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !strings.HasPrefix(results[0].URL, "data:image/png;base64,") {
		t.Fatalf("expected data URI result, got %q", results[0].URL)
	}
}

func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
