package image

import (
	"fmt"
	"strings"
)

const maxImagePromptChars = 1000

func promptSubject(englishTranslation, fallback string) string {
	subject := normalizePromptText(englishTranslation)
	if subject != "" {
		return subject
	}

	subject = normalizePromptText(fallback)
	if subject != "" {
		return subject
	}

	return "the requested term"
}

func normalizePromptText(text string) string {
	text = trimMarkdownFence(text)
	text = strings.TrimSpace(text)
	text = strings.Trim(text, "`\"'")
	text = strings.Join(strings.Fields(text), " ")
	return strings.TrimSpace(text)
}

func sanitizeSceneDescription(scene string) string {
	scene = normalizePromptText(scene)
	lower := strings.ToLower(scene)

	for _, prefix := range []string{"scene description:", "scene:", "description:", "image prompt:", "prompt:"} {
		if strings.HasPrefix(lower, prefix) {
			scene = strings.TrimSpace(scene[len(prefix):])
			break
		}
	}

	return strings.TrimSpace(strings.Trim(scene, "."))
}

func usableSceneDescription(scene string) bool {
	scene = sanitizeSceneDescription(scene)
	if scene == "" {
		return false
	}
	if len(scene) < 24 {
		return false
	}
	if len(strings.Fields(scene)) < 3 {
		return false
	}
	return true
}

func trimMarkdownFence(text string) string {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "```") {
		return text
	}

	lines := strings.Split(text, "\n")
	if len(lines) < 3 {
		return text
	}
	if !strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
		return text
	}
	if strings.TrimSpace(lines[len(lines)-1]) != "```" {
		return text
	}

	return strings.Join(lines[1:len(lines)-1], "\n")
}

func withTerminalPunctuation(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	switch {
	case strings.HasSuffix(text, "."):
		return text
	case strings.HasSuffix(text, "!"):
		return text
	case strings.HasSuffix(text, "?"):
		return text
	default:
		return text + "."
	}
}

// buildEducationalPrompt assembles the final image-generation prompt from a
// pre-chosen artistic style, an optional scene description, and the word subject.
// Both OpenAIClient and NanoBananaClient share this logic so the prompt policy
// has a single authoritative home. The scene parameter may be empty, in which
// case a simpler fallback prompt is used. The result is always capped at
// maxImagePromptChars characters.
func buildEducationalPrompt(style, scene, subject string) string {
	var prompt string

	if scene != "" {
		fullPrompt := fmt.Sprintf(
			"Generate a %s educational flashcard image illustrating \"%s\". Scene: %s "+
				"The image should be educational and suitable for language learning flashcards. "+
				"Requirements: The main subject or concept must be clearly visible, easily recognizable, and prominent in the image. It should occupy the central area with sharp focus and proper lighting. Ensure the scene makes \"%s\" immediately identifiable. "+
				"IMPORTANT: No text whatsoever. Do not include any words, letters, typography, labels, captions, or writing of any kind. Image only, without any text elements.",
			style, subject, withTerminalPunctuation(scene), subject,
		)

		if len(fullPrompt) <= maxImagePromptChars {
			prompt = fullPrompt
		} else {
			// Try a shorter version without the IMPORTANT notice.
			prompt = fmt.Sprintf(
				"Generate a %s flashcard image illustrating \"%s\". Scene: %s "+
					"The image should be educational and suitable for language learning flashcards. "+
					"Requirements: The main subject or concept must be clearly visible, centered, well lit, and easy to identify.",
				style, subject, withTerminalPunctuation(scene),
			)

			// If still too long, truncate the scene to fit.
			if len(prompt) > maxImagePromptChars {
				template := fmt.Sprintf(
					"Generate a %s flashcard image illustrating \"%s\". Scene:  "+
						"The image should be educational and suitable for language learning flashcards. "+
						"Requirements: The main subject or concept must be clearly visible, centered, well lit, and easy to identify.",
					style, subject,
				)
				maxSceneLen := maxImagePromptChars - len(template)
				if maxSceneLen > 3 && len(scene) > maxSceneLen {
					scene = scene[:maxSceneLen] + "..."
				}
				prompt = fmt.Sprintf(
					"Generate a %s flashcard image illustrating \"%s\". Scene: %s "+
						"The image should be educational and suitable for language learning flashcards. "+
						"Requirements: The main subject or concept must be clearly visible, centered, well lit, and easy to identify.",
					style, subject, withTerminalPunctuation(scene),
				)
			}
		}
	} else {
		// No scene available — use a simpler fallback prompt.
		prompt = fmt.Sprintf(
			"Generate a %s educational flashcard image illustrating \"%s\". %s "+
				"The image should be educational and suitable for language learning flashcards. "+
				"Requirements: The main subject or concept must be clearly visible, easily recognizable, and prominent in the image. Show it prominently centered with excellent lighting and sharp focus. "+
				"IMPORTANT: No text whatsoever. Do not include any words, letters, typography, labels, captions, or writing of any kind. Image only, without any text elements.",
			style, subject, fallbackVisualDirection(subject),
		)
	}

	// Hard cap at maxImagePromptChars.
	if len(prompt) > maxImagePromptChars {
		prompt = prompt[:997] + "..."
	}

	return prompt
}

func fallbackVisualDirection(subject string) string {
	subject = normalizePromptText(subject)
	lower := strings.ToLower(subject)

	if strings.HasPrefix(lower, "to ") || len(strings.Fields(subject)) > 1 {
		return "Show a realistic everyday scene with people, actions, facial expressions, and surrounding objects that make the meaning of \"" + subject + "\" obvious without any text."
	}

	return "Show a single " + subject + " as the clear focal point, prominently centered and immediately recognizable."
}
