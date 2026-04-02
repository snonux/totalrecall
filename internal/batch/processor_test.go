package batch

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"codeberg.org/snonux/totalrecall/internal"
)

func TestReadBatchFile(t *testing.T) {
	t.Parallel()
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
				{Bulgarian: "ябълка", Translation: "apple", NeedsTranslation: false, CardType: internal.CardTypeEnBg},
				{Bulgarian: "котка", Translation: "cat", NeedsTranslation: false, CardType: internal.CardTypeEnBg},
				{Bulgarian: "куче", Translation: "dog", NeedsTranslation: false, CardType: internal.CardTypeEnBg},
			},
		},
		{
			name: "mixed format",
			fileContent: `ябълка
котка = cat
куче
хляб = bread`,
			want: []WordEntry{
				{Bulgarian: "ябълка", Translation: "", NeedsTranslation: false, CardType: internal.CardTypeEnBg},
				{Bulgarian: "котка", Translation: "cat", NeedsTranslation: false, CardType: internal.CardTypeEnBg},
				{Bulgarian: "куче", Translation: "", NeedsTranslation: false, CardType: internal.CardTypeEnBg},
				{Bulgarian: "хляб", Translation: "bread", NeedsTranslation: false, CardType: internal.CardTypeEnBg},
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
				{Bulgarian: "ябълка", Translation: "", NeedsTranslation: false, CardType: internal.CardTypeEnBg},
				{Bulgarian: "котка", Translation: "cat", NeedsTranslation: false, CardType: internal.CardTypeEnBg},
				{Bulgarian: "куче", Translation: "", NeedsTranslation: false, CardType: internal.CardTypeEnBg},
			},
		},
		{
			name:        "windows line endings",
			fileContent: "ябълка\r\nкотка = cat\r\nкуче",
			want: []WordEntry{
				{Bulgarian: "ябълка", Translation: "", NeedsTranslation: false, CardType: internal.CardTypeEnBg},
				{Bulgarian: "котка", Translation: "cat", NeedsTranslation: false, CardType: internal.CardTypeEnBg},
				{Bulgarian: "куче", Translation: "", NeedsTranslation: false, CardType: internal.CardTypeEnBg},
			},
		},
		{
			name:        "multiple equals signs",
			fileContent: `test = word = with = equals`,
			want: []WordEntry{
				{Bulgarian: "test", Translation: "word = with = equals", NeedsTranslation: false, CardType: internal.CardTypeEnBg},
			},
		},
		{
			name: "english only format",
			fileContent: `= apple
= cat
= dog`,
			want: []WordEntry{
				{Bulgarian: "", Translation: "apple", NeedsTranslation: true, CardType: internal.CardTypeEnBg},
				{Bulgarian: "", Translation: "cat", NeedsTranslation: true, CardType: internal.CardTypeEnBg},
				{Bulgarian: "", Translation: "dog", NeedsTranslation: true, CardType: internal.CardTypeEnBg},
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
				{Bulgarian: "ябълка", Translation: "", NeedsTranslation: false, CardType: internal.CardTypeEnBg},
				{Bulgarian: "котка", Translation: "cat", NeedsTranslation: false, CardType: internal.CardTypeEnBg},
				{Bulgarian: "", Translation: "dog", NeedsTranslation: true, CardType: internal.CardTypeEnBg},
				{Bulgarian: "хляб", Translation: "bread", NeedsTranslation: false, CardType: internal.CardTypeEnBg},
				{Bulgarian: "", Translation: "table", NeedsTranslation: true, CardType: internal.CardTypeEnBg},
				{Bulgarian: "стол", Translation: "", NeedsTranslation: false, CardType: internal.CardTypeEnBg},
			},
		},
		{
			name: "bulgarian-bulgarian format with double equals",
			fileContent: `ябълка == плод
котка == домашно животно`,
			want: []WordEntry{
				{Bulgarian: "ябълка", Translation: "плод", NeedsTranslation: false, CardType: internal.CardTypeBgBg},
				{Bulgarian: "котка", Translation: "домашно животно", NeedsTranslation: false, CardType: internal.CardTypeBgBg},
			},
		},
		{
			name: "mixed en-bg and bg-bg formats",
			fileContent: `ябълка = apple
котка == домашно животно
куче = dog
вода == течност`,
			want: []WordEntry{
				{Bulgarian: "ябълка", Translation: "apple", NeedsTranslation: false, CardType: internal.CardTypeEnBg},
				{Bulgarian: "котка", Translation: "домашно животно", NeedsTranslation: false, CardType: internal.CardTypeBgBg},
				{Bulgarian: "куче", Translation: "dog", NeedsTranslation: false, CardType: internal.CardTypeEnBg},
				{Bulgarian: "вода", Translation: "течност", NeedsTranslation: false, CardType: internal.CardTypeBgBg},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
	t.Parallel()
	_, err := ReadBatchFile("/nonexistent/file.txt")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

