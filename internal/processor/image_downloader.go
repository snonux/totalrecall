package processor

// image_downloader.go delegates image downloading to the image package.
// It builds provider-specific ImageClient instances from flags/config and
// then calls image.Downloader to handle the actual HTTP download and
// attribution writing. This file implements the image-downloading concern
// so that processor.go can focus on the high-level word-processing flow.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/snonux/totalrecall/internal/cli"
	"codeberg.org/snonux/totalrecall/internal/image"
	"codeberg.org/snonux/totalrecall/internal/registry"
)

// downloadImagesWithTranslation downloads images for a word into its card
// directory. The translation is forwarded to the search options so that
// AI image providers can generate more contextually accurate images.
// ctx is passed to the image downloader so the caller's deadline applies.
func (p *Processor) downloadImagesWithTranslation(ctx context.Context, word, translationText string) error {
	searcher, err := p.newImageSearcher()
	if err != nil {
		return err
	}

	wordDir := p.findOrCreateWordDirectory(word)

	downloader := image.NewDownloader(searcher, &image.DownloadOptions{
		OutputDir:         wordDir,
		OverwriteExisting: true,
		CreateDir:         true,
		FileNamePattern:   "image",
		MaxSizeBytes:      5 * 1024 * 1024, // 5 MB limit
	})

	searchOpts := image.DefaultSearchOptions(word)
	if translationText != "" {
		searchOpts.Translation = translationText
	}

	// Register a prompt callback so the AI-generated prompt is persisted
	// to disk before the download completes (used by the GUI and for debugging).
	var promptSaveErr error
	p.registerPromptCallback(searcher, wordDir, &promptSaveErr)

	_, path, err := downloader.DownloadBestMatchWithOptions(ctx, searchOpts)
	if err != nil {
		return errors.Join(err, promptSaveErr)
	}
	fmt.Printf("    Downloaded: %s\n", path)

	if promptSaveErr != nil {
		return promptSaveErr
	}

	// Persist the final prompt used by the searcher (some providers set it
	// only after the search call; this handles that case as a fallback).
	if err := p.saveImagePrompt(wordDir, searcher); err != nil {
		return err
	}

	return nil
}

// registerPromptCallback wires a prompt-save callback into the searcher. The
// callback fires during the Search call so the prompt is captured even if the
// subsequent download fails. All searchers returned by newImageSearcher
// implement image.PromptAwareClient, so no type-assertion is needed.
// promptErr accumulates write failures so downloadImagesWithTranslation can
// return them to the caller instead of only logging.
func (p *Processor) registerPromptCallback(searcher image.PromptAwareClient, wordDir string, promptErr *error) {
	promptFile := filepath.Join(wordDir, "image_prompt.txt")
	searcher.SetPromptCallback(func(prompt string) {
		if prompt == "" {
			return
		}
		if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
			if promptErr != nil {
				*promptErr = errors.Join(*promptErr, fmt.Errorf("failed to save image prompt: %w", err))
			}
		}
	})
}

// saveImagePrompt persists the last prompt used by a searcher that implements
// GetLastPrompt. This acts as a fallback when the prompt is not available via
// the callback during the search call itself. The local promptGetter interface
// is intentionally narrow: not all PromptAwareClients expose GetLastPrompt.
func (p *Processor) saveImagePrompt(wordDir string, searcher image.PromptAwareClient) error {
	type promptGetter interface {
		GetLastPrompt() string
	}

	promptSource, ok := searcher.(promptGetter)
	if !ok {
		return nil
	}

	usedPrompt := promptSource.GetLastPrompt()
	if usedPrompt == "" {
		return nil
	}

	promptFile := filepath.Join(wordDir, "image_prompt.txt")
	if err := os.WriteFile(promptFile, []byte(usedPrompt), 0644); err != nil {
		return fmt.Errorf("failed to save image prompt: %w", err)
	}
	return nil
}

// processorImageClientFactories maps run-mode image provider name to builder.
// Register new backends here instead of extending a switch in newImageSearcher.
var processorImageClientFactories = func() *registry.Registry[string, func(*Processor) (image.PromptAwareClient, error)] {
	r := registry.New[string, func(*Processor) (image.PromptAwareClient, error)]()
	r.Register(image.ImageProviderOpenAI, (*Processor).newOpenAIImageSearcher)
	r.Register(image.ImageProviderNanoBanana, (*Processor).newNanoBananaImageSearcher)
	return r
}()

// newImageSearcher creates the appropriate PromptAwareClient based on the
// configured image provider (openai or nanobanana). Returning PromptAwareClient
// instead of ImageClient means callers can call SetPromptCallback directly
// without a type-assertion.
func (p *Processor) newImageSearcher() (image.PromptAwareClient, error) {
	key := strings.ToLower(strings.TrimSpace(p.imageProviderForRunMode()))
	fn, ok := processorImageClientFactories.Get(key)
	if !ok {
		return nil, fmt.Errorf("unknown image provider: %s", p.imageProviderForRunMode())
	}
	return fn(p)
}

// imageProviderForRunMode resolves the image provider, giving precedence to
// the CLI flag when it was explicitly set, then the config-file value.
func (p *Processor) imageProviderForRunMode() string {
	if p.flags.ImageAPISpecified {
		return strings.ToLower(strings.TrimSpace(p.flags.ImageAPI))
	}
	if p.cfg.ImageProvider != "" {
		return p.cfg.ImageProvider
	}
	return strings.ToLower(strings.TrimSpace(p.flags.ImageAPI))
}

// newOpenAIImageSearcher builds an OpenAI PromptAwareClient from CLI flags and
// the resolved processor Config. Config-file overrides are applied only when
// the flag still holds its default value so explicit CLI flags always win.
func (p *Processor) newOpenAIImageSearcher() (image.PromptAwareClient, error) {
	openaiConfig := &image.OpenAIConfig{
		APIKey:  cli.GetOpenAIKey(),
		Model:   p.flags.OpenAIImageModel,
		Size:    p.flags.OpenAIImageSize,
		Quality: p.flags.OpenAIImageQuality,
		Style:   p.flags.OpenAIImageStyle,
	}

	// Apply config-file overrides when CLI flag holds its zero/default value.
	if p.flags.OpenAIImageModel == "dall-e-2" && p.cfg.ImageOpenAIModelSet {
		openaiConfig.Model = p.cfg.ImageOpenAIModel
	}
	if p.flags.OpenAIImageSize == "512x512" && p.cfg.ImageOpenAISizeSet {
		openaiConfig.Size = p.cfg.ImageOpenAISize
	}
	if p.flags.OpenAIImageQuality == "standard" && p.cfg.ImageOpenAIQualitySet {
		openaiConfig.Quality = p.cfg.ImageOpenAIQuality
	}
	if p.flags.OpenAIImageStyle == "natural" && p.cfg.ImageOpenAIStyleSet {
		openaiConfig.Style = p.cfg.ImageOpenAIStyle
	}

	if openaiConfig.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required for image generation")
	}

	return p.imageFactories.NewOpenAIClient(openaiConfig), nil
}

// newNanoBananaImageSearcher builds a NanoBanana PromptAwareClient from CLI
// flags and the resolved processor Config, applying overrides in the same
// flag-wins-over-config pattern.
func (p *Processor) newNanoBananaImageSearcher() (image.PromptAwareClient, error) {
	nanoBananaConfig := &image.NanoBananaConfig{
		APIKey:    cli.GetGoogleAPIKey(),
		Model:     p.flags.NanoBananaModel,
		TextModel: p.flags.NanoBananaTextModel,
	}

	if !p.flags.NanoBananaModelSpecified && p.cfg.ImageNanoBananaModelSet {
		nanoBananaConfig.Model = p.cfg.ImageNanoBananaModel
	}
	if !p.flags.NanoBananaTextModelSpecified && p.cfg.ImageNanoBananaTextModelSet {
		nanoBananaConfig.TextModel = p.cfg.ImageNanoBananaTextModel
	}

	if nanoBananaConfig.APIKey == "" {
		return nil, fmt.Errorf("google API key is required for image generation")
	}

	return p.imageFactories.NewNanoBananaClient(nanoBananaConfig), nil
}
