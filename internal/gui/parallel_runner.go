package gui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// GenerateResult holds the outcome of a parallel generation run.
type GenerateResult struct {
	AudioFile     string
	AudioFileBack string
	ImageFile     string
	PhoneticInfo  string
}

// audioGenResult is an internal channel payload for audio goroutines.
type audioGenResult struct {
	file     string
	fileBack string
	err      error
}

// imageGenResult is an internal channel payload for image goroutines.
type imageGenResult struct {
	file string
	err  error
}

// phoneticGenResult is an internal channel payload for phonetic goroutines.
type phoneticGenResult struct {
	info string
	err  error
}

// ParallelRunner coordinates parallel audio, image, and phonetics work for
// material generation. It is stateless; the orchestrator supplies all behaviour
// via the first method argument.
type ParallelRunner struct{}

// GenerateMaterials generates audio, image, and phonetics in parallel for a
// word. translation is the existing translation (may be empty). isBgBg flags
// bg-bg card type. imagePrompt is an optional custom prompt; imageTranslation
// is the translation hint for image prompts.
// The promptUI callback is called on the generating goroutine when the image
// prompt becomes known so callers can update the UI.
// Returns a GenerateResult or an error if any mandatory step fails.
func (ParallelRunner) GenerateMaterials(
	o *GenerationOrchestrator,
	ctx context.Context,
	word, translation, cardDir string,
	isBgBg bool,
	imagePrompt string,
	promptUI func(prompt string),
) (GenerateResult, error) {
	audioChan := make(chan audioGenResult, 1)
	imageChan := make(chan imageGenResult, 1)
	phoneticChan := make(chan phoneticGenResult, 1)

	// 1. Audio generation
	go func() {
		var audioFile, audioFileBack string
		var err error

		if isBgBg && translation != "" {
			audioFile, audioFileBack, err = o.GenerateAudioBgBg(ctx, word, translation, cardDir)
		} else {
			audioFile, err = o.GenerateAudio(ctx, word, cardDir)
		}

		audioChan <- audioGenResult{file: audioFile, fileBack: audioFileBack, err: err}
	}()

	// 2. Image generation (includes scene description from the AI)
	go func() {
		imageFile, err := o.generateImagesWithPromptAndNotify(ctx, word, imagePrompt, translation, cardDir, promptUI)
		imageChan <- imageGenResult{file: imageFile, err: err}
	}()

	// 3. Phonetic information fetching
	go func() {
		phoneticInfo, err := o.GetPhoneticInfo(word)
		if err != nil {
			fmt.Printf("Warning: Failed to get phonetic info: %v\n", err)
			phoneticInfo = "Failed to fetch phonetic information"
		} else {
			fmt.Printf("Successfully fetched phonetic info for '%s': %s\n", word, phoneticInfo)
		}

		savePhoneticIfValid(phoneticInfo, cardDir, word)
		phoneticChan <- phoneticGenResult{info: phoneticInfo}
	}()

	// Collect results.
	audioRes := <-audioChan
	if audioRes.err != nil {
		// Drain remaining channels to avoid goroutine leaks.
		<-imageChan
		<-phoneticChan
		return GenerateResult{}, fmt.Errorf("audio generation failed: %w", audioRes.err)
	}

	imageRes := <-imageChan
	if imageRes.err != nil {
		<-phoneticChan
		return GenerateResult{}, fmt.Errorf("image download failed: %w", imageRes.err)
	}

	phoneticRes := <-phoneticChan

	return GenerateResult{
		AudioFile:     audioRes.file,
		AudioFileBack: audioRes.fileBack,
		ImageFile:     imageRes.file,
		PhoneticInfo:  phoneticRes.info,
	}, nil
}

// savePhoneticIfValid saves phonetic info to disk when the info is valid.
func savePhoneticIfValid(phoneticInfo, cardDir, word string) {
	if phoneticInfo == "" || phoneticInfo == "Failed to fetch phonetic information" {
		return
	}

	phoneticFile := filepath.Join(cardDir, "phonetic.txt")
	if err := os.WriteFile(phoneticFile, []byte(phoneticInfo), 0644); err != nil {
		fmt.Printf("Warning: Failed to save phonetic info for '%s': %v\n", word, err)
	}
}
