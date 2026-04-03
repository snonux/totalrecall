package story

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"

	"codeberg.org/snonux/totalrecall/internal/image"
)

const (
	// comicStripAspectRatio produces a tall portrait image — roughly three 4:3
	// panels stacked — giving the model room to render beginning, middle, and end
	// of the story in a single consistent scene without character drift across files.
	comicStripAspectRatio = "9:16"

	// comicPromptMaxChars caps the story excerpt embedded in the image prompt.
	comicPromptMaxChars = 1200
)

// comicStyles is the pool from which the strip style is drawn each run.
// Ultra realistic is selected 90% of the time; the remaining 10% comes from
// the other styles to provide occasional visual variety.
var comicStyles = []string{
	"ultra realistic comic strip with photographic detail and dramatic lighting",
	"classic American comic book with bold ink outlines, halftone dots, and primary colors",
	"Japanese manga with clean linework, expressive eyes, and speed lines",
	"retro 1960s pop art in the style of Roy Lichtenstein with thick outlines and Ben-Day dots",
	"watercolor illustration with soft washes, delicate linework, and pastel tones",
	"European bande dessinée with detailed backgrounds, clear lines, and rich flat colors",
	"noir black-and-white graphic novel with heavy shadows and high contrast",
	"children's picture book with bright, friendly illustrations and thick outlines",
	"painterly oil-on-canvas comic with loose brushwork and vivid impressionist colors",
	"cyberpunk neon art with glowing outlines, dark backgrounds, and electric accent colors",
}

// ArtistConfig holds settings for comic-book image generation via NanoBanana.
type ArtistConfig struct {
	APIKey    string // Google API key
	Model     string // NanoBanana image model
	TextModel string // NanoBanana text/prompt model
	OutputDir string // target directory; defaults to "."
	// Style overrides the random art-style pick when non-empty.
	Style string
}

// Artist generates a single tall comic-strip image that illustrates the story.
type Artist struct {
	nbClient  image.ImageClient
	outputDir string
	style     string // empty = pick randomly each run
}

// NewArtist creates an Artist backed by the NanoBanana image generator.
func NewArtist(config *ArtistConfig) *Artist {
	dir := "."
	if config != nil && config.OutputDir != "" {
		dir = config.OutputDir
	}

	var nbConfig *image.NanoBananaConfig
	var style string
	if config != nil {
		nbConfig = &image.NanoBananaConfig{
			APIKey:    config.APIKey,
			Model:     config.Model,
			TextModel: config.TextModel,
		}
		style = config.Style
	}

	return &Artist{
		nbClient:  image.NewNanoBananaClient(nbConfig),
		outputDir: dir,
		style:     style,
	}
}

// DrawComicStrip generates a single tall 9:16 comic-strip image covering the
// whole story in one scene with consistent characters.  The image is saved as
// comic_strip.png; attribution is auto-written as comic_strip_attribution.txt
// by the Downloader.  Returns the saved image path.
func (a *Artist) DrawComicStrip(storyText string) (string, error) {
	style := a.style
	if style == "" {
		style = pickStyle()
	}

	fmt.Printf("  Comic style: %s\n", style)

	opts := image.DefaultSearchOptions("vocabulary story")
	opts.CustomPrompt = buildComicStripPrompt(storyText, style)
	opts.AspectRatio = comicStripAspectRatio

	downloader := image.NewDownloader(a.nbClient, &image.DownloadOptions{
		OutputDir:         a.outputDir,
		OverwriteExisting: true,
		CreateDir:         true,
		FileNamePattern:   "comic_strip", // → comic_strip.png + comic_strip_attribution.txt
		MaxSizeBytes:      20 * 1024 * 1024,
	})

	ctx := context.Background()
	_, savedPath, err := downloader.DownloadBestMatchWithOptions(ctx, opts)
	if err != nil {
		return "", fmt.Errorf("comic strip generation failed: %w", err)
	}

	return savedPath, nil
}

// buildComicStripPrompt constructs a single prompt for a 3-panel tall comic
// strip covering the whole story.  The excerpt is capped at comicPromptMaxChars.
func buildComicStripPrompt(storyText, style string) string {
	excerpt := strings.TrimSpace(storyText)
	if len(excerpt) > comicPromptMaxChars {
		excerpt = excerpt[:comicPromptMaxChars]
		if idx := strings.LastIndex(excerpt, " "); idx > 0 {
			excerpt = excerpt[:idx]
		}
		excerpt += "…"
	}

	return fmt.Sprintf(
		"Art style: %s.\n"+
			"A single tall comic strip with 3 vertically stacked panels showing the beginning, "+
			"middle, and end of the story. Keep all characters visually consistent across panels. "+
			"Scene based on this Bulgarian vocabulary story:\n\n%s",
		style, excerpt,
	)
}

// pickStyle returns a randomly chosen art style.
// Ultra realistic is selected 90% of the time; one of the other styles fills
// the remaining 10% to provide occasional visual variety.
func pickStyle() string {
	if rand.Float64() < 0.9 {
		return comicStyles[0] // ultra realistic
	}
	return comicStyles[1+rand.IntN(len(comicStyles)-1)]
}
