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

	"codeberg.org/snonux/totalrecall/internal"
	"codeberg.org/snonux/totalrecall/internal/anki"
	"codeberg.org/snonux/totalrecall/internal/archive"
	"codeberg.org/snonux/totalrecall/internal/audio"
	appconfig "codeberg.org/snonux/totalrecall/internal/config"
	"codeberg.org/snonux/totalrecall/internal/image"
	"codeberg.org/snonux/totalrecall/internal/phonetic"
	"codeberg.org/snonux/totalrecall/internal/translation"
)

// App is the runnable GUI application constructed at the composition root
// (cmd/totalrecall). Callers invoke Run() to start the Fyne event loop.
type App interface {
	Run()
}

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
	prevWordBtn      *ttwidget.Button
	nextWordBtn      *ttwidget.Button
	cardCounterLabel *widget.Label // displays "Card X / N"

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
	archiver        archive.Archiver
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

	// Focused sub-handlers (SRP); initialized lazily via ensureHandlers.
	nav      *NavigationHandler
	export   *ExportHandler
	queueMgr *QueueManager
	keys     *KeyboardShortcuts
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
	// Archiver moves the cards directory to a timestamped archive; nil uses
	// archive.DefaultArchiver.
	Archiver archive.Archiver
}

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
		NanoBananaModel:     appconfig.DefaultNanoBananaModel,
		NanoBananaTextModel: appconfig.DefaultNanoBananaTextModel,
		GeminiTTSModel:      audioDefaults.GeminiTTSModel,
		ImageProvider:       image.ImageProviderNanoBanana,
		TranslationProvider: translation.ProviderGemini,
		PhoneticProvider:    phonetic.ProviderGemini,
		AutoPlay:            true, // Auto-play enabled by default
	}
}

// New constructs and returns a fully initialised App for the given config.
// A nil config receives all defaults. The Fyne application and UI are created here;
// callers should call Run() to start the event loop.
func New(config *Config) App {
	config = applyConfigDefaults(config)

	arch := config.Archiver
	if arch == nil {
		arch = archive.DefaultArchiver{}
	}

	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create output directory %q: %v\n", config.OutputDir, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	myApp := app.NewWithID("org.codeberg.snonux.totalrecall")
	myApp.SetIcon(GetAppIcon())

	a := &Application{
		app:              myApp,
		config:           config,
		archiver:         arch,
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
			config.AudioFormat = appconfig.DefaultAudioOutputFormat
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
	a.ensureHandlers()
	a.queue = NewWordQueue(a.ctx)
	a.queue.SetCallbacks(a.queueMgr.onQueueStatusUpdate, a.queueMgr.onJobComplete)

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
			outputFormat = appconfig.DefaultAudioOutputFormat
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
	// Tracked via WaitGroup and respects ctx.Done() so the callback never
	// writes to freed widgets after the window is closed (Go Mistake #62).
	// NewTimer (not time.After) avoids leaking a pending timer when ctx wins
	// the select. After the delay, ctx is checked again before and inside
	// fyne.Do: the timer can fire just before onWindowClosed cancels ctx, so
	// the UI-thread callback must no-op if shutdown already started.
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		t := time.NewTimer(500 * time.Millisecond)
		defer t.Stop()
		select {
		case <-a.ctx.Done():
			return
		case <-t.C:
		}
		if a.ctx.Err() != nil {
			return
		}
		fyne.Do(func() {
			if a.ctx.Err() != nil {
				return
			}
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
	}()

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
	// Wire the application context into the player so its post-playback goroutine
	// can check ctx.Done() before writing to widgets (Go Mistake #62).
	a.audioPlayer.SetContext(a.ctx)
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
// the window. It contains the status label, queue status, a card counter
// ("Card X / N") and the version string.
func (a *Application) buildStatusSection() fyne.CanvasObject {
	a.statusLabel = widget.NewLabel("Ready")
	a.queueStatusLabel = widget.NewLabel("Queue: Empty")
	a.queueStatusLabel.TextStyle = fyne.TextStyle{Italic: true}

	// Card counter — updated by updateCardCounter whenever navigation changes.
	a.cardCounterLabel = widget.NewLabel("")
	a.cardCounterLabel.TextStyle = fyne.TextStyle{Italic: true}
	a.cardCounterLabel.Alignment = fyne.TextAlignTrailing

	versionLabel := widget.NewLabel(fmt.Sprintf("v%s", internal.Version))
	versionLabel.TextStyle = fyne.TextStyle{Italic: true}
	versionLabel.Alignment = fyne.TextAlignTrailing

	// Right-side column: card counter above version string.
	rightCol := container.NewVBox(a.cardCounterLabel, versionLabel)

	return container.NewBorder(
		nil, nil, nil, rightCol,
		container.NewVBox(a.statusLabel, widget.NewSeparator(), a.queueStatusLabel),
	)
}

// updateCardCounter refreshes the card-counter label to "Card X / N" where X
// is the 1-based position of the current word in the combined word list and N
// is the total number of available words. When no words are present the label
// is cleared.
func (a *Application) updateCardCounter(currentIndex, total int) {
	if a.cardCounterLabel == nil {
		return
	}
	if total == 0 {
		a.cardCounterLabel.SetText("")
		return
	}
	a.cardCounterLabel.SetText(fmt.Sprintf("Card %d / %d", currentIndex+1, total))
}

// onWindowClosed is called when the window is closed. It stops background
// goroutines, cancels ongoing operations, and shuts down the application.
func (a *Application) onWindowClosed() {
	if a.fileCheckTicker != nil {
		a.fileCheckTicker.Stop()
	}
	// Stop the word-change debounce timer so its callback cannot fire
	// after the context is cancelled and widgets are freed.
	if a.wordChangeTimer != nil {
		a.wordChangeTimer.Stop()
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

// onExportToAnki delegates to ExportHandler.
func (a *Application) onExportToAnki() {
	a.ensureHandlers()
	a.export.onExportToAnki()
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

	if err := a.archiver.ArchiveCards(cardsDir); err != nil {
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

// onShowHotkeys delegates to KeyboardShortcuts.
func (a *Application) onShowHotkeys() {
	a.ensureHandlers()
	a.keys.onShowHotkeys()
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
// A tracked goroutine waits 500 ms and then sets tooltips on the main thread.
// WaitGroup tracking and ctx.Done() ensure the callback never writes to freed
// widgets after the window is closed (Go Mistake #62).
func (a *Application) setupTooltips() {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		select {
		case <-a.ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
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

			// Export and help button tooltips are set in the main window setup
			// goroutine (see setupUI) to avoid a double 500 ms wait here.

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
	}()
}

// ensureHandlers lazily wires NavigationHandler, ExportHandler, QueueManager, and KeyboardShortcuts.
func (a *Application) ensureHandlers() {
	if a.nav == nil {
		a.nav = &NavigationHandler{app: a}
	}
	if a.export == nil {
		a.export = &ExportHandler{app: a}
	}
	if a.queueMgr == nil {
		a.queueMgr = &QueueManager{app: a}
	}
	if a.keys == nil {
		a.keys = &KeyboardShortcuts{app: a}
	}
}

func (a *Application) scanExistingWords() {
	a.ensureHandlers()
	a.nav.scanExistingWords()
}

func (a *Application) loadWordByIndex(index int) {
	a.ensureHandlers()
	a.nav.loadWordByIndex(index)
}

func (a *Application) loadExistingFiles(word string) {
	a.ensureHandlers()
	a.nav.loadExistingFiles(word)
}

func (a *Application) onPrevWord() {
	a.ensureHandlers()
	a.nav.onPrevWord()
}

func (a *Application) onNextWord() {
	a.ensureHandlers()
	a.nav.onNextWord()
}

func (a *Application) onDelete() {
	a.ensureHandlers()
	a.nav.onDelete()
}

func (a *Application) processNextInQueue() {
	a.ensureHandlers()
	a.queueMgr.processNextInQueue()
}

func (a *Application) updateQueueStatus() {
	a.ensureHandlers()
	a.queueMgr.updateQueueStatus()
}

func (a *Application) getOrCreateCardContext(word string) (context.Context, context.CancelFunc) {
	a.ensureHandlers()
	return a.queueMgr.getOrCreateCardContext(word)
}

func (a *Application) startOperation(word string) {
	a.ensureHandlers()
	a.queueMgr.startOperation(word)
}

func (a *Application) endOperation(word string) {
	a.ensureHandlers()
	a.queueMgr.endOperation(word)
}

func (a *Application) incrementProcessing() {
	a.ensureHandlers()
	a.queueMgr.incrementProcessing()
}

func (a *Application) decrementProcessing() {
	a.ensureHandlers()
	a.queueMgr.decrementProcessing()
}

func (a *Application) setupKeyboardShortcuts() {
	a.ensureHandlers()
	a.keys.setupKeyboardShortcuts()
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

		// Small delay to ensure UI updates. Tracked via WaitGroup and respects
		// ctx.Done() to prevent writing to freed widgets on shutdown (Go Mistake #62).
		a.wg.Add(1)
		go func() {
			defer a.wg.Done()
			select {
			case <-a.ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
			}
			fyne.Do(func() {
				a.onRegenerateImage()
			})
		}()
	}
}
