package gui

import (
	_ "embed"
	"fyne.io/fyne/v2"
)

//go:embed totalrecall_256.png
var iconData []byte

// GetAppIcon returns the application icon as a Fyne resource
func GetAppIcon() fyne.Resource {
	return &fyne.StaticResource{
		StaticName:    "totalrecall.png",
		StaticContent: iconData,
	}
}