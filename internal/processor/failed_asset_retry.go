package processor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codeberg.org/snonux/totalrecall/internal"
	"codeberg.org/snonux/totalrecall/internal/anki"
	"codeberg.org/snonux/totalrecall/internal/audio"
	"codeberg.org/snonux/totalrecall/internal/store"
)

type failedAssetKind string

const (
	failedAssetAudio          failedAssetKind = "audio"
	failedAssetImage          failedAssetKind = "image"
	failedAssetBgBgAudioPair  failedAssetKind = "front and back audio"
	failedAssetBgBgFrontAudio failedAssetKind = "front audio"
	failedAssetBgBgBackAudio  failedAssetKind = "back audio"
)

type failedAssetPlan struct {
	Card        store.CardDirectory
	CardType    internal.CardType
	Translation string
	ImagePrompt string
	Assets      []failedAssetKind
}

// RetryFailedAssets scans the existing card output directory for incomplete or
// failed asset generations and retries them in deterministic order. The retry
// loop stops immediately on the first fresh error so users can rerun the same
// command after an upstream rate limit clears.
func (p *Processor) RetryFailedAssets() error {
	if err := os.MkdirAll(p.Flags.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	plans, err := p.scanFailedAssetPlans()
	if err != nil {
		return err
	}
	if len(plans) == 0 {
		fmt.Printf("No failed assets found in: %s\n", p.Flags.OutputDir)
		return nil
	}

	totalAssets := 0
	for _, plan := range plans {
		totalAssets += len(plan.Assets)
	}

	fmt.Printf("Found %d failed asset(s) across %d card(s) in %s\n", totalAssets, len(plans), p.Flags.OutputDir)

	regenerated := 0
	for _, plan := range plans {
		fmt.Printf("\nRetrying card: %s\n", plan.Card.Word)
		for _, asset := range plan.Assets {
			fmt.Printf("  Regenerating %s...\n", asset)

			assetCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			err := p.regenerateFailedAsset(assetCtx, plan, asset)
			cancel()
			if err != nil {
				return fmt.Errorf("stopped after %d successful regeneration(s); %s for %q failed: %w", regenerated, asset, plan.Card.Word, err)
			}

			regenerated++
		}
	}

	fmt.Printf("\nRegenerated %d failed asset(s).\n", regenerated)
	return nil
}

func (p *Processor) scanFailedAssetPlans() ([]failedAssetPlan, error) {
	cards := p.cardStore.ListCardDirectories(nil)
	plans := make([]failedAssetPlan, 0, len(cards))

	for _, card := range cards {
		plan := p.buildFailedAssetPlan(card)
		if len(plan.Assets) == 0 {
			continue
		}
		plans = append(plans, plan)
	}

	return plans, nil
}

func (p *Processor) buildFailedAssetPlan(card store.CardDirectory) failedAssetPlan {
	plan := failedAssetPlan{
		Card:        card,
		CardType:    internal.LoadCardType(card.Path),
		Translation: readStoredTranslation(card.Path),
		ImagePrompt: readStoredImagePrompt(card.Path),
	}

	if !p.Flags.SkipAudio {
		if plan.CardType.IsBgBg() {
			frontReady := audioAssetReady(card.Path, "audio_front", p.EffectiveAudioFormat())
			backReady := audioAssetReady(card.Path, "audio_back", p.EffectiveAudioFormat())

			switch {
			case !frontReady && !backReady:
				plan.Assets = append(plan.Assets, failedAssetBgBgAudioPair)
			case !frontReady:
				plan.Assets = append(plan.Assets, failedAssetBgBgFrontAudio)
			case !backReady:
				plan.Assets = append(plan.Assets, failedAssetBgBgBackAudio)
			}
		} else if !audioAssetReady(card.Path, "audio", p.EffectiveAudioFormat()) {
			plan.Assets = append(plan.Assets, failedAssetAudio)
		}
	}

	if !p.Flags.SkipImages && !imageAssetReady(card.Path) {
		plan.Assets = append(plan.Assets, failedAssetImage)
	}

	return plan
}

func (p *Processor) regenerateFailedAsset(ctx context.Context, plan failedAssetPlan, asset failedAssetKind) error {
	switch asset {
	case failedAssetAudio:
		return p.generateAudio(ctx, plan.Card.Word)
	case failedAssetImage:
		return p.downloadImagesWithPrompt(ctx, plan.Card.Word, plan.Translation, plan.ImagePrompt)
	case failedAssetBgBgAudioPair:
		if strings.TrimSpace(plan.Translation) == "" {
			return fmt.Errorf("missing back-side text in translation.txt")
		}
		return p.generateAudioBgBg(ctx, plan.Card.Word, plan.Translation)
	case failedAssetBgBgFrontAudio:
		return p.generateCardAudioSideInDir(ctx, plan.Card.Word, plan.Card.Path, "audio_front", "front audio")
	case failedAssetBgBgBackAudio:
		if strings.TrimSpace(plan.Translation) == "" {
			return fmt.Errorf("missing back-side text in translation.txt")
		}
		return p.generateCardAudioSideInDir(ctx, plan.Translation, plan.Card.Path, "audio_back", "back audio")
	default:
		return fmt.Errorf("unknown failed asset kind %q", asset)
	}
}

func (p *Processor) generateCardAudioSideInDir(ctx context.Context, text, wordDir, filenameBase, label string) error {
	provider := p.AudioProviderName()
	voice := p.audioVoiceForProvider()
	p.logSelectedAudioVoice(provider, voice)

	run := func(candidate string) error {
		if candidate != voice {
			fmt.Printf("  Retrying Gemini audio with voice: %s\n", candidate)
		}
		fmt.Printf("  Generating %s for '%s'...\n", label, text)
		return p.generateAudioWithVoiceAndFilenameInDir(ctx, text, candidate, filenameBase, wordDir)
	}

	if provider == "gemini" && p.GeminiVoice() == "" {
		_, err := audio.RunWithVoiceFallbacks(voice, run, func(candidate string) {
			fmt.Printf("  Warning: Gemini returned no audio for voice %s\n", candidate)
		})
		return err
	}

	return run(voice)
}

func audioAssetReady(wordDir, baseName, preferredFormat string) bool {
	paths := anki.ResolveAudioPaths(wordDir, baseName, preferredFormat)
	if len(paths) == 0 {
		return false
	}

	for _, path := range paths {
		if !fileExistsAndNonEmpty(path) || !fileExistsAndNonEmpty(audio.AttributionPath(path)) {
			return false
		}
	}

	return fileExistsAndNonEmpty(filepath.Join(wordDir, "audio_metadata.txt"))
}

func imageAssetReady(wordDir string) bool {
	if firstUsableImagePath(wordDir) == "" {
		return false
	}
	if !fileExistsAndNonEmpty(filepath.Join(wordDir, "image_attribution.txt")) {
		return false
	}
	return readStoredImagePrompt(wordDir) != ""
}

func firstUsableImagePath(wordDir string) string {
	imagePatterns := []string{
		"image_*.jpg",
		"image_*.png",
		"image_*.webp",
		"image.jpg",
		"image.png",
		"image.webp",
	}

	for _, pattern := range imagePatterns {
		if strings.Contains(pattern, "*") {
			matches, _ := filepath.Glob(filepath.Join(wordDir, pattern))
			for _, match := range matches {
				if fileExistsAndNonEmpty(match) {
					return match
				}
			}
			continue
		}

		path := filepath.Join(wordDir, pattern)
		if fileExistsAndNonEmpty(path) {
			return path
		}
	}

	return ""
}

func readStoredTranslation(wordDir string) string {
	data, err := os.ReadFile(filepath.Join(wordDir, "translation.txt"))
	if err != nil {
		return ""
	}

	parts := strings.SplitN(string(data), "=", 2)
	if len(parts) != 2 {
		return strings.TrimSpace(string(data))
	}

	return strings.TrimSpace(parts[1])
}

func readStoredImagePrompt(wordDir string) string {
	data, err := os.ReadFile(filepath.Join(wordDir, "image_prompt.txt"))
	if err != nil {
		return ""
	}

	prompt := strings.TrimSpace(string(data))
	if prompt == "" || looksLikeFailedPrompt(prompt) {
		return ""
	}

	return prompt
}

func looksLikeFailedPrompt(prompt string) bool {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	if lower == "" {
		return true
	}

	markers := []string{
		"rate limit",
		"too many requests",
		"quota exceeded",
		"resource exhausted",
		"failed to generate",
		"generation failed",
		"temporarily unavailable",
		"http 429",
	}

	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}

	return false
}

func fileExistsAndNonEmpty(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Size() > 0
}
