package models

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/sashabaranov/go-openai"
	"google.golang.org/genai"
)

type openAIModelLister interface {
	ListModels(context.Context) (openai.ModelsList, error)
}

type geminiModelLister interface {
	List(context.Context, *genai.ListModelsConfig) (genai.Page[genai.Model], error)
}

// Lister handles listing available OpenAI and Gemini models.
type Lister struct {
	openAIKey     string
	geminiKey     string
	openAIClient  openAIModelLister
	geminiClient  geminiModelLister
	geminiInitErr error
	out           io.Writer
}

// NewLister creates a new model lister.
func NewLister(openAIKey, geminiKey string, out io.Writer) *Lister {
	lister := &Lister{
		openAIKey: strings.TrimSpace(openAIKey),
		geminiKey: strings.TrimSpace(geminiKey),
		out:       out,
	}

	if lister.out == nil {
		lister.out = os.Stdout
	}

	if lister.openAIKey != "" {
		lister.openAIClient = openai.NewClient(lister.openAIKey)
	}

	if lister.geminiKey != "" {
		client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
			APIKey: lister.geminiKey,
		})
		if err != nil {
			lister.geminiInitErr = err
		} else {
			lister.geminiClient = client.Models
		}
	}

	return lister
}

// ListAvailableModels lists all available OpenAI and Gemini models categorized by provider.
func (l *Lister) ListAvailableModels() error {
	if l.openAIKey == "" && l.geminiKey == "" {
		return fmt.Errorf("no API keys found. Set OPENAI_API_KEY and/or GOOGLE_API_KEY environment variable(s) or configure them in .totalrecall.yaml")
	}

	fmt.Fprintln(l.out, "Available Models:")

	printedSection := false
	if l.openAIKey != "" {
		if err := l.printOpenAIModels(); err != nil {
			return err
		}
		printedSection = true
	}

	if l.geminiKey != "" {
		if printedSection {
			fmt.Fprintln(l.out)
		}
		if err := l.printGeminiModels(); err != nil {
			return err
		}
	}

	return nil
}

func (l *Lister) printOpenAIModels() error {
	if l.openAIClient == nil {
		return fmt.Errorf("OpenAI client not initialized")
	}

	models, err := l.openAIClient.ListModels(context.Background())
	if err != nil {
		return fmt.Errorf("failed to list OpenAI models: %w", err)
	}

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

	fmt.Fprintln(l.out, "OpenAI Models:")
	fmt.Fprintln(l.out, "  Text-to-Speech (TTS) Models:")
	if len(ttsModels) == 0 {
		fmt.Fprintln(l.out, "    No TTS models found")
	} else {
		for _, model := range ttsModels {
			fmt.Fprintf(l.out, "    %s\n", model)
		}
	}

	fmt.Fprintln(l.out, "  Image Generation Models:")
	if len(imageModels) == 0 {
		fmt.Fprintln(l.out, "    No image models found")
	} else {
		for _, model := range imageModels {
			fmt.Fprintf(l.out, "    %s\n", model)
		}
	}

	fmt.Fprintln(l.out, "  Chat/Translation Models (for Bulgarian translation):")
	if len(chatModels) > 10 {
		// Show only relevant models
		relevantModels := []string{}
		for _, model := range chatModels {
			if strings.Contains(model, "gpt-4") || strings.Contains(model, "gpt-3.5") {
				relevantModels = append(relevantModels, model)
			}
		}
		for _, model := range relevantModels {
			fmt.Fprintf(l.out, "    %s\n", model)
		}
		fmt.Fprintf(l.out, "    ... and %d more models\n", len(chatModels)-len(relevantModels))
	} else {
		for _, model := range chatModels {
			fmt.Fprintf(l.out, "    %s\n", model)
		}
	}

	return nil
}

func (l *Lister) printGeminiModels() error {
	if l.geminiInitErr != nil {
		return fmt.Errorf("failed to initialize Gemini client: %w", l.geminiInitErr)
	}
	if l.geminiClient == nil {
		return fmt.Errorf("Gemini client not initialized")
	}

	ctx := context.Background()
	config := &genai.ListModelsConfig{
		QueryBase: genai.Ptr(true),
	}

	geminiModels := []string{}
	for {
		models, err := l.geminiClient.List(ctx, config)
		if err != nil {
			return fmt.Errorf("failed to list Gemini models: %w", err)
		}

		geminiModels = append(geminiModels, collectGeminiModelIDs(models)...)
		if models.NextPageToken == "" {
			break
		}

		config.PageToken = models.NextPageToken
	}

	sort.Strings(geminiModels)

	fmt.Fprintln(l.out, "Gemini Models:")
	if len(geminiModels) == 0 {
		fmt.Fprintln(l.out, "  No Gemini models found")
		return nil
	}

	for _, model := range geminiModels {
		fmt.Fprintf(l.out, "  %s\n", model)
	}

	return nil
}

func collectGeminiModelIDs(page genai.Page[genai.Model]) []string {
	modelIDs := make([]string, 0, len(page.Items))
	for _, model := range page.Items {
		if model == nil {
			continue
		}

		modelID := strings.TrimPrefix(strings.TrimSpace(model.Name), "models/")
		if modelID == "" {
			modelID = strings.TrimSpace(model.DisplayName)
		}
		if modelID != "" {
			modelIDs = append(modelIDs, modelID)
		}
	}

	return modelIDs
}
