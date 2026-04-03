package story

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/genai"

	"codeberg.org/snonux/totalrecall/internal/audio"
)

const (
	// narratorTimeout gives the TTS API up to 2 minutes per chunk.
	// Chunks are much shorter than the full story, so this is generous.
	narratorTimeout = 2 * time.Minute

	// narratorChunkWords is the target word count per TTS chunk.
	// Gemini TTS degrades in quality and voice consistency for long texts;
	// splitting at ~200 words keeps each call short and the voice stable.
	// Chunks are split at paragraph boundaries whenever possible.
	narratorChunkWords = 200

	// conclusionSystemInstruction directs Gemini to write a short cinematic
	// epilogue in Bulgarian — roughly 40–60 words (≈15–30 seconds of narration).
	conclusionSystemInstruction = `You are a dramatic cinematic narrator writing a closing epilogue for a Bulgarian story.
Write a SHORT closing epilogue in the BULGARIAN language (NOT Russian — Bulgarian uses Cyrillic but
is a distinct language with different phonology, vocabulary, and grammar).
Exactly 3–4 sentences, cinematic and poetic, with a warm and conclusive tone — like the final
voice-over of a film that leaves the audience with a sense of wonder and completion.
Do NOT summarise the plot; instead reflect on the deeper meaning or emotion of the story.
Output only the Bulgarian epilogue text, nothing else.`

	// cinematicInstruction is prepended to every chunk before the TTS call.
	// Gemini TTS reads style instructions from the user-turn prompt, so embedding
	// the directive here (rather than as a SystemInstruction) is the supported way
	// to control voice style, pacing, and emotional delivery.
	// The language must be stated explicitly: Gemini TTS can confuse Bulgarian with
	// Russian (both use Cyrillic) and apply Slavic Russian phonology by default.
	cinematicInstruction = `You are a dramatic cinematic narrator performing a story written in BULGARIAN.
IMPORTANT: This text is in the BULGARIAN language — NOT Russian, NOT Serbian, NOT any other Slavic language.
Pronounce every word using authentic BULGARIAN phonology and accent. Bulgarian vowels are clear and distinct;
do not apply Russian stress patterns or Russian vowel reduction. The letter 'ъ' in Bulgarian is a mid-central
vowel (like the 'u' in "but"), not the Russian reduced schwa.
Deliver this as a professional movie trailer narrator would: deep, resonant, and commanding.
Use long dramatic pauses before key moments. Build tension with slower, deliberate pacing,
then accelerate through action. Drop your voice low and gravelly for mysterious or serious
passages; let warmth and energy rise for joyful or triumphant ones. Breathe life into every
sentence — this should sound like an epic Bulgarian film, not a reading exercise.

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
	apiKey   string // stored for the conclusion text-generation call
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

	return &Narrator{provider: provider, apiKey: config.APIKey, voice: voice}, nil
}

// Narrate generates a cinematic MP3 narration of storyText and saves it to
// outputFile. The story is split into short paragraph-aligned chunks before
// calling the TTS API so the voice quality stays high throughout (Gemini TTS
// degrades on long single-call texts). A Gemini-generated cinematic epilogue
// is always appended as a final 15–30 second concluding segment.
func (n *Narrator) Narrate(storyText, outputFile string) error {
	tmpDir, err := os.MkdirTemp("", "totalrecall-narration-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	chunks := splitIntoNarrationChunks(storyText, narratorChunkWords)
	fmt.Printf("    Splitting narration into %d chunks for consistent voice quality...\n", len(chunks))

	var chunkPaths []string
	for i, chunk := range chunks {
		chunkPath := filepath.Join(tmpDir, fmt.Sprintf("chunk_%03d.mp3", i+1))
		fmt.Printf("    Narrating chunk %d/%d...\n", i+1, len(chunks))
		if err := n.narrateChunk(cinematicInstruction+chunk, chunkPath); err != nil {
			return fmt.Errorf("narrate chunk %d: %w", i+1, err)
		}
		chunkPaths = append(chunkPaths, chunkPath)
	}

	// Generate and append a cinematic epilogue as the final segment.
	if conclusionPath, ok := n.narrateConclusion(storyText, tmpDir); ok {
		chunkPaths = append(chunkPaths, conclusionPath)
	}

	// Merge all segments into a single file, then widen to stereo.
	// Gemini TTS produces mono audio; convertToStereo duplicates the channel so
	// the result plays correctly on headphones without audio only in one ear.
	combinedPath := filepath.Join(tmpDir, "combined.mp3")
	if len(chunkPaths) == 1 {
		combinedPath = chunkPaths[0]
	} else {
		if err := concatenateMP3s(chunkPaths, combinedPath, tmpDir); err != nil {
			return err
		}
	}
	return convertToStereo(combinedPath, outputFile)
}

// narrateConclusion generates a short Bulgarian cinematic epilogue via Gemini text,
// then narrates it as an MP3 written to tmpDir. Returns the path and true on success,
// or empty string and false on any failure (non-fatal — the main narration still saves).
func (n *Narrator) narrateConclusion(storyText, tmpDir string) (string, bool) {
	conclusion := n.buildConclusion(storyText)
	if conclusion == "" {
		return "", false
	}

	fmt.Println("    Narrating concluding epilogue...")
	conclusionPath := filepath.Join(tmpDir, "conclusion.mp3")
	if err := n.narrateChunk(cinematicInstruction+conclusion, conclusionPath); err != nil {
		fmt.Printf("    Warning: conclusion narration failed: %v\n", err)
		return "", false
	}
	return conclusionPath, true
}

// buildConclusion calls Gemini text to produce a short Bulgarian cinematic epilogue
// (≈40–60 words, ~15–30 s of narration). Returns empty string on failure.
func (n *Narrator) buildConclusion(storyText string) string {
	if n.apiKey == "" {
		return ""
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: n.apiKey})
	if err != nil {
		fmt.Printf("    Warning: conclusion text generation failed: %v\n", err)
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), helperTimeout)
	defer cancel()

	resp, err := client.Models.GenerateContent(ctx, helperModel,
		[]*genai.Content{genai.NewContentFromText(storyText, genai.RoleUser)},
		&genai.GenerateContentConfig{
			SystemInstruction: &genai.Content{
				Parts: []*genai.Part{{Text: conclusionSystemInstruction}},
			},
			MaxOutputTokens: helperMaxTokens,
		},
	)
	if err != nil {
		fmt.Printf("    Warning: conclusion text generation failed: %v\n", err)
		return ""
	}

	text := strings.TrimSpace(resp.Text())
	if text == "" {
		fmt.Println("    Warning: conclusion text generation returned empty response")
	}
	return text
}

// narrateChunk calls the TTS provider for a single text segment.
func (n *Narrator) narrateChunk(text, outputFile string) error {
	ctx, cancel := context.WithTimeout(context.Background(), narratorTimeout)
	defer cancel()
	return n.provider.GenerateAudio(ctx, text, outputFile)
}

// concatenateMP3s joins chunkPaths into outputFile using ffmpeg's concat demuxer.
// The list file is written to tmpDir and cleaned up with it by the caller.
func concatenateMP3s(chunkPaths []string, outputFile, tmpDir string) error {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found — required for multi-chunk narration: %w", err)
	}

	// Write an ffmpeg concat list: one "file 'path'" line per chunk.
	listPath := filepath.Join(tmpDir, "concat_list.txt")
	var sb strings.Builder
	for _, p := range chunkPaths {
		sb.WriteString(fmt.Sprintf("file '%s'\n", p))
	}
	if err := os.WriteFile(listPath, []byte(sb.String()), 0600); err != nil {
		return fmt.Errorf("write concat list: %w", err)
	}

	cmd := exec.Command(ffmpegPath,
		"-nostdin", "-hide_banner", "-loglevel", "error",
		"-y",
		"-f", "concat", "-safe", "0",
		"-i", listPath,
		"-codec:a", "copy",
		outputFile,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg concat failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// convertToStereo re-encodes a mono MP3 to stereo by duplicating the single
// channel into both left and right. Gemini TTS always outputs mono; without this
// step the audio plays only in one ear on headphones.
func convertToStereo(inputFile, outputFile string) error {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		// ffmpeg absent — fall back to a plain copy so narration still saves.
		fmt.Println("    Warning: ffmpeg not found, narration will be mono")
		return os.Rename(inputFile, outputFile)
	}

	cmd := exec.Command(ffmpegPath,
		"-nostdin", "-hide_banner", "-loglevel", "error",
		"-y",
		"-i", inputFile,
		"-ac", "2", // duplicate mono channel into stereo
		"-codec:a", "libmp3lame", "-q:a", "2",
		outputFile,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg stereo conversion failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// splitIntoNarrationChunks divides text into chunks of at most targetWords words,
// splitting at paragraph boundaries (double newline) whenever possible.
// Each chunk is trimmed and non-empty.
func splitIntoNarrationChunks(text string, targetWords int) []string {
	paragraphs := splitParagraphs(text) // reuse artist.go helper
	if len(paragraphs) == 0 {
		return []string{strings.TrimSpace(text)}
	}

	var chunks []string
	var current strings.Builder
	currentWords := 0

	for _, para := range paragraphs {
		paraWords := len(strings.Fields(para))

		// If adding this paragraph would exceed the target, flush the current chunk.
		if currentWords > 0 && currentWords+paraWords > targetWords {
			chunks = append(chunks, strings.TrimSpace(current.String()))
			current.Reset()
			currentWords = 0
		}

		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(para)
		currentWords += paraWords
	}

	if current.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(current.String()))
	}
	return chunks
}

// pickCinematicVoice returns a random voice from the cinematicVoices pool.
func pickCinematicVoice() string {
	return cinematicVoices[rand.IntN(len(cinematicVoices))]
}
