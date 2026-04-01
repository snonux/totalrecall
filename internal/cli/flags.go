package cli

import "codeberg.org/snonux/totalrecall/internal/audio"

const (
	defaultNanoBananaModel     = "gemini-3.1-flash-image-preview"
	defaultNanoBananaTextModel = "gemini-2.5-flash"
)

// Flags holds all command-line flag values
type Flags struct {
	// General flags
	CfgFile     string
	OutputDir   string
	AudioFormat string
	// AudioProvider selects the text-to-speech backend ("gemini" or "openai").
	AudioProvider     string
	ImageAPI          string
	ImageAPISpecified bool
	BatchFile         string
	SkipAudio         bool
	SkipImages        bool
	GenerateAnki      bool
	AnkiCSV           bool
	DeckName          string
	ListModels        bool
	AllVoices         bool
	NoAutoPlay        bool
	Archive           bool

	// OpenAI flags
	OpenAIModel       string
	OpenAIVoice       string
	OpenAISpeed       float64
	OpenAIInstruction string

	// OpenAI Image flags
	OpenAIImageModel   string
	OpenAIImageSize    string
	OpenAIImageQuality string
	OpenAIImageStyle   string

	// Gemini audio flags
	// GeminiTTSModel is the Gemini TTS model used when Gemini audio is selected.
	GeminiTTSModel string
	// GeminiVoice selects a specific Gemini voice; empty uses the model default.
	GeminiVoice string

	// NanoBananaModel is the Gemini image model used for Nano Banana generation.
	NanoBananaModel string
	// NanoBananaModelSpecified records whether the Nano Banana image model was explicitly set on the CLI.
	NanoBananaModelSpecified bool
	// NanoBananaTextModel is the Gemini text model used for Nano Banana prompt generation.
	NanoBananaTextModel string
	// NanoBananaTextModelSpecified records whether the Nano Banana text model was explicitly set on the CLI.
	NanoBananaTextModelSpecified bool
}

// NewFlags creates a new Flags instance with default values
func NewFlags() *Flags {
	defaults := audio.DefaultProviderConfig()

	return &Flags{
		AudioFormat:         "mp3",
		AudioProvider:       defaults.Provider,
		ImageAPI:            "openai",
		DeckName:            "Bulgarian Vocabulary",
		OpenAIModel:         "gpt-4o-mini-tts",
		OpenAISpeed:         0.9,
		OpenAIImageModel:    "dall-e-2",
		OpenAIImageSize:     "512x512",
		OpenAIImageQuality:  "standard",
		OpenAIImageStyle:    "natural",
		GeminiTTSModel:      defaults.GeminiTTSModel,
		NanoBananaModel:     defaultNanoBananaModel,
		NanoBananaTextModel: defaultNanoBananaTextModel,
	}
}
