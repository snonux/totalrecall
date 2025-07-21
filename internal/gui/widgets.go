package gui

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ImageDisplay is a custom widget for displaying images
type ImageDisplay struct {
	widget.BaseWidget

	container   *fyne.Container
	imageCanvas *canvas.Image
	imageLabel  *widget.Label

	currentImage string
}

// NewImageDisplay creates a new image display widget
func NewImageDisplay() *ImageDisplay {
	d := &ImageDisplay{}

	// Create image canvas
	d.imageCanvas = canvas.NewImageFromResource(nil)
	d.imageCanvas.FillMode = canvas.ImageFillContain
	d.imageCanvas.SetMinSize(fyne.NewSize(200, 150)) // Half the size

	// Create label
	d.imageLabel = widget.NewLabel("No image")
	d.imageLabel.Alignment = fyne.TextAlignCenter

	// Create main container - no navigation buttons here
	d.container = container.NewBorder(
		nil,
		d.imageLabel,
		nil, nil,
		d.imageCanvas,
	)

	d.ExtendBaseWidget(d)
	return d
}

// CreateRenderer implements fyne.Widget
func (d *ImageDisplay) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(d.container)
}

// SetImage sets a single image to display
func (d *ImageDisplay) SetImage(imagePath string) {
	if imagePath == "" {
		d.Clear()
		return
	}

	d.currentImage = imagePath

	// Load image from file
	file, err := os.Open(imagePath)
	if err != nil {
		d.imageLabel.SetText(fmt.Sprintf("Error loading image: %v", err))
		return
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		d.imageLabel.SetText(fmt.Sprintf("Error decoding image: %v", err))
		return
	}

	// Update canvas
	d.imageCanvas.Image = img
	d.imageCanvas.Refresh()

	// Update label
	d.imageLabel.SetText(filepath.Base(imagePath))
}

// SetImages sets multiple images but only displays the first one
func (d *ImageDisplay) SetImages(images []string) {
	if len(images) > 0 {
		d.SetImage(images[0])
	} else {
		d.Clear()
	}
}

// Clear clears the display
func (d *ImageDisplay) Clear() {
	d.currentImage = ""
	d.imageCanvas.Image = nil
	d.imageCanvas.Refresh()
	d.imageLabel.SetText("No image")
}

// SetGenerating shows a generating status
func (d *ImageDisplay) SetGenerating() {
	d.currentImage = ""
	d.imageCanvas.Image = nil
	d.imageCanvas.Refresh()
	d.imageLabel.SetText("Generating...")
}

// ResourceFromPath creates a Fyne resource from a file path
func ResourceFromPath(path string) (fyne.Resource, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return fyne.NewStaticResource(filepath.Base(path), data), nil
}
