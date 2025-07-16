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
	savedCards       []anki.Card
	existingWords    []string  // Words already in anki_cards folder
	currentWordIndex int
	
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
	
	return app
}

// setupUI creates the main user interface
func (a *Application) setupUI() {
	a.window = a.app.NewWindow("TotalRecall - Bulgarian Flashcard Generator")
	a.window.Resize(fyne.NewSize(800, 600))
	
	// Create input section with navigation
	a.wordInput = widget.NewEntry()
	a.wordInput.SetPlaceHolder("Enter Bulgarian word...")
	a.wordInput.OnSubmitted = func(string) { a.onSubmit() }
	
	a.submitButton = widget.NewButton("Generate", a.onSubmit)
	a.prevWordBtn = widget.NewButton("◀ Prev", a.onPrevWord)
	a.nextWordBtn = widget.NewButton("Next ▶", a.onNextWord)
	
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
	
	displaySection := container.NewBorder(
		a.translationText,
		a.audioPlayer,
		nil, nil,
		a.imageDisplay,
	)
	
	// Create action buttons
	a.keepButton = widget.NewButton("Keep & Continue", a.onKeepAndContinue)
	a.regenerateImageBtn = widget.NewButton("Regenerate Image", a.onRegenerateImage)
	a.regenerateAudioBtn = widget.NewButton("Regenerate Audio", a.onRegenerateAudio)
	a.regenerateAllBtn = widget.NewButton("Regenerate All", a.onRegenerateAll)
	a.deleteButton = widget.NewButton("Delete", a.onDelete)
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
	
	statusSection := container.NewBorder(
		nil, nil, nil, nil,
		container.NewVBox(
			a.progressBar,
			a.statusLabel,
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
		a.wg.Wait()
	})
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
	
	// Clear previous content
	a.clearUI()
	
	a.currentWord = word
	a.setUIEnabled(false)
	a.showProgress("Generating materials for: " + word)
	
	// Generate in background
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.generateMaterials(word)
	}()
}

// generateMaterials generates all materials for a word
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
	})
	audioFile, err := a.generateAudio(word)
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
	
	// Generate images
	fyne.Do(func() {
		a.updateStatus("Downloading images...")
	})
	images, err := a.generateImages(word)
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

// onKeepAndContinue saves the current card and clears for next
func (a *Application) onKeepAndContinue() {
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
	
	// Clear UI for next word
	a.clearUI()
	a.updateStatus(fmt.Sprintf("Card saved! Total cards: %d", count))
	a.wordInput.SetText("")
	a.wordInput.FocusGained() // Focus input for next word
}

// onRegenerateImage regenerates only the image
func (a *Application) onRegenerateImage() {
	a.setActionButtonsEnabled(false)
	a.showProgress("Regenerating image...")
	
	// Clear the current image immediately
	a.imageDisplay.Clear()
	
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		
		images, err := a.generateImages(a.currentWord)
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
	
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		
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
		a.keepButton.Disable()
		a.regenerateImageBtn.Disable()
		a.regenerateAudioBtn.Disable()
		a.regenerateAllBtn.Disable()
		a.deleteButton.Disable()
	}
}

func (a *Application) showProgress(message string) {
	a.progressBar.Show()
	a.progressBar.SetValue(0.5) // Indeterminate progress
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
	a.setActionButtonsEnabled(false)
}