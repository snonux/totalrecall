package apicircuit

import (
	"context"
	"errors"
	"testing"

	"github.com/sony/gobreaker"
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

// TestBreakerOpensAndShortCircuits verifies the core resilience guarantee: after
// breakerTripAfterConsecutiveFailures failed calls the breaker opens, and further
// calls are short-circuited with gobreaker.ErrOpenState without invoking fn. This
// is what prevents a degraded API from causing cascading load.
func TestBreakerOpensAndShortCircuits(t *testing.T) {
	t.Parallel()

	// Use a dedicated breaker (not the package singletons) so this test stays
	// isolated from the others while still exercising the shared trip policy.
	cb := newBreaker("test-trip")
	apiErr := errors.New("upstream failure")

	// Drive enough consecutive failures to trip the breaker open.
	for i := uint32(0); i < breakerTripAfterConsecutiveFailures; i++ {
		_, err := runValue(cb, func() (string, error) {
			return "", apiErr
		})
		if !errors.Is(err, apiErr) {
			t.Fatalf("call %d: got %v, want upstream failure", i, err)
		}
	}

	// The breaker must now be open and short-circuit without calling fn.
	called := false
	_, err := runValue(cb, func() (string, error) {
		called = true
		return "ok", nil
	})
	if !errors.Is(err, gobreaker.ErrOpenState) {
		t.Fatalf("expected ErrOpenState once breaker is open, got %v", err)
	}
	if called {
		t.Fatal("open breaker must short-circuit and not invoke fn")
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
