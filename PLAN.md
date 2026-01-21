# Plan: Bulgarian-Bulgarian Flashcard Support

## Status: ✅ IMPLEMENTED

All features have been implemented and tested.

## Overview
Add support for Bulgarian-Bulgarian flashcards alongside the existing English-Bulgarian mode. This enables monolingual learning where both sides of the flashcard are in Bulgarian (e.g., word and definition, or word and synonym).

## Current State
- ✅ English-Bulgarian and Bulgarian-Bulgarian flashcards are supported
- ✅ File-based storage uses `translation.txt` and `cardtype.txt`
- ✅ Audio is generated for both sides of bg-bg cards
- ✅ Batch import supports: `english=bulgarian` (en-bg) and `bulgarian1==bulgarian2` (bg-bg)

---

## 1. Internal Database/Storage Changes ✅

### 1.1 Add Card Type Indicator
**File:** `internal/cardtype.go` (NEW)

- Created new `CardType` type with `CardTypeEnBg` and `CardTypeBgBg` constants
- Added `SaveCardType()` and `LoadCardType()` functions
- Backwards compatible: missing `cardtype.txt` defaults to `en-bg`

### 1.2 Update Structs ✅
**Files:** `internal/batch/processor.go`, `internal/anki/generator.go`, `internal/gui/queue.go`

Added `CardType` field to:
- `WordEntry` struct
- `Card` struct (with `AudioFileBack` for bg-bg)
- `WordJob` struct (with `AudioFileBack` and `CardType`)

### 1.3 Second Audio File ✅
**File:** `internal/processor/processor.go`, `internal/gui/generator.go`

For `bg-bg` cards, stores two audio files:
- `audio_front.mp3` - pronunciation of first Bulgarian term
- `audio_back.mp3` - pronunciation of second Bulgarian term

---

## 2. Audio Generation Changes ✅

### 2.1 Generate Audio for Both Sides
**Files:** `internal/processor/processor.go`, `internal/gui/generator.go`

- Added `generateAudioBgBg()` function for CLI processor
- Added `generateAudioBgBg()` function for GUI
- Both sides use the same voice for consistency

### 2.2 Update Processor ✅
- Detects card type and calls appropriate audio generation
- Saves to `audio_front.mp3` and `audio_back.mp3` for `bg-bg`

---

## 3. GUI Support ✅

### 3.1 Card Type Selector
**File:** `internal/gui/app.go`

Added dropdown selector:
- "English → Bulgarian" (default)
- "Bulgarian → Bulgarian"

### 3.2 Update Input Labels ✅
When `bg-bg` is selected:
- Translation placeholder changes to "Bulgarian definition..."

### 3.3 Audio Preview ✅
**File:** `internal/gui/audio_player.go`

For `bg-bg` cards:
- Added "Play Back Audio" button (skip next icon)
- Button only visible for bg-bg cards

### 3.4 Navigation Updates ✅
**File:** `internal/gui/navigation.go`

- Loads card type when navigating to existing cards
- Updates card type selector based on loaded card
- Loads both front and back audio for bg-bg cards

---

## 4. Batch Importer Support ✅

### 4.1 Input Format Detection
**File:** `internal/batch/processor.go`

Detects card type from line format:
- `bulgarian1==bulgarian2` → `bg-bg` mode (double equals)
- `english=bulgarian` or `bulgarian=english` → `en-bg` mode (single equals)

### 4.2 Parsing Logic ✅
```
Line contains "==" → Split on "==" → bg-bg card
Line contains "=" (single) → Split on "=" → en-bg card
```

### 4.3 Processing Pipeline ✅
- Parses and detects card type per line
- Passes card type through the processing pipeline
- Generates appropriate audio files based on type

---

## 5. Anki Export Changes ✅

### 5.1 Update Export Format
**File:** `internal/anki/apkg_generator.go`

For `bg-bg` cards:
- Created separate note type "Bulgarian-Bulgarian from TotalRecall"
- Fields: BulgarianFront, BulgarianBack, Image, AudioFront, AudioBack, Notes
- Includes both audio files in the Anki package

### 5.2 Card Templates ✅
- Forward: Shows front Bulgarian word + front audio, answer shows back + back audio
- Reverse: Shows back Bulgarian word + back audio, answer shows front + front audio
- Different CSS styling for front/back Bulgarian text

---

## Test Coverage ✅

Added new tests in `internal/batch/processor_test.go`:
- `bulgarian-bulgarian_format_with_double_equals`
- `mixed_en-bg_and_bg-bg_formats`

All tests pass:
```
=== RUN   TestReadBatchFile/bulgarian-bulgarian_format_with_double_equals
--- PASS: TestReadBatchFile/bulgarian-bulgarian_format_with_double_equals
=== RUN   TestReadBatchFile/mixed_en-bg_and_bg-bg_formats
--- PASS: TestReadBatchFile/mixed_en-bg_and_bg-bg_formats
```

---

## Migration/Compatibility ✅

- Existing cards without `cardtype.txt` default to `en-bg`
- No migration needed for existing data
- Batch files can mix `en-bg` and `bg-bg` entries in the same file
