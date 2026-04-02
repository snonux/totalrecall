package image

import "strings"

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

func fallbackVisualDirection(subject string) string {
	subject = normalizePromptText(subject)
	lower := strings.ToLower(subject)

	if strings.HasPrefix(lower, "to ") || len(strings.Fields(subject)) > 1 {
		return "Show a realistic everyday scene with people, actions, facial expressions, and surrounding objects that make the meaning of \"" + subject + "\" obvious without any text."
	}

	return "Show a single " + subject + " as the clear focal point, prominently centered and immediately recognizable."
}
