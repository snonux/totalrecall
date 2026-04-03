package image

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

// Compile-time check that OpenAIClient implements the full ImageClient interface
// (ImageSearcher + AttributionProvider).
var _ ImageClient = (*OpenAIClient)(nil)

// imageHTTPClient is a shared HTTP client with a generous timeout for image
// downloads. http.DefaultClient has no timeout, which can block goroutines
// indefinitely on slow or unresponsive servers.
var imageHTTPClient = &http.Client{Timeout: 60 * time.Second}

// OpenAIClient implements ImageSearcher for OpenAI DALL-E image generation
type OpenAIClient struct {
	client     *openai.Client
	apiKey     string
	model      string // dall-e-2 or dall-e-3
	size       string // 256x256, 512x512, 1024x1024
	quality    string // standard or hd (dall-e-3 only)
	style      string // natural or vivid (dall-e-3 only)
	lastPrompt string // Store the last used prompt for attribution

	// PromptCallback is called when the prompt is generated, before the image is created
	PromptCallback func(prompt string)
}

// OpenAIConfig holds configuration for the OpenAI image provider
type OpenAIConfig struct {
	APIKey  string
	Model   string
	Size    string
	Quality string
	Style   string
}

// NewOpenAIClient creates a new OpenAI DALL-E client
func NewOpenAIClient(config *OpenAIConfig) *OpenAIClient {
	if config.APIKey == "" {
		// Return nil client that will fail on operations
		return &OpenAIClient{}
	}

	client := openai.NewClient(config.APIKey)

	// Set defaults
	if config.Model == "" {
		config.Model = "dall-e-3"
	}
	if config.Size == "" {
		config.Size = "1024x1024"
	}
	if config.Quality == "" {
		config.Quality = "standard"
	}
	if config.Style == "" {
		config.Style = "natural"
	}

	oc := &OpenAIClient{
		client:  client,
		apiKey:  config.APIKey,
		model:   config.Model,
		size:    config.Size,
		quality: config.Quality,
		style:   config.Style,
	}

	return oc
}

// Search generates an image for the Bulgarian word using DALL-E
func (c *OpenAIClient) Search(ctx context.Context, opts *SearchOptions) ([]SearchResult, error) {
	if c.client == nil {
		return nil, &SearchError{
			Provider: "openai",
			Code:     "NO_API_KEY",
			Message:  "OpenAI API key not configured",
		}
	}

	// Use the caller-provided translation. Translating internally would couple
	// the image package to the OpenAI chat API for a concern that belongs in
	// the translation package. Callers (processor, GUI) already resolve the
	// English translation before calling Search.
	translatedWord := opts.Translation
	if translatedWord == "" {
		// No translation provided — fall back to the original query word so
		// image generation still proceeds, albeit potentially with lower quality.
		translatedWord = opts.Query
	} else {
		fmt.Printf("Using provided translation: %s -> %s\n", opts.Query, translatedWord)
	}

	// Create prompt - use custom if provided, otherwise generate educational prompt
	var prompt string
	if opts.CustomPrompt != "" && strings.TrimSpace(opts.CustomPrompt) != "" {
		prompt = strings.TrimSpace(opts.CustomPrompt)
		// Ensure custom prompt doesn't exceed 1000 characters
		if len(prompt) > 1000 {
			prompt = prompt[:997] + "..."
			fmt.Printf("Custom prompt truncated to 1000 chars\n")
		}
		fmt.Printf("Using custom prompt: %s\n", prompt)
	} else {
		prompt = c.createEducationalPrompt(ctx, opts.Query, translatedWord)
		if prompt == "" {
			return nil, &SearchError{
				Provider: "openai",
				Code:     "PROMPT_GENERATION_FAILED",
				Message:  "Failed to generate image prompt - artistic styles could not be loaded",
			}
		}
	}

	// Store the prompt for attribution
	c.lastPrompt = prompt

	// Call the callback if set
	if c.PromptCallback != nil {
		c.PromptCallback(prompt)
	}

	// Log the prompt to stdout for debugging
	fmt.Printf("OpenAI Image Generation Prompt (%d chars): %s\n", len(prompt), prompt)
	fmt.Printf("OpenAI Image Generation: Using model '%s' with size '%s'\n", c.model, c.size)

	// Create the image generation request
	req := openai.ImageRequest{
		Prompt:         prompt,
		Model:          c.model,
		Size:           c.size,
		ResponseFormat: openai.CreateImageResponseFormatURL,
		N:              1,
	}

	// Add model-specific parameters
	if c.model == "dall-e-3" {
		req.Quality = c.quality
		req.Style = c.style
	}

	// Generate the image
	resp, err := c.client.CreateImage(ctx, req)
	if err != nil {
		return nil, &SearchError{
			Provider: "openai",
			Code:     "API_ERROR",
			Message:  fmt.Sprintf("Failed to generate image: %v", err),
		}
	}

	if len(resp.Data) == 0 {
		return nil, &SearchError{
			Provider: "openai",
			Code:     "NO_RESULTS",
			Message:  "No image generated",
		}
	}

	// Get the generated image URL
	imageURL := resp.Data[0].URL

	// Create result
	result := SearchResult{
		ID:           c.generateImageID(opts.Query),
		URL:          imageURL,
		ThumbnailURL: imageURL,
		Width:        c.getSizeWidth(),
		Height:       c.getSizeHeight(),
		Description:  fmt.Sprintf("Generated educational image for %s (%s)", opts.Query, translatedWord),
		Attribution:  "Generated by OpenAI DALL-E",
		Source:       "openai",
	}

	return []SearchResult{result}, nil
}

// Download downloads an image from the given URL
func (c *OpenAIClient) Download(ctx context.Context, url string) (io.ReadCloser, error) {
	// Download from URL
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := imageHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		if closeErr := resp.Body.Close(); closeErr != nil {
			return nil, fmt.Errorf("HTTP %d: %s (failed to close response body: %v)", resp.StatusCode, resp.Status, closeErr)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return resp.Body, nil
}

// GetAttribution returns the required attribution text
func (c *OpenAIClient) GetAttribution(result *SearchResult) string {
	var attribution strings.Builder
	attribution.WriteString("Image generated by OpenAI DALL-E\n\n")
	fmt.Fprintf(&attribution, "Model: %s\n", c.model)
	fmt.Fprintf(&attribution, "Size: %s\n", c.size)
	if c.model == "dall-e-3" {
		fmt.Fprintf(&attribution, "Quality: %s\n", c.quality)
		fmt.Fprintf(&attribution, "Style: %s\n", c.style)
	}
	fmt.Fprintf(&attribution, "\nPrompt used:\n%s\n", c.lastPrompt)
	fmt.Fprintf(&attribution, "\nGenerated at: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	return attribution.String()
}

// Name returns the name of the provider
func (c *OpenAIClient) Name() string {
	return "openai"
}

// GetLastPrompt returns the last prompt used for image generation
func (c *OpenAIClient) GetLastPrompt() string {
	return c.lastPrompt
}

// SetPromptCallback sets a callback function that will be called when the prompt is generated
func (c *OpenAIClient) SetPromptCallback(callback func(prompt string)) {
	c.PromptCallback = callback
}

// createEducationalPrompt generates a prompt optimized for language learning.
// Scene generation and style selection are handled here; the shared
// buildEducationalPrompt helper assembles the actual prompt text so that the
// same policy is used by both OpenAIClient and NanoBananaClient.
func (c *OpenAIClient) createEducationalPrompt(ctx context.Context, bulgarianWord, englishTranslation string) string {
	subject := promptSubject(englishTranslation, bulgarianWord)

	scene, err := c.generateSceneDescription(ctx, bulgarianWord, englishTranslation)
	if err != nil {
		fmt.Printf("  Failed to generate scene: %v, using basic prompt\n", err)
		scene = ""
	}
	if scene != "" {
		scene = sanitizeSceneDescription(scene)
		if !usableSceneDescription(scene) {
			fmt.Printf("  Scene response was too short or generic, using basic prompt\n")
			scene = ""
		}
	}

	// Select a random style from the shared pool. Fall back to a generic style
	// if the pool has been exhausted by tests or other callers.
	selectedStyle := chooseArtisticStyle()
	if selectedStyle == defaultArtisticStyle {
		fmt.Printf("  No artistic styles available, using generic prompt\n")
	}
	fmt.Printf("  Using image style: %s\n", selectedStyle)

	return buildEducationalPrompt(selectedStyle, scene, subject)
}

// generateSceneDescription generates a contextual scene description for the word
func (c *OpenAIClient) generateSceneDescription(ctx context.Context, bulgarianWord, englishTranslation string) (string, error) {
	// Use OpenAI to generate a scene description
	fmt.Printf("OpenAI Scene Generation: Creating scene for '%s' (%s)\n", bulgarianWord, englishTranslation)

	req := openai.ChatCompletionRequest{
		Model: openai.GPT4oMini,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are helping create educational flashcards for language learning. Generate a brief, vivid scene description that incorporates the given English word in a memorable, contextual way. The scene should be visually interesting and help with memory retention. Keep it to 1-2 sentences, focusing on visual elements that can be illustrated. The subject (the English word) should be the clear focal point of the image, prominent and centered.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: fmt.Sprintf("Create a scene description for the English word '%s' that would make a memorable flashcard image. Make sure '%s' is the main focus and most prominent element in the scene.", englishTranslation, englishTranslation),
			},
		},
		Temperature: 0.7, // Balanced temperature for creativity with consistency
		MaxTokens:   100,
	}

	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("scene generation failed: %w", err)
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("no scene description received")
	}

	scene := sanitizeSceneDescription(resp.Choices[0].Message.Content)
	if !usableSceneDescription(scene) {
		return "", fmt.Errorf("scene generation returned unusable content")
	}
	fmt.Printf("Generated scene: %s\n", scene)

	return scene, nil
}

// generateImageID generates a unique ID for the image
func (c *OpenAIClient) generateImageID(word string) string {
	// Create hash of the word for unique ID
	hash := md5.Sum([]byte(word))
	return hex.EncodeToString(hash[:])[:8]
}

// getSizeWidth returns the width based on the configured size
func (c *OpenAIClient) getSizeWidth() int {
	switch c.size {
	case "256x256":
		return 256
	case "512x512":
		return 512
	case "1024x1024":
		return 1024
	default:
		return 1024
	}
}

// getSizeHeight returns the height based on the configured size
func (c *OpenAIClient) getSizeHeight() int {
	// All DALL-E sizes are square
	return c.getSizeWidth()
}
