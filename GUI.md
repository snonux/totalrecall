# GUI Mode for TotalRecall

TotalRecall now includes an interactive GUI mode for a more user-friendly flashcard generation experience.

## Prerequisites

The GUI mode requires Fyne, which has the following system dependencies:

### Linux
```bash
# Debian/Ubuntu
sudo apt-get install gcc libgl1-mesa-dev xorg-dev

# Fedora
sudo dnf install gcc mesa-libGL-devel libXcursor-devel libXrandr-devel libXinerama-devel libXi-devel libXxf86vm-devel
```

### macOS
No additional dependencies required (uses system frameworks).

### Windows
No additional dependencies required if using MinGW or similar.

## Running GUI Mode

```bash
./totalrecall --gui
```

## Features

The GUI provides:

1. **Interactive Input**: Enter Bulgarian words one at a time
2. **Live Preview**: See generated images and hear audio pronunciation
3. **Fine-grained Regeneration**: 
   - Regenerate just the image (cycles through different results)
   - Regenerate just the audio (uses a different voice)
   - Regenerate both
4. **Session Management**: Keep track of all generated cards in a session
5. **Export to Anki**: Export all saved cards to CSV format

## GUI Layout

- **Top Section**: Input field for Bulgarian words with submit button
- **Middle Section**: 
  - Image display with navigation (if multiple images)
  - Audio player with play controls
  - Translation display
- **Bottom Section**: Action buttons
  - "Keep & Continue" - saves the current card
  - "Regenerate Image" - gets a new image
  - "Regenerate Audio" - generates with a different voice
  - "Regenerate All" - regenerates everything

## Building from Source

If you're building from source and encounter issues with the GUI:

1. Ensure you have the system dependencies installed (see Prerequisites)
2. The build might take longer the first time as it compiles Fyne
3. If the build times out, try building without the GUI first:
   ```bash
   go build -tags nogui ./cmd/totalrecall
   ```

## Troubleshooting

- **Build fails**: Check that you have the required system dependencies
- **GUI doesn't start**: Ensure you're running in a graphical environment
- **Audio doesn't play**: The current implementation shows audio controls but actual playback requires additional audio libraries