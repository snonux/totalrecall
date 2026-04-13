package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"

	"codeberg.org/snonux/totalrecall/internal"
)

// NavigationHandler owns word list navigation, loading card files from disk or
// queue state, file-check polling, and delete-to-trash. It holds a reference to
// Application for shared UI state and services (SRP: navigation vs generation).
type NavigationHandler struct {
	app *Application
}

// findCardDirectory finds the directory for a given Bulgarian word.
// Delegates to CardService which wraps the shared internal.FindCardDirectory.
func (n *NavigationHandler) findCardDirectory(word string) string {
	return n.app.getCardService().FindCardDirectory(word)
}

// scanExistingWords scans the output directory for existing words and updates
// the existingWords slice. Delegates file discovery to CardService.
func (n *NavigationHandler) scanExistingWords() {
	n.app.existingWords = n.app.getCardService().ScanExistingWords()

	n.updateNavigation()

	if len(n.app.existingWords) > 0 && n.app.currentWord == "" {
		n.loadWordByIndex(0)
	}
}

// updateNavigation updates the navigation button states based on the current
// combined word list (existing words on disk + completed queue jobs). It also
// refreshes the card counter label ("Card X / N") so it stays in sync with the
// current position.
func (n *NavigationHandler) updateNavigation() {
	allWords := n.getAllAvailableWords()

	if len(allWords) > 1 {
		n.app.prevWordBtn.Enable()
		n.app.nextWordBtn.Enable()

		n.app.currentWordIndex = -1
		for i, word := range allWords {
			if word == n.app.currentWord {
				n.app.currentWordIndex = i
				break
			}
		}
	} else if len(allWords) == 1 {
		n.app.prevWordBtn.Disable()
		n.app.nextWordBtn.Disable()
	} else {
		n.app.prevWordBtn.Disable()
		n.app.nextWordBtn.Disable()
	}

	// Keep the counter label in sync with the updated index and total.
	n.app.updateCardCounter(n.app.currentWordIndex, len(allWords))
}

// getAllAvailableWords returns all words from disk and completed queue jobs,
// merged and sorted.
func (n *NavigationHandler) getAllAvailableWords() []string {
	words := make([]string, len(n.app.existingWords))
	copy(words, n.app.existingWords)

	completedJobs := n.app.queue.GetCompletedJobs()
	for _, job := range completedJobs {
		found := false
		for _, w := range words {
			if w == job.Word {
				found = true
				break
			}
		}
		if !found {
			words = append(words, job.Word)
		}
	}

	sort.Strings(words)
	return words
}

// onPrevWord loads the previous word (with wrap-around).
func (n *NavigationHandler) onPrevWord() {
	currentWord := n.app.currentWord

	n.app.scanExistingWords()

	allWords := n.getAllAvailableWords()
	if len(allWords) == 0 {
		return
	}

	currentIndex := n.findWordIndex(allWords, currentWord)
	newIndex := currentIndex - 1
	if newIndex < 0 {
		newIndex = len(allWords) - 1
	}

	n.loadWordByIndex(newIndex)
}

// onNextWord loads the next word (with wrap-around).
func (n *NavigationHandler) onNextWord() {
	currentWord := n.app.currentWord

	n.app.scanExistingWords()

	allWords := n.getAllAvailableWords()
	if len(allWords) == 0 {
		return
	}

	currentIndex := n.findWordIndex(allWords, currentWord)
	newIndex := currentIndex + 1
	if newIndex >= len(allWords) {
		newIndex = 0
	}

	n.loadWordByIndex(newIndex)
}

// findWordIndex returns the index of word in allWords, falling back to
// currentWordIndex when the word is not found (e.g. after a rescan).
func (n *NavigationHandler) findWordIndex(allWords []string, word string) int {
	for i, w := range allWords {
		if w == word {
			return i
		}
	}
	return n.app.currentWordIndex
}

// loadWordByIndex loads a word by its index in the combined word list.
func (n *NavigationHandler) loadWordByIndex(index int) {
	if n.app.fileCheckTicker != nil {
		n.app.fileCheckTicker.Stop()
		n.app.fileCheckTicker = nil
	}

	allWords := n.getAllAvailableWords()
	if index < 0 || index >= len(allWords) {
		return
	}

	word := allWords[index]
	n.app.currentWord = word
	n.app.currentWordIndex = index

	n.app.wordInput.SetText(word)

	n.app.clearUI()

	var fromQueue bool
	completedJobs := n.app.queue.GetCompletedJobs()
	for _, job := range completedJobs {
		if job.Word == word && job.Status == StatusCompleted {
			fromQueue = true
			n.applyQueueJobToState(job)
			break
		}
	}

	if !fromQueue {
		n.loadExistingFiles(word)
	}

	n.updateNavigation()

	hasContent := n.app.currentAudioFile != "" || n.app.currentImage != "" || n.app.currentTranslation != ""
	if hasContent {
		n.app.setActionButtonsEnabled(true)
	}

	n.startFileCheckTicker()
}

// applyQueueJobToState loads state and UI from a completed WordJob.
// Must only be called from the main goroutine (or inside fyne.Do).
func (n *NavigationHandler) applyQueueJobToState(job *WordJob) {
	a := n.app
	a.currentTranslation = job.Translation
	a.currentAudioFile = job.AudioFile
	a.currentAudioFileBack = job.AudioFileBack
	a.currentImage = job.ImageFile
	a.currentCardType = job.CardType
	a.syncCardTypeSelection(internal.CardType(job.CardType))

	fyne.Do(func() {
		if job.Translation != "" {
			a.translationEntry.SetText(job.Translation)
		}
		if job.AudioFile != "" {
			a.audioPlayer.SetAudioFile(job.AudioFile)
		}
		if job.AudioFileBack != "" {
			a.audioPlayer.SetBackAudioFile(job.AudioFileBack)
		}
		if job.ImageFile != "" {
			a.imageDisplay.SetImages([]string{job.ImageFile})
		}

		n.loadPhoneticInfo(job.Word)

		if prompt := a.getCardService().LoadImagePromptForWord(job.Word); prompt != "" {
			a.imagePromptEntry.SetText(prompt)
		}

		a.updateStatus(fmt.Sprintf("Loaded from queue: %s", job.Word))
	})
}

// loadExistingFiles loads existing card files for word from disk and updates
// UI state. Delegates file I/O to CardService.
func (n *NavigationHandler) loadExistingFiles(word string) {
	a := n.app
	cs := a.getCardService()
	cf := cs.LoadCardFiles(word)
	if cf == nil {
		return
	}

	if cf.Translation != "" {
		a.currentTranslation = cf.Translation
	}

	a.currentCardType = string(cf.CardType)
	a.syncCardTypeSelection(cf.CardType)

	if cf.AudioFile != "" {
		a.currentAudioFile = cf.AudioFile
		if a.window == nil {
			a.audioPlayer.SetAudioFile(cf.AudioFile)
		} else {
			fyne.Do(func() {
				a.audioPlayer.SetAudioFile(cf.AudioFile)
			})
		}
	}

	if cf.AudioBack != "" {
		a.currentAudioFileBack = cf.AudioBack
		if a.window == nil {
			a.audioPlayer.SetBackAudioFile(cf.AudioBack)
		} else {
			fyne.Do(func() {
				a.audioPlayer.SetBackAudioFile(cf.AudioBack)
			})
		}
	} else if !cf.CardType.IsBgBg() {
		a.currentAudioFileBack = ""
		if a.window == nil {
			a.audioPlayer.SetBackAudioFile("")
		} else {
			fyne.Do(func() {
				a.audioPlayer.SetBackAudioFile("")
			})
		}
	}

	if cf.ImageFile != "" {
		a.currentImage = cf.ImageFile
		fyne.Do(func() {
			a.imageDisplay.SetImages([]string{cf.ImageFile})
		})
	}

	fyne.Do(func() {
		if cf.Translation != "" {
			a.translationEntry.SetText(cf.Translation)
		}
		if cf.ImagePrompt != "" {
			a.imagePromptEntry.SetText(cf.ImagePrompt)
		}
		if cf.PhoneticInfo != "" {
			a.audioPlayer.SetPhonetic(cf.PhoneticInfo)
		}
		a.updateStatus(fmt.Sprintf("Loaded: %s", word))
	})
}

// startFileCheckTicker starts a ticker that periodically checks for missing
// files (e.g. audio/image that is still being generated) and updates the UI.
func (n *NavigationHandler) startFileCheckTicker() {
	a := n.app
	if a.fileCheckTicker != nil {
		a.fileCheckTicker.Stop()
	}

	ticker := time.NewTicker(2 * time.Second)
	a.fileCheckTicker = ticker

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		for {
			select {
			case <-ticker.C:
				a.mu.Lock()
				currentWord := a.currentWord
				a.mu.Unlock()

				if currentWord != "" {
					n.checkForMissingFiles(currentWord)
				}
			case <-a.ctx.Done():
				return
			}
		}
	}()
}

// checkForMissingFiles polls for files that may have appeared since the last
// load (e.g. background generation) and updates the UI when found.
func (n *NavigationHandler) checkForMissingFiles(word string) {
	a := n.app
	cs := a.getCardService()

	missing := cs.CheckMissingFiles(
		word,
		a.currentAudioFile,
		a.currentAudioFileBack,
		a.currentImage,
		a.currentTranslation,
		a.imagePromptEntry.Text,
		a.currentPhonetic,
		a.currentCardType,
	)
	if missing == nil {
		return
	}

	n.applyMissingFiles(word, missing)
}

// applyMissingFiles applies newly discovered files to the application state
// and updates the UI. Each field is applied only when non-empty.
func (n *NavigationHandler) applyMissingFiles(word string, missing *CardFiles) {
	a := n.app
	if missing.AudioFile != "" {
		a.currentAudioFile = missing.AudioFile
		fyne.Do(func() {
			a.audioPlayer.SetAudioFile(missing.AudioFile)
			a.updateStatus(fmt.Sprintf("Found audio file for %s", word))
		})
	}

	if missing.AudioBack != "" {
		a.currentAudioFileBack = missing.AudioBack
		fyne.Do(func() {
			a.audioPlayer.SetBackAudioFile(missing.AudioBack)
			a.updateStatus(fmt.Sprintf("Found back audio file for %s", word))
		})
	}

	if missing.ImageFile != "" {
		a.currentImage = missing.ImageFile
		fyne.Do(func() {
			a.imageDisplay.SetImages([]string{missing.ImageFile})
			a.updateStatus(fmt.Sprintf("Found image file for %s", word))
		})
	}

	if missing.Translation != "" {
		a.currentTranslation = missing.Translation
		fyne.Do(func() {
			a.translationEntry.SetText(missing.Translation)
			a.updateStatus(fmt.Sprintf("Found translation for %s", word))
		})
	}

	if missing.ImagePrompt != "" {
		fyne.Do(func() {
			a.imagePromptEntry.SetText(missing.ImagePrompt)
			a.updateStatus(fmt.Sprintf("Found prompt for %s", word))
		})
	}

	if missing.PhoneticInfo != "" {
		a.currentPhonetic = missing.PhoneticInfo
		fyne.Do(func() {
			a.audioPlayer.SetPhonetic(missing.PhoneticInfo)
			a.updateStatus(fmt.Sprintf("Found phonetic info for %s", word))
		})
	}

	hasContent := a.currentAudioFile != "" || a.currentImage != "" || a.currentTranslation != ""
	if hasContent {
		fyne.Do(func() {
			a.setActionButtonsEnabled(true)
		})
	}
}

// onDelete shows a confirmation dialog before moving the current word's files to
// the trash bin. Keyboard shortcuts y/Y and n/N also control the dialog.
func (n *NavigationHandler) onDelete() {
	a := n.app
	if a.currentWord == "" {
		return
	}

	a.ensureHandlers()

	if a.queueMgr.hasActiveOperations(a.currentWord) {
		dialog.ShowError(fmt.Errorf("cannot delete %q while content is being generated; please wait for generation to complete", a.currentWord), a.window)
		return
	}

	if a.queue.IsWordProcessing(a.currentWord) {
		dialog.ShowError(fmt.Errorf("cannot delete %q while it is in the processing queue; please wait for processing to complete", a.currentWord), a.window)
		return
	}

	message := fmt.Sprintf("Move all files for '%s' to trash?\n\nPress y to confirm or n to cancel", a.currentWord)
	confirmDialog := dialog.NewConfirm("Move to Trash", message, func(confirm bool) {
		a.deleteConfirming = false
		if confirm {
			n.deleteCurrentWord()
		}
	}, a.window)

	a.deleteConfirming = true
	oldKeyHandler := a.window.Canvas().OnTypedKey()
	oldRuneHandler := a.window.Canvas().OnTypedRune()

	a.window.Canvas().SetOnTypedRune(func(r rune) {
		if a.deleteConfirming {
			switch r {
			case 'y', 'Y', 'ъ', 'Ъ':
				confirmDialog.Hide()
				a.deleteConfirming = false
				n.deleteCurrentWord()
				a.window.Canvas().SetOnTypedKey(oldKeyHandler)
				a.window.Canvas().SetOnTypedRune(oldRuneHandler)
			case 'n', 'N', 'н', 'Н':
				confirmDialog.Hide()
				a.deleteConfirming = false
				a.window.Canvas().SetOnTypedKey(oldKeyHandler)
				a.window.Canvas().SetOnTypedRune(oldRuneHandler)
			}
		} else if oldRuneHandler != nil {
			oldRuneHandler(r)
		}
	})

	a.window.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if a.deleteConfirming {
			switch ev.Name {
			case fyne.KeyY:
				confirmDialog.Hide()
				a.deleteConfirming = false
				n.deleteCurrentWord()
				a.window.Canvas().SetOnTypedKey(oldKeyHandler)
				a.window.Canvas().SetOnTypedRune(oldRuneHandler)
			case fyne.KeyN, fyne.KeyEscape:
				confirmDialog.Hide()
				a.deleteConfirming = false
				a.window.Canvas().SetOnTypedKey(oldKeyHandler)
				a.window.Canvas().SetOnTypedRune(oldRuneHandler)
			}
		} else if oldKeyHandler != nil {
			oldKeyHandler(ev)
		}
	})

	confirmDialog.Show()
}

// deleteCurrentWord delegates the file removal to CardService and then
// updates Application state and UI accordingly.
func (n *NavigationHandler) deleteCurrentWord() {
	a := n.app
	a.ensureHandlers()

	a.queueMgr.cancelCardOperations(a.currentWord)

	newWords, newCards, err := a.getCardService().DeleteWord(a.currentWord, a.existingWords, a.savedCards)
	if err != nil {
		fyne.Do(func() {
			a.updateStatus(err.Error())
		})
		return
	}

	trashDir := filepath.Join(a.config.OutputDir, ".trashbin")

	a.mu.Lock()
	a.savedCards = newCards
	a.mu.Unlock()
	a.existingWords = newWords

	a.queue.RemoveCompletedJobByWord(a.currentWord)

	a.clearUI()

	fyne.Do(func() {
		a.updateStatus(fmt.Sprintf("Moved '%s' to trash", a.currentWord))
		a.updateQueueStatus()
	})

	deletedWord := a.currentWord
	a.currentWord = ""
	a.wordInput.SetText("")

	if a.currentWordIndex > 0 && a.currentWordIndex <= len(a.existingWords) {
		n.loadWordByIndex(a.currentWordIndex - 1)
	} else if len(a.existingWords) > 0 {
		n.loadWordByIndex(0)
	} else {
		n.updateNavigation()
		a.setActionButtonsEnabled(false)
		a.deleteButton.Enable()
	}

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		timeout := time.NewTimer(5 * time.Second)
		defer timeout.Stop()

		for {
			select {
			case <-timeout.C:
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				if a.queueMgr.hasActiveOperations(deletedWord) {
					continue
				}
			}
			break
		}

		recreatedDir := n.findCardDirectory(deletedWord)
		if recreatedDir != "" {
			ts := time.Now().Format("20060102_150405")
			dest := filepath.Join(trashDir, fmt.Sprintf("%s_%s_cleanup", filepath.Base(recreatedDir), ts))
			if err := os.Rename(recreatedDir, dest); err == nil {
				fmt.Printf("Cleanup: moved recreated directory for '%s' to trash\n", deletedWord)
			}
		}
	}()
}

// loadPhoneticInfo loads phonetic information from disk and updates the UI.
// Delegates file I/O to CardService.
func (n *NavigationHandler) loadPhoneticInfo(word string) {
	a := n.app
	phoneticText := a.getCardService().LoadPhoneticInfo(word)
	if phoneticText == "" {
		return
	}

	a.currentPhonetic = phoneticText
	fyne.Do(func() {
		if phoneticText != "" {
			a.audioPlayer.SetPhonetic(phoneticText)
		} else {
			a.audioPlayer.SetPhonetic("")
		}
	})
}
