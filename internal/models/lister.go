package models

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// Lister handles listing available OpenAI models
type Lister struct {
	apiKey string
	client *openai.Client
}

// NewLister creates a new model lister
func NewLister(apiKey string) *Lister {
	return &Lister{
		apiKey: apiKey,
		client: openai.NewClient(apiKey),
	}
}

// ListAvailableModels lists all available OpenAI models categorized by type
func (l *Lister) ListAvailableModels() error {
	if l.apiKey == "" {
		return fmt.Errorf("OpenAI API key not found. Set OPENAI_API_KEY environment variable or configure in .totalrecall.yaml")
	}

	// List models
	ctx := context.Background()
	models, err := l.client.ListModels(ctx)
	if err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}

	// Categorize models
	ttsModels := []string{}
	imageModels := []string{}
	chatModels := []string{}

	for _, model := range models.Models {
		modelID := model.ID
		if strings.Contains(modelID, "tts") || strings.Contains(modelID, "audio") {
			ttsModels = append(ttsModels, modelID)
		} else if strings.Contains(modelID, "dall-e") {
			imageModels = append(imageModels, modelID)
		} else if strings.Contains(modelID, "gpt") || strings.Contains(modelID, "chat") {
			chatModels = append(chatModels, modelID)
		}
	}

	// Sort models
	sort.Strings(ttsModels)
	sort.Strings(imageModels)
	sort.Strings(chatModels)

	// Print models
	fmt.Println("Available OpenAI Models:")
	fmt.Println("\nText-to-Speech (TTS) Models:")
	if len(ttsModels) == 0 {
		fmt.Println("  No TTS models found")
	} else {
		for _, model := range ttsModels {
			fmt.Printf("  %s\n", model)
		}
	}

	fmt.Println("\nImage Generation Models:")
	if len(imageModels) == 0 {
		fmt.Println("  No image models found")
	} else {
		for _, model := range imageModels {
			fmt.Printf("  %s\n", model)
		}
	}

	fmt.Println("\nChat/Translation Models (for Bulgarian translation):")
	if len(chatModels) > 10 {
		// Show only relevant models
		relevantModels := []string{}
		for _, model := range chatModels {
			if strings.Contains(model, "gpt-4") || strings.Contains(model, "gpt-3.5") {
				relevantModels = append(relevantModels, model)
			}
		}
		for _, model := range relevantModels {
			fmt.Printf("  %s\n", model)
		}
		fmt.Printf("  ... and %d more models\n", len(chatModels)-len(relevantModels))
	} else {
		for _, model := range chatModels {
			fmt.Printf("  %s\n", model)
		}
	}

	return nil
}
