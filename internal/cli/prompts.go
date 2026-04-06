package cli

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// PromptForGalleryVideos lists all *_gallery_*.png files found under outputDir
// (searching recursively), shows them to the user, and asks whether they want
// to generate videos. If the user agrees, it asks which pages to generate
// (e.g. "1,3,5" or "all") and returns the paths of the selected PNGs.
//
// Returning paths (rather than page numbers) lets the caller pass them
// directly to GenerateSelectedVideos without a second directory lookup,
// which would fail because gallery images live in a per-comic subdirectory
// (comics/<slug>/) rather than in the top-level output directory.
//
// Returns an empty slice when the user declines or enters nothing.
// Returns an error only on unexpected I/O or parse failures.
func PromptForGalleryVideos(outputDir string) ([]string, error) {
	pages, pngPaths, err := findGalleryPages(outputDir)
	if err != nil {
		return nil, err
	}

	if len(pages) == 0 {
		fmt.Println("No gallery PNG files found — skipping video generation.")
		return nil, nil
	}

	printGalleryFiles(pngPaths)

	agreed, err := askYesNo("Generate videos for these gallery pages? [y/N]: ")
	if err != nil {
		return nil, err
	}
	if !agreed {
		return nil, nil
	}

	selectedPages, err := askPageSelection(pages)
	if err != nil {
		return nil, err
	}

	return filterPathsByPages(pngPaths, selectedPages), nil
}

// filterPathsByPages returns only those paths whose embedded page number
// appears in the selectedPages slice. The result preserves the order from
// pngPaths (which is already sorted alphabetically by findGalleryPages).
func filterPathsByPages(pngPaths []string, selectedPages []int) []string {
	pageSet := make(map[int]struct{}, len(selectedPages))
	for _, p := range selectedPages {
		pageSet[p] = struct{}{}
	}

	result := make([]string, 0, len(selectedPages))
	for _, path := range pngPaths {
		n := extractPageNumber(filepath.Base(path))
		if _, ok := pageSet[n]; ok {
			result = append(result, path)
		}
	}
	return result
}

// findGalleryPages walks outputDir recursively looking for *_gallery_*.png
// files and returns the sorted list of unique page numbers and matching paths.
// Walking recursively is necessary because the story runner places gallery
// images in a per-comic subdirectory (comics/<slug>/) rather than directly
// in the top-level output directory.
func findGalleryPages(outputDir string) ([]int, []string, error) {
	var matches []string

	err := filepath.WalkDir(outputDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Skip unreadable directories rather than aborting the whole walk.
			return nil
		}
		if d.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		// Match files that follow the *_gallery_N.png naming convention.
		if strings.Contains(base, "_gallery_") && strings.HasSuffix(base, ".png") {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("cli: walking gallery files in %s: %w", outputDir, err)
	}

	sort.Strings(matches)

	pageSet := map[int]struct{}{}
	for _, m := range matches {
		n := extractPageNumber(filepath.Base(m))
		if n > 0 {
			pageSet[n] = struct{}{}
		}
	}

	pages := make([]int, 0, len(pageSet))
	for n := range pageSet {
		pages = append(pages, n)
	}
	sort.Ints(pages)

	return pages, matches, nil
}

// extractPageNumber parses the page number from a gallery file name of the
// form "<slug>_gallery_<N>.png". Returns 0 when the name does not match.
func extractPageNumber(base string) int {
	// Strip extension
	name := strings.TrimSuffix(base, ".png")
	// Find the last "_gallery_" segment and extract the trailing integer.
	const marker = "_gallery_"
	idx := strings.LastIndex(name, marker)
	if idx < 0 {
		return 0
	}
	numStr := name[idx+len(marker):]
	n, err := strconv.Atoi(numStr)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// printGalleryFiles prints each gallery PNG path so the user can review what
// will be animated before confirming.
func printGalleryFiles(paths []string) {
	fmt.Println("Found gallery pages:")
	for _, p := range paths {
		fmt.Printf("  %s\n", p)
	}
}

// askYesNo prints prompt, reads one line from stdin, and returns true only
// when the user types "y" or "Y". Any other input (including empty) returns
// false, matching a safe-default "no" behaviour.
func askYesNo(prompt string) (bool, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("cli: reading user input: %w", err)
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	return answer == "y", nil
}

// askPageSelection prints a prompt asking the user which pages to include
// and parses the reply into a slice of ints. "all" expands to every available
// page number. An empty reply is treated as "all".
func askPageSelection(availablePages []int) ([]int, error) {
	max := 0
	if len(availablePages) > 0 {
		max = availablePages[len(availablePages)-1]
	}

	fmt.Printf("Which pages? (e.g. 1,3,5 or all) [all]: ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("cli: reading page selection: %w", err)
	}

	input := strings.TrimSpace(line)
	if input == "" || strings.ToLower(input) == "all" {
		return availablePages, nil
	}

	selected, err := parseSelection(input, max)
	if err != nil {
		return nil, err
	}

	// Filter to only pages that actually exist.
	pageExists := make(map[int]bool, len(availablePages))
	for _, p := range availablePages {
		pageExists[p] = true
	}

	result := make([]int, 0, len(selected))
	for _, p := range selected {
		if pageExists[p] {
			result = append(result, p)
		} else {
			fmt.Printf("  Warning: page %d not found — skipping.\n", p)
		}
	}

	return result, nil
}

// parseSelection converts a comma-separated string of page numbers (e.g. "1,3,5")
// or the keyword "all" into a sorted, deduplicated slice of ints.
//
// max is used only when input is "all"; individual page numbers may exceed max
// without error (the caller is responsible for validating against real files).
// Returns an error for non-numeric tokens or numbers <= 0.
func parseSelection(input string, max int) ([]int, error) {
	input = strings.TrimSpace(input)
	if strings.ToLower(input) == "all" {
		pages := make([]int, max)
		for i := range pages {
			pages[i] = i + 1
		}
		return pages, nil
	}

	seen := map[int]struct{}{}
	tokens := strings.Split(input, ",")

	for _, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		n, err := strconv.Atoi(tok)
		if err != nil {
			return nil, fmt.Errorf("cli: invalid page number %q: %w", tok, err)
		}
		if n <= 0 {
			return nil, fmt.Errorf("cli: page numbers must be positive, got %d", n)
		}
		seen[n] = struct{}{}
	}

	result := make([]int, 0, len(seen))
	for n := range seen {
		result = append(result, n)
	}
	sort.Ints(result)

	return result, nil
}
