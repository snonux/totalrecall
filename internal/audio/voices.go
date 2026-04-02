package audio

import "strings"

// OpenAIVoices lists the OpenAI voices supported by the app.
var OpenAIVoices = []string{
	"alloy",
	"ash",
	"ballad",
	"coral",
	"echo",
	"fable",
	"onyx",
	"nova",
	"sage",
	"shimmer",
	"verse",
}

// GeminiVoices lists the Gemini prebuilt voices supported by the app.
var GeminiVoices = []string{
	"Zephyr",
	"Puck",
	"Charon",
	"Kore",
	"Fenrir",
	"Leda",
	"Orus",
	"Aoede",
	"Callirrhoe",
	"Autonoe",
	"Enceladus",
	"Iapetus",
	"Umbriel",
	"Algieba",
	"Despina",
	"Erinome",
	"Gacrux",
	"Pulcherrima",
	"Achernar",
	"Rasalgethi",
	"Laomedeia",
	"Sadachbia",
	"Schedar",
	"Sulafat",
	"Vindemiatrix",
	"Zubenelgenubi",
}

// GeminiVoiceFallbacks returns the selected voice first, followed by the remaining known Gemini voices.
func GeminiVoiceFallbacks(selected string) []string {
	selected = strings.TrimSpace(selected)
	if selected == "" {
		return append([]string(nil), GeminiVoices...)
	}

	fallbacks := []string{selected}
	seen := map[string]struct{}{selected: {}}
	for _, voice := range GeminiVoices {
		voice = strings.TrimSpace(voice)
		if voice == "" {
			continue
		}
		if _, ok := seen[voice]; ok {
			continue
		}
		fallbacks = append(fallbacks, voice)
		seen[voice] = struct{}{}
	}

	return fallbacks
}
