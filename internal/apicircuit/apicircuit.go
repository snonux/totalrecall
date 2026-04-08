// Package apicircuit wraps outbound API calls with sony/gobreaker circuit breakers
// so repeated failures against OpenAI or Gemini do not pile up unbounded work.
//
// HTTP deadlines remain in internal/httpctx; breakers add a separate open/half-open
// gate when the remote service is clearly unhealthy.
package apicircuit

import (
	"context"
	"errors"
	"time"

	"github.com/sony/gobreaker"
)

const (
	// breakerInterval clears rolling failure counts in the closed state so stale
	// errors do not keep the breaker sensitive forever.
	breakerInterval = 2 * time.Minute
	// breakerOpenTimeout is how long the breaker stays open before trying half-open.
	breakerOpenTimeout = 45 * time.Second
	// breakerMaxHalfOpenRequests limits trial traffic while recovering.
	breakerMaxHalfOpenRequests = 3
	// breakerTripAfterConsecutiveFailures opens the circuit after this many
	// consecutive failed requests in the closed state.
	breakerTripAfterConsecutiveFailures uint32 = 5
)

// isSuccessful counts only real API outcomes: nil is success; context.Canceled is
// treated as success so user abort does not trip the breaker. Timeouts and
// remote errors still count as failures.
func isSuccessful(err error) bool {
	if err == nil {
		return true
	}
	return errors.Is(err, context.Canceled)
}

func readyToTrip(counts gobreaker.Counts) bool {
	return counts.ConsecutiveFailures >= breakerTripAfterConsecutiveFailures
}

func newBreaker(name string) *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:         name,
		MaxRequests:  breakerMaxHalfOpenRequests,
		Interval:     breakerInterval,
		Timeout:      breakerOpenTimeout,
		ReadyToTrip:  readyToTrip,
		IsSuccessful: isSuccessful,
	})
}

var (
	openAITTSBreaker        = newBreaker("openai-tts")
	geminiTTSBreaker        = newBreaker("gemini-tts")
	openAIImageBreaker      = newBreaker("openai-image")
	geminiNanoBananaBreaker = newBreaker("gemini-nanobanana")
)

func runValue[T any](cb *gobreaker.CircuitBreaker, fn func() (T, error)) (T, error) {
	var zero T
	v, err := cb.Execute(func() (interface{}, error) {
		return fn()
	})
	if err != nil {
		return zero, err
	}
	if v == nil {
		return zero, nil
	}
	return v.(T), nil
}

// OpenAITTS runs one OpenAI text-to-speech call through its circuit breaker.
func OpenAITTS[T any](fn func() (T, error)) (T, error) {
	return runValue(openAITTSBreaker, fn)
}

// GeminiTTS runs one Gemini TTS GenerateContent call through its circuit breaker.
func GeminiTTS[T any](fn func() (T, error)) (T, error) {
	return runValue(geminiTTSBreaker, fn)
}

// OpenAIImage runs one OpenAI image or chat call (DALL-E path) through its breaker.
func OpenAIImage[T any](fn func() (T, error)) (T, error) {
	return runValue(openAIImageBreaker, fn)
}

// GeminiNanoBanana runs one Gemini call from Nano Banana (scene text, image gen).
func GeminiNanoBanana[T any](fn func() (T, error)) (T, error) {
	return runValue(geminiNanoBananaBreaker, fn)
}
