package models

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
	"google.golang.org/genai"
)

type fakeOpenAIClient struct {
	models openai.ModelsList
	err    error
}

func (f *fakeOpenAIClient) ListModels(context.Context) (openai.ModelsList, error) {
	return f.models, f.err
}

type fakeGeminiClient struct {
	pages map[string]genai.Page[genai.Model]
	err   error
	calls []string
}

func (f *fakeGeminiClient) List(_ context.Context, config *genai.ListModelsConfig) (genai.Page[genai.Model], error) {
	token := ""
	if config != nil {
		token = config.PageToken
	}

	f.calls = append(f.calls, token)

	if f.err != nil {
		return genai.Page[genai.Model]{}, f.err
	}

	if page, ok := f.pages[token]; ok {
		return page, nil
	}

	return genai.Page[genai.Model]{}, nil
}

func TestNewLister(t *testing.T) {
	lister := NewLister("  test-openai-key  ", "  test-gemini-key  ", nil)

	if lister == nil {
		t.Fatal("NewLister returned nil")
	}

	if lister.openAIKey != "test-openai-key" {
		t.Fatalf("openAIKey = %q, want %q", lister.openAIKey, "test-openai-key")
	}

	if lister.geminiKey != "test-gemini-key" {
		t.Fatalf("geminiKey = %q, want %q", lister.geminiKey, "test-gemini-key")
	}

	if lister.openAIKey != "" && lister.openAIClient == nil {
		t.Fatal("OpenAI client not initialized")
	}

	if lister.geminiKey != "" && lister.geminiClient == nil && lister.geminiInitErr == nil {
		t.Fatal("Gemini client not initialized")
	}
}

func TestListAvailableModels_NoAPIKeys(t *testing.T) {
	var output bytes.Buffer
	lister := &Lister{out: &output}

	err := lister.ListAvailableModels()
	if err == nil {
		t.Fatal("Expected error for missing API keys")
	}

	expectedError := "no API keys found"
	if !strings.Contains(err.Error(), expectedError) {
		t.Fatalf("Expected error containing %q, got %v", expectedError, err)
	}
}

func TestListAvailableModels_OpenAIOnly(t *testing.T) {
	var output bytes.Buffer
	lister := &Lister{
		openAIKey: "test-openai-key",
		openAIClient: &fakeOpenAIClient{
			models: openai.ModelsList{
				Models: []openai.Model{
					{ID: "tts-1"},
					{ID: "dall-e-3"},
					{ID: "gpt-4o"},
					{ID: "gpt-4o-mini-tts"},
					{ID: "gpt-4.1"},
				},
			},
		},
		out: &output,
	}

	if err := lister.ListAvailableModels(); err != nil {
		t.Fatalf("ListAvailableModels failed: %v", err)
	}

	got := output.String()
	for _, want := range []string{
		"Available Models:",
		"OpenAI Models:",
		"Text-to-Speech (TTS) Models:",
		"Image Generation Models:",
		"Chat/Translation Models (for Bulgarian translation):",
		"tts-1",
		"dall-e-3",
		"gpt-4o",
		"gpt-4o-mini-tts",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}

	if strings.Contains(got, "Gemini Models:") {
		t.Fatalf("output unexpectedly contained Gemini section:\n%s", got)
	}
}

func TestListAvailableModels_GeminiOnly(t *testing.T) {
	var output bytes.Buffer
	geminiClient := &fakeGeminiClient{
		pages: map[string]genai.Page[genai.Model]{
			"": {
				Items: []*genai.Model{
					{Name: "models/gemini-2.5-pro"},
					{Name: "models/gemini-2.5-flash"},
				},
				NextPageToken: "page-2",
			},
			"page-2": {
				Items: []*genai.Model{
					{Name: "models/gemini-2.5-flash-preview-tts"},
					{DisplayName: "Gemini Experimental"},
				},
			},
		},
	}

	lister := &Lister{
		geminiKey:    "test-gemini-key",
		geminiClient: geminiClient,
		out:          &output,
	}

	if err := lister.ListAvailableModels(); err != nil {
		t.Fatalf("ListAvailableModels failed: %v", err)
	}

	got := output.String()
	for _, want := range []string{
		"Available Models:",
		"Gemini Models:",
		"gemini-2.5-flash",
		"gemini-2.5-pro",
		"gemini-2.5-flash-preview-tts",
		"Gemini Experimental",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}

	if len(geminiClient.calls) != 2 {
		t.Fatalf("expected 2 Gemini page requests, got %d", len(geminiClient.calls))
	}
}

func TestListAvailableModels_BothProviders(t *testing.T) {
	var output bytes.Buffer
	lister := &Lister{
		openAIKey: "test-openai-key",
		openAIClient: &fakeOpenAIClient{
			models: openai.ModelsList{
				Models: []openai.Model{{ID: "tts-1"}},
			},
		},
		geminiKey: "test-gemini-key",
		geminiClient: &fakeGeminiClient{
			pages: map[string]genai.Page[genai.Model]{
				"": {
					Items: []*genai.Model{{Name: "models/gemini-2.5-flash"}},
				},
			},
		},
		out: &output,
	}

	if err := lister.ListAvailableModels(); err != nil {
		t.Fatalf("ListAvailableModels failed: %v", err)
	}

	got := output.String()
	openAIIndex := strings.Index(got, "OpenAI Models:")
	geminiIndex := strings.Index(got, "Gemini Models:")
	if openAIIndex == -1 || geminiIndex == -1 {
		t.Fatalf("expected both provider sections, got:\n%s", got)
	}
	if openAIIndex > geminiIndex {
		t.Fatalf("expected OpenAI section before Gemini section, got:\n%s", got)
	}
}
