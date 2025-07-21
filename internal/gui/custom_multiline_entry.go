package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// CustomMultiLineEntry extends widget.Entry to handle Escape key
type CustomMultiLineEntry struct {
	widget.Entry
	onEscape func()
}

// NewCustomMultiLineEntry creates a new custom multi-line entry
func NewCustomMultiLineEntry() *CustomMultiLineEntry {
	entry := &CustomMultiLineEntry{}
	entry.MultiLine = true
	entry.ExtendBaseWidget(entry)
	return entry
}

// TypedKey handles key events
func (e *CustomMultiLineEntry) TypedKey(key *fyne.KeyEvent) {
	if key.Name == fyne.KeyEscape && e.onEscape != nil {
		e.onEscape()
		return
	}
	e.Entry.TypedKey(key)
}

// SetOnEscape sets the callback for when Escape is pressed
func (e *CustomMultiLineEntry) SetOnEscape(f func()) {
	e.onEscape = f
}

// CustomEntry extends widget.Entry to handle Escape key (single-line version)
type CustomEntry struct {
	widget.Entry
	onEscape func()
}

// NewCustomEntry creates a new custom single-line entry
func NewCustomEntry() *CustomEntry {
	entry := &CustomEntry{}
	entry.ExtendBaseWidget(entry)
	return entry
}

// TypedKey handles key events
func (e *CustomEntry) TypedKey(key *fyne.KeyEvent) {
	if key.Name == fyne.KeyEscape && e.onEscape != nil {
		e.onEscape()
		return
	}
	e.Entry.TypedKey(key)
}

// SetOnEscape sets the callback for when Escape is pressed
func (e *CustomEntry) SetOnEscape(f func()) {
	e.onEscape = f
}
