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
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	fynetooltip "github.com/dweymouth/fyne-tooltip"
	ttwidget "github.com/dweymouth/fyne-tooltip/widget"

	"codeberg.org/snonux/totalrecall/internal"
	"codeberg.org/snonux/totalrecall/internal/anki"
	"codeberg.org/snonux/totalrecall/internal/archive"
	"codeberg.org/snonux/totalrecall/internal/audio"
	appconfig "codeberg.org/snonux/totalrecall/internal/config"
	"codeberg.org/snonux/totalrecall/internal/image"
	"codeberg.org/snonux/totalrecall/internal/phonetic"
	"codeberg.org/snonux/totalrecall/internal/translation"
)

// Application represents the main GUI application
type Application struct {
	// Fyne components
	app    fyne.App
	window fyne.Window

	// UI elements
	wordInput        *CustomEntry
	submitButton     *ttwidget.Button
	imageDisplay     *ImageDisplay
	audioPlayer      *AudioPlayer
	translationEntry *CustomEntry
	cardTypeSelect   *widget.Select
	statusLabel      *widget.Label
	queueStatusLabel *widget.Label
	imagePromptEntry *CustomMultiLineEntry
	logViewer        *LogViewer

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
	currentWord          string
	currentAudioFile     string
	currentAudioFileBack string // Back audio file for bg-bg cards
	currentImage         string
	currentTranslation   string
	currentPhonetic      string // Full phonetic information
	currentCardType      string // Card type: "en-bg" or "bg-bg"
	currentJobID         int
	savedCards           []anki.Card
	existingWords        []string // Words already in anki_cards folder
	currentWordIndex     int
	deleteConfirming     bool         // Track if we're in delete confirmation mode
	quitConfirming       bool         // Track if we're in quit confirmation mode
	wordChangeTimer      *time.Timer  // Timer for detecting word changes
	fileCheckTicker      *time.Ticker // Ticker for checking missing files

	// Word processing queue
	queue *WordQueue

	// Processing statistics
	processingCount int // Number of tasks currently processing (audio/image)

	// Auto-play state
	autoPlayEnabled bool // Whether to automatically play audio when generated or navigated to

	// Configuration
	config          *Config
	audioConfig     *audio.Config
	phoneticFetcher *phonetic.Fetcher
	translator      *translation.Translator

	// Background processing
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex

	// Per-card cancellation tracking
	cardContexts map[string]context.CancelFunc // Map of word -> cancel function
	cardMu       sync.Mutex                    // Mutex for cardContexts map

	// Active operations tracking
	activeOperations map[string]int // Map of word -> count of active operations
	activeOpMu       sync.Mutex     // Mutex for activeOperations map

	// Injectable factory functions — replaced in tests to avoid real API calls.
	newOpenAIImageClient    func(*image.OpenAIConfig) promptAwareImageClient
	newNanoBananaImageClient func(*image.NanoBananaConfig) promptAwareImageClient
	newAudioProvider        func(*audio.Config) (audio.Provider, error)
}

// Config holds GUI application configuration
type Config struct {
	OutputDir   string
	AudioFormat string
	// AudioProvider selects the TTS backend used by the GUI.
	AudioProvider string
	ImageProvider string
	OpenAIKey     string
	GoogleAPIKey  string
	// NanoBananaModel selects the Gemini image model for Nano Banana generation.
	NanoBananaModel string
	// NanoBananaTextModel selects the Gemini text model for Nano Banana prompt generation.
	NanoBananaTextModel string
	// GeminiTTSModel selects the Gemini TTS model when Gemini audio is active.
	GeminiTTSModel string
	// GeminiVoice selects a specific Gemini voice; empty picks a random Gemini voice.
	GeminiVoice         string
	TranslationProvider translation.Provider
	PhoneticProvider    phonetic.Provider
	AutoPlay            bool // Whether to automatically play audio when generated or navigated to

	// Injectable dependencies — when non-nil, New() uses them directly instead of
	// constructing new instances from the provider/key fields above.
	PhoneticFetcher *phonetic.Fetcher
	Translator      *translation.Translator
}

const (
	imageProviderOpenAI     = "openai"
	imageProviderNanoBanana = "nanobanana"
)

// DefaultConfig returns default GUI configuration
func DefaultConfig() *Config {
	homeDir, err := appconfig.HomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
	// Use XDG Base Directory specification for state data
	outputDir := filepath.Join(homeDir, ".local", "state", "totalrecall", "cards")
	audioDefaults := audio.DefaultProviderConfig()

	return &Config{
		OutputDir:           outputDir,
		AudioFormat:         audioDefaults.OutputFormat,
		AudioProvider:       audioDefaults.Provider,
		NanoBananaModel:     image.DefaultNanoBananaModel,
		NanoBananaTextModel: image.DefaultNanoBananaTextModel,
		GeminiTTSModel:      audioDefaults.GeminiTTSModel,
		ImageProvider:       imageProviderNanoBanana,
		TranslationProvider: translation.ProviderGemini,
		PhoneticProvider:    phonetic.ProviderGemini,
		AutoPlay:            true, // Auto-play enabled by default
	}
}

// New creates a new GUI application
func New(config *Config) *Application {
	if config == nil {
		config = DefaultConfig()
	} else {
		// Fill in missing fields with defaults
		defaults := DefaultConfig()
		if config.AudioProvider == "" {
			config.AudioProvider = defaults.AudioProvider
		}
		if config.OutputDir == "" {
			config.OutputDir = defaults.OutputDir
		}
		if config.AudioFormat == "" {
			if strings.EqualFold(config.AudioProvider, "gemini") {
				config.AudioFormat = defaults.AudioFormat
			} else {
				config.AudioFormat = "mp3"
			}
		}
		if config.ImageProvider == "" {
			config.ImageProvider = defaults.ImageProvider
		}
		if config.NanoBananaModel == "" {
			config.NanoBananaModel = defaults.NanoBananaModel
		}
		if config.NanoBananaTextModel == "" {
			config.NanoBananaTextModel = defaults.NanoBananaTextModel
		}
		if config.GeminiTTSModel == "" {
			config.GeminiTTSModel = defaults.GeminiTTSModel
		}
		// Don't override AutoPlay if it's explicitly set to false
		// (since bool zero value is false, we can't distinguish between unset and false)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create output directory %q: %v\n", config.OutputDir, err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	myApp := app.NewWithID("org.codeberg.snonux.totalrecall")
	myApp.SetIcon(GetAppIcon())

	app := &Application{
		app:              myApp,
		config:           config,
		ctx:              ctx,
		cancel:           cancel,
		savedCards:       make([]anki.Card, 0),
		cardContexts:     make(map[string]context.CancelFunc),
		activeOperations: make(map[string]int),
		autoPlayEnabled:  config.AutoPlay, // Use config setting

		// Production defaults for factory functions; replaced in tests.
		newOpenAIImageClient:    func(c *image.OpenAIConfig) promptAwareImageClient { return image.NewOpenAIClient(c) },
		newNanoBananaImageClient: func(c *image.NanoBananaConfig) promptAwareImageClient { return image.NewNanoBananaClient(c) },
		newAudioProvider:        audio.NewProvider,
	}

	// Initialize the word processing queue
	app.queue = NewWordQueue(ctx)
	app.queue.SetCallbacks(app.onQueueStatusUpdate, app.onJobComplete)

	// Set up audio configuration
	app.audioConfig = audioConfigForApp(config)

	// Use injected phonetic fetcher when provided; otherwise construct from config fields.
	if config.PhoneticFetcher != nil {
		app.phoneticFetcher = config.PhoneticFetcher
	} else {
		app.phoneticFetcher = phonetic.NewFetcher(&phonetic.Config{
			Provider:     config.PhoneticProvider,
			OpenAIKey:    config.OpenAIKey,
			GoogleAPIKey: config.GoogleAPIKey,
		})
	}

	// Use injected translator when provided; otherwise construct from config fields.
	if config.Translator != nil {
		app.translator = config.Translator
	} else {
		app.translator = translation.NewTranslator(translationConfigForApp(config))
	}

	app.setupUI()

	// Scan existing words in output directory
	app.scanExistingWords()

	// Update initial queue status
	app.updateQueueStatus()

	return app
}

// translationConfigForApp normalizes the GUI translation settings.
// The GUI follows the shared translator defaults unless a provider is
// explicitly selected by the caller.
func translationConfigForApp(config *Config) *translation.Config {
	if config == nil {
		config = DefaultConfig()
	}

	provider := config.TranslationProvider
	if provider == "" {
		provider = translation.ProviderGemini
	}

	return &translation.Config{
		Provider:     provider,
		OpenAIKey:    config.OpenAIKey,
		GoogleAPIKey: config.GoogleAPIKey,
	}
}

// audioConfigForApp normalizes the GUI audio settings using the shared audio defaults.
func audioConfigForApp(config *Config) *audio.Config {
	if config == nil {
		config = DefaultConfig()
	}

	defaults := audio.DefaultProviderConfig()
	provider := strings.ToLower(strings.TrimSpace(config.AudioProvider))
	if provider == "" {
		provider = defaults.Provider
	}
	outputFormat := strings.TrimSpace(config.AudioFormat)
	if outputFormat == "" {
		if provider == "gemini" {
			outputFormat = defaults.OutputFormat
		} else {
			outputFormat = "mp3"
		}
	}

	audioConfig := &audio.Config{
		Provider:          provider,
		OutputDir:         config.OutputDir,
		OutputFormat:      outputFormat,
		OpenAIKey:         config.OpenAIKey,
		GoogleAPIKey:      config.GoogleAPIKey,
		OpenAIModel:       defaults.OpenAIModel,
		OpenAIVoice:       defaults.OpenAIVoice,
		OpenAISpeed:       defaults.OpenAISpeed,
		OpenAIInstruction: defaults.OpenAIInstruction,
		GeminiTTSModel:    defaults.GeminiTTSModel,
		GeminiVoice:       config.GeminiVoice,
		GeminiSpeed:       defaults.GeminiSpeed,
	}

	if config.GeminiTTSModel != "" {
		audioConfig.GeminiTTSModel = config.GeminiTTSModel
	}

	return audioConfig
}

// setupUI creates the main user interface
func (a *Application) setupUI() {
	a.window = a.app.NewWindow("TotalRecall")
	a.window.SetIcon(GetAppIcon())
	a.window.Resize(fyne.NewSize(880, 770))

	// Create input section with navigation
	a.wordInput = NewCustomEntry()
	a.wordInput.SetPlaceHolder("Bulgarian word...")
	a.wordInput.OnSubmitted = func(string) {
		a.onSubmit()
		// Remove focus from input field after submit
		a.window.Canvas().Unfocus()
	}
	// Set escape handler to unfocus
	a.wordInput.SetOnEscape(func() {
		a.window.Canvas().Unfocus()
	})
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
	a.translationEntry = NewCustomEntry()
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
	// Set escape handler to unfocus
	a.translationEntry.SetOnEscape(func() {
		a.window.Canvas().Unfocus()
	})

	// Create card type selector
	a.cardTypeSelect = widget.NewSelect([]string{"English → Bulgarian", "Bulgarian → Bulgarian"}, func(selected string) {
		if selected == "Bulgarian → Bulgarian" {
			a.currentCardType = "bg-bg"
			a.translationEntry.SetPlaceHolder("Bulgarian definition...")
		} else {
			a.currentCardType = "en-bg"
			a.translationEntry.SetPlaceHolder("English translation...")
		}
	})
	a.cardTypeSelect.SetSelected("English → Bulgarian")
	a.currentCardType = "en-bg"

	// Create navigation buttons (tooltips will be set after tooltip layer is created)
	a.submitButton = ttwidget.NewButton("", a.onSubmit)
	a.submitButton.Icon = theme.ConfirmIcon()

	a.prevWordBtn = ttwidget.NewButton("", a.onPrevWord)
	a.prevWordBtn.Icon = theme.NavigateBackIcon()

	a.nextWordBtn = ttwidget.NewButton("", a.onNextWord)
	a.nextWordBtn.Icon = theme.NavigateNextIcon()

	// Create a grid layout for inputs with card type selector
	inputGrid := container.New(layout.NewGridLayout(3),
		a.wordInput,
		a.translationEntry,
		a.cardTypeSelect,
	)

	inputSection := container.NewBorder(
		nil, nil,
		nil,
		a.submitButton,
		inputGrid,
	)

	// Create display section
	a.imageDisplay = NewImageDisplay()
	a.audioPlayer = NewAudioPlayer()
	a.audioPlayer.SetAutoPlayEnabled(&a.autoPlayEnabled)

	// Create image prompt entry with custom escape handling
	a.imagePromptEntry = NewCustomMultiLineEntry()
	a.imagePromptEntry.SetPlaceHolder("Custom image prompt (optional)... Press Escape to exit field")
	a.imagePromptEntry.Wrapping = fyne.TextWrapWord // Enable word wrapping
	a.imagePromptEntry.OnChanged = func(text string) {
		// Save the image prompt immediately when changed
		a.saveImagePrompt()
	}
	// Set escape handler to unfocus
	a.imagePromptEntry.SetOnEscape(func() {
		a.window.Canvas().Unfocus()
	}) // Create container for image and prompt with proper sizing
	promptContainer := container.NewBorder(
		widget.NewLabel("Image prompt:"),
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

	// Create log viewer
	a.logViewer = NewLogViewer()
	a.logViewer.StartCapture() // Start capturing stdout/stderr

	// Create a container for log viewer and audio player
	audioLogSection := container.NewVSplit(
		a.logViewer,
		a.audioPlayer,
	)
	audioLogSection.SetOffset(0.7) // Give more space to log viewer (70/30 split)

	displaySection := container.NewBorder(
		nil,
		audioLogSection,
		nil, nil,
		imageSection,
	)

	// Create action buttons (tooltips will be set after tooltip layer is created)
	a.keepButton = ttwidget.NewButtonWithIcon("", theme.DocumentCreateIcon(), a.onKeepAndContinue)

	a.regenerateImageBtn = ttwidget.NewButtonWithIcon("", theme.ColorPaletteIcon(), a.onRegenerateImage)

	a.regenerateRandomImageBtn = ttwidget.NewButtonWithIcon("", theme.ViewRefreshIcon(), a.onRegenerateRandomImage)

	a.regenerateAudioBtn = ttwidget.NewButtonWithIcon("", theme.MediaRecordIcon(), a.onRegenerateAudio)

	a.regenerateAllBtn = ttwidget.NewButtonWithIcon("", theme.ViewFullScreenIcon(), a.onRegenerateAll)

	a.deleteButton = ttwidget.NewButtonWithIcon("", theme.DeleteIcon(), a.onDelete)
	a.deleteButton.Importance = widget.DangerImportance

	// Initially disable action buttons
	a.setActionButtonsEnabled(false)
	// But keep delete button enabled for cancelling operations
	a.deleteButton.Enable()

	// Create export, archive and help buttons for toolbar
	exportButton := ttwidget.NewButtonWithIcon("", theme.UploadIcon(), a.onExportToAnki)
	archiveButton := ttwidget.NewButtonWithIcon("", theme.FolderOpenIcon(), a.onArchive)
	helpButton := ttwidget.NewButtonWithIcon("", theme.HelpIcon(), a.onShowHotkeys)

	// Create toolbar with navigation buttons first, then action buttons
	toolbar := container.NewHBox(
		a.prevWordBtn,
		a.nextWordBtn,
		widget.NewSeparator(),
		a.keepButton,
		a.deleteButton,
		widget.NewSeparator(),
		a.regenerateImageBtn,
		a.regenerateRandomImageBtn,
		a.regenerateAudioBtn,
		a.regenerateAllBtn,
		widget.NewSeparator(),
		exportButton,
		archiveButton,
		helpButton,
	)

	// Create status section
	a.statusLabel = widget.NewLabel("Ready")
	a.queueStatusLabel = widget.NewLabel("Queue: Empty")
	a.queueStatusLabel.TextStyle = fyne.TextStyle{Italic: true}

	// Create version label
	versionLabel := widget.NewLabel(fmt.Sprintf("v%s", internal.Version))
	versionLabel.TextStyle = fyne.TextStyle{Italic: true}
	versionLabel.Alignment = fyne.TextAlignTrailing

	statusSection := container.NewBorder(
		nil, nil, nil, versionLabel,
		container.NewVBox(
			a.statusLabel,
			widget.NewSeparator(),
			a.queueStatusLabel,
		),
	)

	// No menu needed - all functions are in the toolbar

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

	// Now that tooltip layer is created, set all tooltips
	a.setupTooltips()

	// Set tooltips for export, archive and help buttons after the tooltip layer
	// has had time to initialize. AfterFunc avoids blocking a goroutine.
	time.AfterFunc(500*time.Millisecond, func() {
		fyne.Do(func() {
			if exportButton != nil {
				exportButton.SetToolTip("Export to Anki (x)")
			}
			if archiveButton != nil {
				archiveButton.SetToolTip("Archive all cards (v)")
			}
			if helpButton != nil {
				helpButton.SetToolTip("Show hotkeys (?)")
			}
		})
	})

	a.window.SetOnClosed(func() {
		// Stop file check ticker
		if a.fileCheckTicker != nil {
			a.fileCheckTicker.Stop()
		}
		// Restore stdio streams and close capture pipes.
		if a.logViewer != nil {
			a.logViewer.StopCapture()
		}
		// Cancel any ongoing operations
		if a.cancel != nil {
			a.cancel()
		}
		// Wait for all goroutines to finish with timeout
		done := make(chan struct{})
		go func() {
			a.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// All goroutines finished
		case <-time.After(2 * time.Second):
			// Timeout after 2 seconds
			fmt.Println("Warning: Some operations did not complete before window close")
		}

		// Close the application
		a.app.Quit()
	})

	// Set up keyboard shortcuts
	a.setupKeyboardShortcuts()
}

// Run starts the GUI application
func (a *Application) Run() {
	// Don't focus any input field on startup - let user choose
	a.window.ShowAndRun()
}

// onSubmit handles word submission
func (a *Application) onSubmit() {
	bulgarianText := strings.TrimSpace(a.wordInput.Text)
	secondaryText := strings.TrimSpace(a.translationEntry.Text)
	isBgBg := a.currentCardType == "bg-bg"

	// Determine which word to process and if translation is needed
	var wordToProcess string
	var needsTranslation bool
	var translationDirection string

	if isBgBg {
		// Bulgarian-Bulgarian mode: both fields should be Bulgarian
		if bulgarianText == "" {
			return
		}
		wordToProcess = bulgarianText
		needsTranslation = false
		a.currentTranslation = secondaryText
	} else if bulgarianText != "" && secondaryText != "" {
		// Both provided - use Bulgarian as primary, no translation needed
		wordToProcess = bulgarianText
		needsTranslation = false
		a.currentTranslation = secondaryText
	} else if bulgarianText != "" && secondaryText == "" {
		// Only Bulgarian provided - translate to English
		wordToProcess = bulgarianText
		needsTranslation = true
		translationDirection = "bg-to-en"
	} else if bulgarianText == "" && secondaryText != "" {
		// Only English provided - translate to Bulgarian
		needsTranslation = true
		translationDirection = "en-to-bg"
	} else {
		return
	}

	// Handle translation first if needed.
	switch translationDirection {
	case "en-to-bg":
		a.updateStatus(fmt.Sprintf("Translating '%s' to Bulgarian...", secondaryText))
		bulgarian, err := a.translateEnglishToBulgarian(secondaryText)
		if err != nil {
			dialog.ShowError(fmt.Errorf("translation failed: %w", err), a.window)
			return
		}
		wordToProcess = bulgarian
		a.wordInput.SetText(bulgarian)
		a.currentTranslation = secondaryText
		a.currentWord = bulgarian
		a.saveTranslation()
		needsTranslation = false
	case "bg-to-en":
		a.updateStatus(fmt.Sprintf("Translating '%s' to English...", bulgarianText))
		english, err := a.translateWord(bulgarianText)
		if err != nil {
			dialog.ShowError(fmt.Errorf("translation failed: %w", err), a.window)
			return
		}
		a.currentTranslation = english
		a.translationEntry.SetText(english)
		needsTranslation = false
		a.saveTranslation()
	}

	// Validate Bulgarian text
	if err := audio.ValidateBulgarianText(wordToProcess); err != nil {
		dialog.ShowError(err, a.window)
		return
	}

	// For bg-bg cards, also validate the back text
	if isBgBg && secondaryText != "" {
		if err := audio.ValidateBulgarianText(secondaryText); err != nil {
			dialog.ShowError(fmt.Errorf("invalid back text: %w", err), a.window)
			return
		}
	}

	// Get custom prompt from the UI
	customPrompt := a.imagePromptEntry.Text

	// Add word to processing queue with custom prompt
	job := a.queue.AddWordWithPrompt(wordToProcess, customPrompt)

	// Store whether translation is needed and the translation if already provided
	job.NeedsTranslation = needsTranslation
	job.CardType = a.currentCardType
	if a.currentTranslation != "" {
		job.Translation = a.currentTranslation
	}

	// Update status to show word was queued
	a.updateStatus(fmt.Sprintf("Added '%s' to queue (Job #%d)", wordToProcess, job.ID))

	// Update queue status immediately
	a.updateQueueStatus()

	// Start processing if not already processing
	a.processNextInQueue()
}

// generateMaterials generates all materials for a word (used by regenerate functions)
func (a *Application) generateMaterials(word string) {
	// Get or create context for this card
	cardCtx, _ := a.getOrCreateCardContext(word)

	// Ensure card directory exists
	cardDir, err := a.ensureCardDirectory(word)
	if err != nil {
		fyne.Do(func() {
			a.showError(fmt.Errorf("failed to create card directory: %w", err))
			a.setUIEnabled(true)
		})
		return
	}
	// Check if we already have a translation
	if a.currentTranslation == "" {
		// Translate word
		fyne.Do(func() {
			a.updateStatus("Translating...")
		})
		translation, err := a.translateWord(word)
		if err != nil {
			fyne.Do(func() {
				a.showError(fmt.Errorf("translation failed: %w", err))
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

		// Save translation to disk using the pre-determined directory
		if translation != "" {
			translationFile := filepath.Join(cardDir, "translation.txt")
			content := fmt.Sprintf("%s = %s\n", word, translation)
			if err := os.WriteFile(translationFile, []byte(content), 0644); err != nil {
				fmt.Printf("Warning: Failed to save translation for '%s': %v\n", word, err)
			}
		}
	}
	// Create channels for parallel operations
	type audioResult struct {
		file string
		err  error
	}
	type imageResult struct {
		file string
		err  error
	}
	type phoneticResult struct {
		info string
		err  error
	}

	audioChan := make(chan audioResult, 1)
	imageChan := make(chan imageResult, 1)
	phoneticChan := make(chan phoneticResult, 1)

	// Get custom prompt and translation before starting goroutines
	customPrompt := a.imagePromptEntry.Text
	translation := a.currentTranslation
	if translation == "" {
		// Use the text from translationEntry if currentTranslation is not set
		translation = strings.TrimSpace(a.translationEntry.Text)
	}

	// Update status to show parallel processing
	fyne.Do(func() {
		a.updateStatus("Generating audio, images, and phonetics in parallel...")
	})

	// Start all three operations in parallel

	// 1. Audio generation
	go func() {
		a.startOperation(word)     // Track operation start
		defer a.endOperation(word) // Track operation end

		fyne.Do(func() {
			a.incrementProcessing() // Audio processing starts
		})

		audioFile, err := a.generateAudio(cardCtx, word, cardDir)
		a.decrementProcessing() // Audio processing ends

		audioChan <- audioResult{file: audioFile, err: err}
	}()

	// 2. Image generation
	go func() {
		a.startOperation(word)     // Track operation start
		defer a.endOperation(word) // Track operation end

		fyne.Do(func() {
			a.incrementProcessing() // Image processing starts
			// Show generating status if this is still the current word
			a.mu.Lock()
			if a.currentWord == word {
				a.imageDisplay.SetGenerating()
			}
			a.mu.Unlock()
		})

		imageFile, err := a.generateImagesWithPrompt(cardCtx, word, customPrompt, translation, cardDir)
		a.decrementProcessing() // Image processing ends

		imageChan <- imageResult{file: imageFile, err: err}
	}()

	// 3. Phonetic information fetching
	go func() {
		a.startOperation(word)     // Track operation start
		defer a.endOperation(word) // Track operation end

		fyne.Do(func() {
			a.incrementProcessing() // Phonetic processing starts
		})

		phoneticInfo, err := a.getPhoneticInfo(word)
		if err != nil {
			// Log error but don't fail - phonetic info is optional
			fmt.Printf("Warning: Failed to get phonetic info: %v\n", err)
			phoneticInfo = "Failed to fetch phonetic information"
		} else {
			fmt.Printf("Successfully fetched phonetic info for '%s': %s\n", word, phoneticInfo)
		}

		// Save phonetic info to disk using the pre-determined directory
		if phoneticInfo != "" && phoneticInfo != "Failed to fetch phonetic information" {
			phoneticFile := filepath.Join(cardDir, "phonetic.txt")
			if err := os.WriteFile(phoneticFile, []byte(phoneticInfo), 0644); err != nil {
				fmt.Printf("Warning: Failed to save phonetic info for '%s': %v\n", word, err)
			}
		}
		// Update UI immediately with phonetic info if this is still the current word
		if phoneticInfo != "" && phoneticInfo != "Failed to fetch phonetic information" {
			a.mu.Lock()
			shouldUpdate := a.currentWord == word
			if shouldUpdate {
				a.currentPhonetic = phoneticInfo
			}
			a.mu.Unlock()

			if shouldUpdate {
				fmt.Printf("Updating phonetic display immediately for word '%s': %s\n", word, phoneticInfo)
				fyne.Do(func() {
					// Display the IPA directly
					a.audioPlayer.SetPhonetic(phoneticInfo)
				})
			} else {
				fmt.Printf("Not updating phonetic display immediately - word mismatch (current: %s, this: %s)\n", a.currentWord, word)
			}
		}

		a.decrementProcessing() // Phonetic processing ends
		phoneticChan <- phoneticResult{info: phoneticInfo, err: nil}
	}()

	// Wait for all operations to complete
	var hasError bool

	// Collect audio result
	audioRes := <-audioChan
	if audioRes.err != nil {
		fyne.Do(func() {
			a.showError(fmt.Errorf("audio generation failed: %w", audioRes.err))
		})
		hasError = true
	} else {
		// Only update UI if this word is still the current word
		a.mu.Lock()
		if a.currentWord == word {
			a.currentAudioFile = audioRes.file
			audioFile := audioRes.file
			a.mu.Unlock()
			fyne.Do(func() {
				// Double-check inside the UI update that we're still on the same word
				a.mu.Lock()
				if a.currentWord == word {
					a.audioPlayer.SetAudioFile(audioFile)
				}
				a.mu.Unlock()
			})
		} else {
			a.mu.Unlock()
		}
	}

	// Collect image result
	imageRes := <-imageChan
	if imageRes.err != nil {
		fyne.Do(func() {
			a.showError(fmt.Errorf("image download failed: %w", imageRes.err))
		})
		hasError = true
	} else if imageRes.file != "" {
		// Only update UI if this word is still the current word
		a.mu.Lock()
		if a.currentWord == word {
			a.currentImage = imageRes.file
			imageFile := imageRes.file
			a.mu.Unlock()
			fyne.Do(func() {
				// Double-check inside the UI update that we're still on the same word
				a.mu.Lock()
				if a.currentWord == word {
					a.imageDisplay.SetImages([]string{imageFile})
				}
				a.mu.Unlock()
			})
		} else {
			a.mu.Unlock()
		}
	}

	// Collect phonetic result (UI already updated in the goroutine)
	<-phoneticChan
	// The phonetic info has already been displayed in the UI immediately when fetched

	// If any critical operation failed, re-enable UI
	if hasError {
		fyne.Do(func() {
			a.setUIEnabled(true)
		})
		return
	}

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
	a.currentPhonetic = ""
	a.mu.Unlock()

	// Don't focus any input field - let user choose what to focus

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

	// Show generating status immediately
	a.imageDisplay.SetGenerating()

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

		// Get or create context for this card
		cardCtx, _ := a.getOrCreateCardContext(wordForGeneration)

		// Ensure card directory exists
		cardDir, err := a.ensureCardDirectory(wordForGeneration)
		if err != nil {
			fyne.Do(func() {
				a.showError(fmt.Errorf("failed to create card directory: %w", err))
			})
			return
		}

		imageFile, err := a.generateImagesWithPrompt(cardCtx, wordForGeneration, customPrompt, translation, cardDir)
		if err != nil {
			fyne.Do(func() {
				a.showError(fmt.Errorf("image regeneration failed: %w", err))
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

	// Show generating status immediately
	a.imageDisplay.SetGenerating()

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

		// Get or create context for this card
		cardCtx, _ := a.getOrCreateCardContext(wordForGeneration)

		// Ensure card directory exists
		cardDir, err := a.ensureCardDirectory(wordForGeneration)
		if err != nil {
			fyne.Do(func() {
				a.showError(fmt.Errorf("failed to create card directory: %w", err))
			})
			return
		}

		imageFile, err := a.generateImagesWithPrompt(cardCtx, wordForGeneration, customPrompt, translation, cardDir)
		if err != nil {
			fyne.Do(func() {
				a.showError(fmt.Errorf("random image generation failed: %w", err))
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

// onRegenerateAudio regenerates front audio (or single audio for en-bg cards)
func (a *Application) onRegenerateAudio() {
	// Only disable the audio-related buttons
	a.regenerateAudioBtn.Disable()
	a.regenerateAllBtn.Disable()

	isBgBg := a.currentCardType == "bg-bg"
	if isBgBg {
		a.showProgress("Regenerating front audio...")
	} else {
		a.showProgress("Regenerating audio...")
	}

	a.incrementProcessing() // Audio processing starts

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer a.decrementProcessing()

		// Store the word we're generating for
		wordForGeneration := a.currentWord

		a.startOperation(wordForGeneration)
		defer a.endOperation(wordForGeneration)

		// Get or create context for this card
		cardCtx, _ := a.getOrCreateCardContext(wordForGeneration)

		// Ensure card directory exists
		cardDir, err := a.ensureCardDirectory(wordForGeneration)
		if err != nil {
			fyne.Do(func() {
				a.showError(fmt.Errorf("failed to create card directory: %w", err))
			})
			return
		}

		if isBgBg {
			// For bg-bg cards, regenerate only front audio
			audioFile, err := a.generateAudioFront(cardCtx, wordForGeneration, cardDir)
			if err != nil {
				fyne.Do(func() {
					a.showError(fmt.Errorf("front audio regeneration failed: %w", err))
				})
			} else {
				a.mu.Lock()
				if a.currentWord == wordForGeneration {
					a.currentAudioFile = audioFile
					a.mu.Unlock()
					fyne.Do(func() {
						a.mu.Lock()
						if a.currentWord == wordForGeneration {
							// Set front audio WITHOUT auto-play initially
							a.audioPlayer.SetAudioFileNoAutoPlay(audioFile)
							// Then explicitly play ONLY the front audio
							a.audioPlayer.Play()
						}
						a.mu.Unlock()
					})
				} else {
					a.mu.Unlock()
				}
			}
		} else {
			// For en-bg cards, regenerate single audio file
			audioFile, err := a.generateAudio(cardCtx, wordForGeneration, cardDir)
			if err != nil {
				fyne.Do(func() {
					a.showError(fmt.Errorf("audio regeneration failed: %w", err))
				})
			} else {
				a.mu.Lock()
				if a.currentWord == wordForGeneration {
					a.currentAudioFile = audioFile
					a.mu.Unlock()
					fyne.Do(func() {
						a.mu.Lock()
						if a.currentWord == wordForGeneration {
							a.audioPlayer.SetAudioFile(audioFile)
						}
						a.mu.Unlock()
					})
				} else {
					a.mu.Unlock()
				}
			}
		}

		fyne.Do(func() {
			a.hideProgress()
			a.regenerateAudioBtn.Enable()
			a.regenerateAllBtn.Enable()
		})
	}()
}

// onRegenerateBackAudio regenerates back audio for bg-bg cards
func (a *Application) onRegenerateBackAudio() {
	if a.currentCardType != "bg-bg" {
		return
	}

	a.regenerateAudioBtn.Disable()
	a.regenerateAllBtn.Disable()
	a.showProgress("Regenerating back audio...")

	a.incrementProcessing()

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer a.decrementProcessing()

		// CRITICAL: Get translation from state variable first
		translation := a.currentTranslation

		if translation == "" {
			translation = strings.TrimSpace(a.translationEntry.Text)
		}

		wordForGeneration := a.currentWord

		// For back audio, we need to use the main context, not create a new card context
		// because the front audio regeneration already has an active context for this word.
		// Creating a new context would cancel the front audio operation.

		a.startOperation(wordForGeneration)
		defer a.endOperation(wordForGeneration)

		cardDir, err := a.ensureCardDirectory(wordForGeneration)
		if err != nil {
			fyne.Do(func() {
				a.showError(fmt.Errorf("failed to create card directory: %w", err))
			})
			return
		}

		audioFile, err := a.generateAudioBack(a.ctx, translation, cardDir)

		if err != nil {
			fyne.Do(func() {
				a.showError(fmt.Errorf("back audio regeneration failed: %w", err))
			})
		} else {
			a.mu.Lock()
			if a.currentWord == wordForGeneration {
				a.currentAudioFileBack = audioFile
				a.mu.Unlock()
				fyne.Do(func() {
					a.mu.Lock()
					if a.currentWord == wordForGeneration {
						a.audioPlayer.SetBackAudioFile(audioFile)
						// Auto-play the regenerated back audio
						a.audioPlayer.PlayBack()
					}
					a.mu.Unlock()
				})
			} else {
				a.mu.Unlock()
			}
		}

		fyne.Do(func() {
			a.hideProgress()
			a.regenerateAudioBtn.Enable()
			a.regenerateAllBtn.Enable()
		})
	}()
}

// onRegenerateAll regenerates both audio and images
func (a *Application) onRegenerateAll() {
	a.setUIEnabled(false)
	a.showProgress("Regenerating all materials...")

	// Show generating status immediately
	a.imageDisplay.SetGenerating()

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

	// Export directory selection
	homeDir, err := appconfig.HomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
	defaultExportDir := homeDir // Changed from Downloads to home directory
	selectedDir := defaultExportDir

	dirLabel := widget.NewLabel(selectedDir)

	dirButton := widget.NewButton("Browse...", func() {
		folderDialog := dialog.NewFolderOpen(func(dir fyne.ListableURI, err error) {
			if err != nil || dir == nil {
				return
			}
			selectedDir = dir.Path()
			dirLabel.SetText(selectedDir)
		}, a.window)

		// Try to set initial directory
		if uri, err := storage.ParseURI("file://" + selectedDir); err == nil {
			if listableURI, ok := uri.(fyne.ListableURI); ok {
				folderDialog.SetLocation(listableURI)
			}
		}

		folderDialog.Show()
	})

	dirContainer := container.NewBorder(nil, nil, nil, dirButton, dirLabel)

	content := container.NewVBox(
		widget.NewLabel("Export Format:"),
		formatSelect,
		widget.NewSeparator(),
		widget.NewLabel("Deck Name:"),
		deckNameEntry,
		widget.NewSeparator(),
		widget.NewLabel("Export Directory:"),
		dirContainer,
		widget.NewLabel(""),
		widget.NewRichTextFromMarkdown("**APKG**: Complete package with media files included\n**CSV**: Text only, requires manual media copy"),
	)

	// Store export dialog state
	exportDialogOpen := true

	customDialog := dialog.NewCustomConfirm("Export to Anki", "Export (e)", "Cancel (c/Esc)", content, func(export bool) {
		exportDialogOpen = false
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
			outputPath = filepath.Join(selectedDir, filename)

			// Generate APKG from all cards in directory
			gen := anki.NewGenerator(nil)

			// Load all cards from the anki_cards directory
			if err := gen.GenerateFromDirectory(a.config.OutputDir); err != nil {
				dialog.ShowError(fmt.Errorf("failed to load cards: %w", err), a.window)
				return
			}

			if err := gen.GenerateAPKG(outputPath, deckName); err != nil {
				dialog.ShowError(fmt.Errorf("failed to generate APKG: %w", err), a.window)
				return
			}

			// Get actual card count
			total, withAudio, withImages := gen.Stats()

			// Update status bar instead of showing dialog
			a.updateStatus(fmt.Sprintf("Exported %d cards to %s (%d with audio, %d with images)",
				total, selectedDir, withAudio, withImages))
		} else {
			filename = "anki_import.csv"
			outputPath = filepath.Join(selectedDir, filename)

			// Generate CSV from all cards in directory
			gen := anki.NewGenerator(&anki.GeneratorOptions{
				OutputPath:     outputPath,
				MediaFolder:    a.config.OutputDir,
				IncludeHeaders: true,
				AudioFormat:    a.config.AudioFormat,
			})

			// Load all cards from the anki_cards directory
			if err := gen.GenerateFromDirectory(a.config.OutputDir); err != nil {
				dialog.ShowError(fmt.Errorf("failed to load cards: %w", err), a.window)
				return
			}

			if err := gen.GenerateCSV(); err != nil {
				dialog.ShowError(fmt.Errorf("failed to generate CSV: %w", err), a.window)
				return
			}

			// Get actual card count
			total, withAudio, withImages := gen.Stats()

			// Update status bar instead of showing dialog
			a.updateStatus(fmt.Sprintf("Exported %d cards to %s (%d with audio, %d with images)",
				total, selectedDir, withAudio, withImages))
		}
	}, a.window)

	// Store original keyboard handlers
	originalRuneHandler := a.window.Canvas().OnTypedRune()
	originalKeyHandler := a.window.Canvas().OnTypedKey()

	// Add keyboard shortcuts for the export dialog (both Latin and Cyrillic)
	a.window.Canvas().SetOnTypedRune(func(r rune) {
		if exportDialogOpen {
			switch r {
			case 'e', 'E', 'е', 'Е':
				// Trigger export
				customDialog.Hide()
				exportDialogOpen = false
				customDialog.Confirm()
			case 'c', 'C', 'ц', 'Ц':
				// Cancel dialog
				customDialog.Hide()
				exportDialogOpen = false
			}
			return
		}
		// Call original handler if it exists
		if originalRuneHandler != nil {
			originalRuneHandler(r)
		}
	})

	// Add ESC key handler
	a.window.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if exportDialogOpen && ev.Name == fyne.KeyEscape {
			customDialog.Hide()
			exportDialogOpen = false
			return
		}
		// Call original handler if it exists
		if originalKeyHandler != nil {
			originalKeyHandler(ev)
		}
	})

	// Restore original handlers when dialog closes
	customDialog.SetOnClosed(func() {
		exportDialogOpen = false
		// Restore original keyboard handlers
		a.window.Canvas().SetOnTypedRune(originalRuneHandler)
		a.window.Canvas().SetOnTypedKey(originalKeyHandler)
	})

	customDialog.Resize(fyne.NewSize(400, 300))
	customDialog.Show()
}

// onArchive archives the current cards directory
func (a *Application) onArchive() {
	// Function to perform the archive
	performArchive := func() {
		// Get the cards directory path
		home, err := appconfig.HomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		}
		cardsDir := filepath.Join(home, ".local", "state", "totalrecall", "cards")

		// Archive the cards
		if err := archive.ArchiveCards(cardsDir); err != nil {
			dialog.ShowError(err, a.window)
			return
		}

		// Clear the saved cards list
		a.mu.Lock()
		a.savedCards = []anki.Card{}
		a.existingWords = []string{}
		a.mu.Unlock()

		// Update status
		a.updateStatus("Cards archived successfully")

		// Refresh the current word display
		a.scanExistingWords()
		if a.currentWord != "" {
			a.loadExistingFiles(a.currentWord)
		}
	}

	// Create confirmation dialog
	confirmDialog := dialog.NewConfirm("Archive Cards",
		"Are you sure you want to archive all existing cards?\n\nThis will move the cards directory to:\n~/.local/state/totalrecall/archive/cards-TIMESTAMP",
		func(confirmed bool) {
			if confirmed {
				performArchive()
			}
		},
		a.window,
	)

	// Track if we're in archive confirmation mode
	archiveConfirming := true

	// Save original key handlers
	oldKeyHandler := a.window.Canvas().OnTypedKey()
	oldRuneHandler := a.window.Canvas().OnTypedRune()

	// Handle both Latin and Cyrillic keys
	a.window.Canvas().SetOnTypedRune(func(r rune) {
		if archiveConfirming {
			switch r {
			case 'y', 'Y', 'ъ', 'Ъ':
				confirmDialog.Hide()
				archiveConfirming = false
				performArchive()
				// Restore original handlers
				a.window.Canvas().SetOnTypedKey(oldKeyHandler)
				a.window.Canvas().SetOnTypedRune(oldRuneHandler)
			case 'n', 'N', 'н', 'Н', 'c', 'C', 'ц', 'Ц':
				confirmDialog.Hide()
				archiveConfirming = false
				// Restore original handlers
				a.window.Canvas().SetOnTypedKey(oldKeyHandler)
				a.window.Canvas().SetOnTypedRune(oldRuneHandler)
			}
		} else if oldRuneHandler != nil {
			oldRuneHandler(r)
		}
	})

	// Handle special keys
	a.window.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if archiveConfirming {
			switch ev.Name {
			case fyne.KeyY:
				confirmDialog.Hide()
				archiveConfirming = false
				performArchive()
				// Restore original handlers
				a.window.Canvas().SetOnTypedKey(oldKeyHandler)
				a.window.Canvas().SetOnTypedRune(oldRuneHandler)
			case fyne.KeyN, fyne.KeyC, fyne.KeyEscape:
				confirmDialog.Hide()
				archiveConfirming = false
				// Restore original handlers
				a.window.Canvas().SetOnTypedKey(oldKeyHandler)
				a.window.Canvas().SetOnTypedRune(oldRuneHandler)
			}
		} else if oldKeyHandler != nil {
			oldKeyHandler(ev)
		}
	})

	// Set up dialog close handler to restore key handlers
	confirmDialog.SetOnClosed(func() {
		archiveConfirming = false
		a.window.Canvas().SetOnTypedKey(oldKeyHandler)
		a.window.Canvas().SetOnTypedRune(oldRuneHandler)
	})

	confirmDialog.Show()
}

// onShowHotkeys displays a dialog with all available keyboard shortcuts
func (a *Application) onShowHotkeys() {
	hotkeys := `[Project Page: https://codeberg.org/snonux/totalrecall](https://codeberg.org/snonux/totalrecall)

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

	content := widget.NewRichTextFromMarkdown(hotkeys)
	content.Wrapping = fyne.TextWrapWord

	// Create a container with padding to prevent text cutoff
	paddedContent := container.NewPadded(content)

	// Create a scrollable container for the content
	scroll := container.NewScroll(paddedContent)
	scroll.SetMinSize(fyne.NewSize(700, 480)) // Doubled width from 350 to 700

	// Create the dialog
	d := dialog.NewCustom("Keyboard Shortcuts", "Close", scroll, a.window)

	// Store dialog state
	dialogOpen := true

	// Store original handlers
	originalRuneHandler := a.window.Canvas().OnTypedRune()
	originalKeyHandler := a.window.Canvas().OnTypedKey()

	// Add temporary handler for 'c' to close dialog (both Latin and Cyrillic)
	a.window.Canvas().SetOnTypedRune(func(r rune) {
		if dialogOpen && (r == 'c' || r == 'C' || r == 'ц' || r == 'Ц') {
			d.Hide()
			return
		}
		// Call original handler if it exists
		if originalRuneHandler != nil {
			originalRuneHandler(r)
		}
	})

	// Add ESC key handler
	a.window.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if dialogOpen && ev.Name == fyne.KeyEscape {
			d.Hide()
			return
		}
		// Call original handler if it exists
		if originalKeyHandler != nil {
			originalKeyHandler(ev)
		}
	})

	// Show the dialog
	d.Show()

	// Restore original handlers when dialog closes
	d.SetOnClosed(func() {
		dialogOpen = false
		// Restore original keyboard shortcuts
		a.setupKeyboardShortcuts()
	})
}

// toggleAutoPlay toggles the auto-play feature on/off
func (a *Application) toggleAutoPlay() {
	a.autoPlayEnabled = !a.autoPlayEnabled

	if a.autoPlayEnabled {
		a.updateStatus("Auto-play enabled")
	} else {
		a.updateStatus("Auto-play disabled")
	}
}

// onQuitConfirm shows a confirmation dialog before quitting
func (a *Application) onQuitConfirm() {
	// Don't show if already confirming
	if a.quitConfirming {
		return
	}

	// Create confirmation dialog
	message := "Are you sure you want to quit?\n\nPress y to quit or n to cancel"
	confirmDialog := dialog.NewConfirm("Quit Application", message, func(confirm bool) {
		a.quitConfirming = false
		if confirm {
			a.window.Close()
		}
	}, a.window)

	// Set up keyboard handler for the dialog
	a.quitConfirming = true

	// Store original handlers
	oldKeyHandler := a.window.Canvas().OnTypedKey()
	oldRuneHandler := a.window.Canvas().OnTypedRune()

	// Handle both Latin and Cyrillic keys
	a.window.Canvas().SetOnTypedRune(func(r rune) {
		if a.quitConfirming {
			switch r {
			case 'y', 'Y', 'ъ', 'Ъ':
				confirmDialog.Hide()
				a.quitConfirming = false
				a.window.Close()
			case 'n', 'N', 'н', 'Н':
				confirmDialog.Hide()
				a.quitConfirming = false
				// Restore original handlers
				a.window.Canvas().SetOnTypedKey(oldKeyHandler)
				a.window.Canvas().SetOnTypedRune(oldRuneHandler)
			}
		} else if oldRuneHandler != nil {
			oldRuneHandler(r)
		}
	})

	a.window.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if a.quitConfirming {
			switch ev.Name {
			case fyne.KeyY:
				confirmDialog.Hide()
				a.quitConfirming = false
				a.window.Close()
			case fyne.KeyN, fyne.KeyEscape:
				confirmDialog.Hide()
				a.quitConfirming = false
				// Restore original handlers
				a.window.Canvas().SetOnTypedKey(oldKeyHandler)
				a.window.Canvas().SetOnTypedRune(oldRuneHandler)
			}
		} else if oldKeyHandler != nil {
			oldKeyHandler(ev)
		}
	})

	// Set dialog closed handler
	confirmDialog.SetOnClosed(func() {
		a.quitConfirming = false
		// Restore original handlers
		a.window.Canvas().SetOnTypedKey(oldKeyHandler)
		a.window.Canvas().SetOnTypedRune(oldRuneHandler)
	})

	confirmDialog.Show()
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
		// Keep delete button enabled to allow cancelling generation
		// a.deleteButton.Disable() // Don't disable this
	}
}

func (a *Application) syncCardTypeSelection(cardType internal.CardType) {
	if a.cardTypeSelect == nil {
		return
	}

	selected := "English → Bulgarian"
	if cardType.IsBgBg() {
		selected = "Bulgarian → Bulgarian"
	}

	if a.window == nil {
		a.cardTypeSelect.SetSelected(selected)
		return
	}

	fyne.Do(func() {
		a.cardTypeSelect.SetSelected(selected)
	})
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
	// Stop file check ticker when clearing UI
	if a.fileCheckTicker != nil {
		a.fileCheckTicker.Stop()
		a.fileCheckTicker = nil
	}

	a.imageDisplay.Clear()
	a.audioPlayer.Clear()
	a.currentAudioFile = ""
	a.currentAudioFileBack = ""
	a.currentImage = ""
	a.currentTranslation = ""
	a.currentCardType = ""
	// Don't clear the word input or translation entry - they should stay populated
	// Clear the image prompt entry - it will be loaded from disk if available
	a.imagePromptEntry.SetText("")
	a.audioPlayer.SetPhonetic("")
	a.currentPhonetic = ""
	a.setActionButtonsEnabled(false)
}

// setupTooltips sets up all tooltips after the tooltip layer has been created.
// AfterFunc fires after the tooltip layer is initialized without blocking a goroutine.
func (a *Application) setupTooltips() {
	time.AfterFunc(500*time.Millisecond, func() {
		fyne.Do(func() {
			// Navigation button tooltips
			if a.submitButton != nil {
				a.submitButton.SetToolTip("Generate word (g)")
			}
			if a.prevWordBtn != nil {
				a.prevWordBtn.SetToolTip("Previous word (← / h/х)")
			}
			if a.nextWordBtn != nil {
				a.nextWordBtn.SetToolTip("Next word (→ / l/л)")
			}

			// Action button tooltips
			if a.keepButton != nil {
				a.keepButton.SetToolTip("Keep card and new word (n)")
			}
			if a.regenerateImageBtn != nil {
				a.regenerateImageBtn.SetToolTip("Regenerate image (i)")
			}
			if a.regenerateRandomImageBtn != nil {
				a.regenerateRandomImageBtn.SetToolTip("Random image (m)")
			}
			if a.regenerateAudioBtn != nil {
				a.regenerateAudioBtn.SetToolTip("Regenerate audio (a/A for back)")
			}
			if a.regenerateAllBtn != nil {
				a.regenerateAllBtn.SetToolTip("Regenerate all (r)")
			}
			if a.deleteButton != nil {
				a.deleteButton.SetToolTip("Delete word (d)")
			}

			// Export and help button tooltips need to be set after creation
			// They are set in the main window setup

			// Audio player tooltips
			if a.audioPlayer != nil && a.audioPlayer.playButton != nil {
				a.audioPlayer.playButton.SetToolTip("Play audio (p/P for back)")
			}
			if a.audioPlayer != nil && a.audioPlayer.playBackButton != nil {
				a.audioPlayer.playBackButton.SetToolTip("Play back audio (P)")
			}
			if a.audioPlayer != nil && a.audioPlayer.stopButton != nil {
				a.audioPlayer.stopButton.SetToolTip("Stop audio")
			}
		})
	})
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

// getOrCreateCardContext returns a context for the given word, creating one if needed
func (a *Application) getOrCreateCardContext(word string) (context.Context, context.CancelFunc) {
	a.cardMu.Lock()
	defer a.cardMu.Unlock()

	// Check if we already have a cancel function for this word
	if cancel, exists := a.cardContexts[word]; exists {
		// Cancel the old context first
		cancel()
	}

	// Create new context for this word
	ctx, cancel := context.WithCancel(a.ctx)
	a.cardContexts[word] = cancel

	return ctx, cancel
}

// cancelCardOperations cancels all ongoing operations for a specific word
func (a *Application) cancelCardOperations(word string) {
	a.cardMu.Lock()
	defer a.cardMu.Unlock()

	if cancel, exists := a.cardContexts[word]; exists {
		cancel()
		delete(a.cardContexts, word)
	}
}

// startOperation marks the start of an operation for a word
func (a *Application) startOperation(word string) {
	a.activeOpMu.Lock()
	defer a.activeOpMu.Unlock()
	a.activeOperations[word]++
}

// endOperation marks the end of an operation for a word
func (a *Application) endOperation(word string) {
	a.activeOpMu.Lock()
	defer a.activeOpMu.Unlock()

	if count, exists := a.activeOperations[word]; exists {
		if count > 1 {
			a.activeOperations[word]--
		} else {
			delete(a.activeOperations, word)
		}
	}
}

// hasActiveOperations checks if a word has any active operations
func (a *Application) hasActiveOperations(word string) bool {
	a.activeOpMu.Lock()
	defer a.activeOpMu.Unlock()

	count, exists := a.activeOperations[word]
	return exists && count > 0
}

// processWordJob processes a single word job
func (a *Application) processWordJob(job *WordJob) {
	// Get or create context for this card
	cardCtx, _ := a.getOrCreateCardContext(job.Word)

	// Check if context is already cancelled
	select {
	case <-cardCtx.Done():
		a.queue.FailJob(job.ID, fmt.Errorf("job cancelled"))
		a.finishCurrentJob()
		return
	default:
	}

	// Ensure card directory exists upfront
	cardDir, dirErr := a.ensureCardDirectory(job.Word)
	if dirErr != nil {
		a.queue.FailJob(job.ID, fmt.Errorf("failed to create card directory: %w", dirErr))
		a.finishCurrentJob()
		return
	}

	// Determine if this is a bg-bg card
	isBgBg := job.CardType == "bg-bg"

	// Save card type; fail fast if persistence is unavailable.
	var cardTypeErr error
	if isBgBg {
		cardTypeErr = internal.SaveCardType(cardDir, internal.CardTypeBgBg)
	} else {
		cardTypeErr = internal.SaveCardType(cardDir, internal.CardTypeEnBg)
	}
	if cardTypeErr != nil {
		a.queue.FailJob(job.ID, fmt.Errorf("failed to save card type: %w", cardTypeErr))
		a.finishCurrentJob()
		return
	}

	// Handle translation
	var translation string
	var err error

	if job.NeedsTranslation && !isBgBg {
		// Translate word (only for en-bg cards)
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
		translationFile := filepath.Join(cardDir, "translation.txt")
		content := fmt.Sprintf("%s = %s\n", job.Word, translation)
		if err := os.WriteFile(translationFile, []byte(content), 0644); err != nil {
			a.queue.FailJob(job.ID, fmt.Errorf("failed to save translation: %w", err))
			a.finishCurrentJob()
			return
		}
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

	// Create channels for parallel operations
	type audioResult struct {
		file     string
		fileBack string
		err      error
	}
	type imageResult struct {
		file string
		err  error
	}
	type phoneticResult struct {
		info string
		err  error
	}

	audioChan := make(chan audioResult, 1)
	imageChan := make(chan imageResult, 1)
	phoneticChan := make(chan phoneticResult, 1)

	// Update status to show parallel processing
	fyne.Do(func() {
		a.updateStatus(fmt.Sprintf("Processing '%s' - generating audio, images, and phonetics in parallel...", job.Word))
	})

	// Start all three operations in parallel

	// 1. Audio generation
	go func() {
		a.startOperation(job.Word)
		defer a.endOperation(job.Word)

		fyne.Do(func() {
			a.incrementProcessing()
		})

		var audioFile, audioFileBack string
		var err error

		if isBgBg && translation != "" {
			// Generate audio for both sides
			audioFile, audioFileBack, err = a.generateAudioBgBg(cardCtx, job.Word, translation, cardDir)
		} else {
			audioFile, err = a.generateAudio(cardCtx, job.Word, cardDir)
		}
		a.decrementProcessing()

		audioChan <- audioResult{file: audioFile, fileBack: audioFileBack, err: err}
	}()

	// 2. Image generation (includes scene description)
	go func() {
		a.startOperation(job.Word)     // Track operation start
		defer a.endOperation(job.Word) // Track operation end

		fyne.Do(func() {
			a.incrementProcessing() // Image processing starts
			// Show generating status if this is still the current job
			a.mu.Lock()
			if a.currentJobID == job.ID {
				a.imageDisplay.SetGenerating()
			}
			a.mu.Unlock()
		})

		// Use the custom prompt from the job
		// The translation variable already contains the correct translation (either from job or translated)
		imageFile, err := a.generateImagesWithPrompt(cardCtx, job.Word, job.CustomPrompt, translation, cardDir)
		a.decrementProcessing() // Image processing ends

		imageChan <- imageResult{file: imageFile, err: err}
	}()

	// 3. Phonetic information fetching
	go func() {
		a.startOperation(job.Word)     // Track operation start
		defer a.endOperation(job.Word) // Track operation end

		fyne.Do(func() {
			a.incrementProcessing() // Phonetic processing starts
		})

		phoneticInfo, err := a.getPhoneticInfo(job.Word)
		if err != nil {
			// Log error but don't fail - phonetic info is optional
			fmt.Printf("Warning: Failed to get phonetic info: %v\n", err)
			phoneticInfo = "Failed to fetch phonetic information"
		} else {
			fmt.Printf("Successfully fetched phonetic info for '%s': %s\n", job.Word, phoneticInfo)
		}

		// Save phonetic info to disk immediately for this specific word
		if phoneticInfo != "" && phoneticInfo != "Failed to fetch phonetic information" {
			phoneticFile := filepath.Join(cardDir, "phonetic.txt")
			if err := os.WriteFile(phoneticFile, []byte(phoneticInfo), 0644); err != nil {
				fmt.Printf("Warning: Failed to save phonetic info for '%s': %v\n", job.Word, err)
			}
		}

		// Update UI immediately with phonetic info if this is still the current job
		if phoneticInfo != "" && phoneticInfo != "Failed to fetch phonetic information" {
			a.mu.Lock()
			shouldUpdate := a.currentJobID == job.ID
			if shouldUpdate {
				a.currentPhonetic = phoneticInfo
			}
			a.mu.Unlock()

			if shouldUpdate {
				fmt.Printf("Updating phonetic display immediately for job %d: %s\n", job.ID, phoneticInfo)
				fyne.Do(func() {
					// Display the IPA directly
					a.audioPlayer.SetPhonetic(phoneticInfo)
				})
			} else {
				fmt.Printf("Not updating phonetic display immediately - job mismatch (current job: %d, this job: %d)\n", a.currentJobID, job.ID)
			}
		}

		a.decrementProcessing() // Phonetic processing ends
		phoneticChan <- phoneticResult{info: phoneticInfo, err: nil}
	}()

	// Wait for all operations to complete
	var audioFile, audioFileBack, imageFile string
	var phoneticInfo string
	var hasError bool

	// Collect audio result
	audioRes := <-audioChan
	if audioRes.err != nil {
		a.queue.FailJob(job.ID, fmt.Errorf("audio generation failed: %w", audioRes.err))
		hasError = true
	} else {
		audioFile = audioRes.file
		audioFileBack = audioRes.fileBack

		// Update UI with audio immediately if this is still the current job
		a.mu.Lock()
		isCurrentJob := a.currentJobID == job.ID
		if isCurrentJob {
			a.currentAudioFile = audioFile
			a.currentAudioFileBack = audioFileBack
		}
		a.mu.Unlock()

		if isCurrentJob {
			fyne.Do(func() {
				a.mu.Lock()
				if a.currentJobID != job.ID {
					a.mu.Unlock()
					return
				}
				a.mu.Unlock()

				a.audioPlayer.SetAudioFile(audioFile)
				if isBgBg && audioFileBack != "" {
					a.audioPlayer.SetBackAudioFile(audioFileBack)
				}
				a.regenerateAudioBtn.Enable()
			})
		}
	}

	// Collect image result
	imageRes := <-imageChan
	if imageRes.err != nil {
		a.queue.FailJob(job.ID, fmt.Errorf("image download failed: %w", imageRes.err))
		hasError = true
	} else {
		imageFile = imageRes.file
	}

	// Collect phonetic result (UI already updated in the goroutine)
	phoneticRes := <-phoneticChan
	phoneticInfo = phoneticRes.info

	// If any critical operation failed, finish the job and return
	if hasError {
		a.finishCurrentJob()
		return
	}

	// Mark job as completed
	fyne.Do(func() {
		a.updateStatus(fmt.Sprintf("Finalizing '%s'...", job.Word))
	})

	a.queue.CompleteJob(job.ID, translation, audioFile, audioFileBack, imageFile)

	// Update UI with results if this is still the current job
	a.mu.Lock()
	isCurrentJob := a.currentJobID == job.ID
	if isCurrentJob {
		a.currentTranslation = translation
		a.currentAudioFile = audioFile
		if imageFile != "" {
			a.currentImage = imageFile
		}
		// Make sure we have the phonetic info too
		if phoneticInfo != "" && phoneticInfo != "Failed to fetch phonetic information" {
			a.currentPhonetic = phoneticInfo
		}
	}
	a.mu.Unlock()

	if isCurrentJob {
		fyne.Do(func() {
			// Double-check that we're still on the same job before updating UI
			a.mu.Lock()
			if a.currentJobID != job.ID {
				a.mu.Unlock()
				return
			}
			a.mu.Unlock()

			a.translationEntry.SetText(translation)
			if imageFile != "" {
				a.imageDisplay.SetImages([]string{imageFile})
			}
			a.audioPlayer.SetAudioFile(audioFile)
			// Make sure phonetic info is displayed if we have it
			if a.currentPhonetic != "" {
				fmt.Printf("Setting phonetic in final UI update: %s\n", a.currentPhonetic)
				a.audioPlayer.SetPhonetic(a.currentPhonetic)
			} else {
				fmt.Printf("No phonetic info available in final UI update\n")
			}
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

				// Check if user has navigated back to this word
				a.mu.Lock()
				currentWord := a.currentWord
				a.mu.Unlock()

				if currentWord == job.Word {
					// User is currently viewing this word, reload the files
					a.loadExistingFiles(job.Word)
				}
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
	// Handle character input (for focus shortcuts that shouldn't type the character)
	a.window.Canvas().SetOnTypedRune(func(r rune) {
		// Check if input field is focused
		focused := a.window.Canvas().Focused()
		isInputFocused := focused == a.wordInput || focused == a.imagePromptEntry || focused == a.translationEntry

		// If input is focused, let the character be typed normally
		if isInputFocused {
			return
		}

		// Don't process if we're in delete or quit confirmation mode
		if a.deleteConfirming || a.quitConfirming {
			return
		}

		// Handle focus shortcuts that shouldn't type the character
		// Support both Latin and Cyrillic keyboard layouts
		switch r {
		case 'b', 'B', 'б', 'Б':
			a.window.Canvas().Focus(a.wordInput)
		case 'e', 'E', 'е', 'Е':
			a.window.Canvas().Focus(a.translationEntry)
		case 'o', 'O', 'о', 'О':
			a.window.Canvas().Focus(a.imagePromptEntry)
		// Handle Cyrillic shortcuts for actions
		case 'г', 'Г': // г = g
			if !a.submitButton.Disabled() {
				a.onSubmit()
			}
		case 'н', 'Н': // н = n
			if !a.keepButton.Disabled() {
				a.onKeepAndContinue()
			}
		case 'и', 'И': // и = i
			if !a.regenerateImageBtn.Disabled() {
				a.onRegenerateImage()
			}
		case 'м', 'М': // м = m
			if !a.regenerateRandomImageBtn.Disabled() {
				a.onRegenerateRandomImage()
			}
		case 'a', 'а': // a = regenerate front audio
			if !a.regenerateAudioBtn.Disabled() {
				a.onRegenerateAudio()
			}
		case 'A', 'А': // A = regenerate back audio (for bg-bg cards only)
			if a.currentCardType == "bg-bg" {
				a.onRegenerateBackAudio()
			}
		case 'р', 'Р': // р = r
			if !a.regenerateAllBtn.Disabled() {
				a.onRegenerateAll()
			}
		case 'д', 'Д': // д = d
			if !a.deleteButton.Disabled() {
				a.onDelete()
			}
		case 'p', 'п': // p = play front audio
			if a.currentAudioFile != "" {
				a.audioPlayer.Play()
			}
		case 'P', 'П': // P = play back audio (for bg-bg cards)
			if a.currentAudioFileBack != "" {
				a.audioPlayer.PlayBack()
			}
		case 'ж', 'Ж': // ж = x
			a.onExportToAnki()
		case 'в', 'В': // в = v
			a.onArchive()
		case '?':
			a.onShowHotkeys()
		case 'h', 'H', 'х', 'Х': // h/х = previous (vim-style)
			if !a.prevWordBtn.Disabled() {
				a.onPrevWord()
			}
		case 'l', 'L', 'л', 'Л': // l/л = next (vim-style)
			if !a.nextWordBtn.Disabled() {
				a.onNextWord()
			}
		case 'ч', 'Ч': // ч = q
			a.onQuitConfirm()
		case 'u', 'U', 'у', 'У': // u/у = toggle auto-play
			a.toggleAutoPlay()
		}
	})

	// Create a custom shortcut handler for regular keys (when input fields are not focused)
	a.window.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		// Check if input field is focused
		focused := a.window.Canvas().Focused()
		isInputFocused := focused == a.wordInput || focused == a.imagePromptEntry || focused == a.translationEntry

		// Handle Escape key to unfocus any field (works even when input is focused)
		if ev.Name == fyne.KeyEscape {
			a.window.Canvas().Unfocus()
			a.deleteConfirming = false
			a.quitConfirming = false
			return
		}

		// Handle Tab key for custom focus navigation
		if ev.Name == fyne.KeyTab {
			a.handleTabNavigation()
			return
		}

		// If input is focused, don't process regular shortcuts
		if isInputFocused {
			return
		}

		// Don't process if we're in delete or quit confirmation mode (handled by dialog)
		if a.deleteConfirming || a.quitConfirming {
			return
		}

		// Skip focus keys in SetOnTypedKey since they're handled in SetOnTypedRune
		if ev.Name == fyne.KeyB || ev.Name == fyne.KeyE || ev.Name == fyne.KeyO {
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
	// Don't process if we're in delete or quit confirmation mode
	if a.deleteConfirming || a.quitConfirming {
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

	case fyne.KeyA: // Regenerate Audio (handled by custom OnTypedRune for proper case sensitivity)
		// NOTE: This handler is disabled to use character-based handler instead
		// For bg-bg cards: shift+A = back audio, a = front audio
		// For en-bg cards: a/A = regenerate audio
		// See handleTypedRune for actual implementation

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

	case fyne.KeyX: // Export to APKG
		a.onExportToAnki()

	case fyne.KeyV: // Archive all cards
		a.onArchive()

	case fyne.KeyQ: // Quit application
		a.onQuitConfirm()
	}
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

// getPhoneticInfo fetches phonetic information for a Bulgarian word using the shared phonetic package.
func (a *Application) getPhoneticInfo(word string) (string, error) {
	if a.phoneticFetcher == nil {
		return "", fmt.Errorf("phonetic fetcher not initialized")
	}

	phoneticInfo, err := a.phoneticFetcher.Fetch(word)
	if err != nil {
		return "", fmt.Errorf("failed to get phonetic info: %w", err)
	}

	return phoneticInfo, nil
}
