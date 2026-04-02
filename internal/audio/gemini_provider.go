package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"google.golang.org/genai"
)

const (
	defaultGeminiTTSModel  = "gemini-2.5-flash-preview-tts"
	geminiTTSLanguageCode  = "bg"
	geminiTTSChannels      = 1
	geminiTTSSampleRate    = 24000
	geminiTTSBitsPerSample = 16
)

var ErrGeminiNoAudioData = errors.New("no audio data returned from Gemini")

var execLookPath = exec.LookPath
var execCommand = exec.Command

// GeminiProvider implements Provider interface for Gemini TTS.
// It stores only the Gemini-specific sub-config so it never sees OpenAI fields.
type GeminiProvider struct {
	client *genai.Client
	config GeminiAudioConfig
}

var _ Provider = (*GeminiProvider)(nil)

// NewGeminiProvider creates a new Gemini TTS provider from the Gemini-specific
// sub-config. Callers that have a flat Config should use NewProvider instead.
func NewGeminiProvider(config GeminiAudioConfig, outputFormat string) (Provider, error) {
	normalized := normalizeGeminiAudioConfig(config)
	if normalized.APIKey == "" {
		return nil, errors.New("google API key is required")
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  normalized.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &GeminiProvider{
		client: client,
		config: normalized,
	}, nil
}

// GenerateAudio generates audio using Gemini TTS and writes it to the output file.
func (p *GeminiProvider) GenerateAudio(ctx context.Context, text string, outputFile string) error {
	if err := ValidateBulgarianText(text); err != nil {
		return err
	}
	if p == nil || p.client == nil {
		return errors.New("gemini client not initialized")
	}

	prompt := p.buildPrompt(text)
	req := &genai.GenerateContentConfig{
		ResponseModalities: []string{string(genai.ModalityAudio)},
		SpeechConfig:       p.speechConfig(),
	}

	response, err := p.client.Models.GenerateContent(ctx, p.config.TTSModel, []*genai.Content{
		genai.NewContentFromText(prompt, genai.RoleUser),
	}, req)
	if err != nil {
		return fmt.Errorf("gemini API error: %w", err)
	}

	audioData, mimeType, err := extractAudioData(response)
	if err != nil {
		return err
	}

	if err := writeGeminiAudioFile(outputFile, audioData, mimeType); err != nil {
		return err
	}

	return nil
}

// Name returns the provider name.
func (p *GeminiProvider) Name() string {
	return "gemini"
}

// Voices returns the list of Gemini voices supported by the app.
func (p *GeminiProvider) Voices() []string {
	return GeminiVoices
}

// BuildAttribution returns the Gemini attribution text for a generated audio file.
func (p *GeminiProvider) BuildAttribution(params AttributionParams) string {
	return BuildGeminiAttribution(params)
}

// IsAvailable checks if the Google API key is configured.
func (p *GeminiProvider) IsAvailable() error {
	if p == nil || strings.TrimSpace(p.config.APIKey) == "" {
		return errors.New("google API key not configured")
	}

	return nil
}

func (p *GeminiProvider) buildPrompt(text string) string {
	var prompt strings.Builder
	prompt.WriteString(geminiPromptInstruction(p.config))
	prompt.WriteString("\n")
	prompt.WriteString(strings.TrimSpace(text))

	return prompt.String()
}

func (p *GeminiProvider) speechConfig() *genai.SpeechConfig {
	speechConfig := &genai.SpeechConfig{
		LanguageCode: geminiTTSLanguageCode,
	}

	if voice := strings.TrimSpace(p.config.Voice); voice != "" {
		speechConfig.VoiceConfig = &genai.VoiceConfig{
			PrebuiltVoiceConfig: &genai.PrebuiltVoiceConfig{
				VoiceName: voice,
			},
		}
	}

	return speechConfig
}

// normalizeGeminiAudioConfig applies defaults and trims whitespace from a GeminiAudioConfig.
func normalizeGeminiAudioConfig(config GeminiAudioConfig) GeminiAudioConfig {
	config.APIKey = strings.TrimSpace(config.APIKey)
	config.TTSModel = strings.TrimSpace(config.TTSModel)
	config.Voice = strings.TrimSpace(config.Voice)

	if config.TTSModel == "" {
		config.TTSModel = defaultGeminiTTSModel
	}
	if config.Speed <= 0 {
		config.Speed = 1.0
	}

	return config
}

func extractAudioData(response *genai.GenerateContentResponse) ([]byte, string, error) {
	if response == nil {
		return nil, "", errors.New("no response from Gemini")
	}

	for _, candidate := range response.Candidates {
		if candidate == nil || candidate.Content == nil {
			continue
		}

		for _, part := range candidate.Content.Parts {
			if part == nil || part.InlineData == nil || len(part.InlineData.Data) == 0 {
				continue
			}

			audio := append([]byte(nil), part.InlineData.Data...)
			return audio, part.InlineData.MIMEType, nil
		}
	}

	return nil, "", ErrGeminiNoAudioData
}

// IsGeminiNoAudioDataError reports whether the error means Gemini returned no audio payload.
func IsGeminiNoAudioDataError(err error) bool {
	return errors.Is(err, ErrGeminiNoAudioData)
}

func writeGeminiAudioFile(outputFile string, audioData []byte, mimeType string) error {
	if err := ensureOutputDirectory(outputFile); err != nil {
		return err
	}

	ext := strings.ToLower(filepath.Ext(outputFile))
	encoded, err := encodePCMAsWAV(audioData)
	if err != nil {
		return err
	}

	switch ext {
	case ".wav":
		if err := os.WriteFile(outputFile, encoded, 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		return nil
	case ".mp3":
		return transcodeWAVToMP3(encoded, outputFile)
	default:
		return fmt.Errorf("gemini TTS only supports .wav and .mp3 output files, got %q", outputFile)
	}
}

func ensureOutputDirectory(outputFile string) error {
	dir := filepath.Dir(outputFile)
	if dir == "" || dir == "." {
		return nil
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	return nil
}

func encodePCMAsWAV(pcmData []byte) ([]byte, error) {
	var buffer bytes.Buffer

	if _, err := buffer.WriteString("RIFF"); err != nil {
		return nil, err
	}
	if err := binary.Write(&buffer, binary.LittleEndian, uint32(36+len(pcmData))); err != nil {
		return nil, err
	}
	if _, err := buffer.WriteString("WAVE"); err != nil {
		return nil, err
	}
	if _, err := buffer.WriteString("fmt "); err != nil {
		return nil, err
	}
	if err := binary.Write(&buffer, binary.LittleEndian, uint32(16)); err != nil {
		return nil, err
	}
	if err := binary.Write(&buffer, binary.LittleEndian, uint16(1)); err != nil {
		return nil, err
	}
	if err := binary.Write(&buffer, binary.LittleEndian, uint16(geminiTTSChannels)); err != nil {
		return nil, err
	}
	if err := binary.Write(&buffer, binary.LittleEndian, uint32(geminiTTSSampleRate)); err != nil {
		return nil, err
	}

	byteRate := uint32(geminiTTSSampleRate * geminiTTSChannels * geminiTTSBitsPerSample / 8)
	if err := binary.Write(&buffer, binary.LittleEndian, byteRate); err != nil {
		return nil, err
	}

	blockAlign := uint16(geminiTTSChannels * geminiTTSBitsPerSample / 8)
	if err := binary.Write(&buffer, binary.LittleEndian, blockAlign); err != nil {
		return nil, err
	}
	if err := binary.Write(&buffer, binary.LittleEndian, uint16(geminiTTSBitsPerSample)); err != nil {
		return nil, err
	}
	if _, err := buffer.WriteString("data"); err != nil {
		return nil, err
	}
	if err := binary.Write(&buffer, binary.LittleEndian, uint32(len(pcmData))); err != nil {
		return nil, err
	}
	if _, err := buffer.Write(pcmData); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func transcodeWAVToMP3(wavData []byte, outputFile string) error {
	ffmpegPath, err := execLookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg is required to convert Gemini audio to mp3: %w", err)
	}

	cmd := execCommand(
		ffmpegPath,
		"-nostdin",
		"-hide_banner",
		"-loglevel", "error",
		"-y",
		"-f", "wav",
		"-i", "pipe:0",
		"-codec:a", "libmp3lame",
		"-q:a", "4",
		outputFile,
	)
	cmd.Stdin = bytes.NewReader(wavData)

	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("failed to convert Gemini audio to mp3: %s", message)
	}

	return nil
}
