package cli

// Flags holds all command-line flag values
type Flags struct {
	// General flags
	CfgFile      string
	OutputDir    string
	AudioFormat  string
	ImageAPI     string
	BatchFile    string
	SkipAudio    bool
	SkipImages   bool
	GenerateAnki bool
	AnkiCSV      bool
	DeckName     string
	ListModels   bool
	AllVoices    bool
	GUIMode      bool

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
}

// NewFlags creates a new Flags instance with default values
func NewFlags() *Flags {
	return &Flags{
		AudioFormat:        "mp3",
		ImageAPI:           "openai",
		DeckName:           "Bulgarian Vocabulary",
		OpenAIModel:        "gpt-4o-mini-tts",
		OpenAISpeed:        0.9,
		OpenAIImageModel:   "dall-e-3",
		OpenAIImageSize:    "1024x1024",
		OpenAIImageQuality: "standard",
		OpenAIImageStyle:   "natural",
	}
}
