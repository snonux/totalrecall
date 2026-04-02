package gui

import "codeberg.org/snonux/totalrecall/internal/anki"

func (a *Application) resolveSingleAudioFile(wordDir string) string {
	return anki.ResolveAudioFile(wordDir, "audio", "")
}

func (a *Application) resolveBgBgAudioFiles(wordDir string) (string, string) {
	return anki.ResolveAudioFile(wordDir, "audio_front", ""), anki.ResolveAudioFile(wordDir, "audio_back", "")
}

func (a *Application) hasAnyAudioFile(wordDir string) bool {
	single := a.resolveSingleAudioFile(wordDir)
	if single != "" {
		return true
	}

	front, back := a.resolveBgBgAudioFiles(wordDir)
	return front != "" || back != ""
}
