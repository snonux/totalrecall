package audio

import (
	"fmt"
	"strings"
)

// RunWithVoiceFallbacks tries the selected Gemini voice first, then the remaining known voices.
func RunWithVoiceFallbacks(initialVoice string, generate func(voice string) error, warnNoAudio func(voice string)) (usedVoice string, err error) {
	attempted := make([]string, 0, len(GeminiVoices))
	var lastErr error

	for _, voice := range GeminiVoiceFallbacks(initialVoice) {
		attempted = append(attempted, voice)

		err := generate(voice)
		if err == nil {
			return voice, nil
		}
		if !IsGeminiNoAudioDataError(err) {
			return "", err
		}

		lastErr = err
		if warnNoAudio != nil {
			warnNoAudio(voice)
		}
	}

	return "", fmt.Errorf("gemini returned no audio for voices %s: %w", strings.Join(attempted, ", "), lastErr)
}
