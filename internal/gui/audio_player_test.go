package gui

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLinuxAudioCommandCandidates(t *testing.T) {
	t.Run("mp3 prefers mpg123", func(t *testing.T) {
		audioFile := "/tmp/audio.mp3"
		got := linuxAudioCommandCandidates(audioFile)
		want := []audioCommandCandidate{
			{name: "mpg123", args: []string{"-q", audioFile}},
			{name: "ffplay", args: []string{"-nodisp", "-autoexit", "-loglevel", "quiet", audioFile}},
			{name: "play", args: []string{"-q", audioFile}},
			{name: "paplay", args: []string{audioFile}},
			{name: "aplay", args: []string{"-q", audioFile}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("linuxAudioCommandCandidates(mp3) mismatch\nwant: %#v\ngot:  %#v", want, got)
		}
	})

	t.Run("wav avoids mpg123", func(t *testing.T) {
		audioFile := "/tmp/audio.wav"
		got := linuxAudioCommandCandidates(audioFile)
		if got[0].name != "ffplay" {
			t.Fatalf("first wav candidate = %q, want %q", got[0].name, "ffplay")
		}
		for _, candidate := range got {
			if candidate.name == "mpg123" {
				t.Fatalf("wav candidates unexpectedly include mpg123: %#v", got)
			}
		}
	})
}

func TestLinuxAudioPlaybackCommandUsesFormatCompatiblePlayer(t *testing.T) {
	audioFile := "/tmp/audio.wav"
	cmd, err := linuxAudioPlaybackCommand(audioFile, func(name string) (string, error) {
		switch name {
		case "ffplay":
			return filepath.Join("/usr/bin", name), nil
		default:
			return "", errors.New("not found")
		}
	})
	if err != nil {
		t.Fatalf("linuxAudioPlaybackCommand() unexpected error: %v", err)
	}

	if got, want := filepath.Base(cmd.Path), "ffplay"; got != want {
		t.Fatalf("command path base = %q, want %q", got, want)
	}
	if len(cmd.Args) < 2 || cmd.Args[len(cmd.Args)-1] != audioFile {
		t.Fatalf("command args = %#v, want final arg %q", cmd.Args, audioFile)
	}
}
