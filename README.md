# totalrecall - Bulgarian Anki Flashcard Generator

`totalrecall` is a command-line tool that generates Anki flashcard materials from Bulgarian words. It creates audio pronunciation files and generates images using AI.

It has mainly been vibe coded using Claude Code CLI.

⚠️ **Important:** This tool uses OpenAI services by default, which requires an API key. See [Quick Start](#quick-start) for setup instructions or use the free alternatives with `--audio-provider espeak --image-api pixabay`.

## Features

- Audio generation with multiple providers:
  - **espeak-ng**: Free, offline Bulgarian voices (robotic quality)
  - **OpenAI TTS**: High-quality, natural-sounding voices (requires API key)
- Image search and generation:
  - **Pixabay**: Free stock photo search (optional API key)
  - **Unsplash**: High-quality photo search (requires API key)
  - **OpenAI DALL-E**: AI-generated educational images (requires API key)
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
     - Configuration file: Add to `.totalrecall.yaml`

### Building from Source

```bash
git clone https://github.com/yourusername/totalrecall.git
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

1. Generate materials for a single word (uses OpenAI by default):
   ```bash
   totalrecall ябълка
   ```

2. Use free alternatives (espeak + pixabay):
   ```bash
   totalrecall ябълка --audio-provider espeak --image-api pixabay
   ```

3. Process multiple words from a file:
   ```bash
   totalrecall --batch words.txt
   ```

4. Generate with Anki CSV:
   ```bash
   totalrecall ябълка --anki
   ```

## Configuration

Create a `.totalrecall.yaml` file in your home directory or project folder:

```yaml
audio:
  provider: openai       # Audio provider (espeak or openai) - default: openai
  format: mp3           # Audio format (wav or mp3)
  
  # ESpeak settings
  voice: bg+f1          # Voice variant (bg, bg+m1, bg+f1, etc.)
  speed: 150            # Speech speed (80-450 words/minute)
  pitch: 50             # Pitch adjustment (0-99)
  
  # OpenAI settings
  openai_key: "sk-..."  # Your OpenAI API key
  openai_model: "gpt-4o-mini-tts" # Model: tts-1, tts-1-hd, or gpt-4o-mini-tts
  openai_voice: "nova"  # Voice: alloy, ash, ballad, coral, echo, fable, onyx, nova, sage, shimmer, verse
  openai_speed: 0.8     # Speed: 0.25 to 4.0 (may be ignored by gpt-4o-mini models)
  openai_instruction: "Speak slowly and clearly with natural Bulgarian pronunciation" # For gpt-4o-mini models only
  
  # Caching
  enable_cache: true
  cache_dir: "./.audio_cache"

image:
  provider: openai       # Image provider (pixabay, unsplash, or openai) - default: openai
  pixabay_key: ""        # Optional API key for higher limits
  unsplash_key: ""       # Required for Unsplash
  
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

```bash
totalrecall [word] [flags]
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
- `--image-api string`: Image source - pixabay, unsplash, or openai (default "openai")

#### Audio Provider Options
- `--audio-provider string`: Audio provider - espeak or openai (default "openai")

#### ESpeak Tuning Options
- `--pitch int`: Pitch adjustment 0-99 (default 50, lower=deeper, espeak only)
- `--amplitude int`: Volume 0-200 (default 100, espeak only)
- `--word-gap int`: Gap between words in 10ms units (default 0, espeak only)

#### OpenAI Audio Options
- `--openai-model string`: Model - tts-1, tts-1-hd, or gpt-4o-mini-tts (default "gpt-4o-mini-tts", requires special access)
- `--openai-voice string`: Voice - alloy, ash, ballad, coral, echo, fable, onyx, nova, sage, shimmer, verse (default "nova")
- `--openai-speed float`: Speech speed 0.25-4.0 (default 0.8, may be ignored by gpt-4o-mini-tts)
- `--openai-instruction string`: Voice instructions for gpt-4o-mini-tts model (e.g., "speak with a Bulgarian accent")

#### OpenAI Image Options
- `--openai-image-model string`: Model - dall-e-2 or dall-e-3 (default "dall-e-2")
- `--openai-image-size string`: Size - 256x256, 512x512, 1024x1024 (default "512x512")
- `--openai-image-quality string`: Quality - standard or hd (default "standard", dall-e-3 only)
- `--openai-image-style string`: Style - natural or vivid (default "natural", dall-e-3 only)

## API Keys

### Pixabay
- Optional - works without key but with lower rate limits
- Get your key at: https://pixabay.com/api/docs/

### Unsplash
- Required for Unsplash searches
- Get your key at: https://unsplash.com/developers

### OpenAI
- Required for both OpenAI TTS audio and DALL-E image generation
- Get your key at: https://platform.openai.com/api-keys
- Set via environment variable: `export OPENAI_API_KEY="sk-..."`
- Or add to config file as `audio.openai_key`

## Examples

### Basic Usage
```bash
# Single word (uses OpenAI by default)
totalrecall котка

# Using espeak-ng (free alternative)
totalrecall котка --audio-provider espeak

# High-quality OpenAI with specific voice
totalrecall ябълка --audio-provider openai --openai-model tts-1-hd --openai-voice alloy

# Use gpt-4o-mini-tts with custom voice instructions
totalrecall ябълка --openai-instruction "Speak like a patient Bulgarian teacher, very slowly and clearly"

# Multiple words with custom output
totalrecall --batch animals.txt -o ./animal_cards

# ESpeak with tuning
totalrecall ябълка --pitch 40 --word-gap 3

# Skip images, audio only
totalrecall куче --skip-images

# Generate Anki import file
totalrecall --batch words.txt --anki

# Generate AI images with OpenAI DALL-E
totalrecall ябълка --image-api openai

# High-quality DALL-E 3 images
totalrecall котка --image-api openai --openai-image-model dall-e-3 --openai-image-quality hd

# Combine OpenAI audio and images
totalrecall куче --audio-provider openai --image-api openai
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

## Cost Considerations

### OpenAI Services
- **TTS Audio**: ~$0.015 per 1K characters (tts-1), ~$0.030 (tts-1-hd)
- **DALL-E 2 Images**: ~$0.02 per image (512x512)
- **DALL-E 3 Images**: ~$0.04 per image (standard), ~$0.08 (HD)
- Both services cache results to avoid regenerating identical content

### Free Alternatives
- **Audio**: Use espeak-ng (free but robotic quality)
- **Images**: Use Pixabay without API key (limited rate)

### OpenAI Troubleshooting
- Check the API key has proper permissions enabled
- If you get rate limit errors, wait a moment and try again
- The tool will automatically fall back to espeak-ng if OpenAI audio fails

### Audio sounds robotic
The Bulgarian voice in espeak-ng can sound robotic. To improve quality:

```bash
# Test with different settings
espeak-ng -v bg -p 40 -s 140 "Здравей"  # Deeper, slower
espeak-ng -v bg+f1 -p 60 -g 2 "Здравей"  # Higher pitch, word gaps

# Using totalrecall with tuning
totalrecall ябълка --pitch 40 --word-gap 2 --amplitude 120
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
totalrecall ябълка --audio-provider openai

# Option 2: Set in .totalrecall.yaml
audio:
  provider: openai
  openai_key: "sk-your-key-here"

# Use with custom voice
totalrecall ябълка --audio-provider openai --openai-voice alloy
```

**OpenAI TTS Models**:
- **gpt-4o-mini-tts** (default): New model with voice instruction support for customizable speech styles. Requires special API access.
- **tts-1**: Standard quality at $0.015 per 1K characters (~$0.0001 per word)
- **tts-1-hd**: Higher quality at $0.030 per 1K characters (~$0.0002 per word)

The gpt-4o-mini-tts model allows you to control how the voice speaks using natural language instructions, making it ideal for language learning applications. The tool caches audio to avoid repeated API calls for the same words.

## License

MIT License - see LICENSE file for details
