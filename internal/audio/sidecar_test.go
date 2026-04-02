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
