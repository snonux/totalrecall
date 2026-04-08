package anki

import (
	"embed"
	"fmt"
	"time"
)

//go:embed templates/*.html templates/*.css
var embeddedCardAssets embed.FS

// CardTemplate loads Anki card HTML/CSS from embedded files and builds note-type JSON for the collection.
type CardTemplate struct {
	enBgFront, enBgBack, enBgReverseFront, enBgReverseBack string
	bgBgFront, bgBgBack, bgBgReverseFront, bgBgReverseBack string
	css                                                    string
}

// NewCardTemplate reads embedded template and stylesheet files.
func NewCardTemplate() (*CardTemplate, error) {
	read := func(name string) (string, error) {
		b, err := embeddedCardAssets.ReadFile("templates/" + name)
		if err != nil {
			return "", fmt.Errorf("read template %s: %w", name, err)
		}
		return string(b), nil
	}
	c := &CardTemplate{}
	var err error
	if c.enBgFront, err = read("en_bg_front.html"); err != nil {
		return nil, err
	}
	if c.enBgBack, err = read("en_bg_back.html"); err != nil {
		return nil, err
	}
	if c.enBgReverseFront, err = read("en_bg_reverse_front.html"); err != nil {
		return nil, err
	}
	if c.enBgReverseBack, err = read("en_bg_reverse_back.html"); err != nil {
		return nil, err
	}
	if c.bgBgFront, err = read("bg_bg_front.html"); err != nil {
		return nil, err
	}
	if c.bgBgBack, err = read("bg_bg_back.html"); err != nil {
		return nil, err
	}
	if c.bgBgReverseFront, err = read("bg_bg_reverse_front.html"); err != nil {
		return nil, err
	}
	if c.bgBgReverseBack, err = read("bg_bg_reverse_back.html"); err != nil {
		return nil, err
	}
	if c.css, err = read("card.css"); err != nil {
		return nil, err
	}
	return c, nil
}

// MustCardTemplate returns NewCardTemplate() or panics; suitable for process startup when assets must exist.
func MustCardTemplate() *CardTemplate {
	t, err := NewCardTemplate()
	if err != nil {
		panic(err)
	}
	return t
}

// EnBgNoteTypeConfig builds the English–Bulgarian note type map for Anki's models JSON.
func (c *CardTemplate) EnBgNoteTypeConfig(modelID, deckID int64) map[string]interface{} {
	return map[string]interface{}{
		"id":    modelID,
		"name":  "Vocabulary from TotalRecall (Basic + Reverse)",
		"type":  0,
		"mod":   time.Now().Unix(),
		"usn":   -1,
		"sortf": 0,
		"did":   deckID,
		"req":   [][]interface{}{[]interface{}{0, "all", []int{0}}, []interface{}{1, "all", []int{1}}},
		"vers":  []int{},
		"tags":  []string{},
		"latexPre": `\documentclass[12pt]{article}
\special{papersize=3in,5in}
\usepackage[utf8]{inputenc}
\usepackage{amssymb,amsmath}
\pagestyle{empty}
\setlength{\parindent}{0in}
\begin{document}`,
		"latexPost": `\end{document}`,
		"flds": []map[string]interface{}{
			{
				"name":   "English",
				"ord":    0,
				"sticky": false,
				"rtl":    false,
				"font":   "Arial",
				"size":   20,
				"media":  []string{},
			},
			{
				"name":   "Bulgarian",
				"ord":    1,
				"sticky": false,
				"rtl":    false,
				"font":   "Arial",
				"size":   20,
				"media":  []string{},
			},
			{
				"name":   "Image",
				"ord":    2,
				"sticky": false,
				"rtl":    false,
				"font":   "Arial",
				"size":   20,
				"media":  []string{},
			},
			{
				"name":   "Audio",
				"ord":    3,
				"sticky": false,
				"rtl":    false,
				"font":   "Arial",
				"size":   20,
				"media":  []string{},
			},
			{
				"name":   "Notes",
				"ord":    4,
				"sticky": false,
				"rtl":    false,
				"font":   "Arial",
				"size":   16,
				"media":  []string{},
			},
		},
		"tmpls": []map[string]interface{}{
			{
				"name":  "Forward",
				"ord":   0,
				"qfmt":  c.enBgFront,
				"afmt":  c.enBgBack,
				"did":   nil,
				"bqfmt": "",
				"bafmt": "",
			},
			{
				"name":  "Reverse",
				"ord":   1,
				"qfmt":  c.enBgReverseFront,
				"afmt":  c.enBgReverseBack,
				"did":   nil,
				"bqfmt": "",
				"bafmt": "",
			},
		},
		"css": c.css,
	}
}

// BgBgNoteTypeConfig builds the Bulgarian–Bulgarian note type map for Anki's models JSON.
func (c *CardTemplate) BgBgNoteTypeConfig(modelIDBgBg, deckID int64) map[string]interface{} {
	return map[string]interface{}{
		"id":    modelIDBgBg,
		"name":  "Bulgarian-Bulgarian from TotalRecall",
		"type":  0,
		"mod":   time.Now().Unix(),
		"usn":   -1,
		"sortf": 0,
		"did":   deckID,
		"req":   [][]interface{}{[]interface{}{0, "all", []int{0}}, []interface{}{1, "all", []int{1}}},
		"vers":  []int{},
		"tags":  []string{},
		"latexPre": `\documentclass[12pt]{article}
\special{papersize=3in,5in}
\usepackage[utf8]{inputenc}
\usepackage{amssymb,amsmath}
\pagestyle{empty}
\setlength{\parindent}{0in}
\begin{document}`,
		"latexPost": `\end{document}`,
		"flds": []map[string]interface{}{
			{
				"name":   "BulgarianFront",
				"ord":    0,
				"sticky": false,
				"rtl":    false,
				"font":   "Arial",
				"size":   20,
				"media":  []string{},
			},
			{
				"name":   "BulgarianBack",
				"ord":    1,
				"sticky": false,
				"rtl":    false,
				"font":   "Arial",
				"size":   20,
				"media":  []string{},
			},
			{
				"name":   "Image",
				"ord":    2,
				"sticky": false,
				"rtl":    false,
				"font":   "Arial",
				"size":   20,
				"media":  []string{},
			},
			{
				"name":   "AudioFront",
				"ord":    3,
				"sticky": false,
				"rtl":    false,
				"font":   "Arial",
				"size":   20,
				"media":  []string{},
			},
			{
				"name":   "AudioBack",
				"ord":    4,
				"sticky": false,
				"rtl":    false,
				"font":   "Arial",
				"size":   20,
				"media":  []string{},
			},
			{
				"name":   "Notes",
				"ord":    5,
				"sticky": false,
				"rtl":    false,
				"font":   "Arial",
				"size":   16,
				"media":  []string{},
			},
		},
		"tmpls": []map[string]interface{}{
			{
				"name":  "Forward",
				"ord":   0,
				"qfmt":  c.bgBgFront,
				"afmt":  c.bgBgBack,
				"did":   nil,
				"bqfmt": "",
				"bafmt": "",
			},
			{
				"name":  "Reverse",
				"ord":   1,
				"qfmt":  c.bgBgReverseFront,
				"afmt":  c.bgBgReverseBack,
				"did":   nil,
				"bqfmt": "",
				"bafmt": "",
			},
		},
		"css": c.css,
	}
}
