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

// findCardDirectory finds the directory for a given Bulgarian word.
// Delegates to CardService which wraps the shared internal.FindCardDirectory.
func (a *Application) findCardDirectory(word string) string {
	return a.getCardService().FindCardDirectory(word)
}

// scanExistingWords scans the output directory for existing words and updates
// the existingWords slice. Delegates file discovery to CardService.
func (a *Application) scanExistingWords() {
	a.existingWords = a.getCardService().ScanExistingWords()

	// Update navigation buttons
	a.updateNavigation()

	// Load first word if available and nothing is loaded yet
	if len(a.existingWords) > 0 && a.currentWord == "" {
		a.loadWordByIndex(0)
	}
}

// updateNavigation updates the navigation button states based on the current
// combined word list (existing words on disk + completed queue jobs).
func (a *Application) updateNavigation() {
	// Get all available words (existing + completed from queue)
	allWords := a.getAllAvailableWords()

	if len(allWords) > 1 {
		// Enable both buttons when there's more than one word (allows circular navigation)
		a.prevWordBtn.Enable()
		a.nextWordBtn.Enable()

		// Find current word index
		a.currentWordIndex = -1
		for i, word := range allWords {
			if word == a.currentWord {
				a.currentWordIndex = i
				break
			}
		}
	} else if len(allWords) == 1 {
		// With only one word, disable navigation
		a.prevWordBtn.Disable()
		a.nextWordBtn.Disable()
	} else {
		// No words at all
		a.prevWordBtn.Disable()
		a.nextWordBtn.Disable()
	}
}

// getAllAvailableWords returns all words from disk and completed queue jobs,
// merged and sorted.
func (a *Application) getAllAvailableWords() []string {
	// Start with existing words from disk
	words := make([]string, len(a.existingWords))
	copy(words, a.existingWords)

	// Add completed jobs from queue that are not already in the list
	completedJobs := a.queue.GetCompletedJobs()
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
func (a *Application) onPrevWord() {
	currentWord := a.currentWord

	// Rescan to pick up any new cards added externally
	a.scanExistingWords()

	allWords := a.getAllAvailableWords()
	if len(allWords) == 0 {
		return
	}

	currentIndex := a.findWordIndex(allWords, currentWord)
	newIndex := currentIndex - 1
	if newIndex < 0 {
		newIndex = len(allWords) - 1
	}

	a.loadWordByIndex(newIndex)
}

// onNextWord loads the next word (with wrap-around).
func (a *Application) onNextWord() {
	currentWord := a.currentWord

	// Rescan to pick up any new cards added externally
	a.scanExistingWords()

	allWords := a.getAllAvailableWords()
	if len(allWords) == 0 {
		return
	}

	currentIndex := a.findWordIndex(allWords, currentWord)
	newIndex := currentIndex + 1
	if newIndex >= len(allWords) {
		newIndex = 0
	}

	a.loadWordByIndex(newIndex)
}

// findWordIndex returns the index of word in allWords, falling back to
// a.currentWordIndex when the word is not found (e.g. after a rescan).
func (a *Application) findWordIndex(allWords []string, word string) int {
	for i, w := range allWords {
		if w == word {
			return i
		}
	}
	return a.currentWordIndex
}

// loadWordByIndex loads a word by its index in the combined word list.
func (a *Application) loadWordByIndex(index int) {
	// Stop any existing file check ticker before switching words.
	if a.fileCheckTicker != nil {
		a.fileCheckTicker.Stop()
		a.fileCheckTicker = nil
	}

	allWords := a.getAllAvailableWords()
	if index < 0 || index >= len(allWords) {
		return
	}

	word := allWords[index]
	a.currentWord = word
	a.currentWordIndex = index

	// Update input field
	a.wordInput.SetText(word)

	// Clear UI state before loading new word
	a.clearUI()

	// Check if this word is from a completed queue job
	var fromQueue bool
	completedJobs := a.queue.GetCompletedJobs()
	for _, job := range completedJobs {
		if job.Word == word && job.Status == StatusCompleted {
			fromQueue = true
			a.applyQueueJobToState(job)
			break
		}
	}

	// If not from queue, load existing files from disk
	if !fromQueue {
		a.loadExistingFiles(word)
	}

	// Update navigation
	a.updateNavigation()

	// Enable action buttons if we have content
	hasContent := a.currentAudioFile != "" || a.currentImage != "" || a.currentTranslation != ""
	if hasContent {
		a.setActionButtonsEnabled(true)
	}

	// Start ticker to check for missing files
	a.startFileCheckTicker()
}

// applyQueueJobToState loads state and UI from a completed WordJob.
// Must only be called from the main goroutine (or inside fyne.Do).
func (a *Application) applyQueueJobToState(job *WordJob) {
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

		// Load phonetic info from disk if it exists
		a.loadPhoneticInfo(job.Word)

		// Load image prompt from disk if it exists
		if prompt := a.getCardService().LoadImagePromptForWord(job.Word); prompt != "" {
			a.imagePromptEntry.SetText(prompt)
		}

		a.updateStatus(fmt.Sprintf("Loaded from queue: %s", job.Word))
	})
}

// loadExistingFiles loads existing card files for word from disk and updates
// UI state. Delegates file I/O to CardService.
func (a *Application) loadExistingFiles(word string) {
	cs := a.getCardService()
	cf := cs.LoadCardFiles(word)
	if cf == nil {
		return
	}

	// CRITICAL: Set the translation state BEFORE SetText so it's available
	// whenever another method reads it during the same tick.
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
		// For en-bg cards clear the back audio button explicitly.
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
func (a *Application) startFileCheckTicker() {
	// Stop any existing ticker first
	if a.fileCheckTicker != nil {
		a.fileCheckTicker.Stop()
	}

	ticker := time.NewTicker(2 * time.Second)
	a.fileCheckTicker = ticker

	// Track this goroutine in wg so the shutdown handler waits for it.
	// The ctx.Done() case ensures it exits promptly on application close.
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
					a.checkForMissingFiles(currentWord)
				}
			case <-a.ctx.Done():
				return
			}
		}
	}()
}

// checkForMissingFiles polls for files that may have appeared since the last
// load (e.g. background generation) and updates the UI when found.
func (a *Application) checkForMissingFiles(word string) {
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

	a.applyMissingFiles(word, missing)
}

// applyMissingFiles applies newly discovered files to the application state
// and updates the UI. Each field is applied only when non-empty.
func (a *Application) applyMissingFiles(word string, missing *CardFiles) {
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

	// Enable action buttons when any content is now available.
	hasContent := a.currentAudioFile != "" || a.currentImage != "" || a.currentTranslation != ""
	if hasContent {
		fyne.Do(func() {
			a.setActionButtonsEnabled(true)
		})
	}
}

// onDelete shows a confirmation dialog before moving the current word's files to
// the trash bin. Keyboard shortcuts y/Y and n/N also control the dialog.
func (a *Application) onDelete() {
	if a.currentWord == "" {
		return
	}

	// Check if this word has active operations
	if a.hasActiveOperations(a.currentWord) {
		dialog.ShowError(fmt.Errorf("cannot delete %q while content is being generated; please wait for generation to complete", a.currentWord), a.window)
		return
	}

	// Also check if word is in the processing queue
	if a.queue.IsWordProcessing(a.currentWord) {
		dialog.ShowError(fmt.Errorf("cannot delete %q while it is in the processing queue; please wait for processing to complete", a.currentWord), a.window)
		return
	}

	message := fmt.Sprintf("Move all files for '%s' to trash?\n\nPress y to confirm or n to cancel", a.currentWord)
	confirmDialog := dialog.NewConfirm("Move to Trash", message, func(confirm bool) {
		a.deleteConfirming = false
		if confirm {
			a.deleteCurrentWord()
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
				a.deleteCurrentWord()
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
				a.deleteCurrentWord()
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
func (a *Application) deleteCurrentWord() {
	// Cancel any ongoing operations for this card
	a.cancelCardOperations(a.currentWord)

	// Delegate the filesystem work and state list updates to CardService.
	newWords, newCards, err := a.getCardService().DeleteWord(a.currentWord, a.existingWords, a.savedCards)
	if err != nil {
		fyne.Do(func() {
			a.updateStatus(err.Error())
		})
		return
	}

	// Capture the trash dir for the deferred cleanup goroutine.
	trashDir := filepath.Join(a.config.OutputDir, ".trashbin")

	a.mu.Lock()
	a.savedCards = newCards
	a.mu.Unlock()
	a.existingWords = newWords

	// Remove from completed queue jobs.
	a.queue.RemoveCompletedJobByWord(a.currentWord)

	// Clear UI
	a.clearUI()

	fyne.Do(func() {
		a.updateStatus(fmt.Sprintf("Moved '%s' to trash", a.currentWord))
		a.updateQueueStatus()
	})

	deletedWord := a.currentWord
	a.currentWord = ""
	a.wordInput.SetText("")

	// Navigate to another word or update button states when no words remain.
	if a.currentWordIndex > 0 && a.currentWordIndex <= len(a.existingWords) {
		a.loadWordByIndex(a.currentWordIndex - 1)
	} else if len(a.existingWords) > 0 {
		a.loadWordByIndex(0)
	} else {
		a.updateNavigation()
		a.setActionButtonsEnabled(false)
		a.deleteButton.Enable()
	}

	// Start a cleanup goroutine to catch directory recreation by racing in-flight
	// operations. Polls hasActiveOperations to run as soon as possible.
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
				// Proceed even if some operations are still pending.
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				if a.hasActiveOperations(deletedWord) {
					continue
				}
			}
			break
		}

		// Check if a racing operation recreated the directory.
		recreatedDir := a.findCardDirectory(deletedWord)
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
func (a *Application) loadPhoneticInfo(word string) {
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
