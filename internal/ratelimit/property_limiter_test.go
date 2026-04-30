package ratelimit

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPropertyLimiterWaitSameProperty(t *testing.T) {
	t.Parallel()

	limiter := NewPropertyLimiter(60, 2)
	ctx := context.Background()

	require.NoError(t, limiter.Wait(ctx, "sc-domain:example.com"))
	require.NoError(t, limiter.Wait(ctx, "sc-domain:example.com"))
	require.Equal(t, 1, limiter.Size())
}

func TestPropertyLimiterIndependentBuckets(t *testing.T) {
	t.Parallel()

	limiter := NewPropertyLimiter(60, 1)
	require.NoError(t, limiter.Wait(context.Background(), "sc-domain:property-a.com"))

	// A's bucket is exhausted, so a short timeout should fail for A.
	ctxA, cancelA := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancelA()
	errA := limiter.Wait(ctxA, "sc-domain:property-a.com")
	requireContextError(t, errA)

	// B should still have a fresh bucket and succeed immediately.
	require.NoError(t, limiter.Wait(context.Background(), "sc-domain:property-b.com"))
	require.Equal(t, 2, limiter.Size())
}

func TestPropertyLimiterThrottlesWithShortTimeout(t *testing.T) {
	t.Parallel()

	limiter := NewPropertyLimiter(60, 1)
	property := "sc-domain:example.com"

	require.NoError(t, limiter.Wait(context.Background(), property))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := limiter.Wait(ctx, property)
	requireContextError(t, err)
}

func TestPropertyLimiterGuardrailsForInvalidInputs(t *testing.T) {
	t.Parallel()

	limiter := NewPropertyLimiter(0, 0)
	property := "sc-domain:guardrail.com"

	// Should be usable even with invalid constructor inputs.
	require.NoError(t, limiter.Wait(context.Background(), property))
	require.Equal(t, 1, limiter.Size())

	// With normalized burst/rate, immediate second wait should throttle under short timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := limiter.Wait(ctx, property)
	requireContextError(t, err)
}

func requireContextError(t *testing.T, err error) {
	t.Helper()

	require.Error(t, err)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return
	}

	msg := err.Error()
	require.Truef(
		t,
		strings.Contains(msg, "context deadline") || strings.Contains(msg, "canceled"),
		"expected context deadline/cancel error, got: %v",
		err,
	)
}
