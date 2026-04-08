package processor

// AudioCoordinator assembles audio provider configurations, selects voices,
// generates audio files, and writes attribution/metadata sidecars. It reads
// audio-related values from the processor's Config struct (resolved once at
// startup) and the CLI flags, so the main Processor struct does not need to
// deal with audio details directly.
//
// All methods are on *Processor rather than a separate struct to avoid an
// extra layer of indirection while still keeping the concerns separated into
// their own file (SRP at the file level, as recommended for Go packages).
// Audio provider name and format resolution live on CLIConfigResolver (embedded
// on Processor).

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codeberg.org/snonux/totalrecall/internal/audio"
	"codeberg.org/snonux/totalrecall/internal/cli"
)

// audioVoicesForProvider returns all available voices for the configured provider
// without requiring a Provider instance (uses the package-level VoicesFor helper).
func (p *Processor) audioVoicesForProvider() []string {
	return audio.VoicesFor(p.AudioProviderName())
}

// audioVoiceForProvider selects a single voice for the configured provider.
// If a specific voice is configured, it is returned; otherwise a random voice
// from the provider's list is chosen using the injected randomIntn function.
func (p *Processor) audioVoiceForProvider() string {
	switch p.AudioProviderName() {
	case "gemini":
		if voice := p.GeminiVoice(); voice != "" {
			return voice
		}
		voices := p.audioVoicesForProvider()
		if p.randomIntn != nil {
			return voices[p.randomIntn(len(voices))]
		}
		return voices[rand.Intn(len(voices))]
	default:
		if voice := p.OpenAIVoice(); voice != "" {
			return voice
		}
		voices := p.audioVoicesForProvider()
		if p.randomIntn != nil {
			return voices[p.randomIntn(len(voices))]
		}
		return voices[rand.Intn(len(voices))]
	}
}

// logSelectedAudioVoice prints which voice was selected and whether it was
// specified by the user or picked randomly. Used for informational output only.
func (p *Processor) logSelectedAudioVoice(provider, voice string) {
	switch provider {
	case "gemini":
		if p.GeminiVoice() != "" {
			fmt.Printf("  Using specified Gemini voice: %s\n", voice)
		} else {
			fmt.Printf("  Using random Gemini voice: %s\n", voice)
		}
	default:
		if p.OpenAIVoice() != "" {
			fmt.Printf("  Using specified voice: %s\n", voice)
		} else {
			fmt.Printf("  Using random voice: %s\n", voice)
		}
	}
}

// generateAudio generates audio files for a word using the configured provider.
// When AllVoices is set all provider voices are generated; otherwise a single
// voice is selected (with Gemini fallback retry on empty audio).
// ctx is threaded down to provider.GenerateAudio so the caller's deadline applies.
func (p *Processor) generateAudio(ctx context.Context, word string) error {
	provider := p.AudioProviderName()

	if p.Flags.AllVoices {
		return p.generateAudioForAllVoices(ctx, word)
	}

	voice := p.audioVoiceForProvider()
	p.logSelectedAudioVoice(provider, voice)

	// For Gemini with no explicit voice, use automatic fallback through all voices.
	if provider == "gemini" && p.GeminiVoice() == "" {
		_, err := audio.RunWithVoiceFallbacks(voice, func(candidate string) error {
			if candidate != voice {
				fmt.Printf("  Retrying Gemini audio with voice: %s\n", candidate)
			}
			return p.generateAudioWithVoice(ctx, word, candidate)
		}, func(candidate string) {
			fmt.Printf("  Warning: Gemini returned no audio for voice %s\n", candidate)
		})
		return err
	}

	return p.generateAudioWithVoice(ctx, word, voice)
}

// generateAudioForAllVoices iterates over every voice for the configured
// provider and generates a separate audio file for each.
func (p *Processor) generateAudioForAllVoices(ctx context.Context, word string) error {
	voices := p.audioVoicesForProvider()
	for i, voice := range voices {
		fmt.Printf("  Generating audio %d/%d (voice: %s)...\n", i+1, len(voices), voice)
		if err := p.generateAudioWithVoice(ctx, word, voice); err != nil {
			return fmt.Errorf("failed to generate audio with voice %s: %w", voice, err)
		}
	}
	return nil
}

// generateAudioBgBg generates audio files for both sides of a bg-bg card.
// Both audio files are saved to the same directory as the front-word card.
// ctx is threaded down to provider.GenerateAudio so the caller's deadline applies.
func (p *Processor) generateAudioBgBg(ctx context.Context, front, back string) error {
	provider := p.AudioProviderName()

	voice := p.audioVoiceForProvider()
	p.logSelectedAudioVoice(provider, voice)

	// Find or create the word directory ONCE (for the front word).
	// Both audio files will be saved to this same directory.
	wordDir := p.findOrCreateWordDirectory(front)

	generatePair := func(candidate string) error {
		fmt.Printf("  Generating front audio for '%s'...\n", front)
		if err := p.generateAudioWithVoiceAndFilenameInDir(ctx, front, candidate, "audio_front", wordDir); err != nil {
			return fmt.Errorf("failed to generate front audio: %w", err)
		}

		fmt.Printf("  Generating back audio for '%s'...\n", back)
		if err := p.generateAudioWithVoiceAndFilenameInDir(ctx, back, candidate, "audio_back", wordDir); err != nil {
			return fmt.Errorf("failed to generate back audio: %w", err)
		}

		return nil
	}

	// Use automatic fallback through all Gemini voices when no voice is pinned.
	if provider == "gemini" && p.GeminiVoice() == "" {
		_, err := audio.RunWithVoiceFallbacks(voice, func(candidate string) error {
			if candidate != voice {
				fmt.Printf("  Retrying Gemini audio with voice: %s\n", candidate)
			}
			return generatePair(candidate)
		}, func(candidate string) {
			fmt.Printf("  Warning: Gemini returned no audio for voice %s\n", candidate)
		})
		return err
	}

	return generatePair(voice)
}

// generateAudioWithVoice generates audio for a word with a specific voice,
// saving to the standard "audio" filename in the word's card directory.
func (p *Processor) generateAudioWithVoice(ctx context.Context, word, voice string) error {
	return p.generateAudioWithVoiceAndFilename(ctx, word, voice, "audio")
}

// generateAudioWithVoiceAndFilename generates audio and saves it using the
// given base filename (without extension) inside the word's card directory.
func (p *Processor) generateAudioWithVoiceAndFilename(ctx context.Context, word, voice, filenameBase string) error {
	wordDir := p.findOrCreateWordDirectory(word)
	return p.generateAudioWithVoiceAndFilenameInDir(ctx, word, voice, filenameBase, wordDir)
}

// generateAudioWithVoiceAndFilenameInDir is the core audio generation method.
// It assembles the provider config, creates the provider, runs TTS, and writes
// the audio file plus its attribution/metadata sidecars to wordDir.
// ctx is passed directly to provider.GenerateAudio so the caller's deadline applies.
func (p *Processor) generateAudioWithVoiceAndFilenameInDir(ctx context.Context, word, voice, filenameBase, wordDir string) error {
	providerConfig := p.buildAudioProviderConfig(voice)

	provider, err := p.newAudioProvider(providerConfig)
	if err != nil {
		return err
	}

	outputFile := p.buildAudioOutputPath(wordDir, filenameBase, voice, providerConfig.OutputFormat)

	if err := provider.GenerateAudio(ctx, word, outputFile); err != nil {
		return err
	}

	// Write attribution and metadata sidecars next to the audio file.
	if err := p.saveAudioAttribution(word, outputFile, providerConfig); err != nil {
		return fmt.Errorf("failed to save audio attribution: %w", err)
	}

	return nil
}

// buildAudioProviderConfig assembles an audio.Config from CLI flags and the
// resolved processor Config. The voice argument is the already-resolved voice string for this call.
func (p *Processor) buildAudioProviderConfig(voice string) *audio.Config {
	audioProvider := p.AudioProviderName()
	audioFormat := p.EffectiveAudioFormat()

	// Generate random speed between 0.90 and 1.00 if not explicitly set.
	speed := p.Flags.OpenAISpeed
	if audioProvider == "openai" && p.Flags.OpenAISpeed == 0.9 && !p.Config.OpenAISpeedSet {
		speed = 0.90 + rand.Float64()*0.10
	}

	providerConfig := audio.DefaultProviderConfig()
	providerConfig.Provider = audioProvider
	providerConfig.OutputDir = p.Flags.OutputDir
	providerConfig.OpenAIKey = cli.GetOpenAIKey()
	providerConfig.GoogleAPIKey = cli.GetGoogleAPIKey()

	switch audioProvider {
	case "gemini":
		providerConfig.OutputFormat = audioFormat
		providerConfig.GeminiTTSModel = p.GeminiTTSModel()
		if voice != "" {
			providerConfig.GeminiVoice = voice
		} else {
			providerConfig.GeminiVoice = p.GeminiVoice()
		}
		providerConfig.GeminiSpeed = 1.0
	default:
		p.applyOpenAIAudioConfig(providerConfig, voice, speed, audioFormat)
	}

	return providerConfig
}

// applyOpenAIAudioConfig populates the OpenAI-specific fields of providerConfig,
// applying config-file overrides where the CLI flag still holds its default value.
func (p *Processor) applyOpenAIAudioConfig(providerConfig *audio.Config, voice string, speed float64, audioFormat string) {
	providerConfig.OutputFormat = audioFormat
	providerConfig.OpenAIModel = p.Flags.OpenAIModel
	providerConfig.OpenAIVoice = voice
	providerConfig.OpenAISpeed = speed
	providerConfig.OpenAIInstruction = p.Flags.OpenAIInstruction

	// Override with config-file values when the CLI flag is still at its default.
	if p.Flags.OpenAIModel == "gpt-4o-mini-tts" && p.Config.OpenAIModelSet {
		providerConfig.OpenAIModel = p.Config.OpenAIModel
	}
	if p.Flags.OpenAISpeed == 0.9 && p.Config.OpenAISpeedSet {
		providerConfig.OpenAISpeed = p.Config.OpenAISpeed
	}
	if p.Flags.OpenAIInstruction == "" && p.Config.OpenAIInstructionSet {
		providerConfig.OpenAIInstruction = p.Config.OpenAIInstruction
	}
}

// buildAudioOutputPath constructs the output file path for an audio file.
// When AllVoices is set and filenameBase is "audio", the voice name is embedded
// in the filename to keep each voice's file distinct.
func (p *Processor) buildAudioOutputPath(wordDir, filenameBase, voice, outputFormat string) string {
	if p.Flags.AllVoices && filenameBase == "audio" {
		return filepath.Join(wordDir, fmt.Sprintf("%s_%s.%s", filenameBase, voice, outputFormat))
	}
	return filepath.Join(wordDir, fmt.Sprintf("%s.%s", filenameBase, outputFormat))
}

// saveAudioAttribution writes two sidecar files next to the audio file:
//   - <audioFile>.attribution.txt — human-readable attribution for the clip
//   - audio_metadata.txt          — machine-readable metadata for the GUI
func (p *Processor) saveAudioAttribution(word, audioFile string, config *audio.Config) error {
	processedText := audio.ProcessedTextForProvider(config.Provider, word)
	instruction := audio.InstructionForProvider(config.Provider, config)

	params := audio.AttributionParamsFrom(config, word, instruction, processedText, time.Now())
	attribution := audio.BuildAttributionFor(config.Provider, params)

	attrPath := audio.AttributionPath(audioFile)
	if err := os.WriteFile(attrPath, []byte(attribution), 0644); err != nil {
		return fmt.Errorf("failed to write audio attribution file: %w", err)
	}

	// Also save metadata for GUI display.
	wordDir := filepath.Dir(audioFile)
	metadataFile := filepath.Join(wordDir, "audio_metadata.txt")
	metadata := p.buildAudioMetadata(config, audioFile)
	if err := os.WriteFile(metadataFile, []byte(metadata), 0644); err != nil {
		return fmt.Errorf("failed to save audio metadata: %w", err)
	}

	return nil
}

// buildAudioMetadata constructs the sidecar metadata string for the given
// audio file, resolving front/back file hints for bg-bg cards.
func (p *Processor) buildAudioMetadata(config *audio.Config, audioFile string) string {
	audioFileHint, audioFileBackHint := p.audioMetadataFileHints(audioFile)
	return audio.BuildSidecarMetadata(audio.SidecarMetadataParams{
		Provider:          config.Provider,
		OutputFormat:      config.OutputFormat,
		AudioFile:         audioFileHint,
		AudioFileBack:     audioFileBackHint,
		OpenAIModel:       config.OpenAIModel,
		OpenAIVoice:       config.OpenAIVoice,
		OpenAISpeed:       config.OpenAISpeed,
		OpenAIInstruction: config.OpenAIInstruction,
		GeminiTTSModel:    config.GeminiTTSModel,
		GeminiVoice:       config.GeminiVoice,
		GeminiSpeed:       config.GeminiSpeed,
	})
}

// audioMetadataFileHints returns the (front, back) audio file path hints for
// the sidecar metadata. For a standard "audio" file the back hint is empty;
// for bg-bg front/back pairs both hints are returned when both files exist.
func (p *Processor) audioMetadataFileHints(audioFile string) (string, string) {
	if strings.TrimSpace(audioFile) == "" {
		return "", ""
	}

	wordDir := filepath.Dir(audioFile)
	base := filepath.Base(audioFile)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	switch name {
	case "audio":
		return audioFile, ""
	case "audio_front":
		backFile := filepath.Join(wordDir, "audio_back"+ext)
		if _, err := os.Stat(backFile); err == nil {
			return audioFile, backFile
		}
		return audioFile, ""
	case "audio_back":
		frontFile := filepath.Join(wordDir, "audio_front"+ext)
		return frontFile, audioFile
	default:
		return audioFile, ""
	}
}
