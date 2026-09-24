package prepare

import "strings"

// SplitTranscript cuts a transcript into chunks of roughly maxWords words for
// separate LLM calls. It only cuts between lines (speaker turns in most
// transcripts), so a turn is never split across chunks unless a single line is
// longer than maxWords, in which case it is cut at sentence ends.
func SplitTranscript(text string, maxWords int) []string {
	var chunks []string
	var cur []string
	words := 0
	flush := func() {
		if len(cur) > 0 {
			chunks = append(chunks, strings.Join(cur, "\n"))
			cur, words = nil, 0
		}
	}
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		for _, piece := range splitLongLine(line, maxWords) {
			n := len(strings.Fields(piece))
			if words > 0 && words+n > maxWords {
				flush()
			}
			cur = append(cur, piece)
			words += n
		}
	}
	flush()
	return chunks
}

// splitLongLine breaks a line longer than maxWords at sentence boundaries.
func splitLongLine(line string, maxWords int) []string {
	fields := strings.Fields(line)
	if len(fields) <= maxWords {
		return []string{line}
	}
	var pieces []string
	start := 0
	for i, f := range fields {
		endOfSentence := strings.HasSuffix(f, ".") || strings.HasSuffix(f, "?") || strings.HasSuffix(f, "!")
		if (endOfSentence && i+1-start >= maxWords/2) || i+1-start >= maxWords {
			pieces = append(pieces, strings.Join(fields[start:i+1], " "))
			start = i + 1
		}
	}
	if start < len(fields) {
		pieces = append(pieces, strings.Join(fields[start:], " "))
	}
	return pieces
}
