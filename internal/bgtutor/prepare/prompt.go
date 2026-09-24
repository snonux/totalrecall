package prepare

import (
	"fmt"
	"sort"
	"strings"

	"github.com/snonux/totalrecall/internal/bgtutor"
)

// renderPrompt fills the embedded prompt.md template with episode details.
func renderPrompt(meta bgtutor.Meta, referenceNotes string) string {
	var speakers []string
	for _, s := range meta.Speakers {
		desc := s.Label
		if s.Name != "" || s.Role != "" {
			desc += " (" + strings.TrimSpace(strings.Join(nonEmpty(s.Name, s.Role), ", ")) + ")"
		}
		speakers = append(speakers, desc)
	}
	reference := ""
	if strings.TrimSpace(referenceNotes) != "" {
		reference = "- Follow the terminology, level and sequence of this course reference " +
			"when writing notes:\n\n<course_reference>\n" + strings.TrimSpace(referenceNotes) +
			"\n</course_reference>"
	}
	return strings.NewReplacer(
		"{{LEVEL}}", meta.Difficulty,
		"{{TITLE}}", meta.Title,
		"{{SPEAKERS}}", strings.Join(speakers, "; "),
		"{{REFERENCE}}", reference,
	).Replace(promptTemplate)
}

// chunkMessage is the user turn for one chunk. It carries the last paragraph
// of the previous chunk for continuity and the vocabulary already explained.
func chunkMessage(n, total int, chunk string, previous []bgtutor.Paragraph, covered map[string]bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Transcript part %d of %d.\n\n", n, total)
	if len(previous) > 0 {
		last := previous[len(previous)-1]
		fmt.Fprintf(&b, "The previous part ended with %s saying:\n<previous>\n%s\n</previous>\n"+
			"Do not repeat it; it is context only.\n\n", last.Speaker, last.English)
	}
	if len(covered) > 0 {
		words := make([]string, 0, len(covered))
		for w := range covered {
			words = append(words, w)
		}
		sort.Strings(words)
		fmt.Fprintf(&b, "Already covered vocabulary: %s\n\n", strings.Join(words, ", "))
	}
	fmt.Fprintf(&b, "<transcript>\n%s\n</transcript>", strings.TrimSpace(chunk))
	return b.String()
}

// responseSchema is the JSON schema the model must follow. Speaker labels are
// an enum so the output always matches meta.json.
func responseSchema(speakers []bgtutor.Speaker) map[string]any {
	labels := make([]any, 0, len(speakers))
	for _, s := range speakers {
		labels = append(labels, s.Label)
	}
	str := map[string]any{"type": "string"}
	vocab := map[string]any{
		"type":       "object",
		"properties": map[string]any{"bg": str, "en": str, "note": str},
		"required":   []any{"bg", "en", "note"},
	}
	paragraph := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"speaker":             map[string]any{"type": "string", "enum": labels},
			"english":             str,
			"bulgarian_reference": str,
			"grammar_notes":       map[string]any{"type": "array", "items": str},
			"vocabulary_notes":    map[string]any{"type": "array", "items": vocab},
			"background":          map[string]any{"type": []any{"string", "null"}},
		},
		"required": []any{"speaker", "english", "bulgarian_reference", "grammar_notes", "vocabulary_notes", "background"},
	}
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"paragraphs": map[string]any{"type": "array", "items": paragraph}},
		"required":   []any{"paragraphs"},
	}
}

func nonEmpty(values ...string) []string {
	var out []string
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}
