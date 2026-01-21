# TotalRecall Troubleshooting Guide

## Audio Regeneration Issue - Bulgarian-Bulgarian Cards

### Issue Description

When using keyboard shortcuts to regenerate audio on Bulgarian-Bulgarian (bg-bg) cards:
- **Expected**: `a` regenerates front audio, `A` regenerates back audio
- **Problem**: User reports that `A` is regenerating the "other side" instead of the back audio

### What Should Happen

For a bg-bg card with:
- Front word: "котка" (cat)
- Back definition: "домашно животно" (domestic animal)

**Playback:**
- `p` (lowercase p) should play audio_front.mp3 (pronunciation of "котка")
- `P` (uppercase P) should play audio_back.mp3 (pronunciation of "домашно животно")

**Regeneration:**
- `a` (lowercase a) should regenerate audio_front.mp3 for "котка"
- `A` (uppercase A) should regenerate audio_back.mp3 for "домашно животно"

### Current Implementation

#### File Structure (Correct as of Jan 21, 2025)
```
~/.local/state/totalrecall/cards/[CARD_ID]/
├── word.txt              # "котка"
├── translation.txt       # "котка = домашно животно"
├── cardtype.txt          # "bg-bg"
├── audio_front.mp3       # Audio for "котка"
├── audio_back.mp3        # Audio for "домашно животно"
└── [other files...]
```

#### Code Logic

**Playback** (working correctly):
- `SetOnTypedRune()` in app.go (lines 2445-2452)
  - `p/п` → calls `a.audioPlayer.Play()` → plays `audio_front.mp3`
  - `P/П` → calls `a.audioPlayer.PlayBack()` → plays `audio_back.mp3`

**Regeneration** (potentially buggy):
- `onRegenerateAudio()` (app.go lines 1034-1125)
  - For bg-bg: calls `generateAudioFront(cardCtx, wordForGeneration, cardDir)`
  - `wordForGeneration = a.currentWord` (the front word)
  - Generates audio for the front word ✓ CORRECT

- `onRegenerateBackAudio()` (app.go lines 1128-1191)
  - Extracts: `translation = a.currentTranslation`
  - Calls: `generateAudioBack(cardCtx, translation, cardDir)`
  - Should generate audio for the back definition
  - **VERIFY**: Is `a.currentTranslation` actually the back definition?

### Debugging Steps

#### 1. Check Card Files
```bash
# Navigate to an existing bg-bg card directory
cd ~/.local/state/totalrecall/cards/[CARD_ID]/

# Verify card type
cat cardtype.txt          # Should be "bg-bg"

# Verify translation file format
cat translation.txt       # Should be "front_word = back_definition"

# Verify audio files exist
ls -lh audio_*.mp3        # Should see audio_front.mp3 and audio_back.mp3
```

#### 2. Check UI State When Loading Card
When you open a bg-bg card in the GUI:
1. Check the "Card Type" dropdown - should show "Bulgarian → Bulgarian"
2. Check the Bulgarian input field - should show the front word
3. Check the translation field - should show the back definition (not English!)
4. The placeholder text should say "Bulgarian definition..." not "English translation..."

#### 3. Test Playback (Working Baseline)
Open a bg-bg card and test:
- Press `p` (lowercase) - which audio plays? (Front word or definition?)
- Press `P` (uppercase) - which audio plays? (Front word or definition?)

#### 4. Test Regeneration
After confirming playback works correctly:
- Press `a` (lowercase) - which TEXT is sent for audio generation? Check console output for:
  ```
  Generating front audio for '[TEXT]'
  ```
- Press `A` (uppercase) - which TEXT is sent for audio generation? Check console output for:
  ```
  Generating back audio for '[TEXT]'
  ```

### Data Flow for `A` (Regenerate Back Audio)

```
User presses 'A'
  ↓
SetOnTypedRune() case 'A', 'А'
  ↓
onRegenerateBackAudio()
  ↓
Get translation:
  - translation = a.currentTranslation
  - IF empty: translation = a.translationEntry.Text
  ↓
generateAudioBack(cardCtx, translation, cardDir)
  ↓
Generate audio for [translation] → audio_back.mp3
```

### Potential Issues to Investigate

1. **`a.currentTranslation` not set correctly**
   - Check if `LoadExistingFiles()` properly extracts back definition from translation.txt
   - Verify the split on "=" correctly extracts the part after the equals sign
   - Confirm the value is actually the Bulgarian definition, not English

2. **Translation field contains wrong value**
   - When bg-bg card loads, does `a.translationEntry.SetText()` set the back definition or something else?
   - Is the UI field showing the correct value?

3. **Card type not detected correctly**
   - Confirm `cardtype.txt` exists and contains "bg-bg"
   - Verify `internal.LoadCardType()` works correctly

4. **Field names confusion**
   - The `translationEntry` field is named misleadingly
   - For en-bg: it contains English translation
   - For bg-bg: it contains Bulgarian definition
   - Placeholder text should change to reflect this

### Related Code Files

- `internal/gui/app.go` - Hotkey handlers (lines 1034-1191 for audio, 2405-2453 for playback)
- `internal/gui/navigation.go` - Card loading logic (lines 364-460)
- `internal/gui/generator.go` - Audio generation functions (lines 156-220)
- `internal/gui/audio_player.go` - Audio playback implementation
- `internal/processor/processor.go` - Batch processing audio generation

### Test Case

Use this batch input to create a test card:
```
котка == домашно животно
```

Then in the GUI:
1. Navigate to the "котка" card
2. Verify UI shows correct values
3. Press `p` and listen - should hear "котка"
4. Press `P` and listen - should hear "домашно животно"
5. Press `a` - check console for "Generating front audio for 'котка'"
6. Press `A` - check console for "Generating back audio for 'домашно животно'" (NOT "котка")

### Notes

- Batch processing works correctly (both audio files generated in correct locations as of Jan 21, 2025)
- GUI playback buttons work correctly and show proper labels ("Front" / "Back")
- The issue appears to be isolated to the `A` key regeneration logic
- Audio file generation itself is not the problem - the issue is which TEXT is being passed to the generator

