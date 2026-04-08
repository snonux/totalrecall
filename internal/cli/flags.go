package cli

import (
	"codeberg.org/snonux/totalrecall/internal/audio"
	"codeberg.org/snonux/totalrecall/internal/config"
)

// Flags holds all command-line flag values
type Flags struct {
	// General flags
	CfgFile     string
	OutputDir   string
	AudioFormat string
	// AudioFormatSpecified records whether the audio format was explicitly set on the CLI.
	AudioFormatSpecified bool
	// AudioProvider selects the text-to-speech backend ("gemini" or "openai").
	AudioProvider         string
	ImageAPI              string
	ImageAPISpecified     bool
	BatchFile             string
	StoryFile             string // --story <file>: generate vocabulary story + comic image
	StoryStyle            string // --story-style: override the random art style (empty = random)
	StoryTheme            string // --story-theme: override the random genre pick (empty = random)
	StoryNoUltraRealistic bool   // --no-ultra-realistic: disable photorealistic rendering requirement
	StoryUltraRealistic   bool   // --ultra-realistic: force photorealistic rendering (overrides random 50/50)
	StorySlug             string // --story-slug: force a specific output slug/directory (empty = auto from title)
	NarratorVoice         string // --narrator-voice: Gemini voice for cinematic narration (empty = random)
	NarrateEnabled        bool   // --narrate: generate cinematic MP3 narration after --story (default false)
	VideoEnabled          bool   // --video: whether to prompt for Veo video generation after --story completes
	SkipAudio             bool
	SkipImages            bool
	GenerateAnki          bool
	AnkiCSV               bool
	DeckName              string
	ListModels            bool
	AllVoices             bool
	NoAutoPlay            bool
	Archive               bool

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
	// GeminiVoice selects a specific Gemini voice; empty picks a random Gemini voice.
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
		AudioFormat:         defaults.OutputFormat,
		AudioProvider:       defaults.Provider,
		ImageAPI:            "nanobanana",
		VideoEnabled:        true,
		DeckName:            "Bulgarian Vocabulary",
		OpenAIModel:         "gpt-4o-mini-tts",
		OpenAISpeed:         0.9,
		OpenAIImageModel:    "dall-e-2",
		OpenAIImageSize:     "512x512",
		OpenAIImageQuality:  "standard",
		OpenAIImageStyle:    "natural",
		GeminiTTSModel:      defaults.GeminiTTSModel,
		NanoBananaModel:     config.DefaultNanoBananaModel,
		NanoBananaTextModel: config.DefaultNanoBananaTextModel,
	}
}
