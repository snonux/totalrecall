package testutil

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// MockHTTPClient mocks HTTP client for testing
type MockHTTPClient struct {
	Responses map[string]*MockResponse
	Errors    map[string]error
	Calls     []string
}

// MockResponse represents a mocked HTTP response
type MockResponse struct {
	StatusCode int
	Body       string
	Headers    map[string]string
}

// Get mocks an HTTP GET request
func (m *MockHTTPClient) Get(url string) (*MockResponse, error) {
	m.Calls = append(m.Calls, fmt.Sprintf("GET %s", url))

	if err, ok := m.Errors[url]; ok {
		return nil, err
	}

	if resp, ok := m.Responses[url]; ok {
		return resp, nil
	}

	return &MockResponse{
		StatusCode: 404,
		Body:       "Not Found",
	}, nil
}

// Post mocks an HTTP POST request
func (m *MockHTTPClient) Post(url string, body interface{}) (*MockResponse, error) {
	m.Calls = append(m.Calls, fmt.Sprintf("POST %s", url))

	if err, ok := m.Errors[url]; ok {
		return nil, err
	}

	if resp, ok := m.Responses[url]; ok {
		return resp, nil
	}

	return &MockResponse{
		StatusCode: 404,
		Body:       "Not Found",
	}, nil
}

// MockOpenAIClient mocks OpenAI API client
type MockOpenAIClient struct {
	TTSResponses   map[string][]byte
	ImageResponses map[string]string
	Errors         map[string]error
	Calls          []string
}

// CreateSpeech mocks OpenAI TTS API
func (m *MockOpenAIClient) CreateSpeech(ctx context.Context, text, voice, model string) (io.ReadCloser, error) {
	call := fmt.Sprintf("TTS: %s (voice=%s, model=%s)", text, voice, model)
	m.Calls = append(m.Calls, call)

	key := fmt.Sprintf("%s-%s-%s", text, voice, model)
	if err, ok := m.Errors[key]; ok {
		return nil, err
	}

	if data, ok := m.TTSResponses[key]; ok {
		return io.NopCloser(strings.NewReader(string(data))), nil
	}

	// Default response
	return io.NopCloser(strings.NewReader("mock audio data")), nil
}

// CreateImage mocks OpenAI DALL-E API
func (m *MockOpenAIClient) CreateImage(ctx context.Context, prompt string) (string, error) {
	call := fmt.Sprintf("Image: %s", prompt)
	m.Calls = append(m.Calls, call)

	if err, ok := m.Errors[prompt]; ok {
		return "", err
	}

	if url, ok := m.ImageResponses[prompt]; ok {
		return url, nil
	}

	// Default response
	return "https://example.com/mock-image.jpg", nil
}

// MockFileSystem mocks file system operations
type MockFileSystem struct {
	Files  map[string][]byte
	Errors map[string]error
	Calls  []string
}

// ReadFile mocks reading a file
func (m *MockFileSystem) ReadFile(path string) ([]byte, error) {
	m.Calls = append(m.Calls, fmt.Sprintf("READ %s", path))

	if err, ok := m.Errors[path]; ok {
		return nil, err
	}

	if data, ok := m.Files[path]; ok {
		return data, nil
	}

	return nil, fmt.Errorf("file not found: %s", path)
}

// WriteFile mocks writing a file
func (m *MockFileSystem) WriteFile(path string, data []byte) error {
	m.Calls = append(m.Calls, fmt.Sprintf("WRITE %s (%d bytes)", path, len(data)))

	if err, ok := m.Errors[path]; ok {
		return err
	}

	m.Files[path] = data
	return nil
}

// Exists mocks checking if a file exists
func (m *MockFileSystem) Exists(path string) bool {
	m.Calls = append(m.Calls, fmt.Sprintf("EXISTS %s", path))
	_, exists := m.Files[path]
	return exists
}

// MockTranslator mocks translation service
type MockTranslator struct {
	Translations map[string]string
	Errors       map[string]error
	Calls        []string
}

// Translate mocks translating text
func (m *MockTranslator) Translate(ctx context.Context, text, fromLang, toLang string) (string, error) {
	call := fmt.Sprintf("Translate: %s (%s->%s)", text, fromLang, toLang)
	m.Calls = append(m.Calls, call)

	if err, ok := m.Errors[text]; ok {
		return "", err
	}

	if translation, ok := m.Translations[text]; ok {
		return translation, nil
	}

	// Default mock translation
	return fmt.Sprintf("mock translation of %s", text), nil
}

// TestDataGenerator generates test data
type TestDataGenerator struct{}

// GenerateBulgarianWord generates a test Bulgarian word
func (g *TestDataGenerator) GenerateBulgarianWord() string {
	words := []string{"ябълка", "котка", "куче", "хляб", "вода", "книга", "стол", "прозорец"}
	return words[0] // Simple implementation, could be randomized
}

// GenerateAudioData generates mock audio data
func (g *TestDataGenerator) GenerateAudioData() []byte {
	// Simple mock MP3 header
	return []byte{0xFF, 0xFB, 0x90, 0x00, 0x00, 0x00, 0x00, 0x00}
}

// GenerateImageData generates mock image data
func (g *TestDataGenerator) GenerateImageData() []byte {
	// Simple mock JPEG header
	return []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46}
}
