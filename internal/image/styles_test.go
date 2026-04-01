package image

import (
	"reflect"
	"testing"
)

func TestArtisticStyles(t *testing.T) {
	if len(artisticStyles) == 0 {
		t.Fatal("artisticStyles should not be empty")
	}

	if !hasStyle(artisticStyles, "Photorealism") {
		t.Fatal(`artisticStyles should include "Photorealism"`)
	}

	if !hasStyle(artisticStyles, "Candid Photography") {
		t.Fatal(`artisticStyles should include "Candid Photography"`)
	}
}

func TestChooseArtisticStyle_EmptyPool(t *testing.T) {
	original := artisticStyles
	artisticStyles = nil
	t.Cleanup(func() {
		artisticStyles = original
	})

	if got := chooseArtisticStyle(); got != defaultArtisticStyle {
		t.Fatalf("chooseArtisticStyle() = %q, want %q", got, defaultArtisticStyle)
	}
}

func TestPickArtisticStyle_DoesNotMutateSharedPool(t *testing.T) {
	original := append([]string(nil), artisticStyles...)
	t.Cleanup(func() {
		artisticStyles = original
	})

	artisticStyles = []string{"Photorealism", "Surrealism", "Impressionism"}

	_ = pickArtisticStyle()

	if !reflect.DeepEqual(artisticStyles, []string{"Photorealism", "Surrealism", "Impressionism"}) {
		t.Fatalf("pickArtisticStyle() mutated shared pool: got %v", artisticStyles)
	}
}

func hasStyle(styles []string, want string) bool {
	for _, style := range styles {
		if style == want {
			return true
		}
	}

	return false
}
