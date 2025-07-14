# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview
**bulg** - Bulgarian Anki Flashcard Generator

A Go CLI tool that generates Anki flashcard materials from Bulgarian words:
- Generates audio pronunciation using espeak-ng
- Downloads representative images via web search
- Creates Anki-compatible output files

## Important: Task Tracking
**Always check TODO.md for the current implementation status and pending tasks.** The TODO.md file contains a comprehensive breakdown of all features and their completion status.

## Build and Development Commands

### Available Tasks (via Taskfile)
```bash
# Build the binary
task
# or
task default

# Run the application
task run

# Run tests
task test

# Install to Go bin directory
task install
```

### Common Development Commands
```bash
# Build for current platform
go build -o bulg ./cmd/bulg

# Run without building
go run ./cmd/bulg "ябълка"

# Run tests with coverage
go test -v -cover ./...

# Check for race conditions
go test -race ./...

# Format code
go fmt ./...

# Lint code (requires golangci-lint)
golangci-lint run
```

## Architecture Overview

### Package Structure
```
bulg/
├── cmd/bulg/          # CLI entry point
├── internal/          # Private packages
│   ├── audio/        # Audio generation (espeak-ng wrapper)
│   ├── image/        # Image search functionality
│   ├── anki/         # Anki format generation
│   ├── config/       # Configuration management
│   └── version.go    # Version information
```

### Key Design Decisions
1. **espeak-ng for TTS**: Open source, supports Bulgarian, no API keys needed
2. **Modular image search**: Support multiple providers (Pixabay, Unsplash)
3. **Configuration via YAML**: User-friendly configuration with viper
4. **Cobra for CLI**: Industry-standard CLI framework

### External Dependencies
- **espeak-ng**: Must be installed on the system
  ```bash
  # Ubuntu/Debian
  sudo apt-get install espeak-ng
  
  # macOS
  brew install espeak-ng
  ```

### API Configuration
Image search APIs require configuration in `.bulg.yaml`:
- **Pixabay**: Optional API key for higher rate limits
- **Unsplash**: Required API key

## Testing Approach
1. Unit tests mock external commands (espeak-ng) and API calls
2. Integration tests use real services when available
3. Test with common Bulgarian words: ябълка, котка, куче, хляб

## Common Issues and Solutions

### espeak-ng Bulgarian pronunciation
There have been reported issues with Bulgarian pronunciation in espeak-ng v1.49.3. If pronunciation sounds wrong, try:
```bash
# Check version
espeak-ng --version

# Test Bulgarian voice
espeak-ng -v bg "Здравей"

# Try different voice variants
espeak-ng -v bg+f1 "Здравей"
```

### Package Declaration Error
If you see an error about `package main`, ensure `cmd/bulg/main.go` has:
```go
package main  // NOT package bulg
```

## Development Workflow
1. Check TODO.md for next tasks
2. Create feature branch
3. Implement with tests
4. Update documentation
5. Run full test suite
6. Submit for review

## Bulgarian Language Notes
- Input should be in Cyrillic script
- Common test words: ябълка (apple), котка (cat), куче (dog)
- Voice variants: bg+m1 (male), bg+f1 (female)