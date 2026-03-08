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

	d.imageCanvas = newImageCanvas()
	d.imageLabel = newImageLabel()
	d.container = newImageContainer(d.imageCanvas, d.imageLabel)

	d.ExtendBaseWidget(d)
	return d
}

func newImageCanvas() *canvas.Image {
	img := canvas.NewImageFromResource(nil)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(200, 150)) // Half the size
	return img
}

func newImageLabel() *widget.Label {
	label := widget.NewLabel("No image")
	label.Alignment = fyne.TextAlignCenter
	return label
}

func newImageContainer(img *canvas.Image, label *widget.Label) *fyne.Container {
	// Create main container with the image centered and its label at the bottom.
	return container.NewBorder(
		nil,
		label,
		nil, nil,
		img,
	)
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
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close image file %q: %v\n", imagePath, closeErr)
		}
	}()

	// Get file info to ensure it's fully written
	stat, err := file.Stat()
	if err != nil {
		d.imageLabel.SetText(fmt.Sprintf("Error getting file info: %v", err))
		return
	}

	// If file size is 0, it might still be writing
	if stat.Size() == 0 {
		d.imageLabel.SetText("Image file is empty")
		return
	}

	img, format, err := image.Decode(file)
	if err != nil {
		d.imageLabel.SetText(fmt.Sprintf("Error decoding image: %v", err))
		return
	}

	// Update canvas
	d.imageCanvas.Image = img
	d.imageCanvas.Refresh()

	// Update label with format info
	d.imageLabel.SetText(fmt.Sprintf("%s (%s)", filepath.Base(imagePath), format))
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
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close resource file %q: %v\n", path, closeErr)
		}
	}()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return fyne.NewStaticResource(filepath.Base(path), data), nil
}
