package apicircuit

import (
	"context"
	"errors"
	"testing"
)

func TestOpenAITTS_Success(t *testing.T) {
	t.Parallel()
	v, err := OpenAITTS(func() (string, error) {
		return "ok", nil
	})
	if err != nil || v != "ok" {
		t.Fatalf("OpenAITTS() = %q, %v; want ok, nil", v, err)
	}
}

func TestIsSuccessful_ContextCanceled(t *testing.T) {
	t.Parallel()
	if !isSuccessful(context.Canceled) {
		t.Fatal("context.Canceled should not count as breaker failure")
	}
	if isSuccessful(errors.New("api error")) {
		t.Fatal("arbitrary errors must count as failure")
	}
}
