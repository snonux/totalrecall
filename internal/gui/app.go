package gui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	fynetooltip "github.com/dweymouth/fyne-tooltip"
	ttwidget "github.com/dweymouth/fyne-tooltip/widget"
	"github.com/sashabaranov/go-openai"

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
	wordInput        *widget.Entry
	submitButton     *ttwidget.Button
	imageDisplay     *ImageDisplay
	audioPlayer      *AudioPlayer
	translationEntry *widget.Entry
	statusLabel      *widget.Label
	queueStatusLabel *widget.Label
	imagePromptEntry *widget.Entry
	phoneticDisplay  *widget.Label

	// Navigation buttons
	prevWordBtn *ttwidget.Button
	nextWordBtn *ttwidget.Button

	// Action buttons
	keepButton               *ttwidget.Button
	regenerateImageBtn       *ttwidget.Button
	regenerateRandomImageBtn *ttwidget.Button
	regenerateAudioBtn       *ttwidget.Button
	regenerateAllBtn         *ttwidget.Button
	deleteButton             *ttwidget.Button

	// State management
	currentWord        string
	currentAudioFile   string
	currentImage       string
	currentTranslation string
	currentJobID       int
	savedCards         []anki.Card
	existingWords      []string // Words already in anki_cards folder
	currentWordIndex   int
	deleteConfirming   bool        // Track if we're in delete confirmation mode
	wordChangeTimer    *time.Timer // Timer for detecting word changes

	// Word processing queue
	queue *WordQueue

	// Processing statistics
	processingCount int // Number of tasks currently processing (audio/image)

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
	OutputDir     string
	AudioFormat   string
	ImageProvider string
	EnableCache   bool
	OpenAIKey     string
}

// DefaultConfig returns default GUI configuration
func DefaultConfig() *Config {
	return &Config{
		OutputDir:     "./anki_cards",
		AudioFormat:   "mp3",
		ImageProvider: "openai",
		EnableCache:   true,
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
		app:        app.New(),
		config:     config,
		ctx:        ctx,
		cancel:     cancel,
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
	a.wordInput.SetPlaceHolder("Bulgarian word...")
	a.wordInput.OnSubmitted = func(string) {
		a.onSubmit()
		// Remove focus from input field after submit
		a.window.Canvas().Unfocus()
	}
	a.wordInput.OnChanged = func(text string) {
		// When user starts typing a new word, disconnect from any previous job
		// to prevent mix-ups with background processing
		a.mu.Lock()
		oldWord := a.currentWord
		if a.currentJobID != 0 && text != a.currentWord {
			a.currentJobID = 0
		}
		a.mu.Unlock()

		// Check for word change when user stops typing
		if oldWord != "" && text != "" && oldWord != text {
			// Set a timer to detect when user stops typing
			if a.wordChangeTimer != nil {
				a.wordChangeTimer.Stop()
			}
			a.wordChangeTimer = time.AfterFunc(1*time.Second, func() {
				finalWord := strings.TrimSpace(a.wordInput.Text)
				if finalWord != "" && finalWord != oldWord {
					a.handleWordChange(oldWord, finalWord)
				}
			})
		}
	}

	// Create translation entry
	a.translationEntry = widget.NewEntry()
	a.translationEntry.SetPlaceHolder("English translation...")
	a.translationEntry.OnChanged = func(text string) {
		// When user starts typing in translation field, disconnect from any previous job
		// to prevent mix-ups with background processing
		a.mu.Lock()
		if a.currentJobID != 0 && a.currentTranslation != text {
			a.currentJobID = 0
		}
		a.mu.Unlock()

		a.currentTranslation = text
		// Save the updated translation immediately
		a.saveTranslation()
	}
	a.translationEntry.OnSubmitted = func(string) {
		a.onSubmit()
		// Remove focus from input field after submit
		a.window.Canvas().Unfocus()
	}

	// Create navigation buttons with tooltips
	a.submitButton = ttwidget.NewButton("", a.onSubmit)
	a.submitButton.Icon = theme.ConfirmIcon()
	a.submitButton.SetToolTip("Generate word (G)")

	a.prevWordBtn = ttwidget.NewButton("", a.onPrevWord)
	a.prevWordBtn.Icon = theme.NavigateBackIcon()
	a.prevWordBtn.SetToolTip("Previous word (←)")

	a.nextWordBtn = ttwidget.NewButton("", a.onNextWord)
	a.nextWordBtn.Icon = theme.NavigateNextIcon()
	a.nextWordBtn.SetToolTip("Next word (→)")

	// Create a grid layout for inputs
	inputGrid := container.New(layout.NewGridLayout(2),
		a.wordInput,
		a.translationEntry,
	)

	inputSection := container.NewBorder(
		nil, nil,
		a.prevWordBtn,
		container.NewHBox(a.submitButton, a.nextWordBtn),
		inputGrid,
	)

	// Create display section
	a.imageDisplay = NewImageDisplay()
	a.audioPlayer = NewAudioPlayer()

	// Create image prompt entry
	a.imagePromptEntry = widget.NewMultiLineEntry()
	a.imagePromptEntry.SetPlaceHolder("Custom image prompt (optional)... Press Escape to exit field")
	a.imagePromptEntry.Wrapping = fyne.TextWrapWord // Enable word wrapping
	a.imagePromptEntry.OnChanged = func(text string) {
		// Save the image prompt immediately when changed
		a.saveImagePrompt()
	}

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

	// Create phonetic display section
	a.phoneticDisplay = widget.NewLabel("Phonetic information will appear here...")
	a.phoneticDisplay.Wrapping = fyne.TextWrapWord

	// Set minimum size for phonetic display (reduced to ~5 lines of text)
	// Assuming ~20 pixels per line with standard font
	phoneticScroll := container.NewScroll(a.phoneticDisplay)
	phoneticScroll.SetMinSize(fyne.NewSize(0, 100))

	phoneticContainer := container.NewBorder(
		widget.NewLabel("Phonetic Information:"),
		nil,
		nil,
		nil,
		phoneticScroll,
	)

	// Create a container for audio player and phonetic info
	audioPhoneticSection := container.NewVSplit(
		phoneticContainer,
		a.audioPlayer,
	)
	audioPhoneticSection.SetOffset(0.5) // Equal split between phonetic and audio

	displaySection := container.NewBorder(
		nil,
		audioPhoneticSection,
		nil, nil,
		imageSection,
	)

	// Create action buttons with tooltips
	a.keepButton = ttwidget.NewButtonWithIcon("", theme.DocumentCreateIcon(), a.onKeepAndContinue)
	a.keepButton.SetToolTip("Keep card and new word (N)")

	a.regenerateImageBtn = ttwidget.NewButtonWithIcon("", theme.ViewRefreshIcon(), a.onRegenerateImage)
	a.regenerateImageBtn.SetToolTip("Regenerate image (I)")

	a.regenerateRandomImageBtn = ttwidget.NewButtonWithIcon("", theme.MediaPhotoIcon(), a.onRegenerateRandomImage)
	a.regenerateRandomImageBtn.SetToolTip("Random image (M)")

	a.regenerateAudioBtn = ttwidget.NewButtonWithIcon("", theme.MediaRecordIcon(), a.onRegenerateAudio)
	a.regenerateAudioBtn.SetToolTip("Regenerate audio (A)")

	a.regenerateAllBtn = ttwidget.NewButtonWithIcon("", theme.ViewFullScreenIcon(), a.onRegenerateAll)
	a.regenerateAllBtn.SetToolTip("Regenerate all (R)")

	a.deleteButton = ttwidget.NewButtonWithIcon("", theme.DeleteIcon(), a.onDelete)
	a.deleteButton.Importance = widget.DangerImportance
	a.deleteButton.SetToolTip("Delete word (D)")

	// Initially disable action buttons
	a.setActionButtonsEnabled(false)

	// Create toolbar with all action buttons aligned to the left
	toolbar := container.NewHBox(
		a.keepButton,
		a.deleteButton,
		widget.NewSeparator(),
		a.regenerateImageBtn,
		a.regenerateRandomImageBtn,
		a.regenerateAudioBtn,
		a.regenerateAllBtn,
	)

	// Create status section
	a.statusLabel = widget.NewLabel("Ready")
	a.queueStatusLabel = widget.NewLabel("Queue: Empty")
	a.queueStatusLabel.TextStyle = fyne.TextStyle{Italic: true}

	statusSection := container.NewBorder(
		nil, nil, nil, nil,
		container.NewVBox(
			a.statusLabel,
			widget.NewSeparator(),
			a.queueStatusLabel,
		),
	)

	// Create menu
	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("Export to Anki... (E)", a.onExportToAnki),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Hotkeys... (H)", a.onShowHotkeys),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", a.app.Quit),
	)

	mainMenu := fyne.NewMainMenu(fileMenu)
	a.window.SetMainMenu(mainMenu)

	// Combine all sections with toolbar at the top
	content := container.NewBorder(
		container.NewVBox(
			toolbar,
			widget.NewSeparator(),
			inputSection,
		),
		statusSection,
		nil, nil,
		displaySection,
	)

	// Add the tooltip layer to enable tooltips
	a.window.SetContent(fynetooltip.AddWindowToolTipLayer(content, a.window.Canvas()))
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
	// Focus the Bulgarian word input on startup
	a.window.Canvas().Focus(a.wordInput)
	a.window.ShowAndRun()
}

// onSubmit handles word submission
func (a *Application) onSubmit() {
	bulgarianText := strings.TrimSpace(a.wordInput.Text)
	englishText := strings.TrimSpace(a.translationEntry.Text)

	// Determine which word to process and if translation is needed
	var wordToProcess string
	var needsTranslation bool
	var translationDirection string

	if bulgarianText != "" && englishText != "" {
		// Both provided - use Bulgarian as primary, no translation needed
		wordToProcess = bulgarianText
		needsTranslation = false
		a.currentTranslation = englishText
	} else if bulgarianText != "" && englishText == "" {
		// Only Bulgarian provided - translate to English
		wordToProcess = bulgarianText
		needsTranslation = true
		translationDirection = "bg-to-en"
	} else if bulgarianText == "" && englishText != "" {
		// Only English provided - translate to Bulgarian
		needsTranslation = true
		translationDirection = "en-to-bg"
		// We'll get the Bulgarian word after translation
	} else {
		// Both empty
		return
	}

	// Handle English to Bulgarian translation first if needed
	if translationDirection == "en-to-bg" {
		a.updateStatus(fmt.Sprintf("Translating '%s' to Bulgarian...", englishText))
		bulgarian, err := a.translateEnglishToBulgarian(englishText)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Translation failed: %w", err), a.window)
			return
		}
		wordToProcess = bulgarian
		a.wordInput.SetText(bulgarian)
		a.currentTranslation = englishText
		// Update current word for saving
		a.currentWord = bulgarian
		// Save the translation immediately
		a.saveTranslation()
		needsTranslation = false // We've already done the translation, don't translate back
	} else if translationDirection == "bg-to-en" {
		// Handle Bulgarian to English translation immediately
		a.updateStatus(fmt.Sprintf("Translating '%s' to English...", bulgarianText))
		english, err := a.translateWord(bulgarianText)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Translation failed: %w", err), a.window)
			return
		}
		a.currentTranslation = english
		a.translationEntry.SetText(english)
		needsTranslation = false // We've already done the translation
		// Save the translation immediately
		a.saveTranslation()
	}

	// Validate Bulgarian text
	if err := audio.ValidateBulgarianText(wordToProcess); err != nil {
		dialog.ShowError(err, a.window)
		return
	}

	// Get custom prompt from the UI
	customPrompt := a.imagePromptEntry.Text

	// Add word to processing queue with custom prompt
	job := a.queue.AddWordWithPrompt(wordToProcess, customPrompt)

	// Store whether translation is needed and the translation if already provided
	job.NeedsTranslation = needsTranslation
	if a.currentTranslation != "" {
		job.Translation = a.currentTranslation
	}

	// Don't clear the input fields yet - they should stay populated
	// until the user is ready to enter a new word

	// Update status to show word was queued
	a.updateStatus(fmt.Sprintf("Added '%s' to queue (Job #%d)", wordToProcess, job.ID))

	// Update queue status immediately
	a.updateQueueStatus()

	// Start processing if not already processing
	a.processNextInQueue()
}

// generateMaterials generates all materials for a word (used by regenerate functions)
func (a *Application) generateMaterials(word string) {
	// Check if we already have a translation
	if a.currentTranslation == "" {
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
		// Only update if this word is still the current word
		a.mu.Lock()
		if a.currentWord == word {
			a.currentTranslation = translation
			fyne.Do(func() {
				a.translationEntry.SetText(translation)
			})
		}
		a.mu.Unlock()

		// Save translation to disk regardless
		if translation != "" {
			// Find existing card directory first
			wordDir := a.findCardDirectory(word)
			if wordDir == "" {
				// No existing directory, create new one with card ID
				cardID := internal.GenerateCardID(word)
				wordDir = filepath.Join(a.config.OutputDir, cardID)
				os.MkdirAll(wordDir, 0755) // Ensure directory exists
				// Save word metadata
				metadataFile := filepath.Join(wordDir, "word.txt")
				os.WriteFile(metadataFile, []byte(word), 0644)
			}
			translationFile := filepath.Join(wordDir, "translation.txt")
			content := fmt.Sprintf("%s = %s\n", word, translation)
			os.WriteFile(translationFile, []byte(content), 0644)
		}
	}

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

	// Only update UI if this word is still the current word
	a.mu.Lock()
	if a.currentWord == word {
		a.currentAudioFile = audioFile
		fyne.Do(func() {
			a.audioPlayer.SetAudioFile(audioFile)
		})
	}
	a.mu.Unlock()

	// Generate images with custom prompt if provided
	fyne.Do(func() {
		a.updateStatus("Waiting for/downloading images...")
		a.incrementProcessing() // Image processing starts
	})

	// Get custom prompt from UI
	customPrompt := a.imagePromptEntry.Text

	// Pass the current translation to avoid re-translating
	translation := a.currentTranslation
	if translation == "" {
		// Use the text from translationEntry if currentTranslation is not set
		translation = strings.TrimSpace(a.translationEntry.Text)
	}
	imageFile, err := a.generateImagesWithPrompt(word, customPrompt, translation)
	a.decrementProcessing() // Image processing ends

	if err != nil {
		fyne.Do(func() {
			a.showError(fmt.Errorf("Image download failed: %w", err))
			a.setUIEnabled(true)
		})
		return
	}

	// Only update UI if this word is still the current word
	if imageFile != "" {
		a.mu.Lock()
		if a.currentWord == word {
			a.currentImage = imageFile
			fyne.Do(func() {
				a.imageDisplay.SetImages([]string{imageFile})
			})
		}
		a.mu.Unlock()
	}

	// Fetch phonetic information in a separate goroutine
	go func() {
		fyne.Do(func() {
			a.incrementProcessing() // Phonetic processing starts
		})

		phoneticInfo, err := a.getPhoneticInfo(word)
		if err != nil {
			// Log error but don't fail - phonetic info is optional
			fmt.Printf("Warning: Failed to get phonetic info: %v\n", err)
			phoneticInfo = "Failed to fetch phonetic information"
		}

		// Update UI with phonetic info if this is still the current word
		a.mu.Lock()
		if a.currentWord == word {
			fyne.Do(func() {
				a.phoneticDisplay.SetText(phoneticInfo)
			})
		}
		a.mu.Unlock()

		// Save phonetic info to disk
		if phoneticInfo != "" && phoneticInfo != "Failed to fetch phonetic information" {
			a.savePhoneticInfoForWord(word, phoneticInfo)
		}

		a.decrementProcessing() // Phonetic processing ends
	}()

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
	if a.currentWord != "" && a.currentAudioFile != "" && a.currentImage != "" {
		// Save current card
		card := anki.Card{
			Bulgarian:   a.currentWord,
			AudioFile:   a.currentAudioFile,
			ImageFile:   a.currentImage,
			Translation: a.currentTranslation,
		}

		a.mu.Lock()
		a.savedCards = append(a.savedCards, card)
		count := len(a.savedCards)
		a.mu.Unlock()

		// Save translation, prompt, and phonetic files for future navigation
		a.saveTranslation()
		a.saveImagePrompt()
		a.savePhoneticInfo()

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

	// Clear UI and input fields for next word
	a.clearUI()
	a.wordInput.SetText("")
	a.translationEntry.SetText("")

	// Clear current state to prevent mix-ups with background jobs
	a.mu.Lock()
	a.currentWord = ""
	a.currentTranslation = ""
	a.currentAudioFile = ""
	a.currentImage = ""
	a.mu.Unlock()

	a.wordInput.FocusGained() // Focus input for next word

	// Hide progress bar if it was showing
	a.hideProgress()

	// Re-enable submit button
	a.submitButton.Enable()
}

// onRegenerateImage regenerates only the image
func (a *Application) onRegenerateImage() {
	// Only disable the image-related buttons
	a.regenerateImageBtn.Disable()
	a.regenerateRandomImageBtn.Disable()
	a.regenerateAllBtn.Disable()
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

		// Use the current translation to avoid re-translating
		translation := a.currentTranslation
		if translation == "" {
			// Use the text from translationEntry if currentTranslation is not set
			translation = strings.TrimSpace(a.translationEntry.Text)
		}
		// Store the word we're generating for
		wordForGeneration := a.currentWord
		imageFile, err := a.generateImagesWithPrompt(wordForGeneration, customPrompt, translation)
		if err != nil {
			fyne.Do(func() {
				a.showError(fmt.Errorf("Image regeneration failed: %w", err))
			})
		} else {
			if imageFile != "" {
				// Only update if we're still on the same word
				a.mu.Lock()
				if a.currentWord == wordForGeneration {
					a.currentImage = imageFile
					a.mu.Unlock()
					fyne.Do(func() {
						a.imageDisplay.SetImages([]string{imageFile})
					})
				} else {
					a.mu.Unlock()
				}
			}
		}

		fyne.Do(func() {
			a.hideProgress()
			// Re-enable image-related buttons
			a.regenerateImageBtn.Enable()
			a.regenerateRandomImageBtn.Enable()
			a.regenerateAllBtn.Enable()
		})
	}()
}

// onRegenerateRandomImage generates a new image with a random prompt
func (a *Application) onRegenerateRandomImage() {
	// Only disable the image-related buttons
	a.regenerateImageBtn.Disable()
	a.regenerateRandomImageBtn.Disable()
	a.regenerateAllBtn.Disable()
	a.showProgress("Generating random image...")

	// Clear the current image immediately
	a.imageDisplay.Clear()

	// Clear the custom prompt to let the system generate a new one
	customPrompt := ""

	a.incrementProcessing() // Image processing starts

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer a.decrementProcessing() // Image processing ends

		// Use the current translation to avoid re-translating
		translation := a.currentTranslation
		if translation == "" {
			// Use the text from translationEntry if currentTranslation is not set
			translation = strings.TrimSpace(a.translationEntry.Text)
		}
		// Store the word we're generating for
		wordForGeneration := a.currentWord
		imageFile, err := a.generateImagesWithPrompt(wordForGeneration, customPrompt, translation)
		if err != nil {
			fyne.Do(func() {
				a.showError(fmt.Errorf("Random image generation failed: %w", err))
			})
		} else {
			if imageFile != "" {
				// Only update if we're still on the same word
				a.mu.Lock()
				if a.currentWord == wordForGeneration {
					a.currentImage = imageFile
					a.mu.Unlock()
					fyne.Do(func() {
						a.imageDisplay.SetImages([]string{imageFile})
					})
				} else {
					a.mu.Unlock()
				}
			}
		}

		fyne.Do(func() {
			a.hideProgress()
			// Re-enable image-related buttons
			a.regenerateImageBtn.Enable()
			a.regenerateRandomImageBtn.Enable()
			a.regenerateAllBtn.Enable()
		})
	}()
}

// onRegenerateAudio regenerates audio with a different voice
func (a *Application) onRegenerateAudio() {
	// Only disable the audio-related buttons
	a.regenerateAudioBtn.Disable()
	a.regenerateAllBtn.Disable()
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
			// Re-enable audio-related buttons
			a.regenerateAudioBtn.Enable()
			a.regenerateAllBtn.Enable()
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

// onExportToAnki exports all cards from anki_cards folder to Anki with format selection
func (a *Application) onExportToAnki() {
	// Check if anki_cards directory exists and has content
	entries, err := os.ReadDir(a.config.OutputDir)
	if err != nil || len(entries) == 0 {
		dialog.ShowInformation("No Cards", "No cards found in anki_cards folder. Generate some cards first!", a.window)
		return
	}

	// Count subdirectories (excluding hidden ones)
	cardCount := 0
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			cardCount++
		}
	}

	if cardCount == 0 {
		dialog.ShowInformation("No Cards", "No cards found in anki_cards folder. Generate some cards first!", a.window)
		return
	}

	// Create format selection dialog
	formatOptions := []string{"APKG (Recommended)", "CSV (Legacy)"}
	formatSelect := widget.NewSelect(formatOptions, nil)
	formatSelect.SetSelected(formatOptions[0])

	deckNameEntry := widget.NewEntry()
	deckNameEntry.SetPlaceHolder("Bulgarian Vocabulary")

	content := container.NewVBox(
		widget.NewLabel("Export Format:"),
		formatSelect,
		widget.NewSeparator(),
		widget.NewLabel("Deck Name:"),
		deckNameEntry,
		widget.NewLabel(""),
		widget.NewRichTextFromMarkdown("**APKG**: Complete package with media files included\n**CSV**: Text only, requires manual media copy"),
	)

	customDialog := dialog.NewCustomConfirm("Export to Anki", "Export", "Cancel", content, func(export bool) {
		if !export {
			return
		}

		isAPKG := formatSelect.Selected == formatOptions[0]
		deckName := deckNameEntry.Text
		if deckName == "" {
			deckName = "Bulgarian Vocabulary"
		}

		// Generate export directly to anki_cards folder
		var outputPath string
		var filename string

		if isAPKG {
			filename = fmt.Sprintf("%s.apkg", internal.SanitizeFilename(deckName))
			outputPath = filepath.Join(a.config.OutputDir, filename)

			// Generate APKG from all cards in directory
			gen := anki.NewGenerator(nil)

			// Load all cards from the anki_cards directory
			if err := gen.GenerateFromDirectory(a.config.OutputDir); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to load cards: %w", err), a.window)
				return
			}

			if err := gen.GenerateAPKG(outputPath, deckName); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to generate APKG: %w", err), a.window)
				return
			}

			// Get actual card count
			total, withAudio, withImages := gen.Stats()

			// Update status bar instead of showing dialog
			a.updateStatus(fmt.Sprintf("Exported %d cards to %s (%d with audio, %d with images)",
				total, outputPath, withAudio, withImages))
		} else {
			filename = "anki_import.csv"
			outputPath = filepath.Join(a.config.OutputDir, filename)

			// Generate CSV from all cards in directory
			gen := anki.NewGenerator(&anki.GeneratorOptions{
				OutputPath:     outputPath,
				MediaFolder:    a.config.OutputDir,
				IncludeHeaders: true,
				AudioFormat:    a.config.AudioFormat,
			})

			// Load all cards from the anki_cards directory
			if err := gen.GenerateFromDirectory(a.config.OutputDir); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to load cards: %w", err), a.window)
				return
			}

			if err := gen.GenerateCSV(); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to generate CSV: %w", err), a.window)
				return
			}

			// Get actual card count
			total, withAudio, withImages := gen.Stats()

			// Update status bar instead of showing dialog
			a.updateStatus(fmt.Sprintf("Exported %d cards to %s (%d with audio, %d with images)",
				total, outputPath, withAudio, withImages))
		}
	}, a.window)

	customDialog.Resize(fyne.NewSize(400, 300))
	customDialog.Show()
}

// onShowHotkeys displays a dialog with all available keyboard shortcuts
func (a *Application) onShowHotkeys() {
	hotkeys := `## Navigation
**←** Previous word  
**→** Next word  
**Tab** Navigate fields  
**Esc** Unfocus field  

## Word Processing
**G** Generate word  
**N** New word  
**D** Delete word  

## Regeneration
**I** Regenerate image  
**M** Random image  
**A** Regenerate audio  
**R** Regenerate all  
**P** Play audio  

## Export
**E** Export to Anki  

## Help
**H** Show hotkeys`

	content := widget.NewRichTextFromMarkdown(hotkeys)
	content.Wrapping = fyne.TextWrapWord

	// Create a container with padding to prevent text cutoff
	paddedContent := container.NewPadded(content)

	// Create a scrollable container for the content
	scroll := container.NewScroll(paddedContent)
	scroll.SetMinSize(fyne.NewSize(350, 450))

	dialog.NewCustom("Keyboard Shortcuts", "Close", scroll, a.window).Show()
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
		a.regenerateRandomImageBtn.Enable()
		a.regenerateAudioBtn.Enable()
		a.regenerateAllBtn.Enable()
		a.deleteButton.Enable()
	} else {
		// Keep "New Word" button enabled to allow starting a new word during processing
		// a.keepButton.Disable() // Don't disable this
		a.regenerateImageBtn.Disable()
		a.regenerateRandomImageBtn.Disable()
		a.regenerateAudioBtn.Disable()
		a.regenerateAllBtn.Disable()
		a.deleteButton.Disable()
	}
}

func (a *Application) showProgress(message string) {
	// Check if we're already processing something
	a.mu.Lock()
	processingCount := a.processingCount
	a.mu.Unlock()

	if processingCount > 1 {
		// Show that multiple operations are in progress
		a.statusLabel.SetText(fmt.Sprintf("%s (Processing: %d tasks)", message, processingCount))
	} else {
		a.statusLabel.SetText(message)
	}
}

func (a *Application) hideProgress() {
	// Progress bar removed - nothing to hide
	// Update status to show if other operations are still running
	a.mu.Lock()
	processingCount := a.processingCount
	a.mu.Unlock()

	if processingCount > 0 {
		a.updateStatus(fmt.Sprintf("Processing %d task(s)...", processingCount))
	} else {
		a.updateStatus("Ready")
	}
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
	// Don't clear the word input or translation entry - they should stay populated
	// Clear the image prompt entry - it will be loaded from disk if available
	a.imagePromptEntry.SetText("")
	a.phoneticDisplay.SetText("Phonetic information will appear here...")
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

	// Set current job and clear any previous state
	a.mu.Lock()
	a.currentJobID = job.ID
	a.currentWord = job.Word
	// Clear previous file associations to prevent mix-ups
	a.currentTranslation = ""
	a.currentAudioFile = ""
	a.currentImage = ""
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
	// Handle translation
	var translation string
	var err error

	if job.NeedsTranslation {
		// Translate word
		fyne.Do(func() {
			a.updateStatus(fmt.Sprintf("Translating '%s'...", job.Word))
		})

		translation, err = a.translateWord(job.Word)
		if err != nil {
			a.queue.FailJob(job.ID, fmt.Errorf("translation failed: %w", err))
			a.finishCurrentJob()
			return
		}
	} else if job.Translation != "" {
		// Use provided translation
		translation = job.Translation
	}

	// Save translation to disk immediately for this specific word
	if translation != "" {
		// Find existing card directory first
		wordDir := a.findCardDirectory(job.Word)
		if wordDir == "" {
			// No existing directory, create new one with card ID
			cardID := internal.GenerateCardID(job.Word)
			wordDir = filepath.Join(a.config.OutputDir, cardID)
			os.MkdirAll(wordDir, 0755) // Ensure directory exists
			// Save word metadata
			metadataFile := filepath.Join(wordDir, "word.txt")
			os.WriteFile(metadataFile, []byte(job.Word), 0644)
		}
		translationFile := filepath.Join(wordDir, "translation.txt")
		content := fmt.Sprintf("%s = %s\n", job.Word, translation)
		os.WriteFile(translationFile, []byte(content), 0644)
	}

	// Update UI with translation immediately if this is still the current job
	a.mu.Lock()
	if a.currentJobID == job.ID && translation != "" {
		a.currentTranslation = translation
		fyne.Do(func() {
			a.translationEntry.SetText(translation)
		})
	}
	a.mu.Unlock()

	// Start fetching phonetic information concurrently
	phoneticDone := make(chan struct{})
	go func() {
		defer close(phoneticDone)

		fyne.Do(func() {
			a.incrementProcessing() // Phonetic processing starts
		})

		phoneticInfo, err := a.getPhoneticInfo(job.Word)
		if err != nil {
			// Log error but don't fail the job - phonetic info is optional
			fmt.Printf("Warning: Failed to get phonetic info: %v\n", err)
			phoneticInfo = "Failed to fetch phonetic information"
		}

		// Save phonetic info to disk immediately for this specific word
		if phoneticInfo != "" && phoneticInfo != "Failed to fetch phonetic information" {
			// Find existing card directory first
			wordDir := a.findCardDirectory(job.Word)
			if wordDir == "" {
				// No existing directory, create new one with card ID
				cardID := internal.GenerateCardID(job.Word)
				wordDir = filepath.Join(a.config.OutputDir, cardID)
				os.MkdirAll(wordDir, 0755) // Ensure directory exists
				// Save word metadata
				metadataFile := filepath.Join(wordDir, "word.txt")
				os.WriteFile(metadataFile, []byte(job.Word), 0644)
			}
			phoneticFile := filepath.Join(wordDir, "phonetic.txt")
			os.WriteFile(phoneticFile, []byte(phoneticInfo), 0644)
		}

		// Update UI with phonetic info if this is still the current job
		a.mu.Lock()
		if a.currentJobID == job.ID {
			fyne.Do(func() {
				a.phoneticDisplay.SetText(phoneticInfo)
			})
		}
		a.mu.Unlock()

		a.decrementProcessing() // Phonetic processing ends
	}()

	// Generate audio
	fyne.Do(func() {
		a.updateStatus(fmt.Sprintf("Generating audio for '%s'...", job.Word))
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
	isCurrentJob := a.currentJobID == job.ID
	if isCurrentJob {
		a.currentAudioFile = audioFile
	}
	a.mu.Unlock()

	if isCurrentJob {
		fyne.Do(func() {
			a.audioPlayer.SetAudioFile(audioFile)
			// Enable audio-related actions
			a.regenerateAudioBtn.Enable()
		})
	}

	// Generate images
	fyne.Do(func() {
		a.updateStatus(fmt.Sprintf("Waiting for/downloading images for '%s'...", job.Word))
		a.incrementProcessing() // Image processing starts
	})

	// Use the custom prompt from the job
	// The translation variable already contains the correct translation (either from job or translated)
	imageFile, err := a.generateImagesWithPrompt(job.Word, job.CustomPrompt, translation)
	a.decrementProcessing() // Image processing ends

	if err != nil {
		a.queue.FailJob(job.ID, fmt.Errorf("image download failed: %w", err))
		a.finishCurrentJob()
		return
	}

	// Wait for phonetic fetching to complete before finalizing
	<-phoneticDone

	// Mark job as completed
	fyne.Do(func() {
		a.updateStatus(fmt.Sprintf("Finalizing '%s'...", job.Word))
	})

	a.queue.CompleteJob(job.ID, translation, audioFile, imageFile)

	// Update UI with results if this is still the current job
	a.mu.Lock()
	isCurrentJob = a.currentJobID == job.ID
	if isCurrentJob {
		a.currentTranslation = translation
		a.currentAudioFile = audioFile
		if imageFile != "" {
			a.currentImage = imageFile
		}
	}
	a.mu.Unlock()

	if isCurrentJob {
		fyne.Do(func() {
			a.translationEntry.SetText(translation)
			if imageFile != "" {
				a.imageDisplay.SetImages([]string{imageFile})
			}
			a.audioPlayer.SetAudioFile(audioFile)
			a.hideProgress()
			a.setActionButtonsEnabled(true)
			a.updateStatus(fmt.Sprintf("Completed: %s", job.Word))
		})
	}

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

			// Only show status updates, don't update UI for background jobs
			// This prevents mix-ups when user has moved on to a new word
			a.mu.Lock()
			isCurrentJob := job.ID == a.currentJobID
			a.mu.Unlock()

			if isCurrentJob {
				// This is still the current job, UI update is already handled in processWordJob
				a.updateStatus(fmt.Sprintf("Processing completed: %s", job.Word))
			} else {
				// This is a background job that completed
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
	// Create a custom shortcut handler for regular keys (when input fields are not focused)
	a.window.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		// Handle Escape key to unfocus any field
		if ev.Name == fyne.KeyEscape {
			a.window.Canvas().Unfocus()
			a.deleteConfirming = false
			return
		}

		// Handle Tab key for custom focus navigation
		if ev.Name == fyne.KeyTab {
			a.handleTabNavigation()
			return
		}

		// Check if input field is focused
		focused := a.window.Canvas().Focused()
		isInputFocused := focused == a.wordInput || focused == a.imagePromptEntry || focused == a.translationEntry

		// If input is focused, don't process regular shortcuts
		if isInputFocused {
			return
		}

		// Don't process if we're in delete confirmation mode (handled by dialog)
		if a.deleteConfirming {
			return
		}

		a.handleShortcutKey(ev.Name)
	})
}

// handleTabNavigation manages custom Tab navigation order
func (a *Application) handleTabNavigation() {
	focused := a.window.Canvas().Focused()

	switch focused {
	case a.wordInput:
		// From Bulgarian -> English
		a.window.Canvas().Focus(a.translationEntry)
	case a.translationEntry:
		// From English -> Image prompt
		a.window.Canvas().Focus(a.imagePromptEntry)
	case a.imagePromptEntry:
		// From Image prompt -> Bulgarian (cycle back)
		a.window.Canvas().Focus(a.wordInput)
	default:
		// If nothing focused, start with Bulgarian
		a.window.Canvas().Focus(a.wordInput)
	}
}

// handleShortcutKey handles the actual shortcut action
func (a *Application) handleShortcutKey(key fyne.KeyName) {
	// Don't process if we're in delete confirmation mode
	if a.deleteConfirming {
		return
	}

	switch key {
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

	case fyne.KeyM: // Random Image (M for "magic" or "mixed")
		if a.regenerateRandomImageBtn.Disabled() {
			return
		}
		a.onRegenerateRandomImage()

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

	case fyne.KeyE: // Export to APKG
		a.onExportToAnki()

	case fyne.KeyH: // Show hotkeys
		a.onShowHotkeys()
	}
}

// saveTranslation saves the current translation to a file
func (a *Application) saveTranslation() {
	if a.currentWord != "" && a.currentTranslation != "" {
		// Find existing card directory
		wordDir := a.findCardDirectory(a.currentWord)
		if wordDir == "" {
			// No existing directory, create new one with card ID
			cardID := internal.GenerateCardID(a.currentWord)
			wordDir = filepath.Join(a.config.OutputDir, cardID)
			os.MkdirAll(wordDir, 0755) // Ensure directory exists
			// Save word metadata
			metadataFile := filepath.Join(wordDir, "word.txt")
			os.WriteFile(metadataFile, []byte(a.currentWord), 0644)
		}
		translationFile := filepath.Join(wordDir, "translation.txt")
		content := fmt.Sprintf("%s = %s\n", a.currentWord, a.currentTranslation)
		os.WriteFile(translationFile, []byte(content), 0644)
	}
}

// saveImagePrompt saves the current image prompt to a file
func (a *Application) saveImagePrompt() {
	// With timestamp-based card IDs, we can't update existing prompts
	// The prompt is saved when the image is generated
	// This function is kept for compatibility but does nothing
}

// handleWordChange is called when the Bulgarian word is changed
func (a *Application) handleWordChange(oldWord, newWord string) {
	// Update current word
	a.currentWord = newWord

	// Clear the custom image prompt to force regeneration with new word
	fyne.Do(func() {
		a.imagePromptEntry.SetText("")
	})

	// Check if we have existing materials
	hasExistingMaterials := a.currentImage != "" || a.currentAudioFile != ""

	if hasExistingMaterials {
		// Automatically trigger image regeneration with new prompt
		fyne.Do(func() {
			a.updateStatus(fmt.Sprintf("Word changed from '%s' to '%s' - regenerating image...", oldWord, newWord))
		})

		// Small delay to ensure UI updates
		time.AfterFunc(100*time.Millisecond, func() {
			fyne.Do(func() {
				a.onRegenerateImage()
			})
		})
	}
}

// savePhoneticInfo saves the phonetic information to a file
func (a *Application) savePhoneticInfo() {
	phoneticText := a.phoneticDisplay.Text
	if a.currentWord != "" && phoneticText != "" &&
		phoneticText != "Failed to fetch phonetic information" &&
		phoneticText != "Phonetic information will appear here..." {
		// Find existing card directory
		wordDir := a.findCardDirectory(a.currentWord)
		if wordDir == "" {
			// No existing directory, create new one with card ID
			cardID := internal.GenerateCardID(a.currentWord)
			wordDir = filepath.Join(a.config.OutputDir, cardID)
			os.MkdirAll(wordDir, 0755) // Ensure directory exists
			// Save word metadata
			metadataFile := filepath.Join(wordDir, "word.txt")
			os.WriteFile(metadataFile, []byte(a.currentWord), 0644)
		}
		phoneticFile := filepath.Join(wordDir, "phonetic.txt")
		os.WriteFile(phoneticFile, []byte(phoneticText), 0644)
	}
}

// savePhoneticInfoForWord saves the phonetic information for a specific word
func (a *Application) savePhoneticInfoForWord(word, phoneticText string) {
	if word != "" && phoneticText != "" &&
		phoneticText != "Failed to fetch phonetic information" &&
		phoneticText != "Phonetic information will appear here..." {
		// Find existing card directory first
		wordDir := a.findCardDirectory(word)
		if wordDir == "" {
			// No existing directory, create new one with card ID
			cardID := internal.GenerateCardID(word)
			wordDir = filepath.Join(a.config.OutputDir, cardID)
			os.MkdirAll(wordDir, 0755) // Ensure directory exists
			// Save word metadata
			metadataFile := filepath.Join(wordDir, "word.txt")
			os.WriteFile(metadataFile, []byte(word), 0644)
		}
		phoneticFile := filepath.Join(wordDir, "phonetic.txt")
		os.WriteFile(phoneticFile, []byte(phoneticText), 0644)
	}
}

// loadPhoneticInfo loads phonetic information from a file if it exists
func (a *Application) loadPhoneticInfo(word string) {
	wordDir := a.findCardDirectory(word)
	if wordDir == "" {
		return
	}
	
	phoneticFile := filepath.Join(wordDir, "phonetic.txt")
	if data, err := os.ReadFile(phoneticFile); err == nil {
		phoneticText := string(data)
		fyne.Do(func() {
			a.phoneticDisplay.SetText(phoneticText)
		})
	}
}

// getPhoneticInfo fetches phonetic information for a Bulgarian word using OpenAI GPT-4o
func (a *Application) getPhoneticInfo(word string) (string, error) {
	if a.config.OpenAIKey == "" {
		return "", fmt.Errorf("OpenAI API key not configured")
	}

	client := openai.NewClient(a.config.OpenAIKey)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := openai.ChatCompletionRequest{
		Model: openai.GPT4o,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a Bulgarian language expert helping language learners understand pronunciation. Provide detailed phonetic information using the International Phonetic Alphabet (IPA). For each IPA symbol used, give concrete examples of how it sounds using familiar English words or sounds when possible.",
			},
			{
				Role: openai.ChatMessageRoleUser,
				Content: fmt.Sprintf(`For the Bulgarian word '%s':
1. Provide the complete IPA transcription
2. Break down EACH phonetic symbol used in the transcription
3. For EVERY symbol, explain how it's pronounced with examples:
   - If similar to an English sound, give English word examples
   - If not in English, describe tongue/mouth position or compare to similar sounds
   - Include stress marks and explain which syllable is stressed

Example format:
Word: [IPA transcription]
• /p/ - like 'p' in English 'pot'
• /a/ - like 'a' in 'father'
• /ˈ/ - stress mark (following syllable is stressed)
etc.`, word),
			},
		},
		Temperature: 0.3,
		MaxTokens:   800,
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get phonetic info: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}
