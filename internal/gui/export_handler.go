package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/snonux/totalrecall/internal"
	"codeberg.org/snonux/totalrecall/internal/anki"
	appconfig "codeberg.org/snonux/totalrecall/internal/config"
)

// ExportHandler owns the Export to Anki dialog and APKG/CSV export paths (SRP).
type ExportHandler struct {
	app *Application
}

// onExportToAnki opens the Export to Anki dialog where the user selects a format,
// deck name, and output directory. No-op when no exportable cards exist.
func (e *ExportHandler) onExportToAnki() {
	a := e.app
	if !e.hasExportableCards() {
		dialog.ShowInformation("No Cards", "No cards found in anki_cards folder. Generate some cards first!", a.window)
		return
	}

	formatOptions := []string{"APKG (Recommended)", "CSV (Legacy)"}
	formatSelect := widget.NewSelect(formatOptions, nil)
	formatSelect.SetSelected(formatOptions[0])
	deckNameEntry := widget.NewEntry()
	deckNameEntry.SetPlaceHolder("Bulgarian Vocabulary")

	selectedDir := e.defaultExportDir()
	dirLabel := widget.NewLabel(selectedDir)
	dirButton := widget.NewButton("Browse...", func() {
		e.browseExportDir(&selectedDir, dirLabel)
	})

	content := e.buildExportDialogContent(formatSelect, deckNameEntry, dirLabel, dirButton)
	e.showExportDialog(content, formatOptions, formatSelect, deckNameEntry, &selectedDir)
}

func (e *ExportHandler) buildExportDialogContent(formatSelect *widget.Select, deckNameEntry *widget.Entry, dirLabel *widget.Label, dirButton *widget.Button) fyne.CanvasObject {
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

func (e *ExportHandler) showExportDialog(content fyne.CanvasObject, formatOptions []string, formatSelect *widget.Select, deckNameEntry *widget.Entry, selectedDir *string) {
	a := e.app
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
		e.performExport(formatSelect.Selected == formatOptions[0], deckName, *selectedDir)
	}, a.window)

	e.wireExportDialogKeys(customDialog, &exportDialogOpen)
	customDialog.Resize(fyne.NewSize(400, 300))
	customDialog.Show()
}

// wireExportDialogKeys attaches keyboard shortcuts to the export dialog.
// e/е triggers confirm; c/ц/Esc cancels. Original handlers are restored on close.
func (e *ExportHandler) wireExportDialogKeys(customDialog *dialog.ConfirmDialog, exportDialogOpen *bool) {
	a := e.app
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

func (e *ExportHandler) hasExportableCards() bool {
	entries, err := os.ReadDir(e.app.config.OutputDir)
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

func (e *ExportHandler) defaultExportDir() string {
	homeDir, err := appconfig.HomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
	return homeDir
}

func (e *ExportHandler) browseExportDir(dir *string, label *widget.Label) {
	a := e.app
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

func (e *ExportHandler) performExport(isAPKG bool, deckName, outputDir string) {
	if isAPKG {
		e.exportAPKG(deckName, outputDir)
	} else {
		e.exportCSV(outputDir)
	}
}

func (e *ExportHandler) exportAPKG(deckName, outputDir string) {
	a := e.app
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

func (e *ExportHandler) exportCSV(outputDir string) {
	a := e.app
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
