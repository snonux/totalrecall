package phonetic

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

// Fetcher handles fetching phonetic information for Bulgarian words
type Fetcher struct {
	apiKey string
	client *openai.Client
}

// NewFetcher creates a new phonetic information fetcher
func NewFetcher(apiKey string) *Fetcher {
	return &Fetcher{
		apiKey: apiKey,
		client: openai.NewClient(apiKey),
	}
}

// FetchAndSave fetches phonetic information for a word and saves it to the word directory
func (f *Fetcher) FetchAndSave(word, wordDir string) error {
	if f.apiKey == "" {
		return fmt.Errorf("OpenAI API key not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := openai.ChatCompletionRequest{
		Model: openai.GPT4o,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a Bulgarian language expert. Provide only the IPA (International Phonetic Alphabet) transcription for Bulgarian words. Return ONLY the IPA transcription in square brackets, nothing else. No explanations, no word labels, just the IPA.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: fmt.Sprintf(`%s`, word),
			},
		},
		Temperature: 0.3,
		MaxTokens:   50,
	}

	resp, err := f.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return fmt.Errorf("no response from OpenAI")
	}

	phoneticInfo := strings.TrimSpace(resp.Choices[0].Message.Content)

	// Save phonetic info to file
	phoneticFile := filepath.Join(wordDir, "phonetic.txt")
	if err := os.WriteFile(phoneticFile, []byte(phoneticInfo), 0644); err != nil {
		return fmt.Errorf("failed to write phonetic file: %w", err)
	}

	return nil
}
