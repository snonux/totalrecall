package audio

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SidecarMetadataParams describes the metadata written alongside generated audio.
type SidecarMetadataParams struct {
	Provider      string
	OutputFormat  string
	CardType      string
	AudioFile     string
	AudioFileBack string

	OpenAIModel       string
	OpenAIVoice       string
	OpenAISpeed       float64
	OpenAIInstruction string

	GeminiTTSModel string
	GeminiVoice    string
	GeminiSpeed    float64
}

// ProcessedTextForWord returns the sanitized text sent to TTS providers.
func ProcessedTextForWord(text string) string {
	cleanedText := strings.TrimSpace(text)
	punctuationToRemove := []string{"!", "?", ".", ",", ";", ":", "\"", "'", "(", ")", "[", "]", "{", "}", "-", "—", "–"}
	for _, punct := range punctuationToRemove {
		cleanedText = strings.ReplaceAll(cleanedText, punct, "")
	}

	return fmt.Sprintf("%s...", strings.TrimSpace(cleanedText))
}

// BuildSidecarMetadata formats the audio metadata written for GUI reloads and batch exports.
func BuildSidecarMetadata(params SidecarMetadataParams) string {
	var b strings.Builder
	provider := strings.ToLower(strings.TrimSpace(params.Provider))
	if provider == "" {
		provider = "openai"
	}

	if params.CardType == "" {
		params.CardType = inferCardType(params.AudioFile, params.AudioFileBack)
	}

	fmt.Fprintf(&b, "provider=%s\n", provider)
	switch provider {
	case "gemini":
		model := strings.TrimSpace(params.GeminiTTSModel)
		if model == "" {
			model = DefaultProviderConfig().GeminiTTSModel
		}
		fmt.Fprintf(&b, "model=%s\n", model)
		voice := strings.TrimSpace(params.GeminiVoice)
		if voice == "" {
			voice = "model-default"
		}
		fmt.Fprintf(&b, "voice=%s\n", voice)
		fmt.Fprintf(&b, "speed=%.2f\n", params.GeminiSpeed)
	default:
		model := strings.TrimSpace(params.OpenAIModel)
		if model == "" {
			model = DefaultProviderConfig().OpenAIModel
		}
		fmt.Fprintf(&b, "model=%s\n", model)
		voice := strings.TrimSpace(params.OpenAIVoice)
		if voice != "" {
			fmt.Fprintf(&b, "voice=%s\n", voice)
		}
		speed := params.OpenAISpeed
		if speed <= 0 {
			speed = DefaultProviderConfig().OpenAISpeed
		}
		fmt.Fprintf(&b, "speed=%.2f\n", speed)
		if instruction := strings.TrimSpace(params.OpenAIInstruction); instruction != "" {
			fmt.Fprintf(&b, "instruction=%s\n", instruction)
		}
	}

	format := strings.TrimSpace(params.OutputFormat)
	if format == "" {
		format = DefaultProviderConfig().OutputFormat
	}
	fmt.Fprintf(&b, "format=%s\n", format)
	if params.CardType != "" {
		fmt.Fprintf(&b, "cardtype=%s\n", params.CardType)
	}
	if audioFile := strings.TrimSpace(params.AudioFile); audioFile != "" {
		fmt.Fprintf(&b, "audio_file=%s\n", filepath.Base(audioFile))
	}
	if audioFileBack := strings.TrimSpace(params.AudioFileBack); audioFileBack != "" {
		fmt.Fprintf(&b, "audio_file_back=%s\n", filepath.Base(audioFileBack))
	}

	return b.String()
}

func inferCardType(audioFile, audioFileBack string) string {
	if strings.TrimSpace(audioFileBack) != "" {
		return "bg-bg"
	}

	base := filepath.Base(strings.TrimSpace(audioFile))
	switch {
	case strings.HasPrefix(base, "audio_front."):
		return "bg-bg"
	case strings.HasPrefix(base, "audio_back."):
		return "bg-bg"
	default:
		return "en-bg"
	}
}
