package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
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

// GeminiProvider implements Provider interface for Gemini TTS.
type GeminiProvider struct {
	client *genai.Client
	config *Config
}

var _ Provider = (*GeminiProvider)(nil)

// NewGeminiProvider creates a new Gemini TTS provider.
func NewGeminiProvider(config *Config) (Provider, error) {
	normalized := normalizeGeminiConfig(config)
	if normalized.GoogleAPIKey == "" {
		return nil, errors.New("google API key is required")
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  normalized.GoogleAPIKey,
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
	if p == nil || p.client == nil || p.config == nil {
		return errors.New("gemini client not initialized")
	}

	prompt := p.buildPrompt(text)
	req := &genai.GenerateContentConfig{
		ResponseModalities: []string{string(genai.ModalityAudio)},
		SpeechConfig:       p.speechConfig(),
	}

	response, err := p.client.Models.GenerateContent(ctx, p.config.GeminiTTSModel, []*genai.Content{
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

// IsAvailable checks if the Google API key is configured.
func (p *GeminiProvider) IsAvailable() error {
	if p == nil || p.config == nil || strings.TrimSpace(p.config.GoogleAPIKey) == "" {
		return errors.New("google API key not configured")
	}

	return nil
}

func (p *GeminiProvider) buildPrompt(text string) string {
	config := p.config
	if config == nil {
		config = &Config{}
	}

	var prompt strings.Builder
	prompt.WriteString(geminiPromptInstruction(config))
	prompt.WriteString("\n")
	prompt.WriteString(strings.TrimSpace(text))

	return prompt.String()
}

func (p *GeminiProvider) speechConfig() *genai.SpeechConfig {
	config := p.config
	if config == nil {
		config = &Config{}
	}

	speechConfig := &genai.SpeechConfig{
		LanguageCode: geminiTTSLanguageCode,
	}

	if voice := strings.TrimSpace(config.GeminiVoice); voice != "" {
		speechConfig.VoiceConfig = &genai.VoiceConfig{
			PrebuiltVoiceConfig: &genai.PrebuiltVoiceConfig{
				VoiceName: voice,
			},
		}
	}

	return speechConfig
}

func normalizeGeminiConfig(config *Config) *Config {
	normalized := &Config{}
	if config != nil {
		*normalized = *config
	}

	normalized.GoogleAPIKey = strings.TrimSpace(normalized.GoogleAPIKey)
	normalized.GeminiTTSModel = strings.TrimSpace(normalized.GeminiTTSModel)
	normalized.GeminiVoice = strings.TrimSpace(normalized.GeminiVoice)

	if normalized.GeminiTTSModel == "" {
		normalized.GeminiTTSModel = defaultGeminiTTSModel
	}
	if normalized.GeminiSpeed <= 0 {
		normalized.GeminiSpeed = 1.0
	}

	return normalized
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

	return nil, "", errors.New("no audio data returned from Gemini")
}

func writeGeminiAudioFile(outputFile string, audioData []byte, mimeType string) error {
	if err := ensureOutputDirectory(outputFile); err != nil {
		return err
	}

	ext := strings.ToLower(filepath.Ext(outputFile))
	if ext != ".wav" {
		return fmt.Errorf("gemini TTS only supports .wav output files, got %q", outputFile)
	}

	encoded, err := encodePCMAsWAV(audioData)
	if err != nil {
		return err
	}

	if err := os.WriteFile(outputFile, encoded, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
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
