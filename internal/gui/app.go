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
	// These are kept on Application so tests can set them before construction
	// of the orchestrator; New() copies them into the orchestrator.
	// imageFactories uses the shared image.ClientFactories type so the factory
	// signatures are defined once in the image package rather than duplicated here.
	imageFactories   image.ClientFactories
	newAudioProvider audio.ProviderFactory

	// Service layer — decoupled from the UI event-wiring in Application.
	cardSvc *CardService            // file discovery, directory management, persistence
	gen     *GenerationOrchestrator // audio, image, and phonetics generation
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
// New constructs and returns a fully initialised Application for the given config.
// A nil config receives all defaults. The Fyne application and UI are created here;
// callers should call Run() to start the event loop.
func New(config *Config) *Application {
	config = applyConfigDefaults(config)

	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create output directory %q: %v\n", config.OutputDir, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	myApp := app.NewWithID("org.codeberg.snonux.totalrecall")
	myApp.SetIcon(GetAppIcon())

	a := &Application{
		app:              myApp,
		config:           config,
		ctx:              ctx,
		cancel:           cancel,
		savedCards:       make([]anki.Card, 0),
		cardContexts:     make(map[string]context.CancelFunc),
		activeOperations: make(map[string]int),
		autoPlayEnabled:  config.AutoPlay,

		// Production-default factory functions; replaced in tests.
		// image.DefaultClientFactories() is the single source of truth for the
		// image factory signatures shared with the processor package.
		imageFactories:   image.DefaultClientFactories(),
		newAudioProvider: audio.NewProvider,
	}

	a.initAppServices(config)
	a.setupUI()
	a.scanExistingWords()
	a.updateQueueStatus()

	return a
}

// applyConfigDefaults returns config filled with defaults for any zero-value fields.
// When config is nil the full DefaultConfig is returned.
func applyConfigDefaults(config *Config) *Config {
	if config == nil {
		return DefaultConfig()
	}

	defaults := DefaultConfig()
	if config.AudioProvider == "" {
		config.AudioProvider = defaults.AudioProvider
	}
	if config.OutputDir == "" {
		config.OutputDir = defaults.OutputDir
	}
	if config.AudioFormat == "" {
		// Gemini uses a different default format; everything else gets mp3.
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
	// AutoPlay is not defaulted: bool zero value (false) cannot be distinguished
	// from an explicit false, so we leave it as-is.
	return config
}

// initAppServices wires the queue, audio config, phonetic fetcher, translator,
// CardService, and GenerationOrchestrator onto the Application. This is called
// once from New() after the struct is created.
func (a *Application) initAppServices(config *Config) {
	a.queue = NewWordQueue(a.ctx)
	a.queue.SetCallbacks(a.onQueueStatusUpdate, a.onJobComplete)

	a.audioConfig = audioConfigForApp(config)

	// Prefer injected phonetic fetcher (useful in tests); fall back to real one.
	if config.PhoneticFetcher != nil {
		a.phoneticFetcher = config.PhoneticFetcher
	} else {
		a.phoneticFetcher = phonetic.NewFetcher(&phonetic.Config{
			Provider:     config.PhoneticProvider,
			OpenAIKey:    config.OpenAIKey,
			GoogleAPIKey: config.GoogleAPIKey,
		})
	}

	// Prefer injected translator (useful in tests); fall back to real one.
	if config.Translator != nil {
		a.translator = config.Translator
	} else {
		a.translator = translation.NewTranslator(translationConfigForApp(config))
	}

	a.cardSvc = NewCardService(config)
	a.gen = NewGenerationOrchestrator(
		config,
		a.audioConfig,
		a.phoneticFetcher,
		a.translator,
		a.imageFactories,
		a.newAudioProvider,
	)
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

// getOrchestrator returns the GenerationOrchestrator, constructing one
// on-demand when the field is nil. The nil case occurs when tests create an
// Application struct literal directly without going through New().
func (a *Application) getOrchestrator() *GenerationOrchestrator {
	if a.gen != nil {
		return a.gen
	}

	// Build a temporary orchestrator from the Application's own fields so
	// that tests which set those fields directly still work correctly.
	return NewGenerationOrchestrator(
		a.config,
		a.audioConfig,
		a.phoneticFetcher,
		a.translator,
		a.imageFactories,
		a.newAudioProvider,
	)
}

// getCardService returns the CardService, constructing one on-demand when the
// field is nil. The nil case occurs when tests create an Application struct
// literal directly without going through New().
func (a *Application) getCardService() *CardService {
	if a.cardSvc != nil {
		return a.cardSvc
	}

	return NewCardService(a.config)
}

// setupUI creates the main user interface
// setupUI creates the main user interface and wires all event handlers.
func (a *Application) setupUI() {
	a.window = a.app.NewWindow("TotalRecall")
	a.window.SetIcon(GetAppIcon())
	a.window.Resize(fyne.NewSize(880, 770))

	inputSection := a.buildInputSection()
	displaySection := a.buildDisplaySection()
	exportButton, archiveButton, helpButton, toolbar := a.buildToolbar()
	statusSection := a.buildStatusSection()

	// Combine all sections — toolbar and input at top, status at bottom.
	content := container.NewBorder(
		container.NewVBox(toolbar, widget.NewSeparator(), inputSection),
		statusSection,
		nil, nil,
		displaySection,
	)

	// Wrap in the tooltip layer and wire shortcuts.
	a.window.SetContent(fynetooltip.AddWindowToolTipLayer(content, a.window.Canvas()))
	a.setupTooltips()

	// Secondary toolbar button tooltips need a short delay to initialise.
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

	a.window.SetOnClosed(a.onWindowClosed)
	a.setupKeyboardShortcuts()
}

// buildInputSection constructs and returns the word/translation/card-type input
// row together with the submit button.
// buildInputSection constructs the top row of the UI: Bulgarian word field,
// translation field, card-type selector, and the submit/navigation buttons.
func (a *Application) buildInputSection() fyne.CanvasObject {
	a.buildWordInput()
	a.buildTranslationInput()

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

	a.submitButton = ttwidget.NewButton("", a.onSubmit)
	a.submitButton.Icon = theme.ConfirmIcon()
	a.prevWordBtn = ttwidget.NewButton("", a.onPrevWord)
	a.prevWordBtn.Icon = theme.NavigateBackIcon()
	a.nextWordBtn = ttwidget.NewButton("", a.onNextWord)
	a.nextWordBtn.Icon = theme.NavigateNextIcon()

	inputGrid := container.New(layout.NewGridLayout(3),
		a.wordInput, a.translationEntry, a.cardTypeSelect,
	)
	return container.NewBorder(nil, nil, nil, a.submitButton, inputGrid)
}

// buildWordInput creates and wires the Bulgarian word entry field. The OnChanged
// handler debounces word changes and triggers image regeneration when the user
// edits an existing word.
func (a *Application) buildWordInput() {
	a.wordInput = NewCustomEntry()
	a.wordInput.SetPlaceHolder("Bulgarian word...")
	a.wordInput.OnSubmitted = func(string) {
		a.onSubmit()
		a.window.Canvas().Unfocus()
	}
	a.wordInput.SetOnEscape(func() { a.window.Canvas().Unfocus() })
	a.wordInput.OnChanged = func(text string) {
		a.mu.Lock()
		oldWord := a.currentWord
		if a.currentJobID != 0 && text != a.currentWord {
			a.currentJobID = 0
		}
		a.mu.Unlock()

		// Debounce: trigger image regeneration 1 s after the last keystroke.
		if oldWord != "" && text != "" && oldWord != text {
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
}

// buildTranslationInput creates and wires the translation entry field. OnChanged
// saves the translation and clears the current job ID when the text differs from
// the in-progress translation.
func (a *Application) buildTranslationInput() {
	a.translationEntry = NewCustomEntry()
	a.translationEntry.SetPlaceHolder("English translation...")
	a.translationEntry.OnChanged = func(text string) {
		a.mu.Lock()
		if a.currentJobID != 0 && a.currentTranslation != text {
			a.currentJobID = 0
		}
		a.mu.Unlock()
		a.currentTranslation = text
		a.saveTranslation()
	}
	a.translationEntry.OnSubmitted = func(string) {
		a.onSubmit()
		a.window.Canvas().Unfocus()
	}
	a.translationEntry.SetOnEscape(func() { a.window.Canvas().Unfocus() })
}

// buildDisplaySection constructs and returns the image/prompt and log/audio
// display area.
func (a *Application) buildDisplaySection() fyne.CanvasObject {
	a.imageDisplay = NewImageDisplay()
	a.audioPlayer = NewAudioPlayer()
	a.audioPlayer.SetAutoPlayEnabled(&a.autoPlayEnabled)

	a.imagePromptEntry = NewCustomMultiLineEntry()
	a.imagePromptEntry.SetPlaceHolder("Custom image prompt (optional)... Press Escape to exit field")
	a.imagePromptEntry.Wrapping = fyne.TextWrapWord
	a.imagePromptEntry.OnChanged = func(_ string) { a.saveImagePrompt() }
	a.imagePromptEntry.SetOnEscape(func() { a.window.Canvas().Unfocus() })

	promptContainer := container.NewBorder(
		widget.NewLabel("Image prompt:"), nil, nil, nil,
		container.NewScroll(a.imagePromptEntry),
	)

	imageSection := container.NewHSplit(a.imageDisplay, promptContainer)
	imageSection.SetOffset(0.5)

	a.logViewer = NewLogViewer()
	a.logViewer.StartCapture()

	audioLogSection := container.NewVSplit(a.logViewer, a.audioPlayer)
	audioLogSection.SetOffset(0.7)

	return container.NewBorder(nil, audioLogSection, nil, nil, imageSection)
}

// buildToolbar constructs action/navigation/utility buttons and the toolbar
// container. Returns the three utility buttons (for late tooltip wiring) and
// the toolbar itself.
func (a *Application) buildToolbar() (exportButton, archiveButton, helpButton *ttwidget.Button, toolbar fyne.CanvasObject) {
	a.keepButton = ttwidget.NewButtonWithIcon("", theme.DocumentCreateIcon(), a.onKeepAndContinue)
	a.regenerateImageBtn = ttwidget.NewButtonWithIcon("", theme.ColorPaletteIcon(), a.onRegenerateImage)
	a.regenerateRandomImageBtn = ttwidget.NewButtonWithIcon("", theme.ViewRefreshIcon(), a.onRegenerateRandomImage)
	a.regenerateAudioBtn = ttwidget.NewButtonWithIcon("", theme.MediaRecordIcon(), a.onRegenerateAudio)
	a.regenerateAllBtn = ttwidget.NewButtonWithIcon("", theme.ViewFullScreenIcon(), a.onRegenerateAll)
	a.deleteButton = ttwidget.NewButtonWithIcon("", theme.DeleteIcon(), a.onDelete)
	a.deleteButton.Importance = widget.DangerImportance

	a.setActionButtonsEnabled(false)
	a.deleteButton.Enable() // Keep delete enabled for cancelling operations.

	exportButton = ttwidget.NewButtonWithIcon("", theme.UploadIcon(), a.onExportToAnki)
	archiveButton = ttwidget.NewButtonWithIcon("", theme.FolderOpenIcon(), a.onArchive)
	helpButton = ttwidget.NewButtonWithIcon("", theme.HelpIcon(), a.onShowHotkeys)

	toolbar = container.NewHBox(
		a.prevWordBtn, a.nextWordBtn, widget.NewSeparator(),
		a.keepButton, a.deleteButton, widget.NewSeparator(),
		a.regenerateImageBtn, a.regenerateRandomImageBtn, a.regenerateAudioBtn, a.regenerateAllBtn, widget.NewSeparator(),
		exportButton, archiveButton, helpButton,
	)
	return exportButton, archiveButton, helpButton, toolbar
}

// buildStatusSection constructs and returns the status bar at the bottom of
// the window.
func (a *Application) buildStatusSection() fyne.CanvasObject {
	a.statusLabel = widget.NewLabel("Ready")
	a.queueStatusLabel = widget.NewLabel("Queue: Empty")
	a.queueStatusLabel.TextStyle = fyne.TextStyle{Italic: true}

	versionLabel := widget.NewLabel(fmt.Sprintf("v%s", internal.Version))
	versionLabel.TextStyle = fyne.TextStyle{Italic: true}
	versionLabel.Alignment = fyne.TextAlignTrailing

	return container.NewBorder(
		nil, nil, nil, versionLabel,
		container.NewVBox(a.statusLabel, widget.NewSeparator(), a.queueStatusLabel),
	)
}

// onWindowClosed is called when the window is closed. It stops background
// goroutines, cancels ongoing operations, and shuts down the application.
func (a *Application) onWindowClosed() {
	if a.fileCheckTicker != nil {
		a.fileCheckTicker.Stop()
	}
	if a.logViewer != nil {
		a.logViewer.StopCapture()
	}
	if a.cancel != nil {
		a.cancel()
	}

	// Wait for all goroutines with a 2-second timeout to avoid blocking the OS.
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		fmt.Println("Warning: Some operations did not complete before window close")
	}

	a.app.Quit()
}

// Run starts the GUI application
func (a *Application) Run() {
	// Don't focus any input field on startup - let user choose
	a.window.ShowAndRun()
}

// submitInputs holds the parsed result of a word-submission attempt.
type submitInputs struct {
	wordToProcess        string
	needsTranslation     bool
	translationDirection string // "bg-to-en" | "en-to-bg" | ""
	isBgBg               bool
	secondaryText        string // used for validation and bg-bg back text
}

// onSubmit handles word submission by parsing the input fields, optionally
// performing a pre-submission translation, validating, and enqueuing the job.
func (a *Application) onSubmit() {
	bulgarianText := strings.TrimSpace(a.wordInput.Text)
	secondaryText := strings.TrimSpace(a.translationEntry.Text)
	isBgBg := a.currentCardType == "bg-bg"

	inputs, ok := a.resolveSubmitInputs(bulgarianText, secondaryText, isBgBg)
	if !ok {
		return
	}

	// Perform any pre-queue translation (en→bg or bg→en).
	if !a.applyPreSubmitTranslation(&inputs, bulgarianText, secondaryText) {
		return
	}

	// Validate the word text before enqueueing.
	if err := audio.ValidateBulgarianText(inputs.wordToProcess); err != nil {
		dialog.ShowError(err, a.window)
		return
	}
	if inputs.isBgBg && inputs.secondaryText != "" {
		if err := audio.ValidateBulgarianText(inputs.secondaryText); err != nil {
			dialog.ShowError(fmt.Errorf("invalid back text: %w", err), a.window)
			return
		}
	}

	// Enqueue the job and start processing.
	job := a.queue.AddWordWithPrompt(inputs.wordToProcess, a.imagePromptEntry.Text)
	job.NeedsTranslation = inputs.needsTranslation
	job.CardType = a.currentCardType
	if a.currentTranslation != "" {
		job.Translation = a.currentTranslation
	}

	a.updateStatus(fmt.Sprintf("Added '%s' to queue (Job #%d)", inputs.wordToProcess, job.ID))
	a.updateQueueStatus()
	a.processNextInQueue()
}

// resolveSubmitInputs determines the word to process and translation direction
// from the two input fields. Returns (inputs, true) on success or (_, false)
// when no processable input is available.
func (a *Application) resolveSubmitInputs(bulgarianText, secondaryText string, isBgBg bool) (submitInputs, bool) {
	var inp submitInputs
	inp.isBgBg = isBgBg
	inp.secondaryText = secondaryText

	switch {
	case isBgBg:
		if bulgarianText == "" {
			return inp, false
		}
		inp.wordToProcess = bulgarianText
		a.currentTranslation = secondaryText
	case bulgarianText != "" && secondaryText != "":
		inp.wordToProcess = bulgarianText
		a.currentTranslation = secondaryText
	case bulgarianText != "" && secondaryText == "":
		inp.wordToProcess = bulgarianText
		inp.needsTranslation = true
		inp.translationDirection = "bg-to-en"
	case bulgarianText == "" && secondaryText != "":
		inp.needsTranslation = true
		inp.translationDirection = "en-to-bg"
	default:
		return inp, false
	}

	return inp, true
}

// applyPreSubmitTranslation performs any translation that must complete before
// the word is enqueued (en→bg or bg→en). Updates inputs.wordToProcess and the
// UI in place. Returns false when the translation failed.
func (a *Application) applyPreSubmitTranslation(inputs *submitInputs, bulgarianText, secondaryText string) bool {
	switch inputs.translationDirection {
	case "en-to-bg":
		a.updateStatus(fmt.Sprintf("Translating '%s' to Bulgarian...", secondaryText))
		bulgarian, err := a.translateEnglishToBulgarian(secondaryText)
		if err != nil {
			dialog.ShowError(fmt.Errorf("translation failed: %w", err), a.window)
			return false
		}
		inputs.wordToProcess = bulgarian
		a.wordInput.SetText(bulgarian)
		a.currentTranslation = secondaryText
		a.currentWord = bulgarian
		a.saveTranslation()
		inputs.needsTranslation = false

	case "bg-to-en":
		a.updateStatus(fmt.Sprintf("Translating '%s' to English...", bulgarianText))
		english, err := a.translateWord(bulgarianText)
		if err != nil {
			dialog.ShowError(fmt.Errorf("translation failed: %w", err), a.window)
			return false
		}
		a.currentTranslation = english
		a.translationEntry.SetText(english)
		inputs.needsTranslation = false
		a.saveTranslation()
	}

	return true
}

// generateMaterials orchestrates audio, image, and phonetics generation for a
// word (used by all regenerate functions). Delegates to GenerationOrchestrator
// for the actual generation work and updates UI state after each step.
// generateMaterials is the foreground (non-queue) entry point for generating all
// card materials for a word. It resolves the translation, fires parallel generation
// via the orchestrator, and applies the result to the UI.
func (a *Application) generateMaterials(word string) {
	cardCtx, _ := a.getOrCreateCardContext(word)

	cardDir, err := a.ensureCardDirectory(word)
	if err != nil {
		fyne.Do(func() {
			a.showError(fmt.Errorf("failed to create card directory: %w", err))
			a.setUIEnabled(true)
		})
		return
	}

	translation, ok := a.resolveTranslation(word, cardDir)
	if !ok {
		return // error already shown in resolveTranslation
	}

	// Snapshot prompt and translation before the goroutine to avoid data races.
	customPrompt := a.imagePromptEntry.Text
	if translation == "" {
		translation = strings.TrimSpace(a.translationEntry.Text)
	}

	result, err := a.runMaterialsGeneration(cardCtx, word, translation, cardDir, customPrompt)

	if err != nil {
		fyne.Do(func() {
			a.showError(err)
			a.setUIEnabled(true)
		})
		return
	}

	a.applyMaterialsResult(word, result)

	fyne.Do(func() {
		a.hideProgress()
		a.updateStatus("Ready - Review and decide")
		a.setUIEnabled(true)
		a.setActionButtonsEnabled(true)
	})
}

// runMaterialsGeneration starts the three parallel operations (audio, image, phonetics)
// via the orchestrator and manages the processing counter. Returns the generation
// result or the first error encountered.
func (a *Application) runMaterialsGeneration(cardCtx context.Context, word, translation, cardDir, customPrompt string) (GenerateResult, error) {
	fyne.Do(func() {
		a.updateStatus("Generating audio, images, and phonetics in parallel...")
		a.mu.Lock()
		if a.currentWord == word {
			a.imageDisplay.SetGenerating()
		}
		a.mu.Unlock()
	})

	a.startOperation(word)
	a.startOperation(word)
	a.startOperation(word)
	fyne.Do(func() {
		a.incrementProcessing() // audio
		a.incrementProcessing() // image
		a.incrementProcessing() // phonetic
	})

	// promptUI callback updates the image-prompt entry when the prompt is ready.
	promptUI := func(prompt string) {
		a.mu.Lock()
		isCurrentWord := a.currentWord == word
		a.mu.Unlock()
		if isCurrentWord && a.imagePromptEntry != nil {
			a.imagePromptEntry.SetText(prompt)
		}
	}

	result, err := a.getOrchestrator().GenerateMaterials(
		cardCtx, word, translation, cardDir,
		false, // en-bg card (bg-bg handled in processWordJob)
		customPrompt,
		promptUI,
	)

	a.decrementProcessing()
	a.decrementProcessing()
	a.decrementProcessing()
	a.endOperation(word)
	a.endOperation(word)
	a.endOperation(word)

	return result, err
}

// resolveTranslation ensures a translation is available for word, translating
// via the orchestrator when currentTranslation is empty. It updates the UI
// and saves the translation to disk. Returns the translation and true on
// success; false when a fatal error occurred (already shown in the UI).
func (a *Application) resolveTranslation(word, cardDir string) (string, bool) {
	if a.currentTranslation != "" {
		return a.currentTranslation, true
	}

	fyne.Do(func() {
		a.updateStatus("Translating...")
	})

	translation, err := a.translateWord(word)
	if err != nil {
		fyne.Do(func() {
			a.showError(fmt.Errorf("translation failed: %w", err))
			a.setUIEnabled(true)
		})
		return "", false
	}

	a.mu.Lock()
	if a.currentWord == word {
		a.currentTranslation = translation
		fyne.Do(func() {
			a.translationEntry.SetText(translation)
		})
	}
	a.mu.Unlock()

	if translation != "" {
		if err := a.getCardService().SaveTranslation(word, translation); err != nil {
			fmt.Printf("Warning: Failed to save translation for '%s': %v\n", word, err)
		}
	}

	return translation, true
}

// applyMaterialsResult applies a GenerateResult to Application state and
// updates the UI when this word is still the current word.
func (a *Application) applyMaterialsResult(word string, result GenerateResult) {
	a.mu.Lock()
	isCurrentWord := a.currentWord == word
	if isCurrentWord {
		if result.AudioFile != "" {
			a.currentAudioFile = result.AudioFile
		}
		if result.ImageFile != "" {
			a.currentImage = result.ImageFile
		}
		if result.PhoneticInfo != "" && result.PhoneticInfo != "Failed to fetch phonetic information" {
			a.currentPhonetic = result.PhoneticInfo
		}
	}
	a.mu.Unlock()

	if !isCurrentWord {
		return
	}

	fyne.Do(func() {
		a.mu.Lock()
		if a.currentWord != word {
			a.mu.Unlock()
			return
		}
		a.mu.Unlock()

		if result.AudioFile != "" {
			a.audioPlayer.SetAudioFile(result.AudioFile)
		}
		if result.ImageFile != "" {
			a.imageDisplay.SetImages([]string{result.ImageFile})
		}
		if result.PhoneticInfo != "" && result.PhoneticInfo != "Failed to fetch phonetic information" {
			a.audioPlayer.SetPhonetic(result.PhoneticInfo)
		}
	})
}

// onKeepAndContinue saves the current card and clears for a new word
// onKeepAndContinue saves the current card (when complete) and resets the UI to
// accept the next word. Any background generation job for the current word continues
// running; the job ID is cleared so the results are no longer displayed here.
func (a *Application) onKeepAndContinue() {
	a.saveCurrentCardIfComplete()

	// Detach from any in-progress job so it continues silently in the background.
	a.mu.Lock()
	currentJobID := a.currentJobID
	a.currentJobID = 0
	a.mu.Unlock()
	if currentJobID != 0 {
		a.updateStatus("Previous word continues processing in background")
	}

	a.clearCurrentWordState()
}

// saveCurrentCardIfComplete saves the current card to disk and in-memory list when
// all required files (audio and image) are present.
func (a *Application) saveCurrentCardIfComplete() {
	if a.currentWord == "" || a.currentAudioFile == "" || a.currentImage == "" {
		return
	}

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

	a.saveTranslation()
	a.saveImagePrompt()
	a.savePhoneticInfo()
	a.scanExistingWords()
	a.updateStatus(fmt.Sprintf("Card saved! Total cards: %d", count))
}

// clearCurrentWordState resets all per-word UI and state fields so the application
// is ready for a new word entry.
func (a *Application) clearCurrentWordState() {
	a.clearUI()
	a.wordInput.SetText("")
	a.translationEntry.SetText("")

	a.mu.Lock()
	a.currentWord = ""
	a.currentTranslation = ""
	a.currentAudioFile = ""
	a.currentImage = ""
	a.currentPhonetic = ""
	a.mu.Unlock()

	a.hideProgress()
	a.submitButton.Enable()
}

// onRegenerateImage regenerates the image using the current prompt from the UI.
func (a *Application) onRegenerateImage() {
	a.runImageRegeneration(a.imagePromptEntry.Text, "Regenerating image...")
}

// onRegenerateRandomImage generates a new image with a fresh random prompt by
// passing an empty custom prompt so the orchestrator picks a new one.
func (a *Application) onRegenerateRandomImage() {
	a.runImageRegeneration("", "Generating random image...")
}

// runImageRegeneration disables the image buttons, fires image generation in a
// goroutine, and re-enables the buttons when done. customPrompt is passed
// verbatim to the image generator; an empty string triggers a random prompt.
func (a *Application) runImageRegeneration(customPrompt, statusMsg string) {
	a.regenerateImageBtn.Disable()
	a.regenerateRandomImageBtn.Disable()
	a.regenerateAllBtn.Disable()
	a.showProgress(statusMsg)
	a.imageDisplay.SetGenerating()
	a.incrementProcessing()

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer a.decrementProcessing()

		translation := a.currentTranslation
		if translation == "" {
			translation = strings.TrimSpace(a.translationEntry.Text)
		}
		wordForGeneration := a.currentWord
		cardCtx, _ := a.getOrCreateCardContext(wordForGeneration)

		cardDir, err := a.ensureCardDirectory(wordForGeneration)
		if err != nil {
			fyne.Do(func() {
				a.showError(fmt.Errorf("failed to create card directory: %w", err))
			})
			fyne.Do(a.reenableImageButtons)
			return
		}

		imageFile, err := a.generateImagesWithPrompt(cardCtx, wordForGeneration, customPrompt, translation, cardDir)
		if err != nil {
			fyne.Do(func() { a.showError(fmt.Errorf("image generation failed: %w", err)) })
		} else if imageFile != "" {
			a.mu.Lock()
			if a.currentWord == wordForGeneration {
				a.currentImage = imageFile
				a.mu.Unlock()
				fyne.Do(func() { a.imageDisplay.SetImages([]string{imageFile}) })
			} else {
				a.mu.Unlock()
			}
		}

		fyne.Do(a.reenableImageButtons)
	}()
}

// reenableImageButtons hides the progress bar and re-enables all image-related
// action buttons. Must be called on the UI goroutine (via fyne.Do).
func (a *Application) reenableImageButtons() {
	a.hideProgress()
	a.regenerateImageBtn.Enable()
	a.regenerateRandomImageBtn.Enable()
	a.regenerateAllBtn.Enable()
}

// onRegenerateAudio regenerates front audio (or single audio for en-bg cards)
// onRegenerateAudio regenerates front audio (or the single audio file for en-bg cards).
// It disables audio/regenerate buttons while the goroutine runs.
func (a *Application) onRegenerateAudio() {
	a.regenerateAudioBtn.Disable()
	a.regenerateAllBtn.Disable()

	isBgBg := a.currentCardType == "bg-bg"
	if isBgBg {
		a.showProgress("Regenerating front audio...")
	} else {
		a.showProgress("Regenerating audio...")
	}

	a.incrementProcessing()

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer a.decrementProcessing()

		wordForGeneration := a.currentWord
		a.startOperation(wordForGeneration)
		defer a.endOperation(wordForGeneration)

		cardCtx, _ := a.getOrCreateCardContext(wordForGeneration)
		cardDir, err := a.ensureCardDirectory(wordForGeneration)
		if err != nil {
			fyne.Do(func() { a.showError(fmt.Errorf("failed to create card directory: %w", err)) })
			fyne.Do(a.reenableAudioButtons)
			return
		}

		if isBgBg {
			a.regenerateFrontAudio(cardCtx, wordForGeneration, cardDir)
		} else {
			a.regenerateEnBgAudio(cardCtx, wordForGeneration, cardDir)
		}

		fyne.Do(a.reenableAudioButtons)
	}()
}

// regenerateFrontAudio generates the front audio file for a bg-bg card and updates
// the audio player to play only the front track.
func (a *Application) regenerateFrontAudio(cardCtx context.Context, word, cardDir string) {
	audioFile, err := a.generateAudioFront(cardCtx, word, cardDir)
	if err != nil {
		fyne.Do(func() { a.showError(fmt.Errorf("front audio regeneration failed: %w", err)) })
		return
	}
	a.mu.Lock()
	if a.currentWord != word {
		a.mu.Unlock()
		return
	}
	a.currentAudioFile = audioFile
	a.mu.Unlock()

	fyne.Do(func() {
		a.mu.Lock()
		if a.currentWord == word {
			// Set without auto-play, then play explicitly so only the front track plays.
			a.audioPlayer.SetAudioFileNoAutoPlay(audioFile)
			a.audioPlayer.Play()
		}
		a.mu.Unlock()
	})
}

// regenerateEnBgAudio generates the audio file for an en-bg card and sets it
// on the audio player.
func (a *Application) regenerateEnBgAudio(cardCtx context.Context, word, cardDir string) {
	audioFile, err := a.generateAudio(cardCtx, word, cardDir)
	if err != nil {
		fyne.Do(func() { a.showError(fmt.Errorf("audio regeneration failed: %w", err)) })
		return
	}
	a.mu.Lock()
	if a.currentWord != word {
		a.mu.Unlock()
		return
	}
	a.currentAudioFile = audioFile
	a.mu.Unlock()

	fyne.Do(func() {
		a.mu.Lock()
		if a.currentWord == word {
			a.audioPlayer.SetAudioFile(audioFile)
		}
		a.mu.Unlock()
	})
}

// reenableAudioButtons hides the progress bar and re-enables the audio action
// buttons. Must be called on the UI goroutine (via fyne.Do).
func (a *Application) reenableAudioButtons() {
	a.hideProgress()
	a.regenerateAudioBtn.Enable()
	a.regenerateAllBtn.Enable()
}

// onRegenerateBackAudio regenerates back audio for bg-bg cards
// onRegenerateBackAudio regenerates the back audio file for bg-bg cards. No-op
// for en-bg cards. Uses the application-level context (not a card context) to
// avoid cancelling a concurrent front audio operation on the same word.
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

		// Snapshot translation before entering the goroutine.
		translation := a.currentTranslation
		if translation == "" {
			translation = strings.TrimSpace(a.translationEntry.Text)
		}
		wordForGeneration := a.currentWord

		a.startOperation(wordForGeneration)
		defer a.endOperation(wordForGeneration)

		cardDir, err := a.ensureCardDirectory(wordForGeneration)
		if err != nil {
			fyne.Do(func() { a.showError(fmt.Errorf("failed to create card directory: %w", err)) })
			fyne.Do(a.reenableAudioButtons)
			return
		}

		a.applyRegeneratedBackAudio(wordForGeneration, translation, cardDir)
		fyne.Do(a.reenableAudioButtons)
	}()
}

// applyRegeneratedBackAudio generates the back audio file and, if the word is still
// current, stores the result and triggers playback via the audio player.
func (a *Application) applyRegeneratedBackAudio(word, translation, cardDir string) {
	audioFile, err := a.generateAudioBack(a.ctx, translation, cardDir)
	if err != nil {
		fyne.Do(func() { a.showError(fmt.Errorf("back audio regeneration failed: %w", err)) })
		return
	}
	a.mu.Lock()
	if a.currentWord != word {
		a.mu.Unlock()
		return
	}
	a.currentAudioFileBack = audioFile
	a.mu.Unlock()

	fyne.Do(func() {
		a.mu.Lock()
		if a.currentWord == word {
			a.audioPlayer.SetBackAudioFile(audioFile)
			a.audioPlayer.PlayBack()
		}
		a.mu.Unlock()
	})
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

// onExportToAnki exports all cards from the output directory to Anki.
// Shows a format-selection dialog and performs the actual export on confirm.
// onExportToAnki opens the Export to Anki dialog where the user selects a format,
// deck name, and output directory. No-op when no exportable cards exist.
func (a *Application) onExportToAnki() {
	if !a.hasExportableCards() {
		dialog.ShowInformation("No Cards", "No cards found in anki_cards folder. Generate some cards first!", a.window)
		return
	}

	formatOptions := []string{"APKG (Recommended)", "CSV (Legacy)"}
	formatSelect := widget.NewSelect(formatOptions, nil)
	formatSelect.SetSelected(formatOptions[0])
	deckNameEntry := widget.NewEntry()
	deckNameEntry.SetPlaceHolder("Bulgarian Vocabulary")

	selectedDir := a.defaultExportDir()
	dirLabel := widget.NewLabel(selectedDir)
	dirButton := widget.NewButton("Browse...", func() {
		a.browseExportDir(&selectedDir, dirLabel)
	})

	content := a.buildExportDialogContent(formatSelect, deckNameEntry, dirLabel, dirButton)
	a.showExportDialog(content, formatOptions, formatSelect, deckNameEntry, &selectedDir)
}

// buildExportDialogContent assembles the VBox shown inside the Export to Anki dialog.
func (a *Application) buildExportDialogContent(formatSelect *widget.Select, deckNameEntry *widget.Entry, dirLabel *widget.Label, dirButton *widget.Button) fyne.CanvasObject {
	return container.NewVBox(
		widget.NewLabel("Export Format:"),
		formatSelect,
		widget.NewSeparator(),
		widget.NewLabel("Deck Name:"),
		deckNameEntry,
		widget.NewSeparator(),
		widget.NewLabel("Export Directory:"),
		container.NewBorder(nil, nil, nil, dirButton, dirLabel),
		widget.NewLabel(""),
		widget.NewRichTextFromMarkdown("**APKG**: Complete package with media files included\n**CSV**: Text only, requires manual media copy"),
	)
}

// showExportDialog creates the custom confirm dialog, wires keyboard shortcuts
// (e/е = export, c/ц/Esc = cancel), and shows it.
// showExportDialog creates and shows the Export to Anki custom confirm dialog.
// The confirm callback runs performExport; keyboard shortcuts e/е confirm and
// c/ц/Esc cancel.
func (a *Application) showExportDialog(content fyne.CanvasObject, formatOptions []string, formatSelect *widget.Select, deckNameEntry *widget.Entry, selectedDir *string) {
	exportDialogOpen := true

	customDialog := dialog.NewCustomConfirm("Export to Anki", "Export (e)", "Cancel (c/Esc)", content, func(export bool) {
		exportDialogOpen = false
		if !export {
			return
		}
		deckName := deckNameEntry.Text
		if deckName == "" {
			deckName = "Bulgarian Vocabulary"
		}
		a.performExport(formatSelect.Selected == formatOptions[0], deckName, *selectedDir)
	}, a.window)

	a.wireExportDialogKeys(customDialog, &exportDialogOpen)
	customDialog.Resize(fyne.NewSize(400, 300))
	customDialog.Show()
}

// wireExportDialogKeys attaches keyboard shortcuts to the export dialog.
// e/е triggers confirm; c/ц/Esc cancels. Original handlers are restored on close.
func (a *Application) wireExportDialogKeys(customDialog *dialog.ConfirmDialog, exportDialogOpen *bool) {
	origRune := a.window.Canvas().OnTypedRune()
	origKey := a.window.Canvas().OnTypedKey()

	a.window.Canvas().SetOnTypedRune(func(r rune) {
		if *exportDialogOpen {
			switch r {
			case 'e', 'E', 'е', 'Е':
				customDialog.Hide()
				*exportDialogOpen = false
				customDialog.Confirm()
			case 'c', 'C', 'ц', 'Ц':
				customDialog.Hide()
				*exportDialogOpen = false
			}
			return
		}
		if origRune != nil {
			origRune(r)
		}
	})
	a.window.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if *exportDialogOpen && ev.Name == fyne.KeyEscape {
			customDialog.Hide()
			*exportDialogOpen = false
			return
		}
		if origKey != nil {
			origKey(ev)
		}
	})
	customDialog.SetOnClosed(func() {
		*exportDialogOpen = false
		a.window.Canvas().SetOnTypedRune(origRune)
		a.window.Canvas().SetOnTypedKey(origKey)
	})
}

// hasExportableCards returns true when the output directory has at least one
// non-hidden subdirectory (which represents a card).
func (a *Application) hasExportableCards() bool {
	entries, err := os.ReadDir(a.config.OutputDir)
	if err != nil || len(entries) == 0 {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			return true
		}
	}
	return false
}

// defaultExportDir returns the home directory as the default export location.
func (a *Application) defaultExportDir() string {
	homeDir, err := appconfig.HomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
	return homeDir
}

// browseExportDir opens a folder-picker dialog and updates *dir and the label
// when the user selects a directory.
func (a *Application) browseExportDir(dir *string, label *widget.Label) {
	folderDialog := dialog.NewFolderOpen(func(selected fyne.ListableURI, err error) {
		if err != nil || selected == nil {
			return
		}
		*dir = selected.Path()
		label.SetText(*dir)
	}, a.window)

	if uri, err := storage.ParseURI("file://" + *dir); err == nil {
		if listableURI, ok := uri.(fyne.ListableURI); ok {
			folderDialog.SetLocation(listableURI)
		}
	}
	folderDialog.Show()
}

// performExport runs the actual APKG or CSV export and updates the status bar.
func (a *Application) performExport(isAPKG bool, deckName, outputDir string) {
	if isAPKG {
		a.exportAPKG(deckName, outputDir)
	} else {
		a.exportCSV(outputDir)
	}
}

// exportAPKG generates an APKG file from all cards and updates the status bar.
func (a *Application) exportAPKG(deckName, outputDir string) {
	filename := fmt.Sprintf("%s.apkg", internal.SanitizeFilename(deckName))
	outputPath := filepath.Join(outputDir, filename)

	gen := anki.NewGenerator(nil)
	if err := gen.GenerateFromDirectory(a.config.OutputDir); err != nil {
		dialog.ShowError(fmt.Errorf("failed to load cards: %w", err), a.window)
		return
	}
	if err := gen.GenerateAPKG(outputPath, deckName); err != nil {
		dialog.ShowError(fmt.Errorf("failed to generate APKG: %w", err), a.window)
		return
	}
	total, withAudio, withImages := gen.Stats()
	a.updateStatus(fmt.Sprintf("Exported %d cards to %s (%d with audio, %d with images)", total, outputDir, withAudio, withImages))
}

// exportCSV generates a CSV file from all cards and updates the status bar.
func (a *Application) exportCSV(outputDir string) {
	outputPath := filepath.Join(outputDir, "anki_import.csv")

	gen := anki.NewGenerator(&anki.GeneratorOptions{
		OutputPath:     outputPath,
		MediaFolder:    a.config.OutputDir,
		IncludeHeaders: true,
		AudioFormat:    a.config.AudioFormat,
	})
	if err := gen.GenerateFromDirectory(a.config.OutputDir); err != nil {
		dialog.ShowError(fmt.Errorf("failed to load cards: %w", err), a.window)
		return
	}
	if err := gen.GenerateCSV(); err != nil {
		dialog.ShowError(fmt.Errorf("failed to generate CSV: %w", err), a.window)
		return
	}
	total, withAudio, withImages := gen.Stats()
	a.updateStatus(fmt.Sprintf("Exported %d cards to %s (%d with audio, %d with images)", total, outputDir, withAudio, withImages))
}

// onArchive shows a confirmation dialog and archives the current cards directory
// on user confirmation. Keyboard shortcuts y/ъ confirm, n/н/c/ц/Esc cancel.
func (a *Application) onArchive() {
	confirmDialog := dialog.NewConfirm("Archive Cards",
		"Are you sure you want to archive all existing cards?\n\nThis will move the cards directory to:\n~/.local/state/totalrecall/archive/cards-TIMESTAMP",
		func(confirmed bool) {
			if confirmed {
				a.performArchive()
			}
		},
		a.window,
	)
	a.showArchiveConfirmDialog(confirmDialog)
}

// performArchive moves the cards directory to the archive location, clears in-memory
// state, and refreshes the word list. Errors are shown via the error dialog.
func (a *Application) performArchive() {
	home, err := appconfig.HomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
	cardsDir := filepath.Join(home, ".local", "state", "totalrecall", "cards")

	if err := archive.ArchiveCards(cardsDir); err != nil {
		dialog.ShowError(err, a.window)
		return
	}

	a.mu.Lock()
	a.savedCards = []anki.Card{}
	a.existingWords = []string{}
	a.mu.Unlock()

	a.updateStatus("Cards archived successfully")
	a.scanExistingWords()
	if a.currentWord != "" {
		a.loadExistingFiles(a.currentWord)
	}
}

// showArchiveConfirmDialog wires keyboard shortcuts (y/ъ = confirm, n/н/c/ц/Esc = cancel)
// to the given confirmation dialog and then shows it. Original handlers are restored
// when the dialog closes.
func (a *Application) showArchiveConfirmDialog(confirmDialog *dialog.ConfirmDialog) {
	archiveConfirming := true
	oldKeyHandler := a.window.Canvas().OnTypedKey()
	oldRuneHandler := a.window.Canvas().OnTypedRune()

	restoreHandlers := func() {
		archiveConfirming = false
		a.window.Canvas().SetOnTypedKey(oldKeyHandler)
		a.window.Canvas().SetOnTypedRune(oldRuneHandler)
	}

	a.window.Canvas().SetOnTypedRune(func(r rune) {
		if !archiveConfirming {
			if oldRuneHandler != nil {
				oldRuneHandler(r)
			}
			return
		}
		switch r {
		case 'y', 'Y', 'ъ', 'Ъ':
			confirmDialog.Hide()
			restoreHandlers()
			a.performArchive()
		case 'n', 'N', 'н', 'Н', 'c', 'C', 'ц', 'Ц':
			confirmDialog.Hide()
			restoreHandlers()
		}
	})

	a.window.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if !archiveConfirming {
			if oldKeyHandler != nil {
				oldKeyHandler(ev)
			}
			return
		}
		switch ev.Name {
		case fyne.KeyY:
			confirmDialog.Hide()
			restoreHandlers()
			a.performArchive()
		case fyne.KeyN, fyne.KeyC, fyne.KeyEscape:
			confirmDialog.Hide()
			restoreHandlers()
		}
	})

	confirmDialog.SetOnClosed(restoreHandlers)
	confirmDialog.Show()
}

// onShowHotkeys displays a dialog with all available keyboard shortcuts
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
func (a *Application) onShowHotkeys() {
	content := widget.NewRichTextFromMarkdown(hotkeysMarkdown)
	content.Wrapping = fyne.TextWrapWord

	scroll := container.NewScroll(container.NewPadded(content))
	scroll.SetMinSize(fyne.NewSize(700, 480))

	d := dialog.NewCustom("Keyboard Shortcuts", "Close", scroll, a.window)
	a.wireHotkeysDialog(d)
}

// wireHotkeysDialog attaches temporary c/ц and Esc key handlers that close the
// dialog, then restores normal shortcuts via setupKeyboardShortcuts on close.
func (a *Application) wireHotkeysDialog(d *dialog.CustomDialog) {
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
		a.setupKeyboardShortcuts()
	})

	d.Show()
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

// onQuitConfirm shows a confirmation dialog before quitting. No-op when a quit
// confirmation is already in progress. Keyboard shortcuts y/ъ quit, n/н/Esc cancel.
func (a *Application) onQuitConfirm() {
	if a.quitConfirming {
		return
	}
	a.quitConfirming = true

	message := "Are you sure you want to quit?\n\nPress y to quit or n to cancel"
	confirmDialog := dialog.NewConfirm("Quit Application", message, func(confirm bool) {
		a.quitConfirming = false
		if confirm {
			a.window.Close()
		}
	}, a.window)

	a.wireQuitConfirmDialog(confirmDialog)
}

// wireQuitConfirmDialog attaches keyboard shortcuts (y/ъ = quit, n/н/Esc = cancel)
// to the quit confirmation dialog. Original handlers are restored when the dialog closes.
func (a *Application) wireQuitConfirmDialog(confirmDialog *dialog.ConfirmDialog) {
	oldKeyHandler := a.window.Canvas().OnTypedKey()
	oldRuneHandler := a.window.Canvas().OnTypedRune()

	restoreHandlers := func() {
		a.quitConfirming = false
		a.window.Canvas().SetOnTypedKey(oldKeyHandler)
		a.window.Canvas().SetOnTypedRune(oldRuneHandler)
	}

	a.window.Canvas().SetOnTypedRune(func(r rune) {
		if !a.quitConfirming {
			if oldRuneHandler != nil {
				oldRuneHandler(r)
			}
			return
		}
		switch r {
		case 'y', 'Y', 'ъ', 'Ъ':
			confirmDialog.Hide()
			a.quitConfirming = false
			a.window.Close()
		case 'n', 'N', 'н', 'Н':
			confirmDialog.Hide()
			restoreHandlers()
		}
	})

	a.window.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if !a.quitConfirming {
			if oldKeyHandler != nil {
				oldKeyHandler(ev)
			}
			return
		}
		switch ev.Name {
		case fyne.KeyY:
			confirmDialog.Hide()
			a.quitConfirming = false
			a.window.Close()
		case fyne.KeyN, fyne.KeyEscape:
			confirmDialog.Hide()
			restoreHandlers()
		}
	})

	confirmDialog.SetOnClosed(restoreHandlers)
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

// processWordJob processes a single word job using the GenerationOrchestrator
// for audio/image/phonetics work and updates UI state upon completion.
// processWordJob runs a single word job: creates the card directory, resolves the
// translation, triggers parallel audio/image/phonetics generation via the orchestrator,
// and updates the UI with the results. The job is marked complete (or failed) before
// returning.
func (a *Application) processWordJob(job *WordJob) {
	cardCtx, _ := a.getOrCreateCardContext(job.Word)

	// Bail early if the context was already cancelled before we started.
	select {
	case <-cardCtx.Done():
		a.queue.FailJob(job.ID, fmt.Errorf("job cancelled"))
		a.finishCurrentJob()
		return
	default:
	}

	cardDir, isBgBg, ok := a.prepareJobDirectory(job)
	if !ok {
		return
	}

	translation, ok := a.resolveJobTranslation(job, isBgBg, cardDir)
	if !ok {
		a.finishCurrentJob()
		return
	}

	// Show translation in the UI before generation starts.
	a.mu.Lock()
	if a.currentJobID == job.ID && translation != "" {
		a.currentTranslation = translation
		fyne.Do(func() { a.translationEntry.SetText(translation) })
	}
	a.mu.Unlock()

	result, genErr := a.runJobGeneration(job, cardCtx, translation, cardDir, isBgBg)
	if genErr != nil {
		a.queue.FailJob(job.ID, genErr)
		a.finishCurrentJob()
		return
	}

	a.applyJobResult(job, result, translation, isBgBg)

	a.finishCurrentJob()
	fyne.Do(func() { a.updateQueueStatus() })
}

// prepareJobDirectory ensures a card directory exists and saves the card type.
// Returns the directory path, isBgBg flag, and true on success.
func (a *Application) prepareJobDirectory(job *WordJob) (string, bool, bool) {
	cardDir, dirErr := a.ensureCardDirectory(job.Word)
	if dirErr != nil {
		a.queue.FailJob(job.ID, fmt.Errorf("failed to create card directory: %w", dirErr))
		a.finishCurrentJob()
		return "", false, false
	}

	isBgBg := job.CardType == "bg-bg"
	if err := a.saveJobCardType(job.ID, cardDir, isBgBg); err != nil {
		a.finishCurrentJob()
		return "", false, false
	}

	return cardDir, isBgBg, true
}

// runJobGeneration fires the parallel audio/image/phonetics generation for a job
// and manages the processing counter. Returns the generation result or an error.
func (a *Application) runJobGeneration(job *WordJob, cardCtx context.Context, translation, cardDir string, isBgBg bool) (GenerateResult, error) {
	fyne.Do(func() {
		a.updateStatus(fmt.Sprintf("Processing '%s' - generating audio, images, and phonetics in parallel...", job.Word))
		a.mu.Lock()
		if a.currentJobID == job.ID {
			a.imageDisplay.SetGenerating()
		}
		a.mu.Unlock()
	})

	// promptUI notifies the imagePromptEntry widget when the prompt is determined.
	promptUI := func(prompt string) {
		a.mu.Lock()
		isCurrentJob := a.currentJobID == job.ID
		a.mu.Unlock()
		if isCurrentJob && a.imagePromptEntry != nil {
			a.imagePromptEntry.SetText(prompt)
		}
	}

	// Three parallel operations: audio, image, phonetics.
	a.startOperation(job.Word)
	a.startOperation(job.Word)
	a.startOperation(job.Word)
	fyne.Do(func() {
		a.incrementProcessing()
		a.incrementProcessing()
		a.incrementProcessing()
	})

	result, genErr := a.getOrchestrator().GenerateMaterials(
		cardCtx, job.Word, translation, cardDir, isBgBg, job.CustomPrompt, promptUI,
	)

	a.decrementProcessing()
	a.decrementProcessing()
	a.decrementProcessing()
	a.endOperation(job.Word)
	a.endOperation(job.Word)
	a.endOperation(job.Word)

	return result, genErr
}

// applyJobResult writes the generation result to in-memory state and performs
// intermediate and final UI updates including audio player, image display, and
// phonetics label.
func (a *Application) applyJobResult(job *WordJob, result GenerateResult, translation string, isBgBg bool) {
	// Update audio state immediately so the play button becomes available.
	a.mu.Lock()
	isCurrentJob := a.currentJobID == job.ID
	if isCurrentJob {
		a.currentAudioFile = result.AudioFile
		a.currentAudioFileBack = result.AudioFileBack
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
			a.audioPlayer.SetAudioFile(result.AudioFile)
			if isBgBg && result.AudioFileBack != "" {
				a.audioPlayer.SetBackAudioFile(result.AudioFileBack)
			}
			a.regenerateAudioBtn.Enable()
		})
	}

	// Update phonetics immediately if available.
	if result.PhoneticInfo != "" && result.PhoneticInfo != "Failed to fetch phonetic information" {
		a.mu.Lock()
		shouldUpdate := a.currentJobID == job.ID
		if shouldUpdate {
			a.currentPhonetic = result.PhoneticInfo
		}
		a.mu.Unlock()
		if shouldUpdate {
			fmt.Printf("Updating phonetic display immediately for job %d: %s\n", job.ID, result.PhoneticInfo)
			fyne.Do(func() { a.audioPlayer.SetPhonetic(result.PhoneticInfo) })
		}
	}

	// Mark the job complete in the queue before the final UI paint.
	fyne.Do(func() { a.updateStatus(fmt.Sprintf("Finalizing '%s'...", job.Word)) })
	a.queue.CompleteJob(job.ID, translation, result.AudioFile, result.AudioFileBack, result.ImageFile)

	a.applyFinalJobUI(job, result, translation)
}

// applyFinalJobUI updates the full UI with the completed job result (translation,
// image, audio, phonetics). No-op when the job is no longer the current one.
func (a *Application) applyFinalJobUI(job *WordJob, result GenerateResult, translation string) {
	a.mu.Lock()
	isCurrentJob := a.currentJobID == job.ID
	if isCurrentJob {
		a.currentTranslation = translation
		a.currentAudioFile = result.AudioFile
		if result.ImageFile != "" {
			a.currentImage = result.ImageFile
		}
		if result.PhoneticInfo != "" && result.PhoneticInfo != "Failed to fetch phonetic information" {
			a.currentPhonetic = result.PhoneticInfo
		}
	}
	a.mu.Unlock()

	if !isCurrentJob {
		return
	}

	fyne.Do(func() {
		a.mu.Lock()
		if a.currentJobID != job.ID {
			a.mu.Unlock()
			return
		}
		a.mu.Unlock()

		a.translationEntry.SetText(translation)
		if result.ImageFile != "" {
			a.imageDisplay.SetImages([]string{result.ImageFile})
		}
		a.audioPlayer.SetAudioFile(result.AudioFile)
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

// saveJobCardType persists the card type for job to disk, failing the job on
// error. Returns nil on success.
func (a *Application) saveJobCardType(jobID int, cardDir string, isBgBg bool) error {
	cardType := internal.CardTypeEnBg
	if isBgBg {
		cardType = internal.CardTypeBgBg
	}
	if err := internal.SaveCardType(cardDir, cardType); err != nil {
		a.queue.FailJob(jobID, fmt.Errorf("failed to save card type: %w", err))
		return err
	}
	return nil
}

// resolveJobTranslation returns the translation for a job, translating via the
// orchestrator when needed. Returns the translation and true on success; false
// and fails the job on error.
func (a *Application) resolveJobTranslation(job *WordJob, isBgBg bool, cardDir string) (string, bool) {
	var translation string

	if job.NeedsTranslation && !isBgBg {
		fyne.Do(func() {
			a.updateStatus(fmt.Sprintf("Translating '%s'...", job.Word))
		})

		var err error
		translation, err = a.translateWord(job.Word)
		if err != nil {
			a.queue.FailJob(job.ID, fmt.Errorf("translation failed: %w", err))
			return "", false
		}
	} else if job.Translation != "" {
		translation = job.Translation
	}

	if translation != "" {
		if err := a.getCardService().SaveTranslation(job.Word, translation); err != nil {
			a.queue.FailJob(job.ID, fmt.Errorf("failed to save translation: %w", err))
			return "", false
		}
	}

	_ = cardDir // kept for documentation; SaveTranslation handles the dir internally
	return translation, true
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

// setupKeyboardShortcuts registers rune and key handlers on the window canvas.
// Rune events handle focus shortcuts and Cyrillic action keys; key events handle
// Latin/function keys, Escape, and Tab navigation.
func (a *Application) setupKeyboardShortcuts() {
	a.window.Canvas().SetOnTypedRune(a.handleTypedRune)
	a.window.Canvas().SetOnTypedKey(a.handleTypedKey)
}

// handleTypedRune processes character-based shortcuts, supporting both Latin and
// Cyrillic keyboard layouts. No-op when an input field is focused or a confirmation
// dialog is active.
func (a *Application) handleTypedRune(r rune) {
	focused := a.window.Canvas().Focused()
	isInputFocused := focused == a.wordInput || focused == a.imagePromptEntry || focused == a.translationEntry
	if isInputFocused || a.deleteConfirming || a.quitConfirming {
		return
	}

	switch r {
	// Focus shortcuts — move keyboard focus without typing the character.
	case 'b', 'B', 'б', 'Б':
		a.window.Canvas().Focus(a.wordInput)
	case 'e', 'E', 'е', 'Е':
		a.window.Canvas().Focus(a.translationEntry)
	case 'o', 'O', 'о', 'О':
		a.window.Canvas().Focus(a.imagePromptEntry)
	// Action shortcuts (Cyrillic equivalents; Latin equivalents handled in handleTypedKey).
	case 'г', 'Г': // г = g — generate
		if !a.submitButton.Disabled() {
			a.onSubmit()
		}
	case 'н', 'Н': // н = n — new word
		if !a.keepButton.Disabled() {
			a.onKeepAndContinue()
		}
	case 'и', 'И': // и = i — regenerate image
		if !a.regenerateImageBtn.Disabled() {
			a.onRegenerateImage()
		}
	case 'м', 'М': // м = m — random image
		if !a.regenerateRandomImageBtn.Disabled() {
			a.onRegenerateRandomImage()
		}
	case 'a', 'а': // a — regenerate front audio
		if !a.regenerateAudioBtn.Disabled() {
			a.onRegenerateAudio()
		}
	case 'A', 'А': // A — regenerate back audio (bg-bg only)
		if a.currentCardType == "bg-bg" {
			a.onRegenerateBackAudio()
		}
	case 'р', 'Р': // р = r — regenerate all
		if !a.regenerateAllBtn.Disabled() {
			a.onRegenerateAll()
		}
	case 'д', 'Д': // д = d — delete
		if !a.deleteButton.Disabled() {
			a.onDelete()
		}
	case 'p', 'п': // p — play front audio
		if a.currentAudioFile != "" {
			a.audioPlayer.Play()
		}
	case 'P', 'П': // P — play back audio (bg-bg only)
		if a.currentAudioFileBack != "" {
			a.audioPlayer.PlayBack()
		}
	case 'ж', 'Ж': // ж = x — export to Anki
		a.onExportToAnki()
	case 'в', 'В': // в = v — archive cards
		a.onArchive()
	case '?': // show hotkey reference
		a.onShowHotkeys()
	case 'h', 'H', 'х', 'Х': // h/х — previous word (vim-style)
		if !a.prevWordBtn.Disabled() {
			a.onPrevWord()
		}
	case 'l', 'L', 'л', 'Л': // l/л — next word (vim-style)
		if !a.nextWordBtn.Disabled() {
			a.onNextWord()
		}
	case 'ч', 'Ч': // ч = q — quit
		a.onQuitConfirm()
	case 'u', 'U', 'у', 'У': // u/у — toggle auto-play
		a.toggleAutoPlay()
	}
}

// handleTypedKey processes key-event shortcuts (Latin letters, arrows, Escape, Tab).
// Escape always unfocuses; Tab cycles focus. All others are ignored when an input
// field is focused or a confirmation dialog is active.
func (a *Application) handleTypedKey(ev *fyne.KeyEvent) {
	focused := a.window.Canvas().Focused()
	isInputFocused := focused == a.wordInput || focused == a.imagePromptEntry || focused == a.translationEntry

	// Escape unfocuses and clears confirmation state regardless of focus.
	if ev.Name == fyne.KeyEscape {
		a.window.Canvas().Unfocus()
		a.deleteConfirming = false
		a.quitConfirming = false
		return
	}

	// Tab cycles through input fields regardless of current focus.
	if ev.Name == fyne.KeyTab {
		a.handleTabNavigation()
		return
	}

	// Remaining shortcuts only fire when no input or dialog is active.
	if isInputFocused || a.deleteConfirming || a.quitConfirming {
		return
	}

	// Skip b/e/o here — they are handled in handleTypedRune to avoid typing the character.
	if ev.Name == fyne.KeyB || ev.Name == fyne.KeyE || ev.Name == fyne.KeyO {
		return
	}

	a.handleShortcutKey(ev.Name)
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
