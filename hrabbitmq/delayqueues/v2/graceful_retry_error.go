package v2

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/wagslane/go-rabbitmq"
	"go.uber.org/zap"

	"github.com/stellwerk-labs/golib/hrabbitmq"
)

type gracefulRetryImpl struct {
	inner error
	delay time.Duration
}

func (e *gracefulRetryImpl) Error() string {
	msg := "graceful retry"
	if e.delay > 0 {
		msg += fmt.Sprintf(" after %v", e.delay)
	}
	if e.inner != nil {
		msg += fmt.Sprintf(": %v", e.inner)
	}
	return msg
}

func (e *gracefulRetryImpl) Unwrap() error {
	return e.inner
}

func (e *gracefulRetryImpl) GetGracefulRetryDelay() time.Duration {
	return e.delay
}

func NewGracefulRetryError(err error) GracefulRetryError {
	return NewGracefulRetryErrorWithDelay(err, 0)
}

func NewGracefulRetryErrorWithDelay(err error, delay time.Duration) GracefulRetryError {
	return &gracefulRetryImpl{inner: err, delay: delay}
}

type GracefulRetryError interface {
	error
	GetGracefulRetryDelay() time.Duration
}

func WrapRepublishGracefulRetriesWithDelay(pub hrabbitmq.Publisher, cache *expirable.LRU[string, int32], next HandlerFunc) HandlerFunc {
	return func(ctx context.Context, logger *zap.Logger, msg *rabbitmq.Delivery) error {
		err := next(ctx, logger, msg)
		if gre := (GracefulRetryError)(nil); errors.As(err, &gre) {
			if inner := errors.Unwrap(gre); inner != nil {
				logger = logger.With(zap.Error(inner))
			}

			// Set the retry attempt header to native int type
			msg.Headers = incrementRetryAttempts(msg.Headers)
			logger.Info("graceful retry error")

			if delay := gre.GetGracefulRetryDelay(); delay > 0 {
				err = republishWithFixedDelay(ctx, logger, pub, msg, delay)
			} else {
				err = republishMessageWithExponentialBackoff(ctx, logger, pub, msg)
			}
			if err == nil {
				rememberRetryAttempts(msg, cache)
			}
			return err
		} else if err == nil && cache != nil {
			// If succeeds indicate that duplicates of this message need to be skipped in any case
			cache.Add(msg.MessageId, -1)
		}
		return err
	}
}
