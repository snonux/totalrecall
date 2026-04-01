package image

import (
	"reflect"
	"testing"
)

func TestArtisticStyles(t *testing.T) {
	if len(ArtisticStyles) == 0 {
		t.Fatal("ArtisticStyles should not be empty")
	}

	if !hasStyle(ArtisticStyles, "Photorealism") {
		t.Fatal(`ArtisticStyles should include "Photorealism"`)
	}

	if !hasStyle(ArtisticStyles, "Candid Photography") {
		t.Fatal(`ArtisticStyles should include "Candid Photography"`)
	}
}

func TestChooseArtisticStyle_EmptyPool(t *testing.T) {
	original := ArtisticStyles
	ArtisticStyles = nil
	t.Cleanup(func() {
		ArtisticStyles = original
	})

	if got := chooseArtisticStyle(); got != defaultArtisticStyle {
		t.Fatalf("chooseArtisticStyle() = %q, want %q", got, defaultArtisticStyle)
	}
}

func TestPickArtisticStyle_DoesNotMutateSharedPool(t *testing.T) {
	original := append([]string(nil), ArtisticStyles...)
	t.Cleanup(func() {
		ArtisticStyles = original
	})

	ArtisticStyles = []string{"Photorealism", "Surrealism", "Impressionism"}

	_ = pickArtisticStyle()

	if !reflect.DeepEqual(ArtisticStyles, []string{"Photorealism", "Surrealism", "Impressionism"}) {
		t.Fatalf("pickArtisticStyle() mutated shared pool: got %v", ArtisticStyles)
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
