// Package processor contains the core business logic for processing Bulgarian
// words. Processor composes BatchProcessor (batch file orchestration),
// AnkiExporter (deck export), and CLIConfigResolver (CLI vs config-file
// precedence for run-mode settings). It also coordinates audio generation,
// image downloading, translation, and phonetic fetching.
package processor
