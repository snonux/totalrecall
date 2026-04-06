package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"codeberg.org/snonux/totalrecall/internal"
	"codeberg.org/snonux/totalrecall/internal/anki"
)

// CardService manages card file discovery, directory creation, persistence,
// and state loading from the output directory. It is responsible for all
// non-UI file I/O related to cards, decoupled from UI event-wiring.
type CardService struct {
	config *Config
}

// NewCardService constructs a CardService for the given configuration.
func NewCardService(config *Config) *CardService {
	return &CardService{config: config}
}

// FindCardDirectory finds the directory for a given Bulgarian word.
// Delegates to the shared internal.FindCardDirectory which also handles the
// legacy _word.txt fallback for backward compatibility.
func (cs *CardService) FindCardDirectory(word string) string {
	return internal.FindCardDirectory(cs.config.OutputDir, word)
}

// EnsureWordDirectoryAndMetadata creates a new card directory and writes word
// metadata to word.txt inside it. Returns the directory path.
func (cs *CardService) EnsureWordDirectoryAndMetadata(word string) (string, error) {
	cardID := internal.GenerateCardID(word)
	wordDir := filepath.Join(cs.config.OutputDir, cardID)
	if err := os.MkdirAll(wordDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create card directory: %w", err)
	}

	metadataFile := filepath.Join(wordDir, "word.txt")
	if err := os.WriteFile(metadataFile, []byte(word), 0644); err != nil {
		return "", fmt.Errorf("failed to save word metadata: %w", err)
	}

	return wordDir, nil
}

// EnsureCardDirectory ensures a card directory exists for the given word and
// returns its path. Creates it with metadata if it does not exist yet.
func (cs *CardService) EnsureCardDirectory(word string) (string, error) {
	wordDir := cs.FindCardDirectory(word)
	if wordDir != "" {
		return wordDir, nil
	}

	return cs.EnsureWordDirectoryAndMetadata(word)
}

// ScanExistingWords scans the output directory for existing card subdirectories
// and returns a sorted list of the Bulgarian words found. A directory counts
// only if it contains at least one of: an audio file, an image, or a
// translation file.
func (cs *CardService) ScanExistingWords() []string {
	words := []string{}

	entries, err := os.ReadDir(cs.config.OutputDir)
	if err != nil {
		// Directory doesn't exist yet; return empty list silently.
		return words
	}

	// Each subdirectory represents a card identified by a card ID.
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		cardID := entry.Name()
		wordDir := filepath.Join(cs.config.OutputDir, cardID)

		word, ok := cs.readWordFromDir(wordDir)
		if !ok {
			continue
		}

		if cs.dirHasContent(wordDir) {
			words = append(words, word)
		}
	}

	sort.Strings(words)
	return words
}

// readWordFromDir reads the Bulgarian word from word.txt (or the legacy
// _word.txt) inside a card directory. Returns the word and true on success.
func (cs *CardService) readWordFromDir(wordDir string) (string, bool) {
	wordFile := filepath.Join(wordDir, "word.txt")
	wordData, err := os.ReadFile(wordFile)
	if err != nil {
		// Try old format with underscore for backward compatibility.
		wordFile = filepath.Join(wordDir, "_word.txt")
		wordData, err = os.ReadFile(wordFile)
		if err != nil {
			return "", false
		}
	}

	word := string(wordData)
	if word == "" {
		return "", false
	}

	return word, true
}

// dirHasContent returns true if the card directory contains at least one audio
// file, image file, or translation file.
func (cs *CardService) dirHasContent(wordDir string) bool {
	// Reuse the Application audio-path helpers via package-level functions to
	// avoid duplicating the metadata-parsing logic.
	if hasAnyAudioFileInDir(wordDir) {
		return true
	}

	for _, pattern := range []string{"image.jpg", "image.png"} {
		if _, err := os.Stat(filepath.Join(wordDir, pattern)); err == nil {
			return true
		}
	}

	if _, err := os.Stat(filepath.Join(wordDir, "translation.txt")); err == nil {
		return true
	}

	return false
}

// SaveTranslation persists the translation for the given word to disk.
// It finds or creates the card directory as necessary.
func (cs *CardService) SaveTranslation(word, translation string) error {
	if word == "" || translation == "" {
		return nil
	}

	wordDir, err := cs.EnsureCardDirectory(word)
	if err != nil {
		return err
	}

	translationFile := filepath.Join(wordDir, "translation.txt")
	content := fmt.Sprintf("%s = %s\n", word, translation)
	if err := os.WriteFile(translationFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to save translation: %w", err)
	}

	return nil
}

// SavePhoneticInfo persists phonetic information for the given word to disk.
func (cs *CardService) SavePhoneticInfo(word, phoneticText string) error {
	if word == "" || phoneticText == "" || phoneticText == "Failed to fetch phonetic information" {
		return nil
	}

	wordDir, err := cs.EnsureCardDirectory(word)
	if err != nil {
		return err
	}

	phoneticFile := filepath.Join(wordDir, "phonetic.txt")
	if err := os.WriteFile(phoneticFile, []byte(phoneticText), 0644); err != nil {
		return fmt.Errorf("failed to save phonetic info: %w", err)
	}

	return nil
}

// LoadPhoneticInfo reads phonetic information from disk for the given word.
// Returns empty string if not found.
func (cs *CardService) LoadPhoneticInfo(word string) string {
	wordDir := cs.FindCardDirectory(word)
	if wordDir == "" {
		return ""
	}

	phoneticFile := filepath.Join(wordDir, "phonetic.txt")
	data, err := os.ReadFile(phoneticFile)
	if err != nil {
		return ""
	}

	return string(data)
}

// CardFiles holds the paths to all files loaded for a single card.
type CardFiles struct {
	WordDir      string
	Translation  string
	AudioFile    string
	AudioBack    string
	ImageFile    string
	PhoneticInfo string
	ImagePrompt  string
	CardType     internal.CardType
}

// LoadCardFiles loads all available files for the given word from disk.
// Returns nil if no card directory exists for the word.
func (cs *CardService) LoadCardFiles(word string) *CardFiles {
	wordDir := cs.FindCardDirectory(word)
	if wordDir == "" {
		fmt.Printf("No card directory found for word: %s\n", word)
		return nil
	}

	fmt.Printf("Loading files from directory: %s\n", wordDir)

	cf := &CardFiles{WordDir: wordDir}
	cf.CardType = internal.LoadCardType(wordDir)

	cs.loadTranslation(wordDir, cf)
	cs.loadImagePrompt(wordDir, cf)
	cs.loadPhoneticFile(wordDir, cf)
	cs.loadAudioFiles(wordDir, cf)
	cs.loadImageFile(wordDir, cf)

	return cf
}

// loadTranslation reads translation.txt and stores the translation in cf.
func (cs *CardService) loadTranslation(wordDir string, cf *CardFiles) {
	translationFile := filepath.Join(wordDir, "translation.txt")
	data, err := os.ReadFile(translationFile)
	if err != nil {
		return
	}

	content := string(data)
	parts := strings.Split(content, "=")
	if len(parts) >= 2 {
		cf.Translation = strings.TrimSpace(parts[1])
	}
}

// loadImagePrompt reads image_prompt.txt and stores the prompt in cf.
func (cs *CardService) loadImagePrompt(wordDir string, cf *CardFiles) {
	promptFile := filepath.Join(wordDir, "image_prompt.txt")
	data, err := os.ReadFile(promptFile)
	if err != nil {
		fmt.Printf("No prompt file found at: %s\n", promptFile)
		return
	}

	fmt.Printf("Loaded prompt from file: %s\n", promptFile)
	cf.ImagePrompt = strings.TrimSpace(string(data))
}

// loadPhoneticFile reads phonetic.txt and stores the info in cf.
func (cs *CardService) loadPhoneticFile(wordDir string, cf *CardFiles) {
	phoneticFile := filepath.Join(wordDir, "phonetic.txt")
	data, err := os.ReadFile(phoneticFile)
	if err != nil {
		fmt.Printf("No phonetic file found at: %s (error: %v)\n", phoneticFile, err)
		return
	}

	fmt.Printf("Loaded phonetic info from file: %s\n", phoneticFile)
	cf.PhoneticInfo = string(data)
}

// loadAudioFiles resolves front and/or back audio paths depending on card type.
func (cs *CardService) loadAudioFiles(wordDir string, cf *CardFiles) {
	if cf.CardType.IsBgBg() {
		cf.AudioFile, cf.AudioBack = resolveBgBgAudioFilesInDir(wordDir)
	} else {
		cf.AudioFile = resolveSingleAudioFileInDir(wordDir)
	}
}

// loadImageFile finds the first matching image file in the card directory.
func (cs *CardService) loadImageFile(wordDir string, cf *CardFiles) {
	for _, pattern := range []string{"image.jpg", "image.png"} {
		imagePath := filepath.Join(wordDir, pattern)
		if _, err := os.Stat(imagePath); err == nil {
			cf.ImageFile = imagePath
			break
		}
	}

	if cf.ImageFile == "" {
		return
	}

	// Try to load the image prompt from the attribution file as a fallback
	// when the image provider is AI-based (OpenAI DALL-E or Nano Banana).
	if cs.config.ImageProvider == imageProviderOpenAI || cs.config.ImageProvider == imageProviderNanoBanana {
		cs.loadPromptFromAttribution(cf)
	}
}

// loadPromptFromAttribution reads the image prompt from an _attribution.txt
// file adjacent to the image file, falling back to the already-set ImagePrompt.
func (cs *CardService) loadPromptFromAttribution(cf *CardFiles) {
	if cf.ImageFile == "" {
		return
	}

	baseImagePath := cf.ImageFile
	attrPath := strings.TrimSuffix(baseImagePath, filepath.Ext(baseImagePath)) + "_attribution.txt"
	data, err := os.ReadFile(attrPath)
	if err != nil {
		return
	}

	// The attribution format is: "Prompt used:\n<the prompt>"
	content := string(data)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "Prompt used:") && i+1 < len(lines) {
			cf.ImagePrompt = strings.TrimSpace(lines[i+1])
			break
		}
	}
}

// CheckMissingFiles inspects a card directory for any files that have appeared
// since the last load and returns a partial CardFiles with only the newly found
// fields populated. Callers should merge with existing state.
func (cs *CardService) CheckMissingFiles(word string, existingAudio, existingAudioBack, existingImage, existingTranslation, existingPrompt, existingPhonetic string, cardType string) *CardFiles {
	wordDir := cs.FindCardDirectory(word)
	if wordDir == "" {
		return nil
	}

	result := &CardFiles{}

	// Check for missing audio.
	if existingAudio == "" {
		if cardType == "bg-bg" {
			front, _ := resolveBgBgAudioFilesInDir(wordDir)
			result.AudioFile = front
		} else {
			result.AudioFile = resolveSingleAudioFileInDir(wordDir)
		}
	}

	// Check for missing back audio (bg-bg cards only).
	if existingAudioBack == "" {
		_, back := resolveBgBgAudioFilesInDir(wordDir)
		result.AudioBack = back
	}

	// Check for missing image.
	if existingImage == "" {
		for _, pattern := range []string{"image.jpg", "image.png"} {
			imagePath := filepath.Join(wordDir, pattern)
			if _, err := os.Stat(imagePath); err == nil {
				result.ImageFile = imagePath
				break
			}
		}
	}

	// Check for missing translation.
	if existingTranslation == "" {
		translationFile := filepath.Join(wordDir, "translation.txt")
		if data, err := os.ReadFile(translationFile); err == nil {
			parts := strings.Split(string(data), "=")
			if len(parts) >= 2 {
				result.Translation = strings.TrimSpace(parts[1])
			}
		}
	}

	// Check for missing image prompt.
	if existingPrompt == "" {
		promptFile := filepath.Join(wordDir, "image_prompt.txt")
		if data, err := os.ReadFile(promptFile); err == nil {
			result.ImagePrompt = strings.TrimSpace(string(data))
		}
	}

	// Check for missing phonetic info.
	if existingPhonetic == "" {
		phoneticFile := filepath.Join(wordDir, "phonetic.txt")
		if data, err := os.ReadFile(phoneticFile); err == nil {
			result.PhoneticInfo = string(data)
		}
	}

	return result
}

// DeleteWord moves the card directory for a word to the trash bin directory.
// Returns the updated existingWords slice and updatedSavedCards slice.
func (cs *CardService) DeleteWord(word string, existingWords []string, savedCards []anki.Card) ([]string, []anki.Card, error) {
	wordDir := cs.FindCardDirectory(word)
	if wordDir == "" {
		return existingWords, savedCards, fmt.Errorf("no card directory found for word %q", word)
	}

	trashDir := filepath.Join(cs.config.OutputDir, ".trashbin")
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		return existingWords, savedCards, fmt.Errorf("failed to create trash directory: %w", err)
	}

	dirName := filepath.Base(wordDir)
	trashWordDir := filepath.Join(trashDir, fmt.Sprintf("%s_%s", dirName, trashTimestamp()))
	if err := os.Rename(wordDir, trashWordDir); err != nil {
		return existingWords, savedCards, fmt.Errorf("failed to move files to trash: %w", err)
	}

	// Remove the word from the existingWords list.
	newWords := make([]string, 0, len(existingWords))
	for _, w := range existingWords {
		if w != word {
			newWords = append(newWords, w)
		}
	}

	// Remove the word from savedCards.
	newCards := make([]anki.Card, 0, len(savedCards))
	for _, card := range savedCards {
		if card.Bulgarian != word {
			newCards = append(newCards, card)
		}
	}

	return newWords, newCards, nil
}

// LoadImagePromptForWord reads the image_prompt.txt file for the given word.
// Returns empty string if not found.
func (cs *CardService) LoadImagePromptForWord(word string) string {
	wordDir := cs.FindCardDirectory(word)
	if wordDir == "" {
		return ""
	}

	promptFile := filepath.Join(wordDir, "image_prompt.txt")
	data, err := os.ReadFile(promptFile)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(data))
}

// hasAnyAudioFileInDir is a package-level helper so CardService can check for
// audio files without holding a reference to Application.
func hasAnyAudioFileInDir(wordDir string) bool {
	if resolveSingleAudioFileInDir(wordDir) != "" {
		return true
	}

	front, back := resolveBgBgAudioFilesInDir(wordDir)
	return front != "" || back != ""
}

// resolveSingleAudioFileInDir resolves the single en-bg audio file from a card dir.
func resolveSingleAudioFileInDir(wordDir string) string {
	if audioFile := resolveAudioFileFromMetadata(wordDir, "audio_file"); audioFile != "" {
		return audioFile
	}

	return anki.ResolveAudioFile(wordDir, "audio", "")
}

// resolveBgBgAudioFilesInDir resolves front+back audio files for a bg-bg card dir.
func resolveBgBgAudioFilesInDir(wordDir string) (string, string) {
	front := resolveAudioFileFromMetadata(wordDir, "audio_file")
	if front == "" {
		front = anki.ResolveAudioFile(wordDir, "audio_front", "")
	}

	back := resolveAudioFileFromMetadata(wordDir, "audio_file_back")
	if back == "" {
		back = anki.ResolveAudioFile(wordDir, "audio_back", "")
	}

	return front, back
}

// trashTimestamp returns a formatted timestamp string for use in trash
// directory names, ensuring uniqueness across multiple deletions.
func trashTimestamp() string {
	return time.Now().Format("20060102_150405")
}
