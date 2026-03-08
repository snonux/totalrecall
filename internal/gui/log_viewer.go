package gui

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// LogWriter is a custom writer that captures log output
type LogWriter struct {
	viewer   *LogViewer
	original *os.File
}

// Write implements io.Writer
func (w *LogWriter) Write(p []byte) (n int, err error) {
	// Write to original output
	if w.original != nil {
		if _, err := w.original.Write(p); err != nil {
			return 0, err
		}
	}

	// Send to log viewer
	if w.viewer != nil {
		message := strings.TrimRight(string(p), "\n")
		if message != "" {
			w.viewer.AddMessage(message)
		}
	}

	return len(p), nil
}

// LogViewer is a widget that displays log messages
type LogViewer struct {
	widget.BaseWidget

	container  *fyne.Container
	logEntry   *widget.Entry
	scrollView *container.Scroll

	mu          sync.Mutex
	messages    []string
	maxMessages int

	// For capturing output
	originalStdout *os.File
	originalStderr *os.File
	stdoutWriter   *LogWriter
	stderrWriter   *LogWriter
	stdoutR        *os.File
	stdoutW        *os.File
	stderrR        *os.File
	stderrW        *os.File
}

// NewLogViewer creates a new log viewer widget
func NewLogViewer() *LogViewer {
	v := &LogViewer{
		maxMessages: 1000, // Keep last 1000 messages
		messages:    make([]string, 0),
	}

	// Create log entry (read-only multiline)
	v.logEntry = widget.NewMultiLineEntry()
	v.logEntry.Disable() // Make it read-only
	v.logEntry.Wrapping = fyne.TextWrapWord

	// Create scroll container
	v.scrollView = container.NewScroll(v.logEntry)
	v.scrollView.SetMinSize(fyne.NewSize(0, 180)) // Same height as old phonetic display

	// Configure scroll behavior
	v.scrollView.Direction = container.ScrollBoth

	// Create container with label
	v.container = container.NewBorder(
		widget.NewLabel("Log messages (newest first):"),
		nil,
		nil,
		nil,
		v.scrollView,
	)

	v.ExtendBaseWidget(v)
	return v
}

// CreateRenderer implements fyne.Widget
func (v *LogViewer) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(v.container)
}

// StartCapture starts capturing stdout and stderr
func (v *LogViewer) StartCapture() {
	// Save original outputs
	v.originalStdout = os.Stdout
	v.originalStderr = os.Stderr

	// Create custom writers
	v.stdoutWriter = &LogWriter{viewer: v, original: v.originalStdout}
	v.stderrWriter = &LogWriter{viewer: v, original: v.originalStderr}

	// Create pipe for stdout
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		if v.originalStderr != nil {
			_, _ = fmt.Fprintf(v.originalStderr, "Warning: failed to create stdout capture pipe: %v\n", err)
		}
		return
	}
	v.stdoutR = stdoutR
	v.stdoutW = stdoutW
	os.Stdout = stdoutW

	// Create pipe for stderr
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		if v.originalStderr != nil {
			_, _ = fmt.Fprintf(v.originalStderr, "Warning: failed to create stderr capture pipe: %v\n", err)
		}
		os.Stdout = v.originalStdout
		_ = stdoutR.Close()
		_ = stdoutW.Close()
		v.stdoutR = nil
		v.stdoutW = nil
		return
	}
	v.stderrR = stderrR
	v.stderrW = stderrW
	os.Stderr = stderrW

	// Also redirect log package output
	log.SetOutput(v.stdoutWriter)

	// Start goroutines to read from pipes
	go v.pipeReader(stdoutR, v.stdoutWriter)
	go v.pipeReader(stderrR, v.stderrWriter)
}

// pipeReader reads from a pipe and writes to a LogWriter
func (v *LogViewer) pipeReader(pipe *os.File, writer *LogWriter) {
	defer func() {
		if pipe != nil {
			_ = pipe.Close()
		}
	}()

	buf := make([]byte, 1024)
	for {
		n, err := pipe.Read(buf)
		if n > 0 {
			if _, writeErr := writer.Write(buf[:n]); writeErr != nil {
				break
			}
		}
		if err != nil {
			break
		}
	}
}

// StopCapture stops capturing stdout and stderr
func (v *LogViewer) StopCapture() {
	if v.originalStdout != nil {
		os.Stdout = v.originalStdout
		v.originalStdout = nil
	}
	if v.originalStderr != nil {
		os.Stderr = v.originalStderr
		v.originalStderr = nil
	}

	// Reset log package output
	log.SetOutput(os.Stderr)

	// Close write ends to unblock reader goroutines.
	if v.stdoutW != nil {
		_ = v.stdoutW.Close()
		v.stdoutW = nil
	}
	if v.stderrW != nil {
		_ = v.stderrW.Close()
		v.stderrW = nil
	}

	// Close read ends if still open.
	if v.stdoutR != nil {
		_ = v.stdoutR.Close()
		v.stdoutR = nil
	}
	if v.stderrR != nil {
		_ = v.stderrR.Close()
		v.stderrR = nil
	}
}

// AddMessage adds a message to the log
func (v *LogViewer) AddMessage(message string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Add timestamp
	timestamp := time.Now().Format("15:04:05")
	fullMessage := fmt.Sprintf("[%s] %s", timestamp, message)

	// Prepend to messages (newest first)
	v.messages = append([]string{fullMessage}, v.messages...)

	// Trim if too many messages (remove oldest from the end)
	if len(v.messages) > v.maxMessages {
		v.messages = v.messages[:v.maxMessages]
	}

	// Update UI on main thread
	fyne.Do(func() {
		// Set the full text with newest messages at top
		text := strings.Join(v.messages, "\n")
		v.logEntry.SetText(text)

		// Keep scroll at top to show newest messages
		v.scrollView.Offset = fyne.NewPos(0, 0)
		v.scrollView.Refresh()
	})
}

// Clear clears all log messages
func (v *LogViewer) Clear() {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.messages = v.messages[:0]

	fyne.Do(func() {
		v.logEntry.SetText("")
		v.scrollView.Offset = fyne.NewPos(0, 0)
		v.scrollView.Refresh()
	})
}

// Log adds a message without timestamp (for internal use)
func (v *LogViewer) Log(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	v.AddMessage(message)
}
