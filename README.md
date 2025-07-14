# bulg - Bulgarian Anki Flashcard Generator

`bulg` is a command-line tool that generates Anki flashcard materials from Bulgarian words. It creates audio pronunciation files using espeak-ng or OpenAI TTS and downloads representative images from web search APIs.

## Features

- Audio generation with multiple providers:
  - **espeak-ng**: Free, offline Bulgarian voices (robotic quality)
  - **OpenAI TTS**: High-quality, natural-sounding voices (requires API key)
- Image search via Pixabay and Unsplash APIs
- Batch processing of multiple words
- Anki-compatible CSV export
- Configurable voice variants and speech speed
- Support for WAV and MP3 audio formats
- Audio caching to save API costs (OpenAI)

## Installation

### Prerequisites

1. **For espeak-ng audio** (free, offline):
   ```bash
   # Ubuntu/Debian
   sudo apt-get install espeak-ng
   
   # macOS
   brew install espeak-ng
   ```

2. **ffmpeg** (optional, for MP3 conversion with espeak):
   ```bash
   # Ubuntu/Debian
   sudo apt-get install ffmpeg
   
   # macOS
   brew install ffmpeg
   ```

3. **For OpenAI TTS** (paid, high quality):
   - Create an account at https://platform.openai.com
   - Generate an API key at https://platform.openai.com/api-keys
   - Set the key using one of these methods:
     - Environment variable: `export OPENAI_API_KEY="sk-..."`
     - Configuration file: Add to `.bulg.yaml`

### Building from Source

```bash
git clone https://github.com/yourusername/bulg.git
cd bulg
go build -o bulg ./cmd/bulg
```

Or install directly:

```bash
go install codeberg.org/snonux/bulg/cmd/bulg@latest
```

## Quick Start

1. Generate materials for a single word:
   ```bash
   bulg ябълка
   ```

2. Process multiple words from a file:
   ```bash
   bulg --batch words.txt
   ```

3. Generate with Anki CSV:
   ```bash
   bulg ябълка --anki
   ```

## Configuration

Create a `.bulg.yaml` file in your home directory or project folder:

```yaml
audio:
  provider: openai       # Audio provider (espeak or openai)
  format: mp3           # Audio format (wav or mp3)
  
  # ESpeak settings
  voice: bg+f1          # Voice variant (bg, bg+m1, bg+f1, etc.)
  speed: 150            # Speech speed (80-450 words/minute)
  pitch: 50             # Pitch adjustment (0-99)
  
  # OpenAI settings
  openai_key: "sk-..."  # Your OpenAI API key
  openai_model: "tts-1" # Model: tts-1 or tts-1-hd
  openai_voice: "nova"  # Voice: alloy, echo, fable, onyx, nova, shimmer
  openai_speed: 1.0     # Speed: 0.25 to 4.0
  
  # Caching
  enable_cache: true
  cache_dir: "./.audio_cache"

image:
  provider: pixabay       # Image provider (pixabay or unsplash)
  pixabay_key: ""        # Optional API key for higher limits
  unsplash_key: ""       # Required for Unsplash
  size: medium           # Image size preference

output:
  directory: ./anki_cards
  naming: "{word}_{type}"
```

## Usage

```bash
bulg [word] [flags]
```

### Flags

- `-v, --voice string`: Voice variant (default "bg+f1")
- `-o, --output string`: Output directory (default "./anki_cards")
- `-f, --format string`: Audio format - wav or mp3 (default "mp3")
- `--batch string`: Process words from file (one per line)
- `--anki`: Generate Anki import CSV file
- `--skip-audio`: Skip audio generation
- `--skip-images`: Skip image download
- `--images-per-word int`: Number of images per word (default 1)
- `--image-api string`: Image source - pixabay or unsplash (default "pixabay")

#### Audio Provider Options
- `--audio-provider string`: Audio provider - espeak or openai (default "espeak")

#### ESpeak Tuning Options
- `--pitch int`: Pitch adjustment 0-99 (default 50, lower=deeper, espeak only)
- `--amplitude int`: Volume 0-200 (default 100, espeak only)
- `--word-gap int`: Gap between words in 10ms units (default 0, espeak only)

#### OpenAI Options
- `--openai-model string`: Model - tts-1 or tts-1-hd (default "tts-1")
- `--openai-voice string`: Voice - alloy, echo, fable, onyx, nova, shimmer (default "nova")
- `--openai-speed float`: Speech speed 0.25-4.0 (default 1.0)

## API Keys

### Pixabay
- Optional - works without key but with lower rate limits
- Get your key at: https://pixabay.com/api/docs/

### Unsplash
- Required for Unsplash searches
- Get your key at: https://unsplash.com/developers

## Examples

### Basic Usage
```bash
# Single word with espeak-ng
bulg котка

# Using OpenAI TTS (requires API key in config)
bulg котка --audio-provider openai

# High-quality OpenAI with specific voice
bulg ябълка --audio-provider openai --openai-model tts-1-hd --openai-voice alloy

# Multiple words with custom output
bulg --batch animals.txt -o ./animal_cards

# ESpeak with tuning
bulg ябълка --pitch 40 --word-gap 3

# Skip images, audio only
bulg куче --skip-images

# Generate Anki import file
bulg --batch words.txt --anki
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

## Anki Import

1. Generate materials with the `--anki` flag
2. In Anki, go to File → Import
3. Select the generated `anki_import.csv`
4. Copy all media files to your Anki media folder
5. Map fields appropriately during import

## Voice Variants

Available Bulgarian voices:
- `bg` - Default Bulgarian voice
- `bg+m1`, `bg+m2`, `bg+m3` - Male voices
- `bg+f1`, `bg+f2`, `bg+f3` - Female voices

## Troubleshooting

### espeak-ng not found
Make sure espeak-ng is installed and in your PATH.

### No images found
- Check your internet connection
- Verify API keys in configuration
- Try using English translations for better results

### OpenAI API errors
- Verify your API key is correct and has credits
- Check the API key has TTS permissions enabled
- If you get rate limit errors, wait a moment and try again
- The tool will automatically fall back to espeak-ng if OpenAI fails

### Audio sounds robotic
The Bulgarian voice in espeak-ng can sound robotic. To improve quality:

```bash
# Test with different settings
espeak-ng -v bg -p 40 -s 140 "Здравей"  # Deeper, slower
espeak-ng -v bg+f1 -p 60 -g 2 "Здравей"  # Higher pitch, word gaps

# Using bulg with tuning
bulg ябълка --pitch 40 --word-gap 2 --amplitude 120
```

Recommended settings for clearer pronunciation:
- `--pitch 40`: Slightly deeper voice (less robotic)
- `--word-gap 2-5`: Small gaps between words
- `--amplitude 120`: Slightly louder
- `-v bg+f1`: Female variant often sounds clearer

### Using OpenAI for Better Quality

OpenAI TTS provides much more natural Bulgarian pronunciation:

```bash
# Option 1: Use environment variable
export OPENAI_API_KEY="sk-your-key-here"
bulg ябълка --audio-provider openai

# Option 2: Set in .bulg.yaml
audio:
  provider: openai
  openai_key: "sk-your-key-here"

# Use with custom voice
bulg ябълка --audio-provider openai --openai-voice alloy
```

**OpenAI Pricing**: 
- tts-1: $0.015 per 1K characters (~$0.0001 per word)
- tts-1-hd: $0.030 per 1K characters (~$0.0002 per word)

The tool caches audio to avoid repeated API calls for the same words.

## License

MIT License - see LICENSE file for details