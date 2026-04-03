package story

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// AssembleComicPDF combines the 5 comic images (cover, 3 story pages, back cover)
// into a single portrait PDF using ImageMagick's convert command.
// The PDF is named <titleSlug>.pdf and placed in outputDir.
// The PDF pages are in reading order: front cover → story pages → back cover.
// Returns the path to the written PDF, or an error if ImageMagick is not available
// or any of the required source images are missing.
func AssembleComicPDF(outputDir, titleSlug string, imagePaths []string) (string, error) {
	if len(imagePaths) == 0 {
		return "", fmt.Errorf("no comic images to assemble into PDF")
	}

	if _, err := exec.LookPath("convert"); err != nil {
		return "", fmt.Errorf("ImageMagick 'convert' not found — install ImageMagick to generate the PDF")
	}

	pdfPath := filepath.Join(outputDir, titleSlug+".pdf")

	// Build the convert command:
	//   convert -density 150 page1.png page2.png ... output.pdf
	// -density 150 gives a reasonable print resolution without huge file sizes.
	// Each PNG is added as a separate PDF page in the order provided.
	args := []string{"-density", "150"}
	args = append(args, imagePaths...)
	args = append(args, pdfPath)

	cmd := exec.Command("convert", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("convert failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	return pdfPath, nil
}
