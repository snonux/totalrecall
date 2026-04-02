package audio

import (
	"reflect"
	"testing"
)

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
