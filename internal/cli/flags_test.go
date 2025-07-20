package cli

import (
	"reflect"
	"testing"
)

func TestNewFlags(t *testing.T) {
	flags := NewFlags()

	// Test default values
	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"AudioFormat", flags.AudioFormat, "mp3"},
		{"ImageAPI", flags.ImageAPI, "openai"},
		{"DeckName", flags.DeckName, "Bulgarian Vocabulary"},
		{"OpenAIModel", flags.OpenAIModel, "gpt-4o-mini-tts"},
		{"OpenAISpeed", flags.OpenAISpeed, 0.9},
		{"OpenAIImageModel", flags.OpenAIImageModel, "dall-e-3"},
		{"OpenAIImageSize", flags.OpenAIImageSize, "1024x1024"},
		{"OpenAIImageQuality", flags.OpenAIImageQuality, "standard"},
		{"OpenAIImageStyle", flags.OpenAIImageStyle, "natural"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.got, tt.expected) {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.expected)
			}
		})
	}

	// Test boolean defaults (should be false)
	boolTests := []struct {
		name  string
		value bool
	}{
		{"SkipAudio", flags.SkipAudio},
		{"SkipImages", flags.SkipImages},
		{"GenerateAnki", flags.GenerateAnki},
		{"AnkiCSV", flags.AnkiCSV},
		{"ListModels", flags.ListModels},
		{"AllVoices", flags.AllVoices},
		{"GUIMode", flags.GUIMode},
	}

	for _, tt := range boolTests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != false {
				t.Errorf("%s = %v, want false", tt.name, tt.value)
			}
		})
	}

	// Test string defaults (should be empty)
	stringTests := []struct {
		name  string
		value string
	}{
		{"CfgFile", flags.CfgFile},
		{"OutputDir", flags.OutputDir},
		{"BatchFile", flags.BatchFile},
		{"OpenAIVoice", flags.OpenAIVoice},
		{"OpenAIInstruction", flags.OpenAIInstruction},
	}

	for _, tt := range stringTests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != "" {
				t.Errorf("%s = %v, want empty string", tt.name, tt.value)
			}
		})
	}
}

func TestFlagsStructure(t *testing.T) {
	// Test that Flags struct has all expected fields
	flags := &Flags{}
	flagsType := reflect.TypeOf(*flags)

	expectedFields := []string{
		"CfgFile", "OutputDir", "AudioFormat", "ImageAPI", "BatchFile",
		"SkipAudio", "SkipImages", "GenerateAnki", "AnkiCSV", "DeckName",
		"ListModels", "AllVoices", "GUIMode",
		"OpenAIModel", "OpenAIVoice", "OpenAISpeed", "OpenAIInstruction",
		"OpenAIImageModel", "OpenAIImageSize", "OpenAIImageQuality", "OpenAIImageStyle",
	}

	for _, fieldName := range expectedFields {
		t.Run("has_field_"+fieldName, func(t *testing.T) {
			if _, ok := flagsType.FieldByName(fieldName); !ok {
				t.Errorf("Flags struct missing field: %s", fieldName)
			}
		})
	}
}
