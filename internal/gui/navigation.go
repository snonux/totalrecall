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
		
		// Directory name is the sanitized word
		sanitizedWord := entry.Name()
		
		// Check if this directory contains valid word files
		wordDir := filepath.Join(a.config.OutputDir, sanitizedWord)
		
		// Look for at least one of: audio, image, or translation file
		hasContent := false
		
		// Check for audio file
		audioFile := filepath.Join(wordDir, fmt.Sprintf("%s.%s", sanitizedWord, a.config.AudioFormat))
		if _, err := os.Stat(audioFile); err == nil {
			hasContent = true
		}
		
		// Check for image files
		if !hasContent {
			patterns := []string{
				fmt.Sprintf("%s.jpg", sanitizedWord),
				fmt.Sprintf("%s.png", sanitizedWord),
				fmt.Sprintf("%s_1.jpg", sanitizedWord),
				fmt.Sprintf("%s_1.png", sanitizedWord),
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
			translationFile := filepath.Join(wordDir, fmt.Sprintf("%s_translation.txt", sanitizedWord))
			if _, err := os.Stat(translationFile); err == nil {
				hasContent = true
			}
		}
		
		// If directory has content, add it to the list
		if hasContent {
			// Try to get the original word from translation file
			translationFile := filepath.Join(wordDir, fmt.Sprintf("%s_translation.txt", sanitizedWord))
			if data, err := os.ReadFile(translationFile); err == nil {
				content := string(data)
				parts := strings.Split(content, "=")
				if len(parts) >= 1 {
					originalWord := strings.TrimSpace(parts[0])
					a.existingWords = append(a.existingWords, originalWord)
					continue
				}
			}
			
			// Fallback: use the directory name
			a.existingWords = append(a.existingWords, sanitizedWord)
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
	allWords := a.getAllAvailableWords()
	if len(allWords) == 0 {
		return
	}
	
	newIndex := a.currentWordIndex - 1
	// Wrap around to the end if at beginning
	if newIndex < 0 {
		newIndex = len(allWords) - 1
	}
	
	a.loadWordByIndex(newIndex)
}

// onNextWord loads the next word
func (a *Application) onNextWord() {
	allWords := a.getAllAvailableWords()
	if len(allWords) == 0 {
		return
	}
	
	newIndex := a.currentWordIndex + 1
	// Wrap around to the beginning if at end
	if newIndex >= len(allWords) {
		newIndex = 0
	}
	
	a.loadWordByIndex(newIndex)
}

// loadWordByIndex loads a word by its index in the combined word list
func (a *Application) loadWordByIndex(index int) {
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
				sanitized := sanitizeFilename(word)
				wordDir := filepath.Join(a.config.OutputDir, sanitized)
				promptFile := filepath.Join(wordDir, fmt.Sprintf("%s_prompt.txt", sanitized))
				if data, err := os.ReadFile(promptFile); err == nil {
					prompt := strings.TrimSpace(string(data))
					a.imagePromptEntry.SetText(prompt)
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
	
	// Enable action buttons since we have loaded content
	a.setActionButtonsEnabled(true)
}

// loadExistingFiles loads existing files for a word
func (a *Application) loadExistingFiles(word string) {
	sanitized := sanitizeFilename(word)
	wordDir := filepath.Join(a.config.OutputDir, sanitized)
	
	// Load translation
	translationFile := filepath.Join(wordDir, fmt.Sprintf("%s_translation.txt", sanitized))
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
	promptFile := filepath.Join(wordDir, fmt.Sprintf("%s_prompt.txt", sanitized))
	if data, err := os.ReadFile(promptFile); err == nil {
		prompt := strings.TrimSpace(string(data))
		fyne.Do(func() {
			a.imagePromptEntry.SetText(prompt)
		})
	}
	
	// Load phonetic information
	phoneticFile := filepath.Join(wordDir, fmt.Sprintf("%s_phonetic.txt", sanitized))
	if data, err := os.ReadFile(phoneticFile); err == nil {
		phoneticInfo := string(data)
		fyne.Do(func() {
			a.phoneticDisplay.SetText(phoneticInfo)
		})
	}
	
	// Load audio file
	audioFile := filepath.Join(wordDir, fmt.Sprintf("%s.%s", sanitized, a.config.AudioFormat))
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
		fmt.Sprintf("%s.jpg", sanitized),
		fmt.Sprintf("%s.png", sanitized),
		fmt.Sprintf("%s_1.jpg", sanitized),
		fmt.Sprintf("%s_1.png", sanitized),
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
	a.window.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if a.deleteConfirming {
			switch ev.Name {
			case fyne.KeyY:
				confirmDialog.Hide()
				a.deleteConfirming = false
				a.deleteCurrentWord()
				// Restore original key handler
				a.window.Canvas().SetOnTypedKey(oldKeyHandler)
			case fyne.KeyN, fyne.KeyEscape:
				confirmDialog.Hide()
				a.deleteConfirming = false
				// Restore original key handler
				a.window.Canvas().SetOnTypedKey(oldKeyHandler)
			}
		} else if oldKeyHandler != nil {
			oldKeyHandler(ev)
		}
	})
	
	confirmDialog.Show()
}

// deleteCurrentWord moves the word's subdirectory to trash
func (a *Application) deleteCurrentWord() {
	sanitized := sanitizeFilename(a.currentWord)
	wordDir := filepath.Join(a.config.OutputDir, sanitized)
	
	// Create trash directory if it doesn't exist
	trashDir := filepath.Join(a.config.OutputDir, ".trashbin")
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		fyne.Do(func() {
			a.updateStatus(fmt.Sprintf("Failed to create trash directory: %v", err))
		})
		return
	}
	
	// Check if word directory exists
	if _, err := os.Stat(wordDir); os.IsNotExist(err) {
		fyne.Do(func() {
			a.updateStatus("No files found for this word")
		})
		return
	}
	
	// Create destination path in trash
	timestamp := time.Now().Format("20060102_150405")
	trashWordDir := filepath.Join(trashDir, fmt.Sprintf("%s_%s", sanitized, timestamp))
	
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
	}
}