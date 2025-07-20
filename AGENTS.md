# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview
**totalrecall** - Bulgarian Anki Flashcard Generator

A Go CLI tool that generates Anki flashcard materials from Bulgarian words:
- Generates audio pronunciation using OpenAI TTS
- Generates images using OpenAI DALL-E
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
go build -o totalrecall ./cmd/totalrecall

# Run without building
go run ./cmd/totalrecall "ябълка"

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
totalrecall/
├── cmd/totalrecall/          # CLI entry point
├── internal/          # Private packages
│   ├── audio/        # Audio generation (OpenAI TTS)
│   ├── image/        # Image generation functionality
│   ├── anki/         # Anki format generation
│   ├── config/       # Configuration management
│   └── version.go    # Version information
```

### Key Design Decisions
1. **OpenAI TTS**: High-quality, natural-sounding Bulgarian pronunciation
2. **Image generation**: Uses OpenAI DALL-E for AI-generated images
3. **Configuration via YAML**: User-friendly configuration with viper
4. **Cobra for CLI**: Industry-standard CLI framework

### External Dependencies
- **OpenAI API Key**: Required for both audio generation and image creation

## Testing Approach
1. Unit tests mock API calls
2. Integration tests use real services when available
3. Test with common Bulgarian words: ябълка, котка, куче, хляб

## Common Issues and Solutions


### Package Declaration Error
If you see an error about `package main`, ensure `cmd/totalrecall/main.go` has:
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
- OpenAI voices: nova, alloy, echo, shimmer (work well for Bulgarian)
