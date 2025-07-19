package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	
	"codeberg.org/snonux/totalrecall/internal/anki"
)

// findCardDirectory finds the directory for a given Bulgarian word
func (a *Application) findCardDirectory(word string) string {
	entries, err := os.ReadDir(a.config.OutputDir)
	if err != nil {
		return ""
	}
	
	// Look through all directories to find one with matching _word.txt
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		
		dirPath := filepath.Join(a.config.OutputDir, entry.Name())
		wordFile := filepath.Join(dirPath, "word.txt")
		
		// Read the word file to check if it matches
		if data, err := os.ReadFile(wordFile); err == nil {
			storedWord := strings.TrimSpace(string(data))
			if storedWord == word {
				return dirPath
			}
		} else {
			// Try old format with underscore for backward compatibility
			wordFile = filepath.Join(dirPath, "_word.txt")
			if data, err := os.ReadFile(wordFile); err == nil {
				storedWord := strings.TrimSpace(string(data))
				if storedWord == word {
					return dirPath
				}
			}
		}
	}
	
	return ""
}

// scanExistingWords scans the output directory for existing words
func (a *Application) scanExistingWords() {
	a.existingWords = []string{}
	
	// Read directory
	entries, err := os.ReadDir(a.config.OutputDir)
	if err != nil {
		// Directory doesn't exist yet, that's OK
		return
	}
	
	// Each subdirectory represents a word
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		
		// Directory name is now a card ID
		cardID := entry.Name()
		wordDir := filepath.Join(a.config.OutputDir, cardID)
		
		// Read the original Bulgarian word from word.txt
		wordFile := filepath.Join(wordDir, "word.txt")
		wordData, err := os.ReadFile(wordFile)
		if err != nil {
			// Try old format with underscore for backward compatibility
			wordFile = filepath.Join(wordDir, "_word.txt")
			wordData, err = os.ReadFile(wordFile)
			if err != nil {
				// No word file, skip this directory
				continue
			}
		}
		
		word := string(wordData)
		if word == "" {
			continue
		}
		
		// Look for at least one of: audio, image, or translation file
		hasContent := false
		
		// Check for audio file
		audioFile := filepath.Join(wordDir, fmt.Sprintf("audio.%s", a.config.AudioFormat))
		if _, err := os.Stat(audioFile); err == nil {
			hasContent = true
		}
		
		// Check for image files
		if !hasContent {
			patterns := []string{
				"image.jpg",
				"image.png",
			}
			for _, pattern := range patterns {
				if _, err := os.Stat(filepath.Join(wordDir, pattern)); err == nil {
					hasContent = true
					break
				}
			}
		}
		
		// Check for translation file
		if !hasContent {
			translationFile := filepath.Join(wordDir, "translation.txt")
			if _, err := os.Stat(translationFile); err == nil {
				hasContent = true
			}
		}
		
		// If directory has content, add the word to the list
		if hasContent {
			a.existingWords = append(a.existingWords, word)
		}
	}
	
	// Sort the words
	sort.Strings(a.existingWords)
	
	// Update navigation buttons
	a.updateNavigation()
	
	// Load first word if available and nothing is loaded yet
	if len(a.existingWords) > 0 && a.currentWord == "" {
		a.loadWordByIndex(0)
	}
}

// updateNavigation updates the navigation button states
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

// getAllAvailableWords returns all words (from disk and completed queue jobs)
func (a *Application) getAllAvailableWords() []string {
	// Start with existing words from disk
	words := make([]string, len(a.existingWords))
	copy(words, a.existingWords)
	
	// Add completed jobs from queue
	completedJobs := a.queue.GetCompletedJobs()
	for _, job := range completedJobs {
		// Check if this word is already in the list
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
	
	// Sort the combined list
	sort.Strings(words)
	return words
}

// onPrevWord loads the previous word
func (a *Application) onPrevWord() {
	// Store current word before rescanning
	currentWord := a.currentWord
	
	// Rescan to pick up any new cards added externally
	a.scanExistingWords()
	
	allWords := a.getAllAvailableWords()
	if len(allWords) == 0 {
		return
	}
	
	// Find current word's new index after rescan
	currentIndex := -1
	for i, word := range allWords {
		if word == currentWord {
			currentIndex = i
			break
		}
	}
	
	// If current word not found, use the stored index
	if currentIndex == -1 {
		currentIndex = a.currentWordIndex
	}
	
	newIndex := currentIndex - 1
	// Wrap around to the end if at beginning
	if newIndex < 0 {
		newIndex = len(allWords) - 1
	}
	
	a.loadWordByIndex(newIndex)
}

// onNextWord loads the next word
func (a *Application) onNextWord() {
	// Store current word before rescanning
	currentWord := a.currentWord
	
	// Rescan to pick up any new cards added externally
	a.scanExistingWords()
	
	allWords := a.getAllAvailableWords()
	if len(allWords) == 0 {
		return
	}
	
	// Find current word's new index after rescan
	currentIndex := -1
	for i, word := range allWords {
		if word == currentWord {
			currentIndex = i
			break
		}
	}
	
	// If current word not found, use the stored index
	if currentIndex == -1 {
		currentIndex = a.currentWordIndex
	}
	
	newIndex := currentIndex + 1
	// Wrap around to the beginning if at end
	if newIndex >= len(allWords) {
		newIndex = 0
	}
	
	a.loadWordByIndex(newIndex)
}

// loadWordByIndex loads a word by its index in the combined word list
func (a *Application) loadWordByIndex(index int) {
	// Stop any existing file check ticker
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
	
	// Clear UI
	a.clearUI()
	
	// Check if this word is from a completed queue job
	var fromQueue bool
	completedJobs := a.queue.GetCompletedJobs()
	for _, job := range completedJobs {
		if job.Word == word && job.Status == StatusCompleted {
			// Load from queue job
			a.currentTranslation = job.Translation
			a.currentAudioFile = job.AudioFile
			a.currentImage = job.ImageFile
			
			fyne.Do(func() {
				if job.Translation != "" {
					a.translationEntry.SetText(job.Translation)
				}
				if job.AudioFile != "" {
					a.audioPlayer.SetAudioFile(job.AudioFile)
				}
				if job.ImageFile != "" {
					a.imageDisplay.SetImages([]string{job.ImageFile})
				}
				// Load phonetic info from disk if it exists
				a.loadPhoneticInfo(word)
				
				// Load image prompt from disk if it exists
				if wordDir := a.findCardDirectory(word); wordDir != "" {
					promptFile := filepath.Join(wordDir, "image_prompt.txt")
					if data, err := os.ReadFile(promptFile); err == nil {
						prompt := strings.TrimSpace(string(data))
						a.imagePromptEntry.SetText(prompt)
					}
				}
				
				a.updateStatus(fmt.Sprintf("Loaded from queue: %s", word))
			})
			
			fromQueue = true
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

// loadExistingFiles loads existing files for a word
func (a *Application) loadExistingFiles(word string) {
	// Find the card directory for this word
	wordDir := a.findCardDirectory(word)
	if wordDir == "" {
		// No existing directory found
		fmt.Printf("No card directory found for word: %s\n", word)
		return
	}
	
	fmt.Printf("Loading files from directory: %s\n", wordDir)
	
	// Load translation
	translationFile := filepath.Join(wordDir, "translation.txt")
	if data, err := os.ReadFile(translationFile); err == nil {
		// Parse translation from "word = translation" format
		content := string(data)
		parts := strings.Split(content, "=")
		if len(parts) >= 2 {
			a.currentTranslation = strings.TrimSpace(parts[1])
			fyne.Do(func() {
				a.translationEntry.SetText(a.currentTranslation)
			})
		}
	}
	
	// Load image prompt file
	promptFile := filepath.Join(wordDir, "image_prompt.txt")
	if data, err := os.ReadFile(promptFile); err == nil {
		prompt := strings.TrimSpace(string(data))
		fmt.Printf("Loaded prompt from file: %s\n", promptFile)
		fyne.Do(func() {
			a.imagePromptEntry.SetText(prompt)
		})
	} else {
		fmt.Printf("No prompt file found at: %s\n", promptFile)
	}
	
	// Load phonetic information
	phoneticFile := filepath.Join(wordDir, "phonetic.txt")
	if data, err := os.ReadFile(phoneticFile); err == nil {
		phoneticInfo := string(data)
		fmt.Printf("Loaded phonetic info from file: %s\n", phoneticFile)
		fyne.Do(func() {
			a.phoneticDisplay.SetText(phoneticInfo)
		})
	} else {
		fmt.Printf("No phonetic file found at: %s (error: %v)\n", phoneticFile, err)
	}
	
	// Load audio file
	audioFile := filepath.Join(wordDir, fmt.Sprintf("audio.%s", a.config.AudioFormat))
	if _, err := os.Stat(audioFile); err == nil {
		a.currentAudioFile = audioFile
		fyne.Do(func() {
			a.audioPlayer.SetAudioFile(audioFile)
		})
	}
	
	// Load image file
	a.currentImage = ""
	// Try to find images with different patterns
	patterns := []string{
		"image.jpg",
		"image.png",
	}
	
	for _, pattern := range patterns {
		imagePath := filepath.Join(wordDir, pattern)
		if _, err := os.Stat(imagePath); err == nil {
			a.currentImage = imagePath
			break // Just load the first image found
		}
	}
	
	if a.currentImage != "" {
		fyne.Do(func() {
			a.imageDisplay.SetImages([]string{a.currentImage})
		})
		
		// Try to load the prompt from attribution file if using OpenAI
		if a.config.ImageProvider == "openai" {
			// Look for attribution file
			baseImagePath := a.currentImage
			attrPath := strings.TrimSuffix(baseImagePath, filepath.Ext(baseImagePath)) + "_attribution.txt"
			if data, err := os.ReadFile(attrPath); err == nil {
				// Parse prompt from attribution file
				content := string(data)
				lines := strings.Split(content, "\n")
				for i, line := range lines {
					if strings.HasPrefix(line, "Prompt used:") && i+1 < len(lines) {
						// The prompt is on the next line
						prompt := strings.TrimSpace(lines[i+1])
						fyne.Do(func() {
							a.imagePromptEntry.SetText(prompt)
						})
						break
					}
				}
			}
		}
	}
	
	fyne.Do(func() {
		a.updateStatus(fmt.Sprintf("Loaded: %s", word))
	})
}

// startFileCheckTicker starts a ticker to check for missing files
func (a *Application) startFileCheckTicker() {
	// Stop any existing ticker first
	if a.fileCheckTicker != nil {
		a.fileCheckTicker.Stop()
	}
	
	// Create ticker that checks every 2 seconds
	ticker := time.NewTicker(2 * time.Second)
	a.fileCheckTicker = ticker
	
	go func() {
		for {
			select {
			case <-ticker.C:
				// Only check files for the current word
				a.mu.Lock()
				currentWord := a.currentWord
				a.mu.Unlock()
				
				if currentWord != "" {
					a.checkForMissingFiles(currentWord)
				}
			case <-a.ctx.Done():
				// Application is shutting down
				return
			}
		}
	}()
}

// checkForMissingFiles checks for missing files and attempts to load them
func (a *Application) checkForMissingFiles(word string) {
	// Find the card directory for this word
	wordDir := a.findCardDirectory(word)
	if wordDir == "" {
		return
	}
	
	// Check for missing audio file
	if a.currentAudioFile == "" {
		audioFile := filepath.Join(wordDir, fmt.Sprintf("audio.%s", a.config.AudioFormat))
		if _, err := os.Stat(audioFile); err == nil {
			a.currentAudioFile = audioFile
			fyne.Do(func() {
				a.audioPlayer.SetAudioFile(audioFile)
				a.updateStatus(fmt.Sprintf("Found audio file for %s", word))
			})
		}
	}
	
	// Check for missing image file
	if a.currentImage == "" {
		patterns := []string{"image.jpg", "image.png"}
		for _, pattern := range patterns {
			imagePath := filepath.Join(wordDir, pattern)
			if _, err := os.Stat(imagePath); err == nil {
				a.currentImage = imagePath
				fyne.Do(func() {
					a.imageDisplay.SetImages([]string{imagePath})
					a.updateStatus(fmt.Sprintf("Found image file for %s", word))
				})
				break
			}
		}
	}
	
	// Check for missing translation
	if a.currentTranslation == "" {
		translationFile := filepath.Join(wordDir, "translation.txt")
		if data, err := os.ReadFile(translationFile); err == nil {
			content := string(data)
			parts := strings.Split(content, "=")
			if len(parts) >= 2 {
				a.currentTranslation = strings.TrimSpace(parts[1])
				fyne.Do(func() {
					a.translationEntry.SetText(a.currentTranslation)
					a.updateStatus(fmt.Sprintf("Found translation for %s", word))
				})
			}
		}
	}
	
	// Check for missing prompt
	currentPrompt := a.imagePromptEntry.Text
	if currentPrompt == "" {
		promptFile := filepath.Join(wordDir, "image_prompt.txt")
		if data, err := os.ReadFile(promptFile); err == nil {
			prompt := strings.TrimSpace(string(data))
			fyne.Do(func() {
				a.imagePromptEntry.SetText(prompt)
				a.updateStatus(fmt.Sprintf("Found prompt for %s", word))
			})
		}
	}
	
	// Check for missing phonetic info
	currentPhonetic := a.phoneticDisplay.Text
	if currentPhonetic == "" || currentPhonetic == "Phonetic information will appear here..." {
		phoneticFile := filepath.Join(wordDir, "phonetic.txt")
		if data, err := os.ReadFile(phoneticFile); err == nil {
			phoneticInfo := string(data)
			fyne.Do(func() {
				a.phoneticDisplay.SetText(phoneticInfo)
				a.updateStatus(fmt.Sprintf("Found phonetic info for %s", word))
			})
		}
	}
	
	// Update action buttons if we now have content
	hasContent := a.currentAudioFile != "" || a.currentImage != "" || a.currentTranslation != ""
	if hasContent {
		fyne.Do(func() {
			a.setActionButtonsEnabled(true)
		})
	}
}

// onDelete moves the current word's files to trash bin
func (a *Application) onDelete() {
	if a.currentWord == "" {
		return
	}
	
	// Create custom confirmation dialog with keyboard support
	message := fmt.Sprintf("Move all files for '%s' to trash?\n\nPress y to confirm or n to cancel", a.currentWord)
	confirmDialog := dialog.NewConfirm("Move to Trash", message, func(confirm bool) {
		a.deleteConfirming = false
		if confirm {
			a.deleteCurrentWord()
		}
	}, a.window)
	
	// Set up keyboard handler for the dialog
	a.deleteConfirming = true
	
	// Create a custom key handler for the dialog window
	oldKeyHandler := a.window.Canvas().OnTypedKey()
	oldRuneHandler := a.window.Canvas().OnTypedRune()
	
	// Handle both Latin and Cyrillic keys
	a.window.Canvas().SetOnTypedRune(func(r rune) {
		if a.deleteConfirming {
			switch r {
			case 'y', 'Y', 'ъ', 'Ъ':
				confirmDialog.Hide()
				a.deleteConfirming = false
				a.deleteCurrentWord()
				// Restore original handlers
				a.window.Canvas().SetOnTypedKey(oldKeyHandler)
				a.window.Canvas().SetOnTypedRune(oldRuneHandler)
			case 'n', 'N', 'н', 'Н':
				confirmDialog.Hide()
				a.deleteConfirming = false
				// Restore original handlers
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
				// Restore original handlers
				a.window.Canvas().SetOnTypedKey(oldKeyHandler)
				a.window.Canvas().SetOnTypedRune(oldRuneHandler)
			case fyne.KeyN, fyne.KeyEscape:
				confirmDialog.Hide()
				a.deleteConfirming = false
				// Restore original handlers
				a.window.Canvas().SetOnTypedKey(oldKeyHandler)
				a.window.Canvas().SetOnTypedRune(oldRuneHandler)
			}
		} else if oldKeyHandler != nil {
			oldKeyHandler(ev)
		}
	})
	
	confirmDialog.Show()
}

// deleteCurrentWord moves the word's subdirectory to trash
func (a *Application) deleteCurrentWord() {
	// Cancel any ongoing operations for this card
	a.cancelCardOperations(a.currentWord)
	
	// Find the card directory for this word
	wordDir := a.findCardDirectory(a.currentWord)
	if wordDir == "" {
		fyne.Do(func() {
			a.updateStatus("No files found for this word")
		})
		return
	}
	
	// Create trash directory if it doesn't exist
	trashDir := filepath.Join(a.config.OutputDir, ".trashbin")
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		fyne.Do(func() {
			a.updateStatus(fmt.Sprintf("Failed to create trash directory: %v", err))
		})
		return
	}
	
	// Create destination path in trash  
	// Use the directory name from the card directory
	dirName := filepath.Base(wordDir)
	timestamp := time.Now().Format("20060102_150405")
	trashWordDir := filepath.Join(trashDir, fmt.Sprintf("%s_%s", dirName, timestamp))
	
	// Move entire directory to trash
	if err := os.Rename(wordDir, trashWordDir); err != nil {
		fyne.Do(func() {
			a.updateStatus(fmt.Sprintf("Failed to move files to trash: %v", err))
		})
		return
	}
	
	// Remove from existingWords
	newWords := []string{}
	for _, w := range a.existingWords {
		if w != a.currentWord {
			newWords = append(newWords, w)
		}
	}
	a.existingWords = newWords
	
	// Also remove from saved cards if present
	a.mu.Lock()
	newSavedCards := make([]anki.Card, 0, len(a.savedCards))
	for _, card := range a.savedCards {
		if card.Bulgarian != a.currentWord {
			newSavedCards = append(newSavedCards, card)
		}
	}
	a.savedCards = newSavedCards
	a.mu.Unlock()
	
	// Also remove from completed queue jobs
	a.queue.RemoveCompletedJobByWord(a.currentWord)
	
	// Clear UI
	a.clearUI()
	
	// Update status
	fyne.Do(func() {
		a.updateStatus(fmt.Sprintf("Moved '%s' to trash", a.currentWord))
		// Update queue status to reflect the reduced card count
		a.updateQueueStatus()
	})
	
	// Clear current word
	deletedWord := a.currentWord
	a.currentWord = ""
	a.wordInput.SetText("")
	
	// Try to load previous or next word
	if a.currentWordIndex > 0 && a.currentWordIndex <= len(a.existingWords) {
		a.loadWordByIndex(a.currentWordIndex - 1)
	} else if len(a.existingWords) > 0 {
		a.loadWordByIndex(0)
	} else {
		// No more words
		a.updateNavigation()
		a.setActionButtonsEnabled(false)
		// But keep delete button enabled
		a.deleteButton.Enable()
	}
	
	// Start a cleanup goroutine to remove directory after any pending operations complete
	go func() {
		// Wait a bit for any ongoing operations to notice cancellation
		time.Sleep(500 * time.Millisecond)
		
		// Check if the directory was somehow recreated (by a racing operation)
		recreatedDir := a.findCardDirectory(deletedWord)
		if recreatedDir != "" {
			// Directory was recreated, try to delete it again
			timestamp := time.Now().Format("20060102_150405")
			trashWordDir := filepath.Join(trashDir, fmt.Sprintf("%s_%s_cleanup", filepath.Base(recreatedDir), timestamp))
			
			// Move to trash again
			if err := os.Rename(recreatedDir, trashWordDir); err == nil {
				fmt.Printf("Cleanup: moved recreated directory for '%s' to trash\n", deletedWord)
			}
		}
	}()
}