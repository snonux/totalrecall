package cli

const (
	defaultNanoBananaModel     = "gemini-3.1-flash-image-preview"
	defaultNanoBananaTextModel = "gemini-2.5-flash"
)

// Flags holds all command-line flag values
type Flags struct {
	// General flags
	CfgFile           string
	OutputDir         string
	AudioFormat       string
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

	// NanoBananaModel is the Gemini image model used for Nano Banana generation.
	NanoBananaModel string
	// NanoBananaTextModel is the Gemini text model used for Nano Banana prompt generation.
	NanoBananaTextModel string
}

// NewFlags creates a new Flags instance with default values
func NewFlags() *Flags {
	return &Flags{
		AudioFormat:         "mp3",
		ImageAPI:            "openai",
		DeckName:            "Bulgarian Vocabulary",
		OpenAIModel:         "gpt-4o-mini-tts",
		OpenAISpeed:         0.9,
		OpenAIImageModel:    "dall-e-2",
		OpenAIImageSize:     "512x512",
		OpenAIImageQuality:  "standard",
		OpenAIImageStyle:    "natural",
		NanoBananaModel:     defaultNanoBananaModel,
		NanoBananaTextModel: defaultNanoBananaTextModel,
	}
}
