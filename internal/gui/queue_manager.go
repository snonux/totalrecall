package gui

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"

	"codeberg.org/snonux/totalrecall/internal"
)

// QueueManager owns background word-job processing: card contexts, active
// operation counts, queue draining, and the queue status label (SRP).
type QueueManager struct {
	app *Application
}

// processNextInQueue processes the next word in the queue.
func (qm *QueueManager) processNextInQueue() {
	a := qm.app
	if a.currentJobID != 0 {
		return
	}

	job := a.queue.ProcessNextJob()
	if job == nil {
		return
	}

	a.mu.Lock()
	a.currentJobID = job.ID
	a.currentWord = job.Word
	a.currentTranslation = ""
	a.currentAudioFile = ""
	a.currentImage = ""
	a.mu.Unlock()

	fyne.Do(func() {
		a.clearUI()
		a.showProgress("Processing: " + job.Word)
		qm.updateQueueStatus()
	})

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		qm.processWordJob(job)
	}()
}

// getOrCreateCardContext returns a context for the given word, creating one if needed.
func (qm *QueueManager) getOrCreateCardContext(word string) (context.Context, context.CancelFunc) {
	a := qm.app
	a.cardMu.Lock()
	defer a.cardMu.Unlock()

	if cancel, exists := a.cardContexts[word]; exists {
		cancel()
	}

	ctx, cancel := context.WithCancel(a.ctx)
	a.cardContexts[word] = cancel

	return ctx, cancel
}

// cancelCardOperations cancels all ongoing operations for a specific word.
func (qm *QueueManager) cancelCardOperations(word string) {
	a := qm.app
	a.cardMu.Lock()
	defer a.cardMu.Unlock()

	if cancel, exists := a.cardContexts[word]; exists {
		cancel()
		delete(a.cardContexts, word)
	}
}

// startOperation marks the start of an operation for a word.
func (qm *QueueManager) startOperation(word string) {
	a := qm.app
	a.activeOpMu.Lock()
	defer a.activeOpMu.Unlock()
	a.activeOperations[word]++
}

// endOperation marks the end of an operation for a word.
func (qm *QueueManager) endOperation(word string) {
	a := qm.app
	a.activeOpMu.Lock()
	defer a.activeOpMu.Unlock()

	if count, exists := a.activeOperations[word]; exists {
		if count > 1 {
			a.activeOperations[word]--
		} else {
			delete(a.activeOperations, word)
		}
	}
}

// hasActiveOperations checks if a word has any active operations.
func (qm *QueueManager) hasActiveOperations(word string) bool {
	a := qm.app
	a.activeOpMu.Lock()
	defer a.activeOpMu.Unlock()

	count, exists := a.activeOperations[word]
	return exists && count > 0
}

// processWordJob processes a single word job using the GenerationOrchestrator.
func (qm *QueueManager) processWordJob(job *WordJob) {
	a := qm.app
	cardCtx, _ := qm.getOrCreateCardContext(job.Word)

	select {
	case <-cardCtx.Done():
		a.queue.FailJob(job.ID, fmt.Errorf("job cancelled"))
		qm.finishCurrentJob()
		return
	default:
	}

	cardDir, isBgBg, ok := qm.prepareJobDirectory(job)
	if !ok {
		return
	}

	translation, ok := qm.resolveJobTranslation(job, isBgBg, cardDir)
	if !ok {
		qm.finishCurrentJob()
		return
	}

	a.mu.Lock()
	if a.currentJobID == job.ID && translation != "" {
		a.currentTranslation = translation
		fyne.Do(func() { a.translationEntry.SetText(translation) })
	}
	a.mu.Unlock()

	result, genErr := qm.runJobGeneration(job, cardCtx, translation, cardDir, isBgBg)
	if genErr != nil {
		a.queue.FailJob(job.ID, genErr)
		qm.finishCurrentJob()
		return
	}

	qm.applyJobResult(job, result, translation, isBgBg)

	qm.finishCurrentJob()
	fyne.Do(func() { qm.updateQueueStatus() })
}

func (qm *QueueManager) prepareJobDirectory(job *WordJob) (string, bool, bool) {
	a := qm.app
	cardDir, dirErr := a.ensureCardDirectory(job.Word)
	if dirErr != nil {
		a.queue.FailJob(job.ID, fmt.Errorf("failed to create card directory: %w", dirErr))
		qm.finishCurrentJob()
		return "", false, false
	}

	isBgBg := job.CardType == "bg-bg"
	if err := qm.saveJobCardType(job.ID, cardDir, isBgBg); err != nil {
		qm.finishCurrentJob()
		return "", false, false
	}

	return cardDir, isBgBg, true
}

func (qm *QueueManager) runJobGeneration(job *WordJob, cardCtx context.Context, translation, cardDir string, isBgBg bool) (GenerateResult, error) {
	a := qm.app
	fyne.Do(func() {
		a.updateStatus(fmt.Sprintf("Processing '%s' - generating audio, images, and phonetics in parallel...", job.Word))
		a.mu.Lock()
		if a.currentJobID == job.ID {
			a.imageDisplay.SetGenerating()
		}
		a.mu.Unlock()
	})

	promptUI := func(prompt string) {
		a.mu.Lock()
		isCurrentJob := a.currentJobID == job.ID
		a.mu.Unlock()
		if isCurrentJob && a.imagePromptEntry != nil {
			a.imagePromptEntry.SetText(prompt)
		}
	}

	qm.startOperation(job.Word)
	qm.startOperation(job.Word)
	qm.startOperation(job.Word)
	fyne.Do(func() {
		qm.incrementProcessing()
		qm.incrementProcessing()
		qm.incrementProcessing()
	})

	result, genErr := a.getOrchestrator().GenerateMaterials(
		cardCtx, job.Word, translation, cardDir, isBgBg, job.CustomPrompt, promptUI,
	)

	qm.decrementProcessing()
	qm.decrementProcessing()
	qm.decrementProcessing()
	qm.endOperation(job.Word)
	qm.endOperation(job.Word)
	qm.endOperation(job.Word)

	return result, genErr
}

func (qm *QueueManager) applyJobResult(job *WordJob, result GenerateResult, translation string, isBgBg bool) {
	a := qm.app
	a.mu.Lock()
	isCurrentJob := a.currentJobID == job.ID
	if isCurrentJob {
		a.currentAudioFile = result.AudioFile
		a.currentAudioFileBack = result.AudioFileBack
	}
	a.mu.Unlock()

	if isCurrentJob {
		fyne.Do(func() {
			a.mu.Lock()
			if a.currentJobID != job.ID {
				a.mu.Unlock()
				return
			}
			a.mu.Unlock()
			a.audioPlayer.SetAudioFile(result.AudioFile)
			if isBgBg && result.AudioFileBack != "" {
				a.audioPlayer.SetBackAudioFile(result.AudioFileBack)
			}
			a.regenerateAudioBtn.Enable()
		})
	}

	if result.PhoneticInfo != "" && result.PhoneticInfo != "Failed to fetch phonetic information" {
		a.mu.Lock()
		shouldUpdate := a.currentJobID == job.ID
		if shouldUpdate {
			a.currentPhonetic = result.PhoneticInfo
		}
		a.mu.Unlock()
		if shouldUpdate {
			fmt.Printf("Updating phonetic display immediately for job %d: %s\n", job.ID, result.PhoneticInfo)
			fyne.Do(func() { a.audioPlayer.SetPhonetic(result.PhoneticInfo) })
		}
	}

	fyne.Do(func() { a.updateStatus(fmt.Sprintf("Finalizing '%s'...", job.Word)) })
	a.queue.CompleteJob(job.ID, translation, result.AudioFile, result.AudioFileBack, result.ImageFile)

	qm.applyFinalJobUI(job, result, translation)
}

func (qm *QueueManager) applyFinalJobUI(job *WordJob, result GenerateResult, translation string) {
	a := qm.app
	a.mu.Lock()
	isCurrentJob := a.currentJobID == job.ID
	if isCurrentJob {
		a.currentTranslation = translation
		a.currentAudioFile = result.AudioFile
		if result.ImageFile != "" {
			a.currentImage = result.ImageFile
		}
		if result.PhoneticInfo != "" && result.PhoneticInfo != "Failed to fetch phonetic information" {
			a.currentPhonetic = result.PhoneticInfo
		}
	}
	a.mu.Unlock()

	if !isCurrentJob {
		return
	}

	fyne.Do(func() {
		a.mu.Lock()
		if a.currentJobID != job.ID {
			a.mu.Unlock()
			return
		}
		a.mu.Unlock()

		a.translationEntry.SetText(translation)
		if result.ImageFile != "" {
			a.imageDisplay.SetImages([]string{result.ImageFile})
		}
		a.audioPlayer.SetAudioFile(result.AudioFile)
		if a.currentPhonetic != "" {
			fmt.Printf("Setting phonetic in final UI update: %s\n", a.currentPhonetic)
			a.audioPlayer.SetPhonetic(a.currentPhonetic)
		} else {
			fmt.Printf("No phonetic info available in final UI update\n")
		}
		a.hideProgress()
		a.setActionButtonsEnabled(true)
		a.updateStatus(fmt.Sprintf("Completed: %s", job.Word))
	})
}

func (qm *QueueManager) saveJobCardType(jobID int, cardDir string, isBgBg bool) error {
	a := qm.app
	cardType := internal.CardTypeEnBg
	if isBgBg {
		cardType = internal.CardTypeBgBg
	}
	if err := internal.SaveCardType(cardDir, cardType); err != nil {
		a.queue.FailJob(jobID, fmt.Errorf("failed to save card type: %w", err))
		return err
	}
	return nil
}

func (qm *QueueManager) resolveJobTranslation(job *WordJob, isBgBg bool, cardDir string) (string, bool) {
	a := qm.app
	var translation string

	if job.NeedsTranslation && !isBgBg {
		fyne.Do(func() {
			a.updateStatus(fmt.Sprintf("Translating '%s'...", job.Word))
		})

		var err error
		translation, err = a.translateWord(job.Word)
		if err != nil {
			a.queue.FailJob(job.ID, fmt.Errorf("translation failed: %w", err))
			return "", false
		}
	} else if job.Translation != "" {
		translation = job.Translation
	}

	if translation != "" {
		if err := a.getCardService().SaveTranslation(job.Word, translation); err != nil {
			a.queue.FailJob(job.ID, fmt.Errorf("failed to save translation: %w", err))
			return "", false
		}
	}

	_ = cardDir
	return translation, true
}

// finishCurrentJob clears the current job and processes next in queue.
func (qm *QueueManager) finishCurrentJob() {
	a := qm.app
	a.mu.Lock()
	a.currentJobID = 0
	a.mu.Unlock()

	fyne.Do(func() {
		qm.processNextInQueue()
	})
}

// onQueueStatusUpdate handles queue status updates.
func (qm *QueueManager) onQueueStatusUpdate(job *WordJob) {
	fyne.Do(func() {
		qm.updateQueueStatus()
	})
}

// onJobComplete handles job completion.
func (qm *QueueManager) onJobComplete(job *WordJob) {
	a := qm.app
	fyne.Do(func() {
		a.ensureHandlers()
		qm.updateQueueStatus()

		if job.ID == a.currentJobID && job.Status == StatusFailed {
			a.showError(job.Error)
			a.hideProgress()
			qm.finishCurrentJob()
		}

		if job.Status == StatusCompleted {
			a.nav.updateNavigation()

			a.mu.Lock()
			isCurrentJob := job.ID == a.currentJobID
			a.mu.Unlock()

			if isCurrentJob {
				a.updateStatus(fmt.Sprintf("Processing completed: %s", job.Word))
			} else {
				a.updateStatus(fmt.Sprintf("Background processing completed: %s", job.Word))

				a.mu.Lock()
				currentWord := a.currentWord
				a.mu.Unlock()

				if currentWord == job.Word {
					a.nav.loadExistingFiles(job.Word)
				}
			}
		}
	})
}

// updateQueueStatus updates the queue status label.
func (qm *QueueManager) updateQueueStatus() {
	a := qm.app
	a.mu.Lock()
	processing := a.processingCount
	a.mu.Unlock()

	savedCount := len(a.savedCards)
	existingCount := len(a.existingWords)
	completedJobs := a.queue.GetCompletedJobs()
	queueCompleted := len(completedJobs)

	totalCards := savedCount + existingCount + queueCompleted

	status := fmt.Sprintf("Processing: %d | Total cards: %d", processing, totalCards)

	a.queueStatusLabel.SetText(status)
}

// incrementProcessing increments the processing count and updates the status.
func (qm *QueueManager) incrementProcessing() {
	a := qm.app
	a.mu.Lock()
	a.processingCount++
	a.mu.Unlock()

	fyne.Do(func() {
		qm.updateQueueStatus()
	})
}

// decrementProcessing decrements the processing count and updates the status.
func (qm *QueueManager) decrementProcessing() {
	a := qm.app
	a.mu.Lock()
	if a.processingCount > 0 {
		a.processingCount--
	}
	a.mu.Unlock()

	fyne.Do(func() {
		qm.updateQueueStatus()
	})
}
