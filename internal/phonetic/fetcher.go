package phonetic

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
	"google.golang.org/genai"
)

const (
	// ProviderGemini routes phonetic requests to Gemini.
	ProviderGemini Provider = "gemini"
	// ProviderOpenAI routes phonetic requests to OpenAI.
	ProviderOpenAI Provider = "openai"

	defaultGeminiModel   = "gemini-2.5-flash"
	defaultOpenAIModel   = openai.GPT4o
	phoneticTimeout      = 30 * time.Second
	phoneticTemperature  = 0.3
	phoneticMaxTokens    = 50
	phoneticSystemPrompt = "You are a Bulgarian language expert. Provide only the IPA (International Phonetic Alphabet) transcription for Bulgarian words. Return ONLY the IPA transcription in square brackets, nothing else. No explanations, no word labels, just the IPA."
)

// Provider selects the phonetic backend.
type Provider string

// Config holds phonetic fetcher settings and API credentials.
type Config struct {
	Provider     Provider
	OpenAIKey    string
	GoogleAPIKey string
}

// Fetcher handles fetching phonetic information for Bulgarian words.
type Fetcher struct {
	provider     Provider
	openAIKey    string
	googleAPIKey string

	openAIClient  *openai.Client
	geminiClient  *genai.Client
	geminiInitErr error
}

var newGeminiClient = genai.NewClient

var fetchOpenAIPhonetic = func(ctx context.Context, client *openai.Client, word string) (string, error) {
	req := openai.ChatCompletionRequest{
		Model: defaultOpenAIModel,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: phoneticSystemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: word,
			},
		},
		Temperature: phoneticTemperature,
		MaxTokens:   phoneticMaxTokens,
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

var fetchGeminiPhonetic = func(ctx context.Context, client *genai.Client, word string) (string, error) {
	temp := float32(phoneticTemperature)
	resp, err := client.Models.GenerateContent(ctx, defaultGeminiModel, []*genai.Content{
		genai.NewContentFromText(word, genai.RoleUser),
	}, &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(phoneticSystemPrompt, genai.RoleUser),
		Temperature:       &temp,
		MaxOutputTokens:   phoneticMaxTokens,
	})
	if err != nil {
		return "", fmt.Errorf("Gemini API error: %w", err)
	}

	phoneticInfo := strings.TrimSpace(resp.Text())
	if phoneticInfo == "" {
		return "", fmt.Errorf("no response from Gemini")
	}

	return phoneticInfo, nil
}

// NewFetcher creates a new phonetic information fetcher.
func NewFetcher(config *Config) *Fetcher {
	normalized := normalizeConfig(config)
	fetcher := &Fetcher{
		provider:     normalized.Provider,
		openAIKey:    normalized.OpenAIKey,
		googleAPIKey: normalized.GoogleAPIKey,
	}

	switch fetcher.provider {
	case ProviderOpenAI:
		if fetcher.openAIKey != "" {
			fetcher.openAIClient = openai.NewClient(fetcher.openAIKey)
		}
	case ProviderGemini:
		if fetcher.googleAPIKey != "" {
			client, err := newGeminiClient(context.Background(), &genai.ClientConfig{
				APIKey: fetcher.googleAPIKey,
			})
			if err != nil {
				fetcher.geminiInitErr = err
			} else {
				fetcher.geminiClient = client
			}
		}
	}

	return fetcher
}

// FetchAndSave fetches phonetic information for a word and saves it to the word directory.
func (f *Fetcher) FetchAndSave(word, wordDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), phoneticTimeout)
	defer cancel()

	phoneticInfo, err := f.fetchPhoneticInfo(ctx, word)
	if err != nil {
		return err
	}

	phoneticFile := filepath.Join(wordDir, "phonetic.txt")
	if err := os.WriteFile(phoneticFile, []byte(phoneticInfo), 0644); err != nil {
		return fmt.Errorf("failed to write phonetic file: %w", err)
	}

	return nil
}

// Provider reports the configured phonetic backend.
func (f *Fetcher) Provider() Provider {
	return f.provider
}

func (f *Fetcher) fetchPhoneticInfo(ctx context.Context, word string) (string, error) {
	switch f.provider {
	case ProviderOpenAI:
		return f.fetchWithOpenAI(ctx, word)
	case ProviderGemini:
		return f.fetchWithGemini(ctx, word)
	default:
		return "", fmt.Errorf("unknown phonetic provider: %s", f.provider)
	}
}

func (f *Fetcher) fetchWithOpenAI(ctx context.Context, word string) (string, error) {
	if f.openAIKey == "" {
		return "", fmt.Errorf("OpenAI API key not configured")
	}
	if f.openAIClient == nil {
		return "", fmt.Errorf("OpenAI client not initialized")
	}

	return fetchOpenAIPhonetic(ctx, f.openAIClient, word)
}

func (f *Fetcher) fetchWithGemini(ctx context.Context, word string) (string, error) {
	if f.googleAPIKey == "" {
		return "", fmt.Errorf("Google API key not configured")
	}
	if f.geminiInitErr != nil {
		return "", fmt.Errorf("Gemini client initialization failed: %w", f.geminiInitErr)
	}
	if f.geminiClient == nil {
		return "", fmt.Errorf("Gemini client not initialized")
	}

	return fetchGeminiPhonetic(ctx, f.geminiClient, word)
}

func normalizeConfig(config *Config) Config {
	normalized := Config{
		Provider:     ProviderOpenAI,
		OpenAIKey:    "",
		GoogleAPIKey: "",
	}

	if config == nil {
		return normalized
	}

	normalized.Provider = normalizeProvider(config.Provider)
	normalized.OpenAIKey = strings.TrimSpace(config.OpenAIKey)
	normalized.GoogleAPIKey = strings.TrimSpace(config.GoogleAPIKey)

	return normalized
}

func normalizeProvider(provider Provider) Provider {
	normalized := Provider(strings.ToLower(strings.TrimSpace(string(provider))))
	if normalized == "" {
		return ProviderOpenAI
	}

	return normalized
}
