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

// ProcessedTextForProvider returns the exact text shape the provider path sends to TTS.
func ProcessedTextForProvider(provider, text string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai":
		return openAIProcessedText(text)
	case "gemini":
		return strings.TrimSpace(text)
	default:
		return strings.TrimSpace(text)
	}
}

// InstructionForProvider returns the provider-specific instruction semantics written to attribution files.
func InstructionForProvider(provider string, config *Config) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai":
		return openAIInstructionForAttribution(config)
	case "gemini":
		return geminiPromptInstruction(config)
	default:
		return ""
	}
}

func openAIProcessedText(text string) string {
	cleanedText := strings.TrimSpace(text)
	punctuationToRemove := []string{"!", "?", ".", ",", ";", ":", "\"", "'", "(", ")", "[", "]", "{", "}", "-", "—", "–"}
	for _, punct := range punctuationToRemove {
		cleanedText = strings.ReplaceAll(cleanedText, punct, "")
	}

	return strings.TrimSpace(cleanedText)
}

func openAIInstructionForAttribution(config *Config) string {
	if config == nil {
		return ""
	}

	if !openAIModelUsesInstructions(config.OpenAIModel) {
		return ""
	}

	return strings.TrimSpace(config.OpenAIInstruction)
}

func openAIModelUsesInstructions(model string) bool {
	switch strings.TrimSpace(model) {
	case "gpt-4o-mini-tts", "gpt-4o-mini-audio-preview":
		return true
	default:
		return false
	}
}

func geminiPromptInstruction(config *Config) string {
	if config == nil {
		config = &Config{}
	}

	var prompt strings.Builder
	prompt.WriteString("You are speaking Bulgarian language (български език). ")
	prompt.WriteString("Pronounce the Bulgarian text with authentic Bulgarian phonetics, not Russian.")

	if speedHint := geminiSpeedHint(config.GeminiSpeed); speedHint != "" {
		prompt.WriteString(" ")
		prompt.WriteString(speedHint)
	}

	prompt.WriteString("\n\nSpeak the following Bulgarian text:")

	if voice := strings.TrimSpace(config.GeminiVoice); voice != "" {
		prompt.WriteString("\n\nUse a clear, natural delivery that matches the voice named ")
		prompt.WriteString(voice)
		prompt.WriteString(".")
	}

	return prompt.String()
}

func geminiSpeedHint(speed float64) string {
	switch {
	case speed < 0.95:
		return "Speak slowly and clearly for language learners."
	case speed > 1.05:
		return "Speak slightly faster than normal while staying clear."
	default:
		return "Speak at a natural pace."
	}
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
		if openAIModelUsesInstructions(model) {
			if instruction := strings.TrimSpace(params.OpenAIInstruction); instruction != "" {
				fmt.Fprintf(&b, "instruction=%s\n", instruction)
			}
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
