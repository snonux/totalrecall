package audio

import (
	"errors"
	"reflect"
	"testing"
)

var errTestSentinel = errors.New("test sentinel error")

func TestVoiceLists(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  []string
		want []string
	}{
		{
			name: "openai",
			got:  OpenAIVoices,
			want: []string{"alloy", "ash", "ballad", "coral", "echo", "fable", "onyx", "nova", "sage", "shimmer", "verse"},
		},
		{
			name: "gemini",
			got:  GeminiVoices,
			want: []string{"Zephyr", "Puck", "Charon", "Kore", "Fenrir", "Leda", "Orus", "Aoede", "Callirrhoe", "Autonoe", "Enceladus", "Iapetus", "Umbriel", "Algieba", "Despina", "Erinome", "Gacrux", "Pulcherrima", "Achernar", "Rasalgethi", "Laomedeia", "Sadachbia", "Schedar", "Sulafat", "Vindemiatrix", "Zubenelgenubi"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if !reflect.DeepEqual(tt.got, tt.want) {
				t.Fatalf("%s voice list mismatch\nwant: %#v\ngot:  %#v", tt.name, tt.want, tt.got)
			}
		})
	}
}

func TestGeminiVoiceFallbacks(t *testing.T) {
	t.Parallel()

	t.Run("selected voice comes first", func(t *testing.T) {
		t.Parallel()

		got := GeminiVoiceFallbacks("Kore")
		wantPrefix := []string{"Kore", "Zephyr", "Puck", "Charon"}
		if !reflect.DeepEqual(got[:len(wantPrefix)], wantPrefix) {
			t.Fatalf("GeminiVoiceFallbacks() prefix mismatch\nwant: %#v\ngot:  %#v", wantPrefix, got[:len(wantPrefix)])
		}
	})

	t.Run("empty selection returns known voices", func(t *testing.T) {
		t.Parallel()

		got := GeminiVoiceFallbacks("")
		if !reflect.DeepEqual(got, GeminiVoices) {
			t.Fatalf("GeminiVoiceFallbacks() mismatch\nwant: %#v\ngot:  %#v", GeminiVoices, got)
		}
	})
}

func TestRunWithVoiceFallbacks(t *testing.T) {
	originalVoices := append([]string(nil), GeminiVoices...)
	t.Cleanup(func() {
		GeminiVoices = originalVoices
	})
	GeminiVoices = []string{"Charon", "Kore", "Leda"}

	t.Run("retries no-audio errors until success", func(t *testing.T) {
		var attempted []string
		usedVoice, err := RunWithVoiceFallbacks("Charon", func(voice string) error {
			attempted = append(attempted, voice)
			if voice == "Charon" {
				return ErrGeminiNoAudioData
			}
			return nil
		}, nil)
		if err != nil {
			t.Fatalf("RunWithVoiceFallbacks() unexpected error: %v", err)
		}
		if usedVoice != "Kore" {
			t.Fatalf("used voice = %q, want %q", usedVoice, "Kore")
		}
		if got, want := attempted, []string{"Charon", "Kore"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("attempted voices = %#v, want %#v", got, want)
		}
	})

	t.Run("returns non-retryable errors immediately", func(t *testing.T) {
		sentinel := errTestSentinel
		var attempted []string
		usedVoice, err := RunWithVoiceFallbacks("Charon", func(voice string) error {
			attempted = append(attempted, voice)
			return sentinel
		}, nil)
		if !errors.Is(err, sentinel) {
			t.Fatalf("error = %v, want %v", err, sentinel)
		}
		if usedVoice != "" {
			t.Fatalf("used voice = %q, want empty", usedVoice)
		}
		if got, want := attempted, []string{"Charon"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("attempted voices = %#v, want %#v", got, want)
		}
	})

	t.Run("wraps exhausted Gemini voices", func(t *testing.T) {
		var attempted []string
		usedVoice, err := RunWithVoiceFallbacks("Charon", func(voice string) error {
			attempted = append(attempted, voice)
			return ErrGeminiNoAudioData
		}, nil)
		if !errors.Is(err, ErrGeminiNoAudioData) {
			t.Fatalf("error = %v, want wrapped ErrGeminiNoAudioData", err)
		}
		if usedVoice != "" {
			t.Fatalf("used voice = %q, want empty", usedVoice)
		}
		if got, want := attempted, []string{"Charon", "Kore", "Leda"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("attempted voices = %#v, want %#v", got, want)
		}
		if got := err.Error(); got != "gemini returned no audio for voices Charon, Kore, Leda: no audio data returned from Gemini" {
			t.Fatalf("error text = %q, want wrapped attempted-voices summary", got)
		}
	})
}
