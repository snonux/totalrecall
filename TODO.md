# TODO's

## Completed
- [x] Added Fyne GUI mode with `--gui` flag
- [x] Interactive word input with Bulgarian validation
- [x] Live preview of generated images and audio
- [x] Fine-grained regeneration (image-only, audio-only, or both)
- [x] Session management with "New Word" functionality
- [x] Export to Anki CSV from GUI session
- [x] Progress indicators and status updates
- [x] Concurrent word processing in GUI - users can enter new words while previous ones are being generated
- [x] Word queue system with background processing
- [x] Queue status display showing pending, processing, and completed jobs
- [x] Navigation supports both disk files and queue-completed words

## GUI Enhancements (Future)
- [ ] Implement actual audio playback (currently shows controls only)
- [ ] Add preferences dialog for GUI settings
- [ ] Add drag & drop support for batch word lists
- [ ] Add recent words history
- [ ] Add dark/light theme toggle
- [ ] Keyboard shortcuts (Enter to submit, Space to play audio)
- [ ] Save/restore GUI window size and position

## Known Limitations
- Audio playback requires additional audio library integration (e.g., github.com/hajimehoshi/oto)
- First build with GUI may be slow due to Fyne compilation

