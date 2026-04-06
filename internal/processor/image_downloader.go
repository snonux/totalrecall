package processor

// image_downloader.go delegates image downloading to the image package.
// It builds provider-specific ImageClient instances from flags/config and
// then calls image.Downloader to handle the actual HTTP download and
// attribution writing. This file implements the image-downloading concern
// so that processor.go can focus on the high-level word-processing flow.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/snonux/totalrecall/internal/cli"
	"codeberg.org/snonux/totalrecall/internal/image"
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
	p.registerPromptCallback(searcher, wordDir)

	_, path, err := downloader.DownloadBestMatchWithOptions(ctx, searchOpts)
	if err != nil {
		return err
	}
	fmt.Printf("    Downloaded: %s\n", path)

	// Persist the final prompt used by the searcher (some providers set it
	// only after the search call; this handles that case as a fallback).
	p.saveImagePrompt(wordDir, searcher)

	return nil
}

// registerPromptCallback wires a prompt-save callback into searchers that
// support SetPromptCallback. The callback fires during the Search call so the
// prompt is captured even if the subsequent download fails.
func (p *Processor) registerPromptCallback(searcher image.ImageClient, wordDir string) {
	type promptSetter interface {
		SetPromptCallback(func(prompt string))
	}
	promptAware, ok := searcher.(promptSetter)
	if !ok {
		return
	}

	promptFile := filepath.Join(wordDir, "image_prompt.txt")
	promptAware.SetPromptCallback(func(prompt string) {
		if prompt == "" {
			return
		}
		if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
			fmt.Printf("  Warning: Failed to save image prompt: %v\n", err)
		}
	})
}

// saveImagePrompt persists the last prompt used by a searcher that implements
// GetLastPrompt. This acts as a fallback when the prompt is not available via
// the callback during the search call itself.
func (p *Processor) saveImagePrompt(wordDir string, searcher image.ImageClient) {
	type promptGetter interface {
		GetLastPrompt() string
	}

	promptSource, ok := searcher.(promptGetter)
	if !ok {
		return
	}

	usedPrompt := promptSource.GetLastPrompt()
	if usedPrompt == "" {
		return
	}

	promptFile := filepath.Join(wordDir, "image_prompt.txt")
	if err := os.WriteFile(promptFile, []byte(usedPrompt), 0644); err != nil {
		fmt.Printf("  Warning: Failed to save image prompt: %v\n", err)
	}
}

// newImageSearcher creates the appropriate ImageClient based on the configured
// image provider (openai or nanobanana).
func (p *Processor) newImageSearcher() (image.ImageClient, error) {
	switch p.imageProviderForRunMode() {
	case "openai":
		return p.newOpenAIImageSearcher()
	case "nanobanana":
		return p.newNanoBananaImageSearcher()
	default:
		return nil, fmt.Errorf("unknown image provider: %s", p.imageProviderForRunMode())
	}
}

// imageProviderForRunMode resolves the image provider, giving precedence to
// the CLI flag when it was explicitly set, then the viper config value.
func (p *Processor) imageProviderForRunMode() string {
	if p.flags.ImageAPISpecified {
		return strings.ToLower(strings.TrimSpace(p.flags.ImageAPI))
	}
	if p.viperCfg.imageProvider != "" {
		return p.viperCfg.imageProvider
	}
	return strings.ToLower(strings.TrimSpace(p.flags.ImageAPI))
}

// newOpenAIImageSearcher builds an OpenAI ImageClient from flags and viper
// config. Config-file overrides are applied only when the flag still holds its
// default value so explicit CLI flags always win.
func (p *Processor) newOpenAIImageSearcher() (image.ImageClient, error) {
	openaiConfig := &image.OpenAIConfig{
		APIKey:  cli.GetOpenAIKey(),
		Model:   p.flags.OpenAIImageModel,
		Size:    p.flags.OpenAIImageSize,
		Quality: p.flags.OpenAIImageQuality,
		Style:   p.flags.OpenAIImageStyle,
	}

	// Apply viper overrides when CLI flag holds its zero/default value.
	if p.flags.OpenAIImageModel == "dall-e-2" && p.viperCfg.imageOpenAIModelSet {
		openaiConfig.Model = p.viperCfg.imageOpenAIModel
	}
	if p.flags.OpenAIImageSize == "512x512" && p.viperCfg.imageOpenAISizeSet {
		openaiConfig.Size = p.viperCfg.imageOpenAISize
	}
	if p.flags.OpenAIImageQuality == "standard" && p.viperCfg.imageOpenAIQualitySet {
		openaiConfig.Quality = p.viperCfg.imageOpenAIQuality
	}
	if p.flags.OpenAIImageStyle == "natural" && p.viperCfg.imageOpenAIStyleSet {
		openaiConfig.Style = p.viperCfg.imageOpenAIStyle
	}

	if openaiConfig.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required for image generation")
	}

	return p.newOpenAIImageClient(openaiConfig), nil
}

// newNanoBananaImageSearcher builds a NanoBanana ImageClient from flags and
// viper config, applying overrides in the same flag-wins-over-config pattern.
func (p *Processor) newNanoBananaImageSearcher() (image.ImageClient, error) {
	nanoBananaConfig := &image.NanoBananaConfig{
		APIKey:    cli.GetGoogleAPIKey(),
		Model:     p.flags.NanoBananaModel,
		TextModel: p.flags.NanoBananaTextModel,
	}

	if !p.flags.NanoBananaModelSpecified && p.viperCfg.imageNanoBananaModelSet {
		nanoBananaConfig.Model = p.viperCfg.imageNanoBananaModel
	}
	if !p.flags.NanoBananaTextModelSpecified && p.viperCfg.imageNanoBananaTextModelSet {
		nanoBananaConfig.TextModel = p.viperCfg.imageNanoBananaTextModel
	}

	if nanoBananaConfig.APIKey == "" {
		return nil, fmt.Errorf("google API key is required for image generation")
	}

	return p.newNanoBananaImageClient(nanoBananaConfig), nil
}
