package httpctx

import (
	"context"
	"testing"
	"time"
)

func TestWithTimeoutUnlessSet_AlreadyHasDeadline(t *testing.T) {
	t.Parallel()

	parent, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()

	ctx, childCancel := WithTimeoutUnlessSet(parent, time.Nanosecond)
	defer childCancel()

	if ctx != parent {
		t.Fatal("expected same context when parent already has deadline")
	}

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline on context")
	}
	if time.Until(deadline) < 30*time.Minute {
		t.Fatalf("expected parent ~1h deadline preserved, got %v", time.Until(deadline))
	}
}

func TestWithTimeoutUnlessSet_NoDeadline(t *testing.T) {
	t.Parallel()

	ctx, cancel := WithTimeoutUnlessSet(context.Background(), 50*time.Millisecond)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline")
	}
	if time.Until(deadline) > time.Second {
		t.Fatalf("deadline too far: %v", deadline)
	}
}

func TestWithTimeoutUnlessSet_NilUsesBackground(t *testing.T) {
	t.Parallel()

	ctx, cancel := WithTimeoutUnlessSet(nil, 50*time.Millisecond)
	defer cancel()

	if err := ctx.Err(); err != nil {
		t.Fatalf("context should not be done: %v", err)
	}
}
