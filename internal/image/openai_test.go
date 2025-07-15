package image

import (
	"context"
	"os"
	"testing"
)

func TestOpenAIClient_NewClient(t *testing.T) {
	tests := []struct {
		name   string
		config *OpenAIConfig
		wantNil bool
	}{
		{
			name: "with API key",
			config: &OpenAIConfig{
				APIKey: "test-key",
				Model:  "dall-e-2",
				Size:   "512x512",
			},
			wantNil: false,
		},
		{
			name: "without API key",
			config: &OpenAIConfig{
				APIKey: "",
			},
			wantNil: false, // Client is created but will fail on operations
		},
		{
			name: "with defaults",
			config: &OpenAIConfig{
				APIKey: "test-key",
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewOpenAIClient(tt.config)
			if (client == nil) != tt.wantNil {
				t.Errorf("NewOpenAIClient() returned nil = %v, want %v", client == nil, tt.wantNil)
			}
			
			if client != nil && tt.config.APIKey != "" {
				// Check defaults were set
				if tt.config.Model == "" && client.model != "dall-e-2" {
					t.Errorf("Expected default model dall-e-2, got %s", client.model)
				}
				if tt.config.Size == "" && client.size != "512x512" {
					t.Errorf("Expected default size 512x512, got %s", client.size)
				}
			}
		})
	}
}

func TestOpenAIClient_createEducationalPrompt(t *testing.T) {
	client := &OpenAIClient{}
	
	tests := []struct {
		bulgarian string
		english   string
		wantContains []string
	}{
		{
			bulgarian: "ябълка",
			english:   "apple",
			wantContains: []string{"apple", "educational", "flashcard"},
		},
		{
			bulgarian: "котка",
			english:   "cat",
			wantContains: []string{"cat", "simple", "clear"},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.bulgarian, func(t *testing.T) {
			prompt := client.createEducationalPrompt(tt.bulgarian, tt.english)
			
			for _, want := range tt.wantContains {
				if !contains(prompt, want) {
					t.Errorf("Prompt missing expected word '%s': %s", want, prompt)
				}
			}
		})
	}
}

func TestOpenAIClient_getCacheFilePath(t *testing.T) {
	client := &OpenAIClient{
		model:    "dall-e-2",
		size:     "512x512",
		quality:  "standard",
		style:    "natural",
		cacheDir: "./.test_cache",
	}
	
	// Test that same input produces same cache path
	path1 := client.getCacheFilePath("ябълка")
	path2 := client.getCacheFilePath("ябълка")
	
	if path1 != path2 {
		t.Errorf("Cache paths differ for same input: %s vs %s", path1, path2)
	}
	
	// Test that different inputs produce different paths
	path3 := client.getCacheFilePath("котка")
	if path1 == path3 {
		t.Errorf("Cache paths same for different inputs")
	}
	
	// Test path structure
	if !contains(path1, ".test_cache") {
		t.Errorf("Cache path doesn't contain cache dir: %s", path1)
	}
	
	if !contains(path1, ".png") {
		t.Errorf("Cache path doesn't have .png extension: %s", path1)
	}
}

// translateBulgarianToEnglish test removed - now uses OpenAI API

func TestOpenAIClient_getSizeWidthHeight(t *testing.T) {
	tests := []struct {
		size   string
		width  int
		height int
	}{
		{"256x256", 256, 256},
		{"512x512", 512, 512},
		{"1024x1024", 1024, 1024},
		{"1024x1792", 1024, 1792},
		{"1792x1024", 1792, 1024},
		{"unknown", 512, 512}, // Default
	}
	
	for _, tt := range tests {
		t.Run(tt.size, func(t *testing.T) {
			client := &OpenAIClient{size: tt.size}
			
			if w := client.getSizeWidth(); w != tt.width {
				t.Errorf("getSizeWidth() = %d, want %d", w, tt.width)
			}
			
			if h := client.getSizeHeight(); h != tt.height {
				t.Errorf("getSizeHeight() = %d, want %d", h, tt.height)
			}
		})
	}
}

func TestOpenAIClient_Search_NoAPIKey(t *testing.T) {
	client := NewOpenAIClient(&OpenAIConfig{})
	
	opts := DefaultSearchOptions("ябълка")
	_, err := client.Search(context.Background(), opts)
	
	if err == nil {
		t.Error("Expected error for missing API key")
	}
	
	if searchErr, ok := err.(*SearchError); ok {
		if searchErr.Code != "NO_API_KEY" {
			t.Errorf("Expected NO_API_KEY error, got %s", searchErr.Code)
		}
	} else {
		t.Error("Expected SearchError type")
	}
}

func TestOpenAIClient_Name(t *testing.T) {
	client := &OpenAIClient{}
	if name := client.Name(); name != "openai" {
		t.Errorf("Name() = %s, want 'openai'", name)
	}
}

func TestOpenAIClient_GetAttribution(t *testing.T) {
	client := &OpenAIClient{}
	result := &SearchResult{}
	
	attr := client.GetAttribution(result)
	if !contains(attr, "OpenAI DALL-E") {
		t.Errorf("Attribution doesn't mention OpenAI DALL-E: %s", attr)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		(s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Integration test (skipped by default)
func TestOpenAIClient_Search_Integration(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}
	
	client := NewOpenAIClient(&OpenAIConfig{
		APIKey:      apiKey,
		Model:       "dall-e-2",
		Size:        "256x256", // Smallest size to minimize cost
		EnableCache: true,
		CacheDir:    t.TempDir(),
	})
	
	opts := DefaultSearchOptions("ябълка")
	results, err := client.Search(context.Background(), opts)
	
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	
	result := results[0]
	
	// Check result fields
	if result.ID == "" {
		t.Error("Result ID is empty")
	}
	if result.URL == "" {
		t.Error("Result URL is empty")
	}
	if result.Width != 256 || result.Height != 256 {
		t.Errorf("Expected 256x256, got %dx%d", result.Width, result.Height)
	}
	if result.Source != "openai" {
		t.Errorf("Expected source 'openai', got '%s'", result.Source)
	}
	
	// Test caching - second request should use cache
	results2, err := client.Search(context.Background(), opts)
	if err != nil {
		t.Fatalf("Second search failed: %v", err)
	}
	
	if results2[0].URL != results[0].URL {
		t.Log("Note: URLs differ, cache might not be working as expected")
	}
}