package gui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	
	"codeberg.org/snonux/totalrecall/internal"
	"codeberg.org/snonux/totalrecall/internal/anki"
	"codeberg.org/snonux/totalrecall/internal/audio"
)

// Application represents the main GUI application
type Application struct {
	// Fyne components
	app    fyne.App
	window fyne.Window
	
	// UI elements
	wordInput       *widget.Entry
	submitButton    *widget.Button
	imageDisplay    *ImageDisplay
	audioPlayer     *AudioPlayer
	translationText *widget.Label
	progressBar     *widget.ProgressBar
	statusLabel     *widget.Label
	queueStatusLabel *widget.Label
	imagePromptEntry *widget.Entry
	
	// Navigation buttons
	prevWordBtn     *widget.Button
	nextWordBtn     *widget.Button
	
	// Action buttons
	keepButton         *widget.Button
	regenerateImageBtn *widget.Button
	regenerateAudioBtn *widget.Button
	regenerateAllBtn   *widget.Button
	deleteButton       *widget.Button
	
	// State management
	currentWord      string
	currentAudioFile string
	currentImages    []string
	currentTranslation string
	currentJobID     int
	savedCards       []anki.Card
	existingWords    []string  // Words already in anki_cards folder
	currentWordIndex int
	deleteConfirming bool      // Track if we're in delete confirmation mode
	
	// Word processing queue
	queue *WordQueue
	
	// Processing statistics
	processingCount int  // Number of tasks currently processing (audio/image)
	
	// Configuration
	config      *Config
	audioConfig *audio.Config
	
	// Background processing
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
}

// Config holds GUI application configuration
type Config struct {
	OutputDir      string
	AudioFormat    string
	ImageProvider  string
	ImagesPerWord  int
	EnableCache    bool
	OpenAIKey      string
	PixabayKey     string
	UnsplashKey    string
}

// DefaultConfig returns default GUI configuration
func DefaultConfig() *Config {
	return &Config{
		OutputDir:      "./anki_cards",
		AudioFormat:    "mp3",
		ImageProvider:  "openai",
		ImagesPerWord:  1,
		EnableCache:    true,
	}
}

// New creates a new GUI application
func New(config *Config) *Application {
	if config == nil {
		config = DefaultConfig()
	}
	
	// Ensure output directory exists
	os.MkdirAll(config.OutputDir, 0755)
	
	ctx, cancel := context.WithCancel(context.Background())
	
	app := &Application{
		app:    app.New(),
		config: config,
		ctx:    ctx,
		cancel: cancel,
		savedCards: make([]anki.Card, 0),
	}
	
	// Initialize the word processing queue
	app.queue = NewWordQueue(ctx)
	app.queue.SetCallbacks(app.onQueueStatusUpdate, app.onJobComplete)
	
	// Set up audio configuration
	app.audioConfig = &audio.Config{
		Provider:          "openai",
		OutputDir:         config.OutputDir,
		OutputFormat:      config.AudioFormat,
		OpenAIKey:         config.OpenAIKey,
		OpenAIModel:       "gpt-4o-mini-tts",
		OpenAIVoice:       "nova",
		OpenAISpeed:       0.9,
		OpenAIInstruction: "You are speaking Bulgarian language (български език). Pronounce the Bulgarian text with authentic Bulgarian phonetics, not Russian. Speak slowly and clearly for language learners.",
		EnableCache:       config.EnableCache,
		CacheDir:          "./.audio_cache",
	}
	
	app.setupUI()
	
	// Scan existing words in output directory
	app.scanExistingWords()
	
	// Update initial queue status
	app.updateQueueStatus()
	
	return app
}

// setupUI creates the main user interface
func (a *Application) setupUI() {
	a.window = a.app.NewWindow(fmt.Sprintf("TotalRecall v%s - Bulgarian Flashcard Generator", internal.Version))
	a.window.Resize(fyne.NewSize(800, 600))
	
	// Create input section with navigation
	a.wordInput = widget.NewEntry()
	a.wordInput.SetPlaceHolder("Enter Bulgarian word...")
	a.wordInput.OnSubmitted = func(string) { a.onSubmit() }
	
	a.submitButton = widget.NewButton("Generate (G)", a.onSubmit)
	a.prevWordBtn = widget.NewButton("◀ Prev (←)", a.onPrevWord)
	a.nextWordBtn = widget.NewButton("Next (→) ▶", a.onNextWord)
	
	inputSection := container.NewBorder(
		nil, nil, 
		a.prevWordBtn,
		container.NewHBox(a.submitButton, a.nextWordBtn),
		a.wordInput,
	)
	
	// Create display section
	a.imageDisplay = NewImageDisplay()
	a.audioPlayer = NewAudioPlayer()
	a.translationText = widget.NewLabel("")
	a.translationText.Alignment = fyne.TextAlignCenter
	
	// Create image prompt entry
	a.imagePromptEntry = widget.NewMultiLineEntry()
	a.imagePromptEntry.SetPlaceHolder("Custom image prompt (optional)...")
	a.imagePromptEntry.Wrapping = fyne.TextWrapWord // Enable word wrapping
	
	// Create container for image and prompt with proper sizing
	promptContainer := container.NewBorder(
		widget.NewLabel("Image Prompt:"),
		nil,
		nil,
		nil,
		container.NewScroll(a.imagePromptEntry),
	)
	
	// Use a split container to give equal space to image and prompt
	imageSection := container.NewHSplit(
		a.imageDisplay,
		promptContainer,
	)
	imageSection.SetOffset(0.5) // Equal 50/50 split
	
	displaySection := container.NewBorder(
		a.translationText,
		a.audioPlayer,
		nil, nil,
		imageSection,
	)
	
	// Create action buttons
	a.keepButton = widget.NewButton("New Word (N)", a.onKeepAndContinue)
	a.regenerateImageBtn = widget.NewButton("Regenerate Image (I)", a.onRegenerateImage)
	a.regenerateAudioBtn = widget.NewButton("Regenerate Audio (A)", a.onRegenerateAudio)
	a.regenerateAllBtn = widget.NewButton("Regenerate All (R)", a.onRegenerateAll)
	a.deleteButton = widget.NewButton("Delete (D)", a.onDelete)
	a.deleteButton.Importance = widget.DangerImportance
	
	// Initially disable action buttons
	a.setActionButtonsEnabled(false)
	
	actionSection := container.NewHBox(
		a.keepButton,
		layout.NewSpacer(),
		a.deleteButton,
		widget.NewSeparator(),
		a.regenerateImageBtn,
		a.regenerateAudioBtn,
		a.regenerateAllBtn,
	)
	
	// Create status section
	a.progressBar = widget.NewProgressBar()
	a.progressBar.Hide()
	a.statusLabel = widget.NewLabel("Ready")
	a.queueStatusLabel = widget.NewLabel("Queue: Empty")
	a.queueStatusLabel.TextStyle = fyne.TextStyle{Italic: true}
	
	statusSection := container.NewBorder(
		nil, nil, nil, nil,
		container.NewVBox(
			a.progressBar,
			a.statusLabel,
			widget.NewSeparator(),
			a.queueStatusLabel,
		),
	)
	
	// Create menu
	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("Export to Anki...", a.onExportToAnki),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Preferences...", a.onPreferences),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", a.app.Quit),
	)
	
	mainMenu := fyne.NewMainMenu(fileMenu)
	a.window.SetMainMenu(mainMenu)
	
	// Combine all sections
	content := container.NewBorder(
		inputSection,
		container.NewVBox(
			widget.NewSeparator(),
			actionSection,
			widget.NewSeparator(),
			statusSection,
		),
		nil, nil,
		displaySection,
	)
	
	a.window.SetContent(content)
	a.window.SetOnClosed(func() {
		a.cancel()
		a.queue.Stop()
		a.wg.Wait()
	})
	
	// Set up keyboard shortcuts
	a.setupKeyboardShortcuts()
}

// Run starts the GUI application
func (a *Application) Run() {
	a.window.ShowAndRun()
}

// onSubmit handles word submission
func (a *Application) onSubmit() {
	word := a.wordInput.Text
	if word == "" {
		return
	}
	
	// Validate Bulgarian text
	if err := audio.ValidateBulgarianText(word); err != nil {
		dialog.ShowError(err, a.window)
		return
	}
	
	// Get custom prompt from the UI
	customPrompt := a.imagePromptEntry.Text
	
	// Add word to processing queue with custom prompt
	job := a.queue.AddWordWithPrompt(word, customPrompt)
	
	// Clear the input field for next word
	a.wordInput.SetText("")
	
	// Update status to show word was queued
	a.updateStatus(fmt.Sprintf("Added '%s' to queue (Job #%d)", word, job.ID))
	
	// Update queue status immediately
	a.updateQueueStatus()
	
	// Start processing if not already processing
	a.processNextInQueue()
}

// generateMaterials generates all materials for a word (used by regenerate functions)
func (a *Application) generateMaterials(word string) {
	// Translate word
	fyne.Do(func() {
		a.updateStatus("Translating...")
	})
	translation, err := a.translateWord(word)
	if err != nil {
		fyne.Do(func() {
			a.showError(fmt.Errorf("Translation failed: %w", err))
			a.setUIEnabled(true)
		})
		return
	}
	a.currentTranslation = translation
	fyne.Do(func() {
		a.translationText.SetText(fmt.Sprintf("%s = %s", word, translation))
	})
	
	// Generate audio
	fyne.Do(func() {
		a.updateStatus("Generating audio...")
		a.incrementProcessing() // Audio processing starts
	})
	audioFile, err := a.generateAudio(word)
	a.decrementProcessing() // Audio processing ends
	
	if err != nil {
		fyne.Do(func() {
			a.showError(fmt.Errorf("Audio generation failed: %w", err))
			a.setUIEnabled(true)
		})
		return
	}
	a.currentAudioFile = audioFile
	fyne.Do(func() {
		a.audioPlayer.SetAudioFile(audioFile)
	})
	
	// Generate images with custom prompt if provided
	fyne.Do(func() {
		a.updateStatus("Downloading images...")
		a.incrementProcessing() // Image processing starts
	})
	
	// Get custom prompt from UI
	customPrompt := a.imagePromptEntry.Text
	
	images, err := a.generateImagesWithPrompt(word, customPrompt)
	a.decrementProcessing() // Image processing ends
	
	if err != nil {
		fyne.Do(func() {
			a.showError(fmt.Errorf("Image download failed: %w", err))
			a.setUIEnabled(true)
		})
		return
	}
	a.currentImages = images
	fyne.Do(func() {
		a.imageDisplay.SetImages(images)
	})
	
	// Enable action buttons
	fyne.Do(func() {
		a.hideProgress()
		a.updateStatus("Ready - Review and decide")
		a.setUIEnabled(true)
		a.setActionButtonsEnabled(true)
	})
}

// onKeepAndContinue saves the current card and clears for a new word
func (a *Application) onKeepAndContinue() {
	// Check if we have a complete word to save
	if a.currentWord != "" && a.currentAudioFile != "" && len(a.currentImages) > 0 {
		// Save current card
		card := anki.Card{
			Bulgarian:   a.currentWord,
			AudioFile:   a.currentAudioFile,
			ImageFiles:  a.currentImages,
			Translation: a.currentTranslation,
		}
		
		a.mu.Lock()
		a.savedCards = append(a.savedCards, card)
		count := len(a.savedCards)
		a.mu.Unlock()
		
		// Save translation file for future navigation
		if a.currentTranslation != "" {
			filename := sanitizeFilename(a.currentWord)
			translationFile := filepath.Join(a.config.OutputDir, fmt.Sprintf("%s_translation.txt", filename))
			content := fmt.Sprintf("%s = %s\n", a.currentWord, a.currentTranslation)
			os.WriteFile(translationFile, []byte(content), 0644)
		}
		
		// Rescan existing words to include the new one
		a.scanExistingWords()
		
		a.updateStatus(fmt.Sprintf("Card saved! Total cards: %d", count))
	}
	
	// Clear current job ID to allow navigation back to this word
	a.mu.Lock()
	currentJobID := a.currentJobID
	a.currentJobID = 0
	a.mu.Unlock()
	
	// If there was a job in progress, it will continue in the background
	if currentJobID != 0 {
		a.updateStatus("Previous word continues processing in background")
	}
	
	// Clear UI for next word
	a.clearUI()
	a.wordInput.SetText("")
	a.wordInput.FocusGained() // Focus input for next word
	
	// Hide progress bar if it was showing
	a.hideProgress()
	
	// Re-enable submit button
	a.submitButton.Enable()
}

// onRegenerateImage regenerates only the image
func (a *Application) onRegenerateImage() {
	a.setActionButtonsEnabled(false)
	a.showProgress("Regenerating image...")
	
	// Clear the current image immediately
	a.imageDisplay.Clear()
	
	// Get custom prompt from UI
	customPrompt := a.imagePromptEntry.Text
	
	a.incrementProcessing() // Image processing starts
	
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer a.decrementProcessing() // Image processing ends
		
		images, err := a.generateImagesWithPrompt(a.currentWord, customPrompt)
		if err != nil {
			fyne.Do(func() {
				a.showError(fmt.Errorf("Image regeneration failed: %w", err))
			})
		} else {
			a.currentImages = images
			fyne.Do(func() {
				a.imageDisplay.SetImages(images)
			})
		}
		
		fyne.Do(func() {
			a.hideProgress()
			a.setActionButtonsEnabled(true)
		})
	}()
}

// onRegenerateAudio regenerates audio with a different voice
func (a *Application) onRegenerateAudio() {
	a.setActionButtonsEnabled(false)
	a.showProgress("Regenerating audio...")
	
	a.incrementProcessing() // Audio processing starts
	
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer a.decrementProcessing() // Audio processing ends
		
		audioFile, err := a.generateAudio(a.currentWord)
		if err != nil {
			fyne.Do(func() {
				a.showError(fmt.Errorf("Audio regeneration failed: %w", err))
			})
		} else {
			a.currentAudioFile = audioFile
			fyne.Do(func() {
				a.audioPlayer.SetAudioFile(audioFile)
			})
		}
		
		fyne.Do(func() {
			a.hideProgress()
			a.setActionButtonsEnabled(true)
		})
	}()
}

// onRegenerateAll regenerates both audio and images
func (a *Application) onRegenerateAll() {
	a.setUIEnabled(false)
	a.showProgress("Regenerating all materials...")
	
	// Clear the current image immediately
	a.imageDisplay.Clear()
	
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.generateMaterials(a.currentWord)
	}()
}

// onExportToAnki exports saved cards to Anki CSV
func (a *Application) onExportToAnki() {
	if len(a.savedCards) == 0 {
		dialog.ShowInformation("No Cards", "No cards to export. Generate some cards first!", a.window)
		return
	}
	
	// Create save dialog
	saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		if writer == nil {
			return
		}
		defer writer.Close()
		
		// Generate Anki CSV
		outputPath := writer.URI().Path()
		gen := anki.NewGenerator(&anki.GeneratorOptions{
			OutputPath:     outputPath,
			MediaFolder:    a.config.OutputDir,
			IncludeHeaders: true,
			AudioFormat:    a.config.AudioFormat,
		})
		
		// Add all saved cards
		for _, card := range a.savedCards {
			gen.AddCard(card)
		}
		
		// Generate CSV
		if err := gen.GenerateCSV(); err != nil {
			dialog.ShowError(fmt.Errorf("Failed to generate CSV: %w", err), a.window)
			return
		}
		
		dialog.ShowInformation("Export Complete", 
			fmt.Sprintf("Exported %d cards to:\n%s", len(a.savedCards), outputPath), 
			a.window)
	}, a.window)
	
	saveDialog.SetFileName("anki_import.csv")
	saveDialog.SetFilter(storage.NewExtensionFileFilter([]string{".csv"}))
	saveDialog.Show()
}

// onPreferences shows the preferences dialog
func (a *Application) onPreferences() {
	// This will be implemented in preferences.go
	dialog.ShowInformation("Preferences", "Preferences dialog coming soon!", a.window)
}

// Helper methods
func (a *Application) setUIEnabled(enabled bool) {
	if enabled {
		a.wordInput.Enable()
		a.submitButton.Enable()
	} else {
		a.wordInput.Disable()
		a.submitButton.Disable()
	}
}

func (a *Application) setActionButtonsEnabled(enabled bool) {
	if enabled {
		a.keepButton.Enable()
		a.regenerateImageBtn.Enable()
		a.regenerateAudioBtn.Enable()
		a.regenerateAllBtn.Enable()
		a.deleteButton.Enable()
	} else {
		// Keep "New Word" button enabled to allow starting a new word during processing
		// a.keepButton.Disable() // Don't disable this
		a.regenerateImageBtn.Disable()
		a.regenerateAudioBtn.Disable()
		a.regenerateAllBtn.Disable()
		a.deleteButton.Disable()
	}
}

func (a *Application) showProgress(message string) {
	a.progressBar.Show()
	a.progressBar.SetValue(0.1) // Start at 10%
	a.statusLabel.SetText(message)
}

func (a *Application) hideProgress() {
	a.progressBar.Hide()
}

func (a *Application) updateStatus(message string) {
	a.statusLabel.SetText(message)
}

func (a *Application) showError(err error) {
	dialog.ShowError(err, a.window)
	a.updateStatus("Error: " + err.Error())
}

func (a *Application) clearUI() {
	a.imageDisplay.Clear()
	a.audioPlayer.Clear()
	a.translationText.SetText("")
	a.imagePromptEntry.SetText("")
	a.setActionButtonsEnabled(false)
}

// processNextInQueue processes the next word in the queue
func (a *Application) processNextInQueue() {
	// Check if we're already processing
	if a.currentJobID != 0 {
		return
	}
	
	// Get next job from queue
	job := a.queue.ProcessNextJob()
	if job == nil {
		return
	}
	
	// Set current job
	a.mu.Lock()
	a.currentJobID = job.ID
	a.currentWord = job.Word
	a.mu.Unlock()
	
	// Clear UI for new word
	fyne.Do(func() {
		a.clearUI()
		a.showProgress("Processing: " + job.Word)
		a.updateQueueStatus() // Update to show item moved from queued to processing
	})
	
	// Process in background
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.processWordJob(job)
	}()
}

// processWordJob processes a single word job
func (a *Application) processWordJob(job *WordJob) {
	// Translate word
	fyne.Do(func() {
		a.updateStatus(fmt.Sprintf("Translating '%s'...", job.Word))
	})
	
	translation, err := a.translateWord(job.Word)
	if err != nil {
		a.queue.FailJob(job.ID, fmt.Errorf("translation failed: %w", err))
		a.finishCurrentJob()
		return
	}
	
	// Update UI with translation immediately if this is still the current job
	a.mu.Lock()
	if a.currentJobID == job.ID {
		a.currentTranslation = translation
		fyne.Do(func() {
			a.translationText.SetText(fmt.Sprintf("%s = %s", job.Word, translation))
		})
	}
	a.mu.Unlock()
	
	// Generate audio
	fyne.Do(func() {
		a.updateStatus(fmt.Sprintf("Generating audio for '%s'...", job.Word))
		a.progressBar.SetValue(0.4)
		a.incrementProcessing() // Audio processing starts
	})
	
	audioFile, err := a.generateAudio(job.Word)
	a.decrementProcessing() // Audio processing ends
	
	if err != nil {
		a.queue.FailJob(job.ID, fmt.Errorf("audio generation failed: %w", err))
		a.finishCurrentJob()
		return
	}
	
	// Update UI with audio immediately if this is still the current job
	a.mu.Lock()
	if a.currentJobID == job.ID {
		a.currentAudioFile = audioFile
		fyne.Do(func() {
			a.audioPlayer.SetAudioFile(audioFile)
			// Enable audio-related actions
			a.regenerateAudioBtn.Enable()
		})
	}
	a.mu.Unlock()
	
	// Generate images
	fyne.Do(func() {
		a.updateStatus(fmt.Sprintf("Downloading images for '%s'...", job.Word))
		a.progressBar.SetValue(0.7)
		a.incrementProcessing() // Image processing starts
	})
	
	// Use the custom prompt from the job
	imageFiles, err := a.generateImagesWithPrompt(job.Word, job.CustomPrompt)
	a.decrementProcessing() // Image processing ends
	
	if err != nil {
		a.queue.FailJob(job.ID, fmt.Errorf("image download failed: %w", err))
		a.finishCurrentJob()
		return
	}
	
	// Mark job as completed
	fyne.Do(func() {
		a.progressBar.SetValue(0.95)
		a.updateStatus(fmt.Sprintf("Finalizing '%s'...", job.Word))
	})
	
	a.queue.CompleteJob(job.ID, translation, audioFile, imageFiles)
	
	// Update UI with results if this is still the current job
	a.mu.Lock()
	if a.currentJobID == job.ID {
		a.currentTranslation = translation
		a.currentAudioFile = audioFile
		a.currentImages = imageFiles
		
		fyne.Do(func() {
			a.translationText.SetText(fmt.Sprintf("%s = %s", job.Word, translation))
			a.imageDisplay.SetImages(imageFiles)
			a.audioPlayer.SetAudioFile(audioFile)
			a.hideProgress()
			a.setActionButtonsEnabled(true)
			a.updateStatus(fmt.Sprintf("Completed: %s", job.Word))
		})
	}
	a.mu.Unlock()
	
	// Finish this job
	a.finishCurrentJob()
	
	// Update queue status
	fyne.Do(func() {
		a.updateQueueStatus()
	})
}

// finishCurrentJob clears the current job and processes next in queue
func (a *Application) finishCurrentJob() {
	a.mu.Lock()
	a.currentJobID = 0
	a.mu.Unlock()
	
	// Process next in queue
	fyne.Do(func() {
		a.processNextInQueue()
	})
}

// onQueueStatusUpdate handles queue status updates
func (a *Application) onQueueStatusUpdate(job *WordJob) {
	fyne.Do(func() {
		a.updateQueueStatus()
	})
}

// onJobComplete handles job completion
func (a *Application) onJobComplete(job *WordJob) {
	fyne.Do(func() {
		a.updateQueueStatus()
		
		// If this was the current job and it failed, show error
		if job.ID == a.currentJobID && job.Status == StatusFailed {
			a.showError(job.Error)
			a.hideProgress()
			a.finishCurrentJob()
		}
		
		// Update navigation to include the newly completed word
		if job.Status == StatusCompleted {
			a.updateNavigation()
			
			// Check if the completed job is for the currently displayed word
			// Only update UI if the current word is still empty (waiting for this job)
			if job.Word == a.currentWord && job.ID != a.currentJobID {
				// Check if the UI is still empty/waiting for content
				hasContent := a.currentAudioFile != "" || len(a.currentImages) > 0
				
				if !hasContent {
					// Update the UI with the completed results since it's still waiting
					// Update each component individually to show progress
					if job.Translation != "" && a.currentTranslation == "" {
						a.currentTranslation = job.Translation
						a.translationText.SetText(fmt.Sprintf("%s = %s", job.Word, job.Translation))
					}
					if job.AudioFile != "" && a.currentAudioFile == "" {
						a.currentAudioFile = job.AudioFile
						a.audioPlayer.SetAudioFile(job.AudioFile)
						a.regenerateAudioBtn.Enable()
					}
					if len(job.ImageFiles) > 0 && len(a.currentImages) == 0 {
						a.currentImages = job.ImageFiles
						a.imageDisplay.SetImages(job.ImageFiles)
						a.regenerateImageBtn.Enable()
					}
					
					// Enable all action buttons since we now have complete content
					a.setActionButtonsEnabled(true)
					a.updateStatus(fmt.Sprintf("Processing completed: %s", job.Word))
				} else {
					// Word already has content, just show notification
					a.updateStatus(fmt.Sprintf("Background processing completed: %s", job.Word))
				}
			} else if job.ID != a.currentJobID {
				// Show a subtle notification for other background completions
				a.updateStatus(fmt.Sprintf("Background processing completed: %s", job.Word))
			}
		}
	})
}

// updateQueueStatus updates the queue status label
func (a *Application) updateQueueStatus() {
	a.mu.Lock()
	processing := a.processingCount
	a.mu.Unlock()
	
	// Count total cards from various sources
	// 1. Saved cards from the session
	savedCount := len(a.savedCards)
	
	// 2. Existing words from disk
	existingCount := len(a.existingWords)
	
	// 3. Completed jobs from queue
	completedJobs := a.queue.GetCompletedJobs()
	queueCompleted := len(completedJobs)
	
	totalCards := savedCount + existingCount + queueCompleted
	
	status := fmt.Sprintf("Processing: %d | Total cards: %d", processing, totalCards)
	
	a.queueStatusLabel.SetText(status)
}

// incrementProcessing increments the processing count and updates the status
func (a *Application) incrementProcessing() {
	a.mu.Lock()
	a.processingCount++
	a.mu.Unlock()
	
	// Update UI on main thread
	fyne.Do(func() {
		a.updateQueueStatus()
	})
}

// decrementProcessing decrements the processing count and updates the status
func (a *Application) decrementProcessing() {
	a.mu.Lock()
	if a.processingCount > 0 {
		a.processingCount--
	}
	a.mu.Unlock()
	
	// Update UI on main thread
	fyne.Do(func() {
		a.updateQueueStatus()
	})
}

// setupKeyboardShortcuts sets up keyboard shortcuts for the application
func (a *Application) setupKeyboardShortcuts() {
	// Create a custom shortcut handler
	a.window.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		// Don't process shortcuts if the word input is focused
		if a.window.Canvas().Focused() == a.wordInput || a.window.Canvas().Focused() == a.imagePromptEntry {
			return
		}
		
		// Don't process if we're in delete confirmation mode (handled by dialog)
		if a.deleteConfirming {
			return
		}
		
		switch ev.Name {
		case fyne.KeyG: // Generate
			if a.submitButton.Disabled() {
				return
			}
			a.onSubmit()
			
		case fyne.KeyN: // New Word
			if a.keepButton.Disabled() {
				return
			}
			a.onKeepAndContinue()
			
		case fyne.KeyI: // Regenerate Image
			if a.regenerateImageBtn.Disabled() {
				return
			}
			a.onRegenerateImage()
			
		case fyne.KeyA: // Regenerate Audio
			if a.regenerateAudioBtn.Disabled() {
				return
			}
			a.onRegenerateAudio()
			
		case fyne.KeyR: // Regenerate All
			if a.regenerateAllBtn.Disabled() {
				return
			}
			a.onRegenerateAll()
			
		case fyne.KeyD: // Delete
			if a.deleteButton.Disabled() {
				return
			}
			a.onDelete()
			
		case fyne.KeyLeft: // Previous word
			if a.prevWordBtn.Disabled() {
				return
			}
			a.onPrevWord()
			
		case fyne.KeyRight: // Next word
			if a.nextWordBtn.Disabled() {
				return
			}
			a.onNextWord()
			
		case fyne.KeyP: // Play audio
			if a.currentAudioFile != "" {
				a.audioPlayer.Play()
			}
			
		case fyne.KeyEscape: // Cancel any operation
			a.deleteConfirming = false
		}
	})
}

