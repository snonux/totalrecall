package translation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// Translator handles Bulgarian to English translation
type Translator struct {
	apiKey string
	client *openai.Client
}

// NewTranslator creates a new translator instance
func NewTranslator(apiKey string) *Translator {
	return &Translator{
		apiKey: apiKey,
		client: openai.NewClient(apiKey),
	}
}

// TranslateWord translates a Bulgarian word to English
func (t *Translator) TranslateWord(word string) (string, error) {
	if t.apiKey == "" {
		return "", fmt.Errorf("OpenAI API key not found")
	}

	ctx := context.Background()

	req := openai.ChatCompletionRequest{
		Model: openai.GPT4oMini,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: fmt.Sprintf("Translate the Bulgarian word '%s' to English. Respond with only the English translation, nothing else.", word),
			},
		},
		MaxTokens:   50,
		Temperature: 0.3,
	}

	resp, err := t.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no translation returned")
	}

	translation := strings.TrimSpace(resp.Choices[0].Message.Content)
	return translation, nil
}

// SaveTranslation saves the translation to a file in the word directory
func SaveTranslation(wordDir, word, translation string) error {
	outputFile := filepath.Join(wordDir, "translation.txt")
	content := fmt.Sprintf("%s = %s\n", word, translation)

	if err := os.WriteFile(outputFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write translation file: %w", err)
	}

	return nil
}

// TranslationCache stores translations in memory for batch operations
type TranslationCache struct {
	translations map[string]string
}

// NewTranslationCache creates a new translation cache
func NewTranslationCache() *TranslationCache {
	return &TranslationCache{
		translations: make(map[string]string),
	}
}

// Add adds a translation to the cache
func (tc *TranslationCache) Add(word, translation string) {
	tc.translations[word] = translation
}

// Get retrieves a translation from the cache
func (tc *TranslationCache) Get(word string) (string, bool) {
	translation, ok := tc.translations[word]
	return translation, ok
}

// GetAll returns all cached translations
func (tc *TranslationCache) GetAll() map[string]string {
	// Return a copy to prevent external modification
	result := make(map[string]string)
	for k, v := range tc.translations {
		result[k] = v
	}
	return result
}
