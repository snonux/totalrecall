package config

import (
	"errors"
	"testing"
)

func TestHomeDirReturnsFallbackWhenResolutionFails(t *testing.T) {
	oldUserHomeDir := userHomeDir
	t.Cleanup(func() {
		userHomeDir = oldUserHomeDir
	})

	userHomeDir = func() (string, error) {
		return "", errors.New("boom")
	}

	homeDir, err := HomeDir()
	if err == nil {
		t.Fatal("HomeDir() error = nil, want error")
	}
	if homeDir != "." {
		t.Fatalf("HomeDir() homeDir = %q, want %q", homeDir, ".")
	}
}
