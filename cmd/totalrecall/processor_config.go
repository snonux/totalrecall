package main

import (
	"strings"

	"github.com/spf13/viper"

	"codeberg.org/snonux/totalrecall/internal/cli"
	"codeberg.org/snonux/totalrecall/internal/processor"
)

// newProcessorConfig reads all Viper-sourced settings in a single pass and
// returns a fully-resolved processor.Config. Centralising all Viper access
// here means the processor package is free of any Viper dependency, which
// improves testability and removes tight coupling to the global config singleton.
func newProcessorConfig() *processor.Config {
	return &processor.Config{
		// Translation & phonetic
		TranslationProvider:    strings.TrimSpace(viper.GetString("translation.provider")),
		PhoneticProvider:       strings.TrimSpace(viper.GetString("phonetic.provider")),
		TranslationGeminiModel: viper.GetString("translation.gemini_model"),

		// Audio
		AudioProvider:        strings.ToLower(strings.TrimSpace(viper.GetString("audio.provider"))),
		AudioFormat:          strings.ToLower(strings.TrimSpace(viper.GetString("audio.format"))),
		AudioFormatSet:       viper.IsSet("audio.format"),
		GeminiTTSModel:       strings.TrimSpace(viper.GetString("audio.gemini_tts_model")),
		GeminiVoice:          strings.TrimSpace(viper.GetString("audio.gemini_voice")),
		OpenAIVoice:          strings.TrimSpace(viper.GetString("audio.openai_voice")),
		OpenAIModel:          viper.GetString("audio.openai_model"),
		OpenAIModelSet:       viper.IsSet("audio.openai_model"),
		OpenAISpeed:          viper.GetFloat64("audio.openai_speed"),
		OpenAISpeedSet:       viper.IsSet("audio.openai_speed"),
		OpenAIInstruction:    viper.GetString("audio.openai_instruction"),
		OpenAIInstructionSet: viper.IsSet("audio.openai_instruction"),

		// Image
		ImageProvider:               strings.ToLower(strings.TrimSpace(viper.GetString("image.provider"))),
		ImageOpenAIModel:            viper.GetString("image.openai_model"),
		ImageOpenAIModelSet:         viper.IsSet("image.openai_model"),
		ImageOpenAISize:             viper.GetString("image.openai_size"),
		ImageOpenAISizeSet:          viper.IsSet("image.openai_size"),
		ImageOpenAIQuality:          viper.GetString("image.openai_quality"),
		ImageOpenAIQualitySet:       viper.IsSet("image.openai_quality"),
		ImageOpenAIStyle:            viper.GetString("image.openai_style"),
		ImageOpenAIStyleSet:         viper.IsSet("image.openai_style"),
		ImageNanoBananaModel:        strings.TrimSpace(viper.GetString("image.nanobanana_model")),
		ImageNanoBananaModelSet:     viper.IsSet("image.nanobanana_model"),
		ImageNanoBananaTextModel:    strings.TrimSpace(viper.GetString("image.nanobanana_text_model")),
		ImageNanoBananaTextModelSet: viper.IsSet("image.nanobanana_text_model"),
	}
}

// newProcessor builds a processor from CLI flags and the Viper-backed config.
func newProcessor(flags *cli.Flags) *processor.Processor {
	return processor.NewProcessor(flags, newProcessorConfig())
}
