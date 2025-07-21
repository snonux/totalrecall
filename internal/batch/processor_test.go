package batch

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadBatchFile(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		want        []WordEntry
		wantErr     bool
	}{
		{
			name:        "empty file",
			fileContent: "",
			want:        nil,
		},
		{
			name:        "only whitespace",
			fileContent: "   \n\t\r\n   ",
			want:        nil,
		},
		{
			name: "words with translations",
			fileContent: `ябълка = apple
котка = cat
куче = dog`,
			want: []WordEntry{
				{Bulgarian: "ябълка", Translation: "apple", NeedsTranslation: false},
				{Bulgarian: "котка", Translation: "cat", NeedsTranslation: false},
				{Bulgarian: "куче", Translation: "dog", NeedsTranslation: false},
			},
		},
		{
			name: "mixed format",
			fileContent: `ябълка
котка = cat
куче
хляб = bread`,
			want: []WordEntry{
				{Bulgarian: "ябълка", Translation: "", NeedsTranslation: false},
				{Bulgarian: "котка", Translation: "cat", NeedsTranslation: false},
				{Bulgarian: "куче", Translation: "", NeedsTranslation: false},
				{Bulgarian: "хляб", Translation: "bread", NeedsTranslation: false},
			},
		},
		{
			name: "empty lines and whitespace",
			fileContent: `
ябълка

котка = cat  

  куче  

`,
			want: []WordEntry{
				{Bulgarian: "ябълка", Translation: "", NeedsTranslation: false},
				{Bulgarian: "котка", Translation: "cat", NeedsTranslation: false},
				{Bulgarian: "куче", Translation: "", NeedsTranslation: false},
			},
		},
		{
			name:        "windows line endings",
			fileContent: "ябълка\r\nкотка = cat\r\nкуче",
			want: []WordEntry{
				{Bulgarian: "ябълка", Translation: "", NeedsTranslation: false},
				{Bulgarian: "котка", Translation: "cat", NeedsTranslation: false},
				{Bulgarian: "куче", Translation: "", NeedsTranslation: false},
			},
		},
		{
			name:        "multiple equals signs",
			fileContent: `test = word = with = equals`,
			want: []WordEntry{
				{Bulgarian: "test", Translation: "word = with = equals", NeedsTranslation: false},
			},
		},
		{
			name: "english only format",
			fileContent: `= apple
= cat
= dog`,
			want: []WordEntry{
				{Bulgarian: "", Translation: "apple", NeedsTranslation: true},
				{Bulgarian: "", Translation: "cat", NeedsTranslation: true},
				{Bulgarian: "", Translation: "dog", NeedsTranslation: true},
			},
		},
		{
			name: "all three formats mixed",
			fileContent: `ябълка
котка = cat
= dog
хляб = bread
= table
стол`,
			want: []WordEntry{
				{Bulgarian: "ябълка", Translation: "", NeedsTranslation: false},
				{Bulgarian: "котка", Translation: "cat", NeedsTranslation: false},
				{Bulgarian: "", Translation: "dog", NeedsTranslation: true},
				{Bulgarian: "хляб", Translation: "bread", NeedsTranslation: false},
				{Bulgarian: "", Translation: "table", NeedsTranslation: true},
				{Bulgarian: "стол", Translation: "", NeedsTranslation: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test.txt")
			err := os.WriteFile(tmpFile, []byte(tt.fileContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			got, err := ReadBatchFile(tmpFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadBatchFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReadBatchFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReadBatchFile_FileNotFound(t *testing.T) {
	_, err := ReadBatchFile("/nonexistent/file.txt")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "unix line endings",
			input: "line1\nline2\nline3",
			want:  []string{"line1", "line2", "line3"},
		},
		{
			name:  "windows line endings",
			input: "line1\r\nline2\r\nline3",
			want:  []string{"line1", "line2", "line3"},
		},
		{
			name:  "mixed line endings",
			input: "line1\nline2\r\nline3",
			want:  []string{"line1", "line2", "line3"},
		},
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:  "single line no ending",
			input: "single line",
			want:  []string{"single line"},
		},
		{
			name:  "trailing newline",
			input: "line1\nline2\n",
			want:  []string{"line1", "line2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitLines(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitLines() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no whitespace",
			input: "hello",
			want:  "hello",
		},
		{
			name:  "leading spaces",
			input: "   hello",
			want:  "hello",
		},
		{
			name:  "trailing spaces",
			input: "hello   ",
			want:  "hello",
		},
		{
			name:  "both sides",
			input: "   hello   ",
			want:  "hello",
		},
		{
			name:  "tabs and spaces",
			input: "\t  hello  \t",
			want:  "hello",
		},
		{
			name:  "newlines",
			input: "\nhello\n",
			want:  "hello",
		},
		{
			name:  "all whitespace types",
			input: " \t\n\rhello \t\n\r",
			want:  "hello",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only whitespace",
			input: "   \t\n\r   ",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimSpace(tt.input)
			if got != tt.want {
				t.Errorf("trimSpace() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsSpace(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{' ', true},
		{'\t', true},
		{'\n', true},
		{'\r', true},
		{'a', false},
		{'1', false},
		{'!', false},
		{0, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.r), func(t *testing.T) {
			if got := isSpace(tt.r); got != tt.want {
				t.Errorf("isSpace(%q) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}
