package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
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
	
	// Collect unique words
	wordMap := make(map[string]bool)
	
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		name := entry.Name()
		// Skip attribution and translation files
		if strings.Contains(name, "_attribution") || strings.Contains(name, "_translation") {
			continue
		}
		
		// Extract word from filename (before first underscore or dot)
		base := strings.TrimSuffix(name, filepath.Ext(name))
		parts := strings.Split(base, "_")
		if len(parts) > 0 {
			word := parts[0]
			wordMap[word] = true
		}
	}
	
	// Convert map to sorted slice
	for word := range wordMap {
		a.existingWords = append(a.existingWords, word)
	}
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
	if len(a.existingWords) > 0 {
		a.prevWordBtn.Enable()
		a.nextWordBtn.Enable()
		
		// Find current word index
		a.currentWordIndex = -1
		for i, word := range a.existingWords {
			if word == a.currentWord {
				a.currentWordIndex = i
				break
			}
		}
		
		// Disable at boundaries
		if a.currentWordIndex <= 0 {
			a.prevWordBtn.Disable()
		}
		if a.currentWordIndex >= len(a.existingWords)-1 || a.currentWordIndex == -1 {
			a.nextWordBtn.Disable()
		}
	} else {
		a.prevWordBtn.Disable()
		a.nextWordBtn.Disable()
	}
}

// onPrevWord loads the previous word
func (a *Application) onPrevWord() {
	if a.currentWordIndex > 0 {
		a.loadWordByIndex(a.currentWordIndex - 1)
	}
}

// onNextWord loads the next word
func (a *Application) onNextWord() {
	if a.currentWordIndex < len(a.existingWords)-1 && a.currentWordIndex >= 0 {
		a.loadWordByIndex(a.currentWordIndex + 1)
	}
}

// loadWordByIndex loads a word by its index in existingWords
func (a *Application) loadWordByIndex(index int) {
	if index < 0 || index >= len(a.existingWords) {
		return
	}
	
	word := a.existingWords[index]
	a.currentWord = word
	a.currentWordIndex = index
	
	// Update input field
	a.wordInput.SetText(word)
	
	// Clear UI
	a.clearUI()
	
	// Load existing files
	a.loadExistingFiles(word)
	
	// Update navigation
	a.updateNavigation()
	
	// Enable action buttons since we have loaded content
	a.setActionButtonsEnabled(true)
}

// loadExistingFiles loads existing files for a word
func (a *Application) loadExistingFiles(word string) {
	sanitized := sanitizeFilename(word)
	
	// Load translation
	translationFile := filepath.Join(a.config.OutputDir, fmt.Sprintf("%s_translation.txt", sanitized))
	if data, err := os.ReadFile(translationFile); err == nil {
		// Parse translation from "word = translation" format
		content := string(data)
		parts := strings.Split(content, "=")
		if len(parts) >= 2 {
			a.currentTranslation = strings.TrimSpace(parts[1])
			fyne.Do(func() {
				a.translationText.SetText(fmt.Sprintf("%s = %s", word, a.currentTranslation))
			})
		}
	}
	
	// Load audio file
	audioFile := filepath.Join(a.config.OutputDir, fmt.Sprintf("%s.%s", sanitized, a.config.AudioFormat))
	if _, err := os.Stat(audioFile); err == nil {
		a.currentAudioFile = audioFile
		fyne.Do(func() {
			a.audioPlayer.SetAudioFile(audioFile)
		})
	}
	
	// Load image files
	a.currentImages = []string{}
	// Try to find images with different patterns
	patterns := []string{
		fmt.Sprintf("%s.jpg", sanitized),
		fmt.Sprintf("%s.png", sanitized),
		fmt.Sprintf("%s_0.jpg", sanitized),
		fmt.Sprintf("%s_0.png", sanitized),
		fmt.Sprintf("%s_1.jpg", sanitized),
		fmt.Sprintf("%s_1.png", sanitized),
	}
	
	for _, pattern := range patterns {
		imagePath := filepath.Join(a.config.OutputDir, pattern)
		if _, err := os.Stat(imagePath); err == nil {
			a.currentImages = append(a.currentImages, imagePath)
			break // Just load the first image found
		}
	}
	
	if len(a.currentImages) > 0 {
		fyne.Do(func() {
			a.imageDisplay.SetImages(a.currentImages)
		})
	}
	
	fyne.Do(func() {
		a.updateStatus(fmt.Sprintf("Loaded: %s", word))
	})
}

// onDelete deletes the current word's files
func (a *Application) onDelete() {
	if a.currentWord == "" {
		return
	}
	
	// Confirm deletion
	dialog.ShowConfirm("Delete Word",
		fmt.Sprintf("Delete all files for '%s'?", a.currentWord),
		func(confirm bool) {
			if confirm {
				a.deleteCurrentWord()
			}
		}, a.window)
}

// deleteCurrentWord deletes all files for the current word
func (a *Application) deleteCurrentWord() {
	sanitized := sanitizeFilename(a.currentWord)
	deletedCount := 0
	
	// List of possible files to delete
	patterns := []string{
		fmt.Sprintf("%s.mp3", sanitized),
		fmt.Sprintf("%s.wav", sanitized),
		fmt.Sprintf("%s.jpg", sanitized),
		fmt.Sprintf("%s.png", sanitized),
		fmt.Sprintf("%s.gif", sanitized),
		fmt.Sprintf("%s_*.jpg", sanitized),
		fmt.Sprintf("%s_*.png", sanitized),
		fmt.Sprintf("%s_translation.txt", sanitized),
		fmt.Sprintf("%s_attribution.txt", sanitized),
		fmt.Sprintf("%s_*_attribution.txt", sanitized),
	}
	
	// Delete files matching patterns
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(a.config.OutputDir, pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			if err := os.Remove(match); err == nil {
				deletedCount++
			}
		}
	}
	
	// Remove from existingWords
	newWords := []string{}
	for _, w := range a.existingWords {
		if w != a.currentWord {
			newWords = append(newWords, w)
		}
	}
	a.existingWords = newWords
	
	// Clear UI
	a.clearUI()
	
	// Update status
	fyne.Do(func() {
		a.updateStatus(fmt.Sprintf("Deleted %d files for '%s'", deletedCount, a.currentWord))
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