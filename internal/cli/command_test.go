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
			os.Setenv("TOTALRECALL_TEST_VAR", "test-value")
			defer os.Unsetenv("TOTALRECALL_TEST_VAR")

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
				os.Setenv("OPENAI_API_KEY", tt.envKey)
				defer os.Unsetenv("OPENAI_API_KEY")
			} else {
				os.Unsetenv("OPENAI_API_KEY")
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
	cmd.Flags().Set("output", "/test/output")
	cmd.Flags().Set("format", "wav")
	cmd.Flags().Set("openai-model", "tts-1-hd")

	bindFlagsToViper(cmd)

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
}
