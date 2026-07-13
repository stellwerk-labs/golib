package reliableoutbox

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"go.uber.org/zap"

	"github.com/stellwerk-labs/golib/hlogger"
	"github.com/stellwerk-labs/golib/hrabbitmq"
	"github.com/stellwerk-labs/golib/htelemetry"
)

// DefaultScheduledFlushPeriodFunc is the default schedule period for ScheduledFlushPendingMessages.
func DefaultScheduledFlushPeriodFunc() time.Duration {
	// these are constants right now, but can become options later.
	const base = time.Minute
	const jitter = 0.5
	return base * time.Duration(1+jitter*rand.Float64())
}

// ScheduledFlushPendingMessages repeatedly runs the flush routine until the context closes.
// It flushes all available messages and then waits for some period, DefaultScheduledFlushPeriodFunc defaults to 60-90s.
// This function is provider-agnostic and works with both Datadog and OpenTelemetry backends.
func ScheduledFlushPendingMessages[k PendingMessage](
	ctx context.Context, store Store[k], publisher hrabbitmq.Publisher,
	periodFunc func() time.Duration,
) {
	if periodFunc == nil {
		periodFunc = DefaultScheduledFlushPeriodFunc
	}
	const parallelism = 2
	// this is designed to run in a goroutine on its own, and panic would be bad so we catch it here.
	defer func() {
		if p := recover(); p != nil {
			// convert the goroutine local panic into a program wide error
			zap.L().Fatal("background pending event messages panicked", zap.Any("panic", p))
		}
	}()
	var more bool
	var err error
	for {
		// flush all the pages immediately until there's no more available messages, then wait a bit.
		if !more {
			select {
			case <-ctx.Done():
				return
			case <-time.After(periodFunc()):
			}
		}
		// run each send within its own trace so that we get good APM events and our existing alerts work as expected.
		// Use provider-agnostic htelemetry for span creation
		span := htelemetry.StartSpan("flush-next-page", htelemetry.ResourceName("flush-pending-messages"))
		subCtx := htelemetry.ContextWithSpan(ctx, span)
		// Use provider-agnostic logger scoping
		subLogger := hlogger.TraceScopedLoggerFromSpan(zap.L(), span)

		// flush the page and capture whether there's another page or not
		more, err = FlushNextPage[k](subCtx, subLogger, store, parallelism, publisher)
		if err != nil {
			more = false
			if !errors.Is(err, context.Canceled) {
				subLogger.Error("failure while emitting pending event messages", zap.Error(err))
				span.Finish(htelemetry.WithError(err))
				continue
			}
		}
		span.Finish()
	}
}
