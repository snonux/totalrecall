package phonetic

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
	"google.golang.org/genai"

	appconfig "codeberg.org/snonux/totalrecall/internal/config"
)

const (
	// ProviderGemini routes phonetic requests to Gemini.
	ProviderGemini Provider = "gemini"
	// ProviderOpenAI routes phonetic requests to OpenAI.
	ProviderOpenAI Provider = "openai"

	defaultGeminiModel  = "gemini-2.5-flash"
	defaultOpenAIModel  = openai.GPT4o
	phoneticTimeout     = 30 * time.Second
	phoneticRetryCount  = 3
	phoneticTemperature = 0.3
	// 200 tokens gives ample room for IPA of multi-syllable Bulgarian phrases.
	// 50 was too tight: Gemini 2.5 Flash can emit several thinking tokens before
	// the IPA bracket pair, causing the output to be silently truncated mid-symbol.
	phoneticMaxTokens    = 200
	phoneticSystemPrompt = "You are a Bulgarian language expert. Provide only the IPA (International Phonetic Alphabet) transcription for Bulgarian words. Return ONLY the IPA transcription in square brackets, nothing else. No explanations, no word labels, just the IPA."
)

var geminiIPAPattern = regexp.MustCompile(`\[[^\[\]\n]+\]`)
var errNoGeminiPhoneticResponse = errors.New("no response from Gemini")

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
	thinkingBudget := int32(0) // IPA lookup needs no reasoning; disable thinking so
	// it doesn't consume MaxOutputTokens before any visible text is emitted.
	resp, err := client.Models.GenerateContent(ctx, defaultGeminiModel, []*genai.Content{
		genai.NewContentFromText(buildGeminiPhoneticPrompt(word), genai.RoleUser),
	}, &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: phoneticSystemPrompt}},
		},
		Temperature:     &temp,
		MaxOutputTokens: phoneticMaxTokens,
		ThinkingConfig:  &genai.ThinkingConfig{ThinkingBudget: &thinkingBudget},
	})
	if err != nil {
		return "", fmt.Errorf("gemini API error: %w", err)
	}

	return normalizeGeminiPhoneticResponse(resp.Text())
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
	phoneticInfo, err := f.Fetch(word)
	if err != nil {
		return err
	}

	phoneticFile := filepath.Join(wordDir, "phonetic.txt")
	if err := os.WriteFile(phoneticFile, []byte(phoneticInfo), 0644); err != nil {
		return fmt.Errorf("failed to write phonetic file: %w", err)
	}

	return nil
}

// Fetch fetches phonetic information for a word.
func (f *Fetcher) Fetch(word string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), phoneticTimeout)
	defer cancel()

	return f.fetchPhoneticInfo(ctx, word)
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
		return "", fmt.Errorf("google API key not configured")
	}
	if f.geminiInitErr != nil {
		return "", fmt.Errorf("gemini client initialization failed: %w", f.geminiInitErr)
	}
	if f.geminiClient == nil {
		return "", fmt.Errorf("gemini client not initialized")
	}

	var lastErr error
	for attempt := 0; attempt < phoneticRetryCount; attempt++ {
		phoneticInfo, err := fetchGeminiPhonetic(ctx, f.geminiClient, word)
		if err == nil {
			return phoneticInfo, nil
		}
		if !errors.Is(err, errNoGeminiPhoneticResponse) {
			return "", err
		}

		lastErr = err
	}

	if lastErr != nil {
		return "", lastErr
	}

	return "", errNoGeminiPhoneticResponse
}

func buildGeminiPhoneticPrompt(word string) string {
	return fmt.Sprintf("Bulgarian text or phrase:\n%s\n\nReturn only its IPA transcription in square brackets.", strings.TrimSpace(word))
}

func normalizeGeminiPhoneticResponse(raw string) (string, error) {
	trimmed := stripMarkdownCodeFence(strings.TrimSpace(raw))
	if trimmed == "" {
		return "", errNoGeminiPhoneticResponse
	}

	if match := geminiIPAPattern.FindString(trimmed); match != "" {
		return match, nil
	}

	return trimmed, nil
}

func stripMarkdownCodeFence(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}

	trimmed = strings.TrimPrefix(trimmed, "```")
	if newline := strings.Index(trimmed, "\n"); newline >= 0 {
		trimmed = trimmed[newline+1:]
	}
	if closing := strings.LastIndex(trimmed, "```"); closing >= 0 {
		trimmed = trimmed[:closing]
	}

	return strings.TrimSpace(trimmed)
}

func normalizeConfig(config *Config) Config {
	normalized := Config{
		Provider:     ProviderGemini,
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

// normalizeProvider delegates to the shared config.NormalizeProvider so the
// normalization rule (lowercase, trim, default to "gemini") has one home.
func normalizeProvider(provider Provider) Provider {
	return Provider(appconfig.NormalizeProvider(string(provider)))
}
