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

	"codeberg.org/snonux/totalrecall/internal/httpctx"
)

// ModelLister lists available OpenAI and Gemini models to the configured
// writer. *Lister satisfies this interface.
type ModelLister interface {
	ListAvailableModels() error
}

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

var _ ModelLister = (*Lister)(nil)

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
		lister.openAIClient = httpctx.NewOpenAIClient(lister.openAIKey)
	}

	if lister.geminiKey != "" {
		client, err := httpctx.NewGenAIClient(context.Background(), &genai.ClientConfig{
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

	if err := l.writeLine("Available Models:"); err != nil {
		return err
	}

	printedSection := false
	if l.openAIKey != "" {
		if err := l.printOpenAIModels(); err != nil {
			return err
		}
		printedSection = true
	}

	if l.geminiKey != "" {
		if printedSection {
			if err := l.writeLine(""); err != nil {
				return err
			}
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

	ctx, cancel := context.WithTimeout(context.Background(), httpctx.ListModelsTimeout)
	defer cancel()

	models, err := l.openAIClient.ListModels(ctx)
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

	if err := l.writeLine("OpenAI Models:"); err != nil {
		return err
	}
	if err := l.writeLine("  Text-to-Speech (TTS) Models:"); err != nil {
		return err
	}
	if len(ttsModels) == 0 {
		if err := l.writeLine("    No TTS models found"); err != nil {
			return err
		}
	} else {
		for _, model := range ttsModels {
			if err := l.writeLine("    " + model); err != nil {
				return err
			}
		}
	}

	if err := l.writeLine("  Image Generation Models:"); err != nil {
		return err
	}
	if len(imageModels) == 0 {
		if err := l.writeLine("    No image models found"); err != nil {
			return err
		}
	} else {
		for _, model := range imageModels {
			if err := l.writeLine("    " + model); err != nil {
				return err
			}
		}
	}

	if err := l.writeLine("  Chat/Translation Models (for Bulgarian translation):"); err != nil {
		return err
	}
	if len(chatModels) > 10 {
		// Show only relevant models
		relevantModels := []string{}
		for _, model := range chatModels {
			if strings.Contains(model, "gpt-4") || strings.Contains(model, "gpt-3.5") {
				relevantModels = append(relevantModels, model)
			}
		}
		for _, model := range relevantModels {
			if err := l.writeLine("    " + model); err != nil {
				return err
			}
		}
		if err := l.writeFormatted("    ... and %d more models\n", len(chatModels)-len(relevantModels)); err != nil {
			return err
		}
	} else {
		for _, model := range chatModels {
			if err := l.writeLine("    " + model); err != nil {
				return err
			}
		}
	}

	return nil
}

func (l *Lister) printGeminiModels() error {
	if l.geminiInitErr != nil {
		return fmt.Errorf("failed to initialize Gemini client: %w", l.geminiInitErr)
	}
	if l.geminiClient == nil {
		return fmt.Errorf("gemini client not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), httpctx.ListModelsTimeout)
	defer cancel()

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

	if err := l.writeLine("Gemini Models:"); err != nil {
		return err
	}
	if len(geminiModels) == 0 {
		if err := l.writeLine("  No Gemini models found"); err != nil {
			return err
		}
		return nil
	}

	for _, model := range geminiModels {
		if err := l.writeLine("  " + model); err != nil {
			return err
		}
	}

	return nil
}

func (l *Lister) writeLine(text string) error {
	if _, err := fmt.Fprintln(l.out, text); err != nil {
		return fmt.Errorf("write model list output: %w", err)
	}

	return nil
}

func (l *Lister) writeFormatted(format string, args ...any) error {
	if _, err := fmt.Fprintf(l.out, format, args...); err != nil {
		return fmt.Errorf("write model list output: %w", err)
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
