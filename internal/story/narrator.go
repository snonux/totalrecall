package story

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"codeberg.org/snonux/totalrecall/internal/audio"
)

const (
	// narratorTimeout gives the TTS API up to 3 minutes to narrate a full story.
	// A ~500-word story is much longer than a flashcard word, so the generous
	// timeout prevents premature cancellation on slow API responses.
	narratorTimeout = 3 * time.Minute

	// cinematicInstruction is prepended to the story text before the TTS call.
	// Gemini TTS reads style instructions from the user-turn prompt, so embedding
	// the directive here (rather than as a SystemInstruction) is the supported way
	// to control voice style, pacing, and emotional delivery.
	cinematicInstruction = `You are a dramatic cinematic narrator performing a Bulgarian story.
Deliver this as a professional movie trailer narrator would: deep, resonant, and commanding.
Use long dramatic pauses before key moments. Build tension with slower, deliberate pacing,
then accelerate through action. Drop your voice low and gravelly for mysterious or serious
passages; let warmth and energy rise for joyful or triumphant ones. Breathe life into every
sentence — this should sound like an epic film, not a reading exercise. Pronounce all
Bulgarian words with authentic clarity and expressive intonation.

`
)

// cinematicVoices is a curated subset of Gemini voices chosen for their
// narrative quality. A voice is picked randomly each run so repeated story
// generations sound different. Users can override with --narrator-voice.
var cinematicVoices = []string{
	"Charon",    // deep, measured — authoritative narrator feel
	"Fenrir",    // strong, resonant — good for dramatic pacing
	"Enceladus", // breathy, intimate — cinematic closeness
	"Algieba",   // smooth, warm — classic storytelling tone
	"Aoede",     // breezy, expressive — light narrative energy
	"Schedar",   // steady, grounded presence — suits long-form stories
}

// NarratorConfig holds credentials and voice preferences for Gemini TTS narration.
type NarratorConfig struct {
	APIKey string // Google API key — the same GOOGLE_API_KEY already used by the project
	Voice  string // empty → random pick from cinematicVoices each run
}

// Narrator wraps a Gemini TTS Provider and generates cinematic MP3 narrations.
type Narrator struct {
	provider audio.Provider
	voice    string // resolved voice name, stored for progress logging
}

// NewNarrator wires a GeminiProvider with the cinematic voice and returns a
// Narrator ready to call. Returns an error if the API key is missing or the
// provider cannot be initialised.
func NewNarrator(config *NarratorConfig) (*Narrator, error) {
	if config == nil || config.APIKey == "" {
		return nil, fmt.Errorf("Google API key is required for story narration")
	}

	voice := config.Voice
	if voice == "" {
		voice = pickCinematicVoice()
	}

	provider, err := audio.NewProvider(&audio.Config{
		Provider:     "gemini",
		OutputFormat: "mp3",
		GoogleAPIKey: config.APIKey,
		GeminiVoice:  voice,
	})
	if err != nil {
		return nil, fmt.Errorf("narrator: initialise Gemini TTS: %w", err)
	}

	return &Narrator{provider: provider, voice: voice}, nil
}

// Narrate generates a cinematic MP3 narration of storyText and saves it to
// outputFile. The cinematic instruction is prepended to the text so the TTS
// model applies dramatic pacing and expressive intonation.
func (n *Narrator) Narrate(storyText, outputFile string) error {
	ctx, cancel := context.WithTimeout(context.Background(), narratorTimeout)
	defer cancel()

	cinematicText := cinematicInstruction + storyText
	return n.provider.GenerateAudio(ctx, cinematicText, outputFile)
}

// pickCinematicVoice returns a random voice from the cinematicVoices pool.
func pickCinematicVoice() string {
	return cinematicVoices[rand.IntN(len(cinematicVoices))]
}
