package gui

import (
	"fmt"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/snonux/totalrecall/internal/cli"
)

// showGalleryVideoDialog presents a multi-checkbox list of gallery PNG paths
// so the user can choose which pages to animate into MP4 clips using Veo.
// All items are selected by default. On confirmation the selected paths are
// passed to cli.GenerateSelectedVideos which runs in a background goroutine
// while a progress dialog keeps the user informed.
//
// galleryPNGs contains the absolute paths of candidate PNG files (typically
// story page images). outputDir is informational — videos are always written
// next to their source PNGs by the generator.
// apiKey is the Google/Gemini API key required by the Veo model.
// The function must be called on the Fyne main goroutine.
func (a *Application) showGalleryVideoDialog(galleryPNGs []string, outputDir, apiKey string) {
	if len(galleryPNGs) == 0 {
		dialog.ShowInformation("No Gallery Images",
			"No gallery PNG images were found to generate videos from.", a.window)
		return
	}

	// Build checkbox list — all ticked by default.
	selected := buildSelectionMap(galleryPNGs)
	checkboxes := buildCheckboxList(galleryPNGs, selected)

	content := buildGalleryDialogContent(outputDir, checkboxes)

	// Show a custom confirm dialog. On "Generate" the user-selected subset is
	// collected and passed to the video generator in a goroutine.
	customDialog := dialog.NewCustomConfirm(
		"Generate Videos",
		"Generate",
		"Cancel",
		content,
		func(confirmed bool) {
			if !confirmed {
				return
			}
			paths := collectSelectedPaths(galleryPNGs, selected)
			a.runVideoGeneration(paths, apiKey)
		},
		a.window,
	)

	customDialog.Resize(fyne.NewSize(520, 400))
	customDialog.Show()
}

// buildSelectionMap creates a map of path -> *bool with all entries set to true
// so every gallery image is selected by default.
func buildSelectionMap(galleryPNGs []string) map[string]*bool {
	selected := make(map[string]*bool, len(galleryPNGs))
	for _, p := range galleryPNGs {
		v := true
		selected[p] = &v
	}
	return selected
}

// buildCheckboxList creates one labelled checkbox per gallery PNG. Each checkbox
// mutates the corresponding *bool in selected so the confirm callback can read
// the final state without iterating widgets again.
func buildCheckboxList(galleryPNGs []string, selected map[string]*bool) []fyne.CanvasObject {
	checkboxes := make([]fyne.CanvasObject, 0, len(galleryPNGs))
	for _, p := range galleryPNGs {
		p := p // capture loop variable
		label := filepath.Base(p)
		check := widget.NewCheck(label, func(checked bool) {
			*selected[p] = checked
		})
		check.SetChecked(true)
		checkboxes = append(checkboxes, check)
	}
	return checkboxes
}

// buildGalleryDialogContent assembles the scrollable VBox shown inside the
// gallery video dialog. outputDir is displayed as a hint so the user knows
// where the output will land.
func buildGalleryDialogContent(outputDir string, checkboxes []fyne.CanvasObject) fyne.CanvasObject {
	hint := widget.NewLabel(fmt.Sprintf("Output directory: %s\nSelect which pages to animate:", outputDir))
	hint.Wrapping = fyne.TextWrapWord

	checkList := container.NewVBox(checkboxes...)
	scroll := container.NewVScroll(checkList)
	scroll.SetMinSize(fyne.NewSize(480, 260))

	return container.NewVBox(hint, widget.NewSeparator(), scroll)
}

// collectSelectedPaths returns the subset of galleryPNGs for which the user
// left the checkbox ticked.
func collectSelectedPaths(galleryPNGs []string, selected map[string]*bool) []string {
	paths := make([]string, 0, len(galleryPNGs))
	for _, p := range galleryPNGs {
		if v, ok := selected[p]; ok && *v {
			paths = append(paths, p)
		}
	}
	return paths
}

// runVideoGeneration shows an infinite-progress dialog and calls
// cli.GenerateSelectedVideos in a background goroutine. The progress dialog is
// dismissed and the result is shown to the user once generation completes.
// Must be called on the Fyne main goroutine.
func (a *Application) runVideoGeneration(paths []string, apiKey string) {
	if len(paths) == 0 {
		dialog.ShowInformation("Nothing Selected",
			"No images were selected for video generation.", a.window)
		return
	}

	// Infinite progress bar to indicate background work.
	progressDialog := dialog.NewCustom(
		"Generating Videos",
		"Please wait…",
		buildProgressContent(len(paths)),
		a.window,
	)
	progressDialog.Show()

	// Run generation in a background goroutine; update the UI when done.
	go func() {
		err := cli.GenerateSelectedVideos(apiKey, paths)
		fyne.Do(func() {
			progressDialog.Hide()
			if err != nil {
				dialog.ShowError(fmt.Errorf("video generation failed: %w", err), a.window)
				return
			}
			dialog.ShowInformation("Videos Generated",
				fmt.Sprintf("Successfully generated %d video(s).\nVideos are saved next to their source images.", len(paths)),
				a.window,
			)
		})
	}()
}

// buildProgressContent returns the widget shown inside the progress dialog:
// a spinning activity indicator together with a short explanatory label.
func buildProgressContent(count int) fyne.CanvasObject {
	bar := widget.NewProgressBarInfinite()
	label := widget.NewLabel(fmt.Sprintf("Generating %d video(s) with Veo — this may take several minutes…", count))
	label.Wrapping = fyne.TextWrapWord
	return container.NewVBox(label, bar)
}
