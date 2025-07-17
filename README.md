# totalrecall - Bulgarian Anki Flashcard Generator

`totalrecall` is a versatile tool for generating Anki flashcard materials from Bulgarian words. It offers both a command-line interface (CLI) and a graphical user interface (GUI) for creating audio pronunciation files and AI-generated images.

It has mainly been vibe coded using Claude Code CLI.

⚠️ **Important:** This tool uses OpenAI services for audio generation, which requires an API key. See [Quick Start](#quick-start) for setup instructions.

## Features

### Core Features
- Audio generation using **OpenAI TTS**: High-quality, natural-sounding voices (requires API key)
  - Random voice selection by default for variety
  - Option to generate in all 11 available voices
- Automatic Bulgarian to English translation
  - Saves translations to separate text files
  - Includes translations in Anki CSV export
- Image generation:
  - **OpenAI DALL-E**: AI-generated educational images with contextual scenes and random art styles (requires API key)
  - Scene generation creates memorable contexts for each word
- Batch processing of multiple words
- Anki-compatible CSV export with translations
- Configurable voice variants and speech speed
- Support for WAV and MP3 audio formats
- Audio and image caching to save API costs

### GUI Mode Features (--gui flag)
- **Interactive flashcard management** with visual interface
- **Real-time preview** of generated images and audio
- **Keyboard shortcuts** for efficient workflow:
  - `G` - Generate new word
  - `N` - New card
  - `I` - Regenerate image
  - `A` - Regenerate audio
  - `R` - Regenerate all
  - `D` - Delete card
  - `P` - Play audio
  - `←/→` - Navigate cards
- **Custom image prompts** with dedicated text area
- **Queue system** for processing multiple words concurrently
- **Built-in audio player** with system integration
- **Browse existing flashcards** with navigation controls

## Installation

### Prerequisites

1. **For OpenAI TTS** (required for audio generation):
   - Create an account at https://platform.openai.com
   - Generate an API key at https://platform.openai.com/api-keys
   - Set the key using one of these methods:
     - Environment variable: `export OPENAI_API_KEY="sk-..."`
     - Configuration file: Add to `.totalrecall.yaml`

### Building from Source

```bash
git clone https://codeberg.org/snonux/totalrecall.git
cd totalrecall
go build -o totalrecall ./cmd/totalrecall
```

Or install directly:

```bash
go install codeberg.org/snonux/totalrecall/cmd/totalrecall@latest
```

## Quick Start

**Note:** By default, totalrecall uses OpenAI for both audio and images. Make sure to set your OpenAI API key:
```bash
export OPENAI_API_KEY="sk-..."
```

### CLI Mode
1. Generate materials for a single word (uses OpenAI by default):
   ```bash
   totalrecall ябълка
   ```

2. Generate with specific DALL-E model:
   ```bash
   totalrecall ябълка --openai-image-model dall-e-3
   ```

3. Process multiple words from a file:
   ```bash
   totalrecall --batch words.txt
   ```

4. Generate with Anki CSV:
   ```bash
   totalrecall ябълка --anki
   ```

### GUI Mode
Launch the interactive graphical interface:
```bash
totalrecall --gui
```

Then use keyboard shortcuts or buttons to generate and manage flashcards interactively.

## Configuration

Create a `.totalrecall.yaml` file in your home directory or project folder:

```yaml
audio:
  format: mp3           # Audio format (wav or mp3)
  
  # OpenAI settings
  openai_key: "sk-..."  # Your OpenAI API key
  openai_model: "gpt-4o-mini-tts" # Model: tts-1, tts-1-hd, or gpt-4o-mini-tts
  openai_voice: "nova"  # Voice: alloy, ash, ballad, coral, echo, fable, onyx, nova, sage, shimmer, verse
  openai_speed: 0.8     # Speed: 0.25 to 4.0 (may be ignored by gpt-4o-mini models)
  openai_instruction: "You are speaking Bulgarian language (български език). Pronounce the Bulgarian text with authentic Bulgarian phonetics, not Russian." # For gpt-4o-mini models only
  
  # Caching
  enable_cache: true
  cache_dir: "./.audio_cache"

image:
  provider: openai       # Image provider (currently only openai is supported)
  
  # OpenAI DALL-E settings
  openai_model: "dall-e-2"  # Model: dall-e-2 or dall-e-3
  openai_size: "512x512"    # Size: 256x256, 512x512, 1024x1024
  openai_quality: "standard" # Quality: standard or hd (dall-e-3 only)
  openai_style: "natural"    # Style: natural or vivid (dall-e-3 only)
  
  # Caching
  enable_cache: true
  cache_dir: "./.image_cache"

output:
  directory: ./anki_cards
  naming: "{word}_{type}"
```

## Usage

### CLI Mode
```bash
totalrecall [word] [flags]
```

### GUI Mode
```bash
totalrecall --gui
```

### Common Flags

- `--gui`: Launch interactive GUI mode
- `-v, --voice string`: Voice variant (default "bg+f1")
- `-o, --output string`: Output directory (default "./anki_cards")
- `-f, --format string`: Audio format - wav or mp3 (default "mp3")
- `--batch string`: Process words from file (one per line) [CLI mode only]
- `--anki`: Generate Anki import CSV file [CLI mode only]
- `--skip-audio`: Skip audio generation
- `--skip-images`: Skip image download
- `--images-per-word int`: Number of images per word (default 1)
- `--image-api string`: Image source - currently only openai is supported (default "openai")
- `--all-voices`: Generate audio in all available OpenAI voices (creates 11 files per word)

#### Audio Options

#### OpenAI Audio Options
- `--openai-model string`: Model - tts-1, tts-1-hd, or gpt-4o-mini-tts (default "gpt-4o-mini-tts", requires special access)
- `--openai-voice string`: Voice - alloy, ash, ballad, coral, echo, fable, onyx, nova, sage, shimmer, verse (default: random)
- `--openai-speed float`: Speech speed 0.25-4.0 (default 0.8, may be ignored by gpt-4o-mini-tts)
- `--openai-instruction string`: Voice instructions for gpt-4o-mini-tts model (e.g., "speak with a Bulgarian accent")

#### OpenAI Image Options
- `--openai-image-model string`: Model - dall-e-2 or dall-e-3 (default "dall-e-2")
- `--openai-image-size string`: Size - 256x256, 512x512, 1024x1024 (default "512x512")
- `--openai-image-quality string`: Quality - standard or hd (default "standard", dall-e-3 only)
- `--openai-image-style string`: Style - natural or vivid (default "natural", dall-e-3 only)

## API Keys

### OpenAI
- Required for both OpenAI TTS audio and DALL-E image generation
- Get your key at: https://platform.openai.com/api-keys
- Set via environment variable: `export OPENAI_API_KEY="sk-..."`
- Or add to config file as `audio.openai_key`

## Examples

### GUI Mode
```bash
# Launch interactive GUI
totalrecall --gui

# In GUI mode, use these keyboard shortcuts:
# G - Generate new word
# I - Regenerate image with new style
# A - Regenerate audio with different voice
# P - Play audio
# ←/→ - Navigate between cards
```

### CLI Mode - Basic Usage
```bash
# Single word (uses OpenAI by default)
totalrecall котка

# High-quality OpenAI with specific voice
totalrecall ябълка --openai-model tts-1-hd --openai-voice alloy

# Use gpt-4o-mini-tts with custom voice instructions
totalrecall ябълка --openai-instruction "Speak like a patient Bulgarian teacher, very slowly and clearly"

# Multiple words with custom output
totalrecall --batch animals.txt -o ./animal_cards

# Skip images, audio only
totalrecall куче --skip-images

# Generate Anki import file
totalrecall --batch words.txt --anki

# Generate AI images with OpenAI DALL-E
totalrecall ябълка --image-api openai

# High-quality DALL-E 3 images
totalrecall котка --image-api openai --openai-image-model dall-e-3 --openai-image-quality hd

# Combine OpenAI audio and images
totalrecall куче --image-api openai

# Generate audio in all 11 OpenAI voices
totalrecall котка --all-voices --skip-images
```

### Batch File Format
Create a text file with one Bulgarian word per line:
```
ябълка
котка
куче
хляб
вода
```

### Output Files
For each word, the tool generates:
- `word.mp3` - Audio pronunciation (random voice)
- `word_translation.txt` - English translation
- `word_1.jpg`, `word_2.jpg`, etc. - Generated images
- `anki_import.csv` - Anki import file (when using --anki flag)

With `--all-voices` flag:
- `word_alloy.mp3`, `word_nova.mp3`, etc. - Audio in all 11 voices

## Anki Import

1. Generate materials with the `--anki` flag
2. In Anki, go to File → Import
3. Select the generated `anki_import.csv`
4. Copy all media files to your Anki media folder
5. Map fields appropriately during import

## GUI Mode Keyboard Shortcuts

When using the GUI mode, these keyboard shortcuts are available:
- `G` - Generate: Submit new word for processing
- `N` - New Word: Save current card and start fresh
- `I` - Regenerate Image: Generate new image with different style
- `A` - Regenerate Audio: Generate new audio with different voice
- `R` - Regenerate All: Regenerate both audio and image
- `D` - Delete: Delete current flashcard materials
- `P` - Play: Play the generated audio file
- `←/→` - Navigate: Browse through existing flashcards
- `Escape` - Cancel current operations

## Voice Variants

Available Bulgarian voices:
- `bg` - Default Bulgarian voice
- `bg+m1`, `bg+m2`, `bg+m3` - Male voices
- `bg+f1`, `bg+f2`, `bg+f3` - Female voices

## Troubleshooting


### No images found
- Check your internet connection
- Verify API keys in configuration
- Try using English translations for better results

### OpenAI API errors
- Verify your API key is correct and has credits

## Cost Considerations

### OpenAI Services
- **TTS Audio**: ~$0.015 per 1K characters (tts-1), ~$0.030 (tts-1-hd)
- **DALL-E 2 Images**: ~$0.02 per image (512x512)
- **DALL-E 3 Images**: ~$0.04 per image (standard), ~$0.08 (HD)
- Both services cache results to avoid regenerating identical content

### Cost Savings
- Both audio and images are cached to avoid regenerating identical content

### OpenAI Troubleshooting
- Check the API key has proper permissions enabled
- If you get rate limit errors, wait a moment and try again


### OpenAI TTS Configuration

OpenAI TTS provides natural Bulgarian pronunciation:

```bash
# Option 1: Use environment variable
export OPENAI_API_KEY="sk-your-key-here"
totalrecall ябълка

# Option 2: Set in .totalrecall.yaml
audio:
  openai_key: "sk-your-key-here"

# Use with custom voice
totalrecall ябълка --openai-voice alloy
```

**OpenAI TTS Models**:
- **gpt-4o-mini-tts** (default): New model with voice instruction support for customizable speech styles. Requires special API access.
- **tts-1**: Standard quality at $0.015 per 1K characters (~$0.0001 per word)
- **tts-1-hd**: Higher quality at $0.030 per 1K characters (~$0.0002 per word)

The gpt-4o-mini-tts model allows you to control how the voice speaks using natural language instructions, making it ideal for language learning applications. The tool caches audio to avoid repeated API calls for the same words.

## License

MIT License - see LICENSE file for details
