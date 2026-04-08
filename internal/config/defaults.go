package config

// Central defaults for Nano Banana (Gemini image) models and shared audio settings.
// CLI, GUI, internal/audio, and callers resolve flag/config zero values against these
// constants so model IDs and format strings are not duplicated (DRY).

const (
	// DefaultNanoBananaModel is the Gemini image model used for Nano Banana generation.
	DefaultNanoBananaModel = "gemini-3.1-flash-image-preview"

	// DefaultNanoBananaTextModel is the Gemini text model used for Nano Banana prompt
	// and scene-related text generation.
	DefaultNanoBananaTextModel = "gemini-2.5-flash"

	// DefaultAudioOutputFormat is the default TTS file extension / container (e.g. mp3, wav).
	DefaultAudioOutputFormat = "mp3"

	// DefaultOpenAIAudioModel is the default OpenAI TTS model when that provider is selected.
	DefaultOpenAIAudioModel = "gpt-4o-mini-tts"

	// DefaultOpenAIVoice is the default OpenAI TTS voice.
	DefaultOpenAIVoice = "alloy"

	// DefaultGeminiTTSModel is the default Gemini TTS model identifier.
	DefaultGeminiTTSModel = "gemini-2.5-flash-preview-tts"

	// DefaultOpenAIAudioInstruction is the default system instruction for OpenAI TTS
	// when generating Bulgarian learner audio.
	DefaultOpenAIAudioInstruction = "You are speaking Bulgarian language (български език). Pronounce the Bulgarian text with authentic Bulgarian phonetics, not Russian. Speak slowly and clearly for language learners."
)

// Default audio tuning (non-string defaults for internal/audio.Config).
const (
	DefaultOpenAIAudioSpeed = 1.0
	DefaultGeminiAudioSpeed = 1.0
)
