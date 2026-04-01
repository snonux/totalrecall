package translation

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
	// ProviderGemini routes translation requests to Gemini.
	ProviderGemini Provider = "gemini"
	// ProviderOpenAI routes translation requests to OpenAI.
	ProviderOpenAI Provider = "openai"

	defaultGeminiModel     = "gemini-2.5-flash"
	translationTimeout     = 30 * time.Second
	translationMaxTokens   = 50
	translationTemperature = 0.3
)

// Provider selects the translation backend.
type Provider string

// Config holds translator settings and API credentials.
type Config struct {
	Provider     Provider
	OpenAIKey    string
	GoogleAPIKey string
	OpenAIModel  string
	GeminiModel  string
}

// DefaultConfig returns a translator configuration with OpenAI as the default backend.
func DefaultConfig() *Config {
	return &Config{
		Provider:    ProviderOpenAI,
		OpenAIModel: openai.GPT4oMini,
		GeminiModel: defaultGeminiModel,
	}
}

// Translator handles Bulgarian and English translation using the configured backend.
type Translator struct {
	provider     Provider
	openAIKey    string
	googleAPIKey string
	openAIClient *openai.Client
	geminiClient *genai.Client
	openAIModel  string
	geminiModel  string
}

// NewTranslator creates a new translator instance from the provided config.
func NewTranslator(config *Config) *Translator {
	if config == nil {
		config = DefaultConfig()
	}

	normalized := normalizeConfig(config)
	translator := &Translator{
		provider:     normalized.Provider,
		openAIKey:    normalized.OpenAIKey,
		googleAPIKey: normalized.GoogleAPIKey,
		openAIModel:  normalized.OpenAIModel,
		geminiModel:  normalized.GeminiModel,
	}

	if normalized.OpenAIKey != "" {
		translator.openAIClient = openai.NewClient(normalized.OpenAIKey)
	}

	if normalized.GoogleAPIKey != "" {
		client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
			APIKey: normalized.GoogleAPIKey,
		})
		if err == nil {
			translator.geminiClient = client
		}
	}

	return translator
}

// TranslateWord translates a Bulgarian word to English.
//
// Example:
//
//	translator := translation.NewTranslator(&translation.Config{
//		Provider: translation.ProviderGemini,
//		GoogleAPIKey: os.Getenv("GOOGLE_API_KEY"),
//	})
//	english, err := translator.TranslateWord("ябълка")
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Println(english)
func (t *Translator) TranslateWord(word string) (string, error) {
	return t.translate(fmt.Sprintf(
		"Translate the Bulgarian word '%s' to English. Respond with only the English translation, nothing else.",
		word,
	))
}

// TranslateEnglishToBulgarian translates an English word to Bulgarian.
func (t *Translator) TranslateEnglishToBulgarian(word string) (string, error) {
	return t.translate(fmt.Sprintf(
		"Translate the English word '%s' to Bulgarian. Respond with only the Bulgarian translation in Cyrillic script, nothing else.",
		word,
	))
}

func (t *Translator) translate(prompt string) (string, error) {
	switch normalizeProvider(t.provider) {
	case ProviderGemini:
		return t.translateWithGemini(prompt)
	case ProviderOpenAI:
		return t.translateWithOpenAI(prompt)
	default:
		return "", fmt.Errorf("unknown translation provider: %s", t.provider)
	}
}

func (t *Translator) translateWithOpenAI(prompt string) (string, error) {
	if t.openAIKey == "" {
		return "", fmt.Errorf("OpenAI API key not found")
	}
	if t.openAIClient == nil {
		return "", fmt.Errorf("OpenAI client not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), translationTimeout)
	defer cancel()

	req := openai.ChatCompletionRequest{
		Model: t.openAIModel,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		MaxTokens:   translationMaxTokens,
		Temperature: translationTemperature,
	}

	resp, err := t.openAIClient.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("OpenAI API error: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no translation returned")
	}

	translation := strings.TrimSpace(resp.Choices[0].Message.Content)
	if translation == "" {
		return "", fmt.Errorf("no translation returned")
	}

	return translation, nil
}

func (t *Translator) translateWithGemini(prompt string) (string, error) {
	if t.googleAPIKey == "" {
		return "", fmt.Errorf("Google API key not found")
	}
	if t.geminiClient == nil {
		return "", fmt.Errorf("Gemini client not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), translationTimeout)
	defer cancel()

	temp := float32(translationTemperature)
	resp, err := t.geminiClient.Models.GenerateContent(ctx, t.geminiModel, []*genai.Content{
		genai.NewContentFromText(prompt, genai.RoleUser),
	}, &genai.GenerateContentConfig{
		Temperature:     &temp,
		MaxOutputTokens: translationMaxTokens,
	})
	if err != nil {
		return "", fmt.Errorf("Gemini API error: %w", err)
	}

	translation := strings.TrimSpace(resp.Text())
	if translation == "" {
		return "", fmt.Errorf("no translation returned")
	}

	return translation, nil
}

// SaveTranslation saves the translation to a file in the word directory.
func SaveTranslation(wordDir, word, translation string) error {
	outputFile := filepath.Join(wordDir, "translation.txt")
	content := fmt.Sprintf("%s = %s\n", word, translation)

	if err := os.WriteFile(outputFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write translation file: %w", err)
	}

	return nil
}

// TranslationCache stores translations in memory for batch operations.
type TranslationCache struct {
	translations map[string]string
}

// NewTranslationCache creates a new translation cache.
func NewTranslationCache() *TranslationCache {
	return &TranslationCache{
		translations: make(map[string]string),
	}
}

// Add adds a translation to the cache.
func (tc *TranslationCache) Add(word, translation string) {
	tc.translations[word] = translation
}

// Get retrieves a translation from the cache.
func (tc *TranslationCache) Get(word string) (string, bool) {
	translation, ok := tc.translations[word]
	return translation, ok
}

// GetAll returns all cached translations.
func (tc *TranslationCache) GetAll() map[string]string {
	result := make(map[string]string)
	for k, v := range tc.translations {
		result[k] = v
	}
	return result
}

func normalizeConfig(config *Config) *Config {
	normalized := *config
	normalized.Provider = normalizeProvider(normalized.Provider)

	if normalized.Provider == "" {
		normalized.Provider = ProviderOpenAI
	}
	if normalized.OpenAIModel == "" {
		normalized.OpenAIModel = openai.GPT4oMini
	}
	if normalized.GeminiModel == "" {
		normalized.GeminiModel = defaultGeminiModel
	}

	return &normalized
}

func normalizeProvider(provider Provider) Provider {
	switch strings.ToLower(strings.TrimSpace(string(provider))) {
	case "", string(ProviderOpenAI):
		return ProviderOpenAI
	case string(ProviderGemini):
		return ProviderGemini
	default:
		return Provider(strings.ToLower(strings.TrimSpace(string(provider))))
	}
}
