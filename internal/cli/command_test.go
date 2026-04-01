package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func TestCreateRootCommand(t *testing.T) {
	flags := NewFlags()
	cmd := CreateRootCommand(flags)

	// Test basic command properties
	if cmd.Use != "totalrecall [word]" {
		t.Errorf("Expected Use to be 'totalrecall [word]', got %s", cmd.Use)
	}

	if !strings.Contains(cmd.Short, "Bulgarian Anki Flashcard Generator") {
		t.Errorf("Expected Short description to contain 'Bulgarian Anki Flashcard Generator'")
	}
	if !strings.Contains(cmd.Long, "uses Nano Banana for images by default") {
		t.Errorf("Expected Long description to describe the Nano Banana GUI default")
	}
	if !strings.Contains(cmd.Long, "Explicit CLI runs can use OpenAI or Nano Banana via --image-api") {
		t.Errorf("Expected Long description to describe explicit CLI Nano Banana support")
	}

	// Test that flags are set up
	flagTests := []struct {
		name     string
		expected bool
	}{
		{"config", true},
		{"output", true},
		{"format", true},
		{"image-api", true},
		{"batch", true},
		{"skip-audio", true},
		{"skip-images", true},
		{"anki", true},
		{"anki-csv", true},
		{"deck-name", true},
		{"list-models", true},
		{"all-voices", true},
		{"no-auto-play", true},
		{"openai-model", true},
		{"openai-voice", true},
		{"openai-speed", true},
		{"openai-instruction", true},
		{"openai-image-model", true},
		{"openai-image-size", true},
		{"openai-image-quality", true},
		{"openai-image-style", true},
		{"nanobanana-model", true},
		{"nanobanana-text-model", true},
	}

	for _, tt := range flagTests {
		t.Run("flag_"+tt.name, func(t *testing.T) {
			var flag *pflag.Flag
			if tt.name == "config" {
				flag = cmd.PersistentFlags().Lookup(tt.name)
			} else {
				flag = cmd.Flags().Lookup(tt.name)
			}
			if flag == nil && tt.expected {
				t.Errorf("Expected flag %s to exist", tt.name)
			}
		})
	}
}

func TestSetupFlags(t *testing.T) {
	cmd := &cobra.Command{}
	flags := NewFlags()

	setupFlags(cmd, flags)

	// Test default values
	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Fatal("output flag not found")
	}

	home, _ := os.UserHomeDir()
	expectedDefault := filepath.Join(home, ".local", "state", "totalrecall", "cards")
	if outputFlag.DefValue != expectedDefault {
		t.Errorf("Expected default output dir to be %s, got %s", expectedDefault, outputFlag.DefValue)
	}

	// Test audio format default
	formatFlag := cmd.Flags().Lookup("format")
	if formatFlag == nil {
		t.Fatal("format flag not found")
	}
	if formatFlag.DefValue != "mp3" {
		t.Errorf("Expected default format to be mp3, got %s", formatFlag.DefValue)
	}

	imageAPIFlag := cmd.Flags().Lookup("image-api")
	if imageAPIFlag == nil {
		t.Fatal("image-api flag not found")
	}
	if imageAPIFlag.DefValue != "openai" {
		t.Errorf("Expected default image-api to be openai, got %s", imageAPIFlag.DefValue)
	}
	if imageAPIFlag.Usage != "Image source for explicit CLI runs (OpenAI or Nano Banana; config file image.provider also applies when unset)" {
		t.Errorf("Expected image-api help to describe CLI Nano Banana support and config fallback, got %q", imageAPIFlag.Usage)
	}

	nanoBananaModelFlag := cmd.Flags().Lookup("nanobanana-model")
	if nanoBananaModelFlag == nil {
		t.Fatal("nanobanana-model flag not found")
	}
	if nanoBananaModelFlag.DefValue != "gemini-3.1-flash-image-preview" {
		t.Errorf("Expected default nanobanana-model to be gemini-3.1-flash-image-preview, got %s", nanoBananaModelFlag.DefValue)
	}
	if !strings.Contains(nanoBananaModelFlag.Usage, "selected") {
		t.Errorf("Expected nanobanana-model help to describe supported Nano Banana selection, got %q", nanoBananaModelFlag.Usage)
	}

	nanoBananaTextModelFlag := cmd.Flags().Lookup("nanobanana-text-model")
	if nanoBananaTextModelFlag == nil {
		t.Fatal("nanobanana-text-model flag not found")
	}
	if nanoBananaTextModelFlag.DefValue != "gemini-2.5-flash" {
		t.Errorf("Expected default nanobanana-text-model to be gemini-2.5-flash, got %s", nanoBananaTextModelFlag.DefValue)
	}
	if !strings.Contains(nanoBananaTextModelFlag.Usage, "selected") {
		t.Errorf("Expected nanobanana-text-model help to describe supported Nano Banana selection, got %q", nanoBananaTextModelFlag.Usage)
	}
}

func TestInitConfig(t *testing.T) {
	// Save original viper state
	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()

	tests := []struct {
		name        string
		cfgFile     string
		setupFunc   func(t *testing.T) string
		cleanupFunc func(string)
	}{
		{
			name:    "with config file",
			cfgFile: "test-config.yaml",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				cfgPath := filepath.Join(tmpDir, "test-config.yaml")
				content := `audio:
  provider: openai
  openai_key: test-key
output:
  directory: /test/output`
				err := os.WriteFile(cfgPath, []byte(content), 0644)
				if err != nil {
					t.Fatalf("Failed to create test config: %v", err)
				}
				return cfgPath
			},
			cleanupFunc: func(path string) {},
		},
		{
			name:    "without config file",
			cfgFile: "",
			setupFunc: func(t *testing.T) string {
				return ""
			},
			cleanupFunc: func(path string) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset viper for each test
			viper.Reset()

			cfgPath := tt.setupFunc(t)
			if tt.cfgFile != "" && cfgPath != "" {
				tt.cfgFile = cfgPath
			}

			InitConfig(tt.cfgFile)

			// Test environment variable prefix
			if err := os.Setenv("TOTALRECALL_TEST_VAR", "test-value"); err != nil {
				t.Fatalf("Failed to set env var: %v", err)
			}
			defer func() {
				if err := os.Unsetenv("TOTALRECALL_TEST_VAR"); err != nil {
					t.Errorf("Failed to unset env var: %v", err)
				}
			}()

			if viper.GetString("test_var") != "test-value" {
				t.Error("Environment variable not properly loaded")
			}

			tt.cleanupFunc(cfgPath)
		})
	}
}

func TestGetOpenAIKey(t *testing.T) {
	// Save original viper state
	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()

	tests := []struct {
		name      string
		envKey    string
		configKey string
		expected  string
	}{
		{
			name:      "from environment",
			envKey:    "env-test-key",
			configKey: "config-test-key",
			expected:  "env-test-key",
		},
		{
			name:      "from config when no env",
			envKey:    "",
			configKey: "config-test-key",
			expected:  "config-test-key",
		},
		{
			name:      "empty when neither set",
			envKey:    "",
			configKey: "",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset viper
			viper.Reset()

			// Set up environment
			if tt.envKey != "" {
				if err := os.Setenv("OPENAI_API_KEY", tt.envKey); err != nil {
					t.Fatalf("Failed to set OPENAI_API_KEY: %v", err)
				}
				defer func() {
					if err := os.Unsetenv("OPENAI_API_KEY"); err != nil {
						t.Errorf("Failed to unset OPENAI_API_KEY: %v", err)
					}
				}()
			} else {
				if err := os.Unsetenv("OPENAI_API_KEY"); err != nil {
					t.Fatalf("Failed to unset OPENAI_API_KEY: %v", err)
				}
			}

			// Set up config
			if tt.configKey != "" {
				viper.Set("audio.openai_key", tt.configKey)
			}

			got := GetOpenAIKey()
			if got != tt.expected {
				t.Errorf("GetOpenAIKey() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetGoogleAPIKey(t *testing.T) {
	// Save original viper state
	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()

	tests := []struct {
		name      string
		envKey    string
		configKey string
		legacyKey string
		expected  string
	}{
		{
			name:      "from environment",
			envKey:    "env-google-key",
			configKey: "config-google-key",
			legacyKey: "legacy-google-key",
			expected:  "env-google-key",
		},
		{
			name:      "from config when no env",
			envKey:    "",
			configKey: "config-google-key",
			legacyKey: "legacy-google-key",
			expected:  "config-google-key",
		},
		{
			name:      "falls back to legacy config key",
			envKey:    "",
			configKey: "",
			legacyKey: "legacy-google-key",
			expected:  "legacy-google-key",
		},
		{
			name:      "empty when neither set",
			envKey:    "",
			configKey: "",
			legacyKey: "",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()

			if tt.envKey != "" {
				if err := os.Setenv("GOOGLE_API_KEY", tt.envKey); err != nil {
					t.Fatalf("Failed to set GOOGLE_API_KEY: %v", err)
				}
				defer func() {
					if err := os.Unsetenv("GOOGLE_API_KEY"); err != nil {
						t.Errorf("Failed to unset GOOGLE_API_KEY: %v", err)
					}
				}()
			} else {
				if err := os.Unsetenv("GOOGLE_API_KEY"); err != nil {
					t.Fatalf("Failed to unset GOOGLE_API_KEY: %v", err)
				}
			}

			if tt.configKey != "" {
				viper.Set("image.google_api_key", tt.configKey)
			}
			if tt.legacyKey != "" {
				viper.Set("google.api_key", tt.legacyKey)
			}

			got := GetGoogleAPIKey()
			if got != tt.expected {
				t.Errorf("GetGoogleAPIKey() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetGoogleAPIKey_PrefersImageConfigOverLegacyConfig(t *testing.T) {
	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()

	viper.Reset()
	if err := os.Unsetenv("GOOGLE_API_KEY"); err != nil {
		t.Fatalf("Failed to unset GOOGLE_API_KEY: %v", err)
	}

	viper.Set("image.google_api_key", "new-google-key")
	viper.Set("google.api_key", "legacy-google-key")

	if got := GetGoogleAPIKey(); got != "new-google-key" {
		t.Fatalf("GetGoogleAPIKey() = %v, want %v", got, "new-google-key")
	}
}

func TestBindFlagsToViper(t *testing.T) {
	// Save original viper state
	originalConfig := viper.New()
	*originalConfig = *viper.GetViper()
	defer func() {
		*viper.GetViper() = *originalConfig
	}()

	// Reset viper
	viper.Reset()

	cmd := &cobra.Command{}
	flags := NewFlags()
	setupFlags(cmd, flags)

	// Set some flag values
	if err := cmd.Flags().Set("output", "/test/output"); err != nil {
		t.Fatalf("Failed to set output flag: %v", err)
	}
	if err := cmd.Flags().Set("format", "wav"); err != nil {
		t.Fatalf("Failed to set format flag: %v", err)
	}
	if err := cmd.Flags().Set("openai-model", "tts-1-hd"); err != nil {
		t.Fatalf("Failed to set openai-model flag: %v", err)
	}
	if err := cmd.Flags().Set("nanobanana-model", "gemini-3.1-flash-image-preview"); err != nil {
		t.Fatalf("Failed to set nanobanana-model flag: %v", err)
	}
	if err := cmd.Flags().Set("nanobanana-text-model", "gemini-2.5-flash"); err != nil {
		t.Fatalf("Failed to set nanobanana-text-model flag: %v", err)
	}

	if err := bindFlagsToViper(cmd); err != nil {
		t.Fatalf("bindFlagsToViper() failed: %v", err)
	}

	// Test that values are bound
	if viper.GetString("output.directory") != "/test/output" {
		t.Errorf("Expected output.directory to be /test/output, got %s", viper.GetString("output.directory"))
	}

	if viper.GetString("audio.format") != "wav" {
		t.Errorf("Expected audio.format to be wav, got %s", viper.GetString("audio.format"))
	}

	if viper.GetString("audio.openai_model") != "tts-1-hd" {
		t.Errorf("Expected audio.openai_model to be tts-1-hd, got %s", viper.GetString("audio.openai_model"))
	}
	if viper.GetString("image.nanobanana_model") != "gemini-3.1-flash-image-preview" {
		t.Errorf("Expected image.nanobanana_model to be gemini-3.1-flash-image-preview, got %s", viper.GetString("image.nanobanana_model"))
	}
	if viper.GetString("image.nanobanana_text_model") != "gemini-2.5-flash" {
		t.Errorf("Expected image.nanobanana_text_model to be gemini-2.5-flash, got %s", viper.GetString("image.nanobanana_text_model"))
	}
	if viper.GetString("image.provider") != "openai" {
		t.Errorf("Expected image.provider to be openai by default, got %s", viper.GetString("image.provider"))
	}
}
