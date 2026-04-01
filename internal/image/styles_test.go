package image

import "testing"

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

func hasStyle(styles []string, want string) bool {
	for _, style := range styles {
		if style == want {
			return true
		}
	}

	return false
}
