package processor

// BatchProcessor runs multi-word batch files: translation pass, validation,
// per-word processing with timeouts, and summary output. It delegates
// per-word work and skip detection to Processor.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"codeberg.org/snonux/totalrecall/internal/audio"
	"codeberg.org/snonux/totalrecall/internal/batch"
)

// BatchProcessor orchestrates batch file processing. It holds a reference to
// the main Processor for shared services (translation, card directories,
// ProcessWordWithTranslationAndType).
type BatchProcessor struct {
	p *Processor
}

// ProcessBatch processes multiple words from a batch file.
// It first translates any entries that have English-to-Bulgarian translation
// needs, then validates all Bulgarian words, and finally processes each word
// with a per-word timeout to prevent a single hung API call from stalling the batch.
func (b *BatchProcessor) ProcessBatch() error {
	p := b.p
	entries, err := batch.ReadBatchFile(p.Flags.BatchFile)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(p.Flags.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := b.translateBatchEntries(entries); err != nil {
		return err
	}

	if err := b.validateBatchEntries(entries); err != nil {
		return err
	}

	skipped, processed, errCount := b.processBatchEntries(entries)

	b.printBatchSummary(len(entries), processed, skipped, errCount)
	return nil
}

// translateBatchEntries runs the first pass over entries that need English→Bulgarian
// translation and mutates the slice in place with the result.
func (b *BatchProcessor) translateBatchEntries(entries []batch.WordEntry) error {
	p := b.p
	for i, entry := range entries {
		if !entry.NeedsTranslation || entry.Translation == "" {
			continue
		}
		bulgarian, err := p.translator.TranslateEnglishToBulgarian(entry.Translation)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error translating '%s' to Bulgarian: %v\n", entry.Translation, err)
			continue
		}
		entries[i].Bulgarian = bulgarian
		fmt.Printf("Translated '%s' to Bulgarian: %s\n", entry.Translation, bulgarian)
	}
	return nil
}

// validateBatchEntries checks that every entry with a Bulgarian word contains
// only valid Bulgarian text. Returns on the first validation failure.
func (b *BatchProcessor) validateBatchEntries(entries []batch.WordEntry) error {
	for _, entry := range entries {
		if entry.Bulgarian == "" {
			continue
		}
		if err := audio.ValidateBulgarianText(entry.Bulgarian); err != nil {
			return fmt.Errorf("invalid word '%s': %w", entry.Bulgarian, err)
		}
	}
	return nil
}

// processBatchEntries iterates the validated entries and processes each word,
// skipping words that are already fully processed. Returns skip, process, and
// error counts for the summary.
func (b *BatchProcessor) processBatchEntries(entries []batch.WordEntry) (skipped, processed, errCount int) {
	p := b.p
	for i, entry := range entries {
		if entry.Bulgarian == "" {
			continue
		}

		fmt.Printf("\nProcessing %d/%d: %s\n", i+1, len(entries), entry.Bulgarian)

		if p.isWordFullyProcessed(entry.Bulgarian) {
			wordDir := p.findCardDirectory(entry.Bulgarian)
			fmt.Printf("  ✓ Skipping '%s' - already fully processed in %s\n", entry.Bulgarian, filepath.Base(wordDir))
			skipped++
			continue
		}

		wordCtx, wordCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		err := p.ProcessWordWithTranslationAndType(wordCtx, entry.Bulgarian, entry.Translation, entry.CardType)
		wordCancel()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing '%s': %v\n", entry.Bulgarian, err)
			errCount++
		} else {
			processed++
		}
	}
	return
}

// printBatchSummary prints a human-readable summary of the batch run.
func (b *BatchProcessor) printBatchSummary(total, processed, skipped, errCount int) {
	fmt.Printf("\n=== Batch Processing Summary ===\n")
	fmt.Printf("Total words: %d\n", total)
	fmt.Printf("Processed: %d\n", processed)
	fmt.Printf("Skipped (already complete): %d\n", skipped)
	if errCount > 0 {
		fmt.Printf("Errors: %d\n", errCount)
	}
	fmt.Printf("================================\n")
}
