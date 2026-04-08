package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// KeyboardShortcuts wires global canvas shortcuts and the hotkey help dialog (SRP).
type KeyboardShortcuts struct {
	app *Application
}

// hotkeysMarkdown is the markdown reference text shown in the hotkeys dialog.
const hotkeysMarkdown = `[Project Page: https://codeberg.org/snonux/totalrecall](https://codeberg.org/snonux/totalrecall)

---

## Navigation
**← / h/х** Previous word (vim-style)
**→ / l/л** Next word (vim-style)
**Tab** Navigate fields
**Esc** Unfocus field

## Focus Fields
**b/б** Focus Bulgarian input
**e/е** Focus English input
**o/о** Focus image prompt

## Word Processing
**g/г** Generate word
**n/н** New word
**d/д** Delete word

## Regeneration
**i/и** Regenerate image
**m/м** Random image
**a/а** Regenerate audio (front for bg-bg)
**A/А** Regenerate back audio (bg-bg only)
**r/р** Regenerate all

## Playback
**p/п** Play front audio (or audio for en-bg)
**P/П** Play back audio (bg-bg only)
**u/у** Toggle auto-play

## Export & Archive
**x/ж** Export to Anki
**v/в** Archive all cards

## Help
**?** Show hotkeys
**c/ц** Close dialog
**q/ч** Quit application

## Dialogs
**y/ъ** Confirm action
**n/н** Cancel action
**c/ц** Cancel action
**Esc** Cancel action

---
*All hotkeys work with both Latin and Cyrillic keyboards*

Press **c/ц** or **Esc** to close this dialog`

// onShowHotkeys builds the keyboard-shortcut reference dialog and wires temporary
// c/ц and Esc handlers to close it. Original handlers are restored via setupKeyboardShortcuts
// when the dialog is dismissed.
func (ks *KeyboardShortcuts) onShowHotkeys() {
	a := ks.app
	content := widget.NewRichTextFromMarkdown(hotkeysMarkdown)
	content.Wrapping = fyne.TextWrapWord

	scroll := container.NewScroll(container.NewPadded(content))
	scroll.SetMinSize(fyne.NewSize(700, 480))

	d := dialog.NewCustom("Keyboard Shortcuts", "Close", scroll, a.window)
	ks.wireHotkeysDialog(d)
}

// wireHotkeysDialog attaches temporary c/ц and Esc key handlers that close the
// dialog, then restores normal shortcuts via setupKeyboardShortcuts on close.
func (ks *KeyboardShortcuts) wireHotkeysDialog(d *dialog.CustomDialog) {
	a := ks.app
	dialogOpen := true
	originalRuneHandler := a.window.Canvas().OnTypedRune()
	originalKeyHandler := a.window.Canvas().OnTypedKey()

	a.window.Canvas().SetOnTypedRune(func(r rune) {
		if dialogOpen && (r == 'c' || r == 'C' || r == 'ц' || r == 'Ц') {
			d.Hide()
			return
		}
		if originalRuneHandler != nil {
			originalRuneHandler(r)
		}
	})

	a.window.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if dialogOpen && ev.Name == fyne.KeyEscape {
			d.Hide()
			return
		}
		if originalKeyHandler != nil {
			originalKeyHandler(ev)
		}
	})

	d.SetOnClosed(func() {
		dialogOpen = false
		ks.setupKeyboardShortcuts()
	})

	d.Show()
}

// setupKeyboardShortcuts registers rune and key handlers on the window canvas.
// Rune events handle focus shortcuts and Cyrillic action keys; key events handle
// Latin/function keys, Escape, and Tab navigation.
func (ks *KeyboardShortcuts) setupKeyboardShortcuts() {
	a := ks.app
	a.window.Canvas().SetOnTypedRune(ks.handleTypedRune)
	a.window.Canvas().SetOnTypedKey(ks.handleTypedKey)
}

// handleTypedRune processes character-based shortcuts, supporting both Latin and
// Cyrillic keyboard layouts. No-op when an input field is focused or a confirmation
// dialog is active.
func (ks *KeyboardShortcuts) handleTypedRune(r rune) {
	a := ks.app
	focused := a.window.Canvas().Focused()
	isInputFocused := focused == a.wordInput || focused == a.imagePromptEntry || focused == a.translationEntry
	if isInputFocused || a.deleteConfirming || a.quitConfirming {
		return
	}

	switch r {
	case 'b', 'B', 'б', 'Б':
		a.window.Canvas().Focus(a.wordInput)
	case 'e', 'E', 'е', 'Е':
		a.window.Canvas().Focus(a.translationEntry)
	case 'o', 'O', 'о', 'О':
		a.window.Canvas().Focus(a.imagePromptEntry)
	case 'г', 'Г':
		if !a.submitButton.Disabled() {
			a.onSubmit()
		}
	case 'н', 'Н':
		if !a.keepButton.Disabled() {
			a.onKeepAndContinue()
		}
	case 'и', 'И':
		if !a.regenerateImageBtn.Disabled() {
			a.onRegenerateImage()
		}
	case 'м', 'М':
		if !a.regenerateRandomImageBtn.Disabled() {
			a.onRegenerateRandomImage()
		}
	case 'a', 'а':
		if !a.regenerateAudioBtn.Disabled() {
			a.onRegenerateAudio()
		}
	case 'A', 'А':
		if a.currentCardType == "bg-bg" {
			a.onRegenerateBackAudio()
		}
	case 'р', 'Р':
		if !a.regenerateAllBtn.Disabled() {
			a.onRegenerateAll()
		}
	case 'д', 'Д':
		if !a.deleteButton.Disabled() {
			a.onDelete()
		}
	case 'p', 'п':
		if a.currentAudioFile != "" {
			a.audioPlayer.Play()
		}
	case 'P', 'П':
		if a.currentAudioFileBack != "" {
			a.audioPlayer.PlayBack()
		}
	case 'ж', 'Ж':
		a.export.onExportToAnki()
	case 'в', 'В':
		a.onArchive()
	case '?':
		ks.onShowHotkeys()
	case 'h', 'H', 'х', 'Х':
		if !a.prevWordBtn.Disabled() {
			a.onPrevWord()
		}
	case 'l', 'L', 'л', 'Л':
		if !a.nextWordBtn.Disabled() {
			a.onNextWord()
		}
	case 'ч', 'Ч':
		a.onQuitConfirm()
	case 'u', 'U', 'у', 'У':
		a.toggleAutoPlay()
	}
}

// handleTypedKey processes key-event shortcuts (Latin letters, arrows, Escape, Tab).
// Escape always unfocuses; Tab cycles focus. All others are ignored when an input
// field is focused or a confirmation dialog is active.
func (ks *KeyboardShortcuts) handleTypedKey(ev *fyne.KeyEvent) {
	a := ks.app
	focused := a.window.Canvas().Focused()
	isInputFocused := focused == a.wordInput || focused == a.imagePromptEntry || focused == a.translationEntry

	if ev.Name == fyne.KeyEscape {
		a.window.Canvas().Unfocus()
		a.deleteConfirming = false
		a.quitConfirming = false
		return
	}

	if ev.Name == fyne.KeyTab {
		ks.handleTabNavigation()
		return
	}

	if isInputFocused || a.deleteConfirming || a.quitConfirming {
		return
	}

	if ev.Name == fyne.KeyB || ev.Name == fyne.KeyE || ev.Name == fyne.KeyO {
		return
	}

	ks.handleShortcutKey(ev.Name)
}

// handleTabNavigation manages custom Tab navigation order.
func (ks *KeyboardShortcuts) handleTabNavigation() {
	a := ks.app
	focused := a.window.Canvas().Focused()

	switch focused {
	case a.wordInput:
		a.window.Canvas().Focus(a.translationEntry)
	case a.translationEntry:
		a.window.Canvas().Focus(a.imagePromptEntry)
	case a.imagePromptEntry:
		a.window.Canvas().Focus(a.wordInput)
	default:
		a.window.Canvas().Focus(a.wordInput)
	}
}

// handleShortcutKey handles the actual shortcut action.
func (ks *KeyboardShortcuts) handleShortcutKey(key fyne.KeyName) {
	a := ks.app
	if a.deleteConfirming || a.quitConfirming {
		return
	}

	switch key {
	case fyne.KeyG:
		if a.submitButton.Disabled() {
			return
		}
		a.onSubmit()

	case fyne.KeyN:
		if a.keepButton.Disabled() {
			return
		}
		a.onKeepAndContinue()

	case fyne.KeyI:
		if a.regenerateImageBtn.Disabled() {
			return
		}
		a.onRegenerateImage()

	case fyne.KeyM:
		if a.regenerateRandomImageBtn.Disabled() {
			return
		}
		a.onRegenerateRandomImage()

	case fyne.KeyA:

	case fyne.KeyR:
		if a.regenerateAllBtn.Disabled() {
			return
		}
		a.onRegenerateAll()

	case fyne.KeyD:
		if a.deleteButton.Disabled() {
			return
		}
		a.onDelete()

	case fyne.KeyLeft:
		if a.prevWordBtn.Disabled() {
			return
		}
		a.onPrevWord()

	case fyne.KeyRight:
		if a.nextWordBtn.Disabled() {
			return
		}
		a.onNextWord()

	case fyne.KeyX:
		a.export.onExportToAnki()

	case fyne.KeyV:
		a.onArchive()

	case fyne.KeyQ:
		a.onQuitConfirm()
	}
}
