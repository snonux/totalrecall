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
	// narratorTimeout gives the TTS API up to 3 minutes per chunk.
	// Gemini TTS can be slow under load; 3 minutes avoids spurious timeouts
	// while still bounding runaway requests.
	narratorTimeout = 3 * time.Minute

	// narratorChunkWords is the target word count per TTS chunk.
	// Gemini TTS degrades in quality and voice consistency after ~1 minute;
	// ~100 words at typical Bulgarian speech rate (~120 words/min) keeps each
	// call to ~50 seconds, safely under the 1-minute quality threshold.
	// Chunks are split at paragraph boundaries whenever possible.
	narratorChunkWords = 100

	// introSystemInstruction directs Gemini to write a short cinematic teaser
	// in Bulgarian — roughly 30–40 words (≈15 seconds of narration) — that hooks
	// the listener before the main story begins.
	introSystemInstruction = `You are a dramatic cinematic narrator writing an opening teaser for a Bulgarian story.
Write a SHORT opening teaser in the BULGARIAN language (NOT Russian — Bulgarian uses Cyrillic but
is a distinct language with different phonology, vocabulary, and grammar).
Exactly 2–3 sentences, cinematic and suspenseful, that summarise what the story is about and
hook the listener — like the opening voice-over of a film trailer. Do NOT spoil the ending.
Output only the Bulgarian teaser text, nothing else.`

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

// Narrator wraps two Gemini TTS providers: one for the main story and a
// different voice for the epilogue, so the moral/conclusion is clearly
// distinguished from the narrative.
type Narrator struct {
	provider           audio.Provider
	conclusionProvider audio.Provider // different voice for the epilogue
	apiKey             string         // stored for the conclusion text-generation call
	voice              string         // main voice name, for progress logging
	conclusionVoice    string         // epilogue voice name, for progress logging
}

// NewNarrator wires a main GeminiProvider (story) and a second provider with a
// different voice (epilogue). Returns an error if the API key is missing or the
// main provider cannot be initialised. Epilogue provider failure is non-fatal —
// it falls back to the main voice.
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

	// Pick a different voice for the epilogue so it sounds distinct from
	// the main narrative — signals to the listener that it is a separate segment.
	conclusionVoice := pickDifferentVoice(voice)
	conclusionProvider, err := audio.NewProvider(&audio.Config{
		Provider:     "gemini",
		OutputFormat: "mp3",
		GoogleAPIKey: config.APIKey,
		GeminiVoice:  conclusionVoice,
	})
	if err != nil {
		// Non-fatal — fall back to the main voice.
		fmt.Printf("    Warning: epilogue voice init failed (%v), using main voice\n", err)
		conclusionProvider = provider
		conclusionVoice = voice
	}

	return &Narrator{
		provider:           provider,
		conclusionProvider: conclusionProvider,
		apiKey:             config.APIKey,
		voice:              voice,
		conclusionVoice:    conclusionVoice,
	}, nil
}

// Narrate generates a cinematic MP3 narration of storyText and saves it to
// outputFile. Structure:
//  1. Intro (conclusion voice + ambient music): 15-second teaser summarising the story
//  2. Main story (main voice): split into ~200-word chunks for consistent quality
//  3. Epilogue (conclusion voice + ambient music): cinematic moral/outro
func (n *Narrator) Narrate(storyText, outputFile string) error {
	tmpDir, err := os.MkdirTemp("", "totalrecall-narration-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Intro and epilogue share the conclusion voice and ambient music so they
	// frame the main story as distinct cinematic bookends.
	var allPaths []string
	if introPath, ok := n.narrateIntro(storyText, tmpDir); ok {
		allPaths = append(allPaths, introPath)
	}

	chunkPaths, err := n.narrateMainStory(storyText, tmpDir)
	if err != nil {
		return err
	}
	allPaths = append(allPaths, chunkPaths...)

	if conclusionPath, ok := n.narrateConclusion(storyText, tmpDir); ok {
		allPaths = append(allPaths, conclusionPath)
	}

	// Merge intro + story + epilogue into one file, then widen to stereo.
	combinedPath := filepath.Join(tmpDir, "combined.mp3")
	if len(allPaths) == 1 {
		combinedPath = allPaths[0]
	} else {
		if err := concatenateMP3s(allPaths, combinedPath, tmpDir); err != nil {
			return err
		}
	}
	return convertToStereo(combinedPath, outputFile)
}

// narrateMainStory splits the story into chunks and narrates each with the
// main voice. Returns the list of chunk MP3 paths.
func (n *Narrator) narrateMainStory(storyText, tmpDir string) ([]string, error) {
	chunks := splitIntoNarrationChunks(storyText, narratorChunkWords)
	fmt.Printf("    Splitting narration into %d chunks for consistent voice quality...\n", len(chunks))

	var chunkPaths []string
	for i, chunk := range chunks {
		chunkPath := filepath.Join(tmpDir, fmt.Sprintf("chunk_%03d.mp3", i+1))
		fmt.Printf("    Narrating chunk %d/%d...\n", i+1, len(chunks))
		if err := n.narrateChunkWith(n.provider, cinematicInstruction+chunk, chunkPath); err != nil {
			return nil, fmt.Errorf("narrate chunk %d: %w", i+1, err)
		}
		chunkPaths = append(chunkPaths, chunkPath)
	}
	return chunkPaths, nil
}

// narrateIntro generates a short Bulgarian cinematic teaser (~15 s) via Gemini
// text, narrates it in the conclusion voice with ambient music, and returns the
// path. Non-fatal — main narration continues if intro fails.
func (n *Narrator) narrateIntro(storyText, tmpDir string) (string, bool) {
	intro := n.buildIntro(storyText)
	if intro == "" {
		return "", false
	}

	fmt.Printf("    Narrating intro teaser (voice: %s)...\n", n.conclusionVoice)
	introRaw := filepath.Join(tmpDir, "intro_narration.mp3")
	if err := n.narrateChunkWith(n.conclusionProvider, cinematicInstruction+intro, introRaw); err != nil {
		fmt.Printf("    Warning: intro narration failed: %v\n", err)
		return "", false
	}

	introWithMusic := filepath.Join(tmpDir, "intro_with_music.mp3")
	if err := mixAmbientMusic(introRaw, introWithMusic, tmpDir); err != nil {
		fmt.Printf("    Warning: intro music mix failed (%v) — using narration only\n", err)
		return introRaw, true
	}
	return introWithMusic, true
}

// buildIntro calls Gemini to produce a short Bulgarian cinematic opening teaser
// (~30–40 words, ≈15 s). Returns empty string on failure.
func (n *Narrator) buildIntro(storyText string) string {
	if n.apiKey == "" {
		return ""
	}
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: n.apiKey})
	if err != nil {
		fmt.Printf("    Warning: intro text generation failed: %v\n", err)
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), helperTimeout)
	defer cancel()
	resp, err := client.Models.GenerateContent(ctx, helperModel,
		[]*genai.Content{genai.NewContentFromText(storyText, genai.RoleUser)},
		&genai.GenerateContentConfig{
			SystemInstruction: &genai.Content{
				Parts: []*genai.Part{{Text: introSystemInstruction}},
			},
			MaxOutputTokens: helperMaxTokens,
		},
	)
	if err != nil {
		fmt.Printf("    Warning: intro text generation failed: %v\n", err)
		return ""
	}
	text := strings.TrimSpace(resp.Text())
	if text == "" {
		fmt.Println("    Warning: intro text generation returned empty response")
	}
	return text
}

// narrateConclusion generates a short Bulgarian cinematic epilogue via Gemini
// text, splits it into chunks, narrates each with the epilogue voice, then
// mixes in ambient background music. Returns the final path and true on success.
// Non-fatal on any failure — the main narration always saves.
func (n *Narrator) narrateConclusion(storyText, tmpDir string) (string, bool) {
	conclusion := n.buildConclusion(storyText)
	if conclusion == "" {
		return "", false
	}

	fmt.Printf("    Narrating concluding epilogue (voice: %s)...\n", n.conclusionVoice)

	chunks := splitIntoNarrationChunks(conclusion, narratorChunkWords)
	var chunkPaths []string
	for i, chunk := range chunks {
		chunkPath := filepath.Join(tmpDir, fmt.Sprintf("conclusion_%03d.mp3", i+1))
		if err := n.narrateChunkWith(n.conclusionProvider, cinematicInstruction+chunk, chunkPath); err != nil {
			fmt.Printf("    Warning: conclusion narration failed: %v\n", err)
			return "", false
		}
		chunkPaths = append(chunkPaths, chunkPath)
	}

	// Join conclusion chunks if there is more than one.
	conclusionNarration := filepath.Join(tmpDir, "conclusion_narration.mp3")
	if len(chunkPaths) == 1 {
		conclusionNarration = chunkPaths[0]
	} else if err := concatenateMP3s(chunkPaths, conclusionNarration, tmpDir); err != nil {
		fmt.Printf("    Warning: conclusion concat failed: %v\n", err)
		return chunkPaths[len(chunkPaths)-1], true
	}

	// Mix ambient background music under the epilogue for a cinematic feel.
	conclusionWithMusic := filepath.Join(tmpDir, "conclusion_with_music.mp3")
	if err := mixAmbientMusic(conclusionNarration, conclusionWithMusic, tmpDir); err != nil {
		fmt.Printf("    Warning: background music mix failed (%v) — using narration only\n", err)
		return conclusionNarration, true
	}
	return conclusionWithMusic, true
}

// buildConclusion calls Gemini text to produce a short Bulgarian cinematic
// epilogue (≈40–60 words, ~15–30 s of narration). Returns empty string on failure.
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

// narrateChunkWith calls the given TTS provider for a single text segment.
func (n *Narrator) narrateChunkWith(provider audio.Provider, text, outputFile string) error {
	ctx, cancel := context.WithTimeout(context.Background(), narratorTimeout)
	defer cancel()
	return provider.GenerateAudio(ctx, text, outputFile)
}

// mixAmbientMusic generates a soft cinematic ambient pad using ffmpeg's built-in
// signal generators (bass drone sine waves + quiet pink noise) and mixes it under
// narrationFile at low volume. The music fades in over 4 seconds and is cut to
// exactly the length of the narration. Falls back gracefully when ffmpeg is absent.
func mixAmbientMusic(narrationFile, outputFile, tmpDir string) error {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found")
	}

	musicPath := filepath.Join(tmpDir, "ambient_pad.mp3")
	if err := generateAmbientPad(ffmpegPath, musicPath); err != nil {
		return err
	}

	// Mix: narration at full weight, ambient pad at 30% weight.
	// duration=first stops when narration (first input) ends.
	cmd := exec.Command(ffmpegPath,
		"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-i", narrationFile,
		"-i", musicPath,
		"-filter_complex", "[0:a][1:a]amix=inputs=2:weights=1 0.3:duration=first[aout]",
		"-map", "[aout]",
		"-ac", "2",
		"-codec:a", "libmp3lame", "-q:a", "2",
		outputFile,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("background music mix failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// generateAmbientPad creates a 5-minute ambient cinematic pad MP3 using ffmpeg's
// aevalsrc filter. The pad consists of a C-major bass drone (C2+G2+C3) mixed with
// quiet pink noise for atmosphere, with a 4-second fade-in. The long duration
// ensures it always outlasts the epilogue narration; mixing uses duration=first
// to cut it cleanly at the end of the voice track.
func generateAmbientPad(ffmpegPath, outputFile string) error {
	// Bass drone: C2 (65 Hz), G2 (98 Hz), C3 (130 Hz) at low amplitudes.
	// Pink noise adds cinematic texture without dominating the voice.
	droneExpr := "0.04*sin(65*2*PI*t)+0.03*sin(98*2*PI*t)+0.02*sin(130*2*PI*t)"
	cmd := exec.Command(ffmpegPath,
		"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi",
		"-i", fmt.Sprintf("aevalsrc=%s:sample_rate=44100", droneExpr),
		"-f", "lavfi", "-i", "anoisesrc=color=pink:amplitude=0.008",
		"-filter_complex", "[0:a][1:a]amix=inputs=2:duration=first[mixed];[mixed]afade=t=in:st=0:d=4[aout]",
		"-map", "[aout]",
		"-t", "300", // 5 minutes — always longer than the epilogue
		"-ac", "2",
		outputFile,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ambient pad generation failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
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

// pickDifferentVoice returns a random voice from the pool that is not currentVoice.
// Used to ensure the epilogue sounds distinct from the main narration.
func pickDifferentVoice(currentVoice string) string {
	pool := make([]string, 0, len(cinematicVoices)-1)
	for _, v := range cinematicVoices {
		if !strings.EqualFold(v, currentVoice) {
			pool = append(pool, v)
		}
	}
	if len(pool) == 0 {
		return currentVoice
	}
	return pool[rand.IntN(len(pool))]
}
