package delayqueues

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/wagslane/go-rabbitmq"
	"go.uber.org/zap"
)

// GracefulRetryError is an error type which can be used by the worker to gracefully retry a step without logging or
// tracing it as an error. This can wrap a specific internal error and
type GracefulRetryError struct {
	inner error
	delay time.Duration
}

func (e *GracefulRetryError) Error() string {
	msg := "graceful retry"
	if e.delay > 0 {
		msg += fmt.Sprintf(" after %v", e.delay)
	}
	if e.inner != nil {
		msg += fmt.Sprintf(": %v", e.inner)
	}
	return msg
}

func (e *GracefulRetryError) Unwrap() error {
	return e.inner
}

func (e *GracefulRetryError) GetGracefulRetryDelay() time.Duration {
	return e.delay
}

func NewGracefulRetryError(err error) *GracefulRetryError {
	return NewGracefulRetryErrorWithDelay(err, 0)
}

func NewGracefulRetryErrorWithDelay(err error, delay time.Duration) *GracefulRetryError {
	return &GracefulRetryError{inner: err, delay: delay}
}

// HandleGracefulRetryError is a handler function that can be called in a middleware to process errors returned during a
// consumer execution.
func (c *DelayQueueConfig) HandleGracefulRetryError(ctx context.Context, logger *zap.Logger, d *rabbitmq.Delivery, err error) (bool, error) {
	var egr *GracefulRetryError
	if errors.As(err, &egr) {
		if inner := errors.Unwrap(egr); inner != nil {
			logger = logger.With(zap.Error(inner))
		}

		// Set the retry attempt header to native int type
		if intValue, ok := d.Headers[c.gracefulRetryAttemptHeader].(int); ok {
			d.Headers[c.gracefulRetryAttemptHeader] = intValue + 1
		} else {
			if d.Headers == nil {
				d.Headers = map[string]interface{}{}
			}
			d.Headers[c.gracefulRetryAttemptHeader] = 1
		}
		if egr.delay > 0 {
			return true, c.RepublishMessageWithDelay(ctx, logger, d, egr.delay)
		}
		return true, c.RepublishMessageWithExponentialBackoff(ctx, logger, d)
	}
	return false, err
}
