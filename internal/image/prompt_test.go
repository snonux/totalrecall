package image

import (
	"strings"
	"testing"
)

func TestPromptSubjectUsesTranslationFirst(t *testing.T) {
	if got := promptSubject(" apple ", "ябълка"); got != "apple" {
		t.Fatalf("promptSubject() = %q, want %q", got, "apple")
	}
}

func TestPromptSubjectFallsBackToOriginalWord(t *testing.T) {
	if got := promptSubject("   ", "ябълка"); got != "ябълка" {
		t.Fatalf("promptSubject() = %q, want %q", got, "ябълка")
	}
}

func TestSanitizeSceneDescriptionRemovesLabelsAndFences(t *testing.T) {
	scene := "```text\nScene: A bright apple sits centered on a wooden table.\n```"
	if got := sanitizeSceneDescription(scene); got != "A bright apple sits centered on a wooden table" {
		t.Fatalf("sanitizeSceneDescription() = %q", got)
	}
}

func TestUsableSceneDescriptionRejectsTrivialContent(t *testing.T) {
	if usableSceneDescription("A") {
		t.Fatal("usableSceneDescription() unexpectedly accepted trivial scene")
	}
	if usableSceneDescription("A single, perfectly") {
		t.Fatal("usableSceneDescription() unexpectedly accepted incomplete scene fragment")
	}
}

func TestFallbackVisualDirectionForPhrase(t *testing.T) {
	got := fallbackVisualDirection("to indulge someone")
	if got == "" || !usableSceneDescription(got) {
		t.Fatalf("fallbackVisualDirection() returned unusable phrase direction: %q", got)
	}
}

func TestFallbackVisualDirectionForSingleWord(t *testing.T) {
	got := fallbackVisualDirection("apple")
	if got == "" || !strings.Contains(got, "single apple") {
		t.Fatalf("fallbackVisualDirection() = %q", got)
	}
}
