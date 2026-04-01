package image

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"strings"
	"testing"

	"google.golang.org/genai"
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

func TestNanoBananaClient_Search_CustomPromptSkipsTextGeneration(t *testing.T) {
	originalText := nanoBananaGenerateText
	originalImage := nanoBananaGenerateImage
	t.Cleanup(func() {
		nanoBananaGenerateText = originalText
		nanoBananaGenerateImage = originalImage
	})

	nanoBananaGenerateText = func(_ context.Context, _ *NanoBananaClient, _, _, _ string, _ float32, _ int32) (string, error) {
		t.Fatal("unexpected text generation for custom prompt")
		return "", nil
	}

	var gotPrompt string
	nanoBananaGenerateImage = func(_ context.Context, _ *NanoBananaClient, prompt string) ([]byte, string, error) {
		gotPrompt = prompt
		return mustPNGBytes(t), "image/png", nil
	}

	client := NewNanoBananaClient(&NanoBananaConfig{APIKey: "test-key"})
	callbackCalled := false
	client.SetPromptCallback(func(prompt string) {
		callbackCalled = true
		if prompt != "custom flashcard prompt" {
			t.Fatalf("callback prompt = %q, want %q", prompt, "custom flashcard prompt")
		}
	})

	results, err := client.Search(context.Background(), &SearchOptions{
		Query:        "ябълка",
		CustomPrompt: " custom flashcard prompt ",
	})
	if err != nil {
		t.Fatalf("Search() unexpected error: %v", err)
	}
	if gotPrompt != "custom flashcard prompt" {
		t.Fatalf("image prompt = %q, want %q", gotPrompt, "custom flashcard prompt")
	}
	if !callbackCalled {
		t.Fatal("expected prompt callback to be called")
	}
	if client.GetLastPrompt() != "custom flashcard prompt" {
		t.Fatalf("GetLastPrompt() = %q, want %q", client.GetLastPrompt(), "custom flashcard prompt")
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !strings.Contains(results[0].Description, "ябълка") {
		t.Fatalf("result description = %q, want it to mention the query", results[0].Description)
	}
	if !strings.HasPrefix(results[0].URL, "data:image/png;base64,") {
		t.Fatalf("expected PNG data URI, got %q", results[0].URL)
	}
}

func TestNanoBananaClient_Search_InvalidOptions(t *testing.T) {
	t.Parallel()

	client := NewNanoBananaClient(&NanoBananaConfig{APIKey: "test-key"})
	_, err := client.Search(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil options")
	}

	searchErr, ok := err.(*SearchError)
	if !ok {
		t.Fatalf("expected SearchError, got %T", err)
	}
	if searchErr.Code != "INVALID_OPTIONS" {
		t.Fatalf("expected INVALID_OPTIONS, got %s", searchErr.Code)
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

func TestNormalizePNG_FromJPEG(t *testing.T) {
	t.Parallel()

	jpegBytes := mustJPEGBytes(t)
	pngBytes, err := normalizePNG(jpegBytes, "image/jpeg")
	if err != nil {
		t.Fatalf("normalizePNG() unexpected error: %v", err)
	}
	if !bytes.HasPrefix(pngBytes, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatalf("normalizePNG() did not return PNG data")
	}

	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("png.Decode() unexpected error: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 1 || bounds.Dy() != 1 {
		t.Fatalf("decoded PNG bounds = %v, want 1x1", bounds)
	}
}

func TestExtractGeneratedImageErrors(t *testing.T) {
	t.Parallel()

	_, _, err := extractGeneratedImage(nil)
	if err == nil {
		t.Fatal("expected error for nil response")
	}

	_, _, err = extractGeneratedImage(&genai.GenerateContentResponse{})
	if err == nil {
		t.Fatal("expected error for empty response")
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

func mustPNGBytes(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatalf("png.Encode() unexpected error: %v", err)
	}

	return buffer.Bytes()
}

func mustJPEGBytes(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 0, G: 128, B: 255, A: 255})

	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("jpeg.Encode() unexpected error: %v", err)
	}

	return buffer.Bytes()
}
