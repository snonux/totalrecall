package image

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
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
		Translation:  "banana",
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
	wantDescription := "Generated educational image for ябълка (banana)"
	if results[0].Description != wantDescription {
		t.Fatalf("result description = %q, want %q", results[0].Description, wantDescription)
	}
	if !strings.HasPrefix(results[0].URL, "data:image/png;base64,") {
		t.Fatalf("expected PNG data URI, got %q", results[0].URL)
	}
}

func TestNanoBananaClient_Search_GeneratedPromptFlow(t *testing.T) {
	originalText := nanoBananaGenerateText
	originalImage := nanoBananaGenerateImage
	originalStyles := append([]string(nil), ArtisticStyles...)
	t.Cleanup(func() {
		nanoBananaGenerateText = originalText
		nanoBananaGenerateImage = originalImage
		ArtisticStyles = originalStyles
	})

	ArtisticStyles = []string{"Photorealism"}

	var translationCalls int
	var sceneCalls int
	var gotPrompt string
	var callbackPrompt string

	nanoBananaGenerateText = func(_ context.Context, _ *NanoBananaClient, _, systemPrompt, userPrompt string, temperature float32, maxOutputTokens int32) (string, error) {
		switch {
		case strings.Contains(systemPrompt, "Bulgarian language expert"):
			translationCalls++
			if temperature != 0.3 || maxOutputTokens != 50 {
				t.Fatalf("translation params = %v/%d, want 0.3/50", temperature, maxOutputTokens)
			}
			if !strings.Contains(userPrompt, "ябълка") {
				t.Fatalf("translation prompt = %q, want Bulgarian query", userPrompt)
			}
			return "apple", nil
		case strings.Contains(systemPrompt, "educational flashcards for language learning"):
			sceneCalls++
			if temperature != 0.7 || maxOutputTokens != 100 {
				t.Fatalf("scene params = %v/%d, want 0.7/100", temperature, maxOutputTokens)
			}
			if !strings.Contains(userPrompt, "apple") {
				t.Fatalf("scene prompt = %q, want English translation", userPrompt)
			}
			return "A bright apple sits centered on a wooden table.", nil
		default:
			t.Fatalf("unexpected system prompt: %q", systemPrompt)
			return "", nil
		}
	}

	nanoBananaGenerateImage = func(_ context.Context, _ *NanoBananaClient, prompt string) ([]byte, string, error) {
		gotPrompt = prompt
		return mustJPEGBytes(t), "image/jpeg", nil
	}

	client := NewNanoBananaClient(&NanoBananaConfig{APIKey: "test-key"})
	callbackCalled := false
	client.SetPromptCallback(func(prompt string) {
		callbackCalled = true
		callbackPrompt = prompt
	})

	results, err := client.Search(context.Background(), DefaultSearchOptions("ябълка"))
	if err != nil {
		t.Fatalf("Search() unexpected error: %v", err)
	}
	if translationCalls != 1 {
		t.Fatalf("translationCalls = %d, want 1", translationCalls)
	}
	if sceneCalls != 1 {
		t.Fatalf("sceneCalls = %d, want 1", sceneCalls)
	}
	if !callbackCalled {
		t.Fatal("expected prompt callback to be called")
	}
	if callbackPrompt != gotPrompt {
		t.Fatalf("prompt callback = %q, want %q", callbackPrompt, gotPrompt)
	}
	if client.GetLastPrompt() != gotPrompt {
		t.Fatalf("GetLastPrompt() = %q, want %q", client.GetLastPrompt(), gotPrompt)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Source != nanoBananaSource {
		t.Fatalf("Source = %q, want %q", result.Source, nanoBananaSource)
	}
	if result.Width != 1 || result.Height != 1 {
		t.Fatalf("Size = %dx%d, want %dx%d", result.Width, result.Height, 1, 1)
	}
	if !strings.Contains(result.Description, "apple") {
		t.Fatalf("Description = %q, want translated word", result.Description)
	}
	if !strings.Contains(gotPrompt, "Generate a Photorealism educational flashcard image illustrating \"apple\".") {
		t.Fatalf("Prompt = %q, want translated subject in generated prompt", gotPrompt)
	}
	if !strings.Contains(gotPrompt, "Scene: A bright apple sits centered on a wooden table.") {
		t.Fatalf("Prompt = %q, want generated scene in prompt", gotPrompt)
	}

	reader, err := client.Download(context.Background(), result.URL)
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
	if !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatalf("Search output was not normalized to PNG")
	}
}

func TestNanoBananaClient_Search_TranslationFailureFallsBackToQuery(t *testing.T) {
	originalText := nanoBananaGenerateText
	originalImage := nanoBananaGenerateImage
	originalStyles := append([]string(nil), ArtisticStyles...)
	t.Cleanup(func() {
		nanoBananaGenerateText = originalText
		nanoBananaGenerateImage = originalImage
		ArtisticStyles = originalStyles
	})

	ArtisticStyles = []string{"Photorealism"}

	var sceneSawOriginalQuery bool
	nanoBananaGenerateText = func(_ context.Context, _ *NanoBananaClient, _, systemPrompt, userPrompt string, _ float32, _ int32) (string, error) {
		if strings.Contains(systemPrompt, "Bulgarian language expert") {
			if !strings.Contains(userPrompt, "ябълка") {
				t.Fatalf("translation prompt = %q, want Bulgarian query", userPrompt)
			}
			return "", fmt.Errorf("translation unavailable")
		}
		if strings.Contains(systemPrompt, "educational flashcards for language learning") {
			if !strings.Contains(userPrompt, "ябълка") {
				t.Fatalf("scene prompt = %q, want original query fallback", userPrompt)
			}
			sceneSawOriginalQuery = true
			return "Fallback scene", nil
		}
		t.Fatalf("unexpected system prompt: %q", systemPrompt)
		return "", nil
	}

	nanoBananaGenerateImage = func(_ context.Context, _ *NanoBananaClient, prompt string) ([]byte, string, error) {
		return mustPNGBytes(t), "image/png", nil
	}

	client := NewNanoBananaClient(&NanoBananaConfig{APIKey: "test-key"})
	results, err := client.Search(context.Background(), DefaultSearchOptions("ябълка"))
	if err != nil {
		t.Fatalf("Search() unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !sceneSawOriginalQuery {
		t.Fatal("expected scene generation to use the original query after translation failure")
	}
	if !strings.Contains(results[0].Description, "ябълка") {
		t.Fatalf("Description = %q, want original query in fallback description", results[0].Description)
	}
	if client.GetLastPrompt() == "" {
		t.Fatal("expected last prompt to be recorded")
	}
}

func TestNanoBananaClient_Search_TrivialSceneFallsBackToSubjectPrompt(t *testing.T) {
	originalText := nanoBananaGenerateText
	originalImage := nanoBananaGenerateImage
	originalStyles := append([]string(nil), ArtisticStyles...)
	t.Cleanup(func() {
		nanoBananaGenerateText = originalText
		nanoBananaGenerateImage = originalImage
		ArtisticStyles = originalStyles
	})

	ArtisticStyles = []string{"Slow Design"}

	nanoBananaGenerateText = func(_ context.Context, _ *NanoBananaClient, _, systemPrompt, _ string, _ float32, _ int32) (string, error) {
		switch {
		case strings.Contains(systemPrompt, "Bulgarian language expert"):
			return "apple", nil
		case strings.Contains(systemPrompt, "educational flashcards for language learning"):
			return "A", nil
		default:
			t.Fatalf("unexpected system prompt: %q", systemPrompt)
			return "", nil
		}
	}

	var gotPrompt string
	nanoBananaGenerateImage = func(_ context.Context, _ *NanoBananaClient, prompt string) ([]byte, string, error) {
		gotPrompt = prompt
		return mustPNGBytes(t), "image/png", nil
	}

	client := NewNanoBananaClient(&NanoBananaConfig{APIKey: "test-key"})
	_, err := client.Search(context.Background(), &SearchOptions{
		Query:       "ябълка",
		Translation: "apple",
	})
	if err != nil {
		t.Fatalf("Search() unexpected error: %v", err)
	}
	if !strings.Contains(gotPrompt, "illustrating \"apple\"") {
		t.Fatalf("Prompt = %q, want subject preserved in fallback prompt", gotPrompt)
	}
	if strings.Contains(gotPrompt, "Scene: A.") {
		t.Fatalf("Prompt = %q, did not expect trivial scene to survive", gotPrompt)
	}
}

func TestNanoBananaClient_Search_ImageGenerationError(t *testing.T) {
	originalText := nanoBananaGenerateText
	originalImage := nanoBananaGenerateImage
	originalStyles := append([]string(nil), ArtisticStyles...)
	t.Cleanup(func() {
		nanoBananaGenerateText = originalText
		nanoBananaGenerateImage = originalImage
		ArtisticStyles = originalStyles
	})

	ArtisticStyles = []string{"Photorealism"}

	nanoBananaGenerateText = func(_ context.Context, _ *NanoBananaClient, _, systemPrompt, userPrompt string, _ float32, _ int32) (string, error) {
		switch {
		case strings.Contains(systemPrompt, "Bulgarian language expert"):
			if !strings.Contains(userPrompt, "ябълка") {
				t.Fatalf("translation prompt = %q, want Bulgarian query", userPrompt)
			}
			return "apple", nil
		case strings.Contains(systemPrompt, "educational flashcards for language learning"):
			if !strings.Contains(userPrompt, "apple") {
				t.Fatalf("scene prompt = %q, want translated word", userPrompt)
			}
			return "A bright apple sits centered on a wooden table.", nil
		default:
			t.Fatalf("unexpected system prompt: %q", systemPrompt)
			return "", nil
		}
	}

	nanoBananaGenerateImage = func(_ context.Context, _ *NanoBananaClient, _ string) ([]byte, string, error) {
		return nil, "", fmt.Errorf("image generation failed")
	}

	client := NewNanoBananaClient(&NanoBananaConfig{APIKey: "test-key"})
	_, err := client.Search(context.Background(), DefaultSearchOptions("ябълка"))
	if err == nil {
		t.Fatal("expected image generation error")
	}

	searchErr, ok := err.(*SearchError)
	if !ok {
		t.Fatalf("expected SearchError, got %T", err)
	}
	if searchErr.Code != "API_ERROR" {
		t.Fatalf("expected API_ERROR, got %s", searchErr.Code)
	}
}

func TestNanoBananaClient_Search_CustomPromptPreservesTranslationMetadata(t *testing.T) {
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
	nanoBananaGenerateImage = func(_ context.Context, _ *NanoBananaClient, _ string) ([]byte, string, error) {
		return mustPNGBytes(t), "image/png", nil
	}

	client := NewNanoBananaClient(&NanoBananaConfig{APIKey: "test-key"})
	results, err := client.Search(context.Background(), &SearchOptions{
		Query:        "ябълка",
		Translation:  "banana",
		CustomPrompt: "flashcard prompt",
	})
	if err != nil {
		t.Fatalf("Search() unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !strings.Contains(results[0].Description, "banana") {
		t.Fatalf("result description = %q, want translation metadata", results[0].Description)
	}
}

func TestNanoBananaClient_Search_CustomPromptIgnoresWhitespaceTranslation(t *testing.T) {
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
	nanoBananaGenerateImage = func(_ context.Context, _ *NanoBananaClient, _ string) ([]byte, string, error) {
		return mustPNGBytes(t), "image/png", nil
	}

	client := NewNanoBananaClient(&NanoBananaConfig{APIKey: "test-key"})
	results, err := client.Search(context.Background(), &SearchOptions{
		Query:        "ябълка",
		Translation:  "   ",
		CustomPrompt: "flashcard prompt",
	})
	if err != nil {
		t.Fatalf("Search() unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if strings.Contains(results[0].Description, "(") || strings.Contains(results[0].Description, ")") {
		t.Fatalf("result description = %q, want no translation metadata for whitespace-only input", results[0].Description)
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
	payload := mustPNGBytes(t)
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
	if !bytes.Equal(data, payload) {
		t.Fatalf("Download() = %v, want %v", data, payload)
	}
}

func TestNanoBananaClient_DownloadHTTPFallback(t *testing.T) {
	client := &NanoBananaClient{}
	payload := []byte("fallback image bytes")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("request method = %s, want GET", r.Method)
		}
		_, _ = w.Write(payload)
	}))
	t.Cleanup(server.Close)

	reader, err := client.Download(context.Background(), server.URL)
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
	if !bytes.Equal(data, payload) {
		t.Fatalf("Download() = %v, want %v", data, payload)
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
