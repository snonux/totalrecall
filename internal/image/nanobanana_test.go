package image

import "testing"

func TestNewNanoBananaConfig(t *testing.T) {
	t.Parallel()

	cfg := NewNanoBananaConfig("test-key")
	if cfg.APIKey != "test-key" {
		t.Fatalf("expected API key to be preserved, got %q", cfg.APIKey)
	}
	if cfg.Model != DefaultNanoBananaModel {
		t.Fatalf("expected default model %q, got %q", DefaultNanoBananaModel, cfg.Model)
	}
	if cfg.TextModel != DefaultNanoBananaTextModel {
		t.Fatalf("expected default text model %q, got %q", DefaultNanoBananaTextModel, cfg.TextModel)
	}

	clientCfg := cfg.ClientConfig()
	if clientCfg == nil {
		t.Fatal("expected client config")
	}
	if clientCfg.APIKey != "test-key" {
		t.Fatalf("expected client config API key to be preserved, got %q", clientCfg.APIKey)
	}
}
