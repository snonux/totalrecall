package audio

import (
	"strings"
	"testing"
)

func TestProcessedTextForWord(t *testing.T) {
	got := ProcessedTextForWord("  ябълка!?  ")
	if got != "ябълка..." {
		t.Fatalf("ProcessedTextForWord() = %q, want %q", got, "ябълка...")
	}
}

func TestProcessedTextForProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		input    string
		want     string
	}{
		{
			name:     "openai strips punctuation",
			provider: "openai",
			input:    "  ябълка!?  ",
			want:     "ябълка",
		},
		{
			name:     "gemini keeps punctuation",
			provider: "gemini",
			input:    "  ябълка!?  ",
			want:     "ябълка!?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProcessedTextForProvider(tt.provider, tt.input); got != tt.want {
				t.Fatalf("ProcessedTextForProvider(%q, %q) = %q, want %q", tt.provider, tt.input, got, tt.want)
			}
		})
	}
}

func TestInstructionForProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		config   *Config
		want     []string
		wantNot  []string
	}{
		{
			name:     "openai model supports instructions",
			provider: "openai",
			config: &Config{
				OpenAIModel:       "gpt-4o-mini-tts",
				OpenAIInstruction: "Speak clearly.",
			},
			want: []string{"Speak clearly."},
		},
		{
			name:     "openai unsupported model omits instructions",
			provider: "openai",
			config: &Config{
				OpenAIModel:       "tts-1",
				OpenAIInstruction: "Speak clearly.",
			},
			want:    []string{""},
			wantNot: []string{"Speak clearly."},
		},
		{
			name:     "gemini default voice and speed semantics",
			provider: "gemini",
			config: &Config{
				GeminiSpeed: 1.0,
			},
			want: []string{"Speak at a natural pace.", "Speak the following Bulgarian text:"},
		},
		{
			name:     "gemini explicit voice semantics",
			provider: "gemini",
			config: &Config{
				GeminiSpeed: 0.9,
				GeminiVoice: "Kore",
			},
			want: []string{"Speak slowly and clearly for language learners.", "voice named Kore."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InstructionForProvider(tt.provider, tt.config)
			for _, want := range tt.want {
				if want == "" {
					if got != "" {
						t.Fatalf("InstructionForProvider(%q, %+v) = %q, want empty", tt.provider, tt.config, got)
					}
					continue
				}
				if !strings.Contains(got, want) {
					t.Fatalf("InstructionForProvider(%q, %+v) = %q, missing %q", tt.provider, tt.config, got, want)
				}
			}
			for _, want := range tt.wantNot {
				if strings.Contains(got, want) {
					t.Fatalf("InstructionForProvider(%q, %+v) = %q, unexpectedly contained %q", tt.provider, tt.config, got, want)
				}
			}
		})
	}
}

func TestBuildSidecarMetadata(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want []string
	}{
		{
			name: "gemini bg-bg",
			got: BuildSidecarMetadata(SidecarMetadataParams{
				Provider:       "gemini",
				OutputFormat:   "wav",
				CardType:       "bg-bg",
				AudioFile:      "audio_front.wav",
				AudioFileBack:  "audio_back.wav",
				GeminiTTSModel: "gemini-2.5-flash-preview-tts",
				GeminiVoice:    "",
				GeminiSpeed:    1.00,
			}),
			want: []string{
				"provider=gemini",
				"model=gemini-2.5-flash-preview-tts",
				"voice=model-default",
				"speed=1.00",
				"format=wav",
				"cardtype=bg-bg",
				"audio_file=audio_front.wav",
				"audio_file_back=audio_back.wav",
			},
		},
		{
			name: "openai en-bg",
			got: BuildSidecarMetadata(SidecarMetadataParams{
				Provider:          "openai",
				OutputFormat:      "mp3",
				CardType:          "en-bg",
				AudioFile:         "audio.mp3",
				OpenAIModel:       "gpt-4o-mini-tts",
				OpenAIVoice:       "alloy",
				OpenAISpeed:       0.95,
				OpenAIInstruction: "Speak clearly.",
			}),
			want: []string{
				"provider=openai",
				"model=gpt-4o-mini-tts",
				"voice=alloy",
				"speed=0.95",
				"instruction=Speak clearly.",
				"format=mp3",
				"cardtype=en-bg",
				"audio_file=audio.mp3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, want := range tt.want {
				if !strings.Contains(tt.got, want) {
					t.Fatalf("metadata = %q, missing %q", tt.got, want)
				}
			}
		})
	}
}
