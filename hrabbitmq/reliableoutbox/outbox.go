// Package reliableoutbox provides an outbox pattern for services that intend to emit messages reliably to rabbitmq
// without losing them. This assumes that the downstream systems are tolerant of at-least-once message delivery.
//
// The general model is to allow the caller to provide an implementation of the Store interface which can be used
// to persist messages, complete messages that have been confirmed as sent, and then read a page of messages back
// in order to support a scheduled and reliable flushing mechanism.
//
// The implementation of the Store is up to the caller, this could be a database transaction, a file system, or even
// just in-memory.
//
// The outbox takes a Publisher which should generally be a *rabbitmq.Publisher but can be something else in order to
// provide mocking and testability.
package reliableoutbox

import (
	"context"
	goerrors "errors"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/wagslane/go-rabbitmq"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/stellwerk-labs/golib/hlogger"
	"github.com/stellwerk-labs/golib/hrabbitmq"
	"github.com/stellwerk-labs/golib/htelemetry"
)

const operationPrefix = "reliable-outbox."

type contextKeyType string

var testCallbackContextKey = contextKeyType("test-callback")

// OptimisticPublish attempts to publish the set of messages to the publisher and clear them from the storage.
// If it fails to do so, errors will be logged and will need to wait until the FlushNextPage routine is called.
func OptimisticPublish[k PendingMessage](ctx context.Context, logger *zap.Logger, storage Store[k], publisher hrabbitmq.Publisher, messages []k) {
	for _, m := range messages {
		if ctx.Err() != nil {
			break
		}
		m := m
		_ = observedFlushMessage(ctx, logger, storage, publisher, m)
	}
	// see InstallTestCallbackOnContext.
	if v, ok := ctx.Value(testCallbackContextKey).(chan bool); ok {
		close(v)
	}
}

// PreparedOptimisticPublish is the output of PrepareOptimisticPublish
type PreparedOptimisticPublish func(ctx context.Context, publisher hrabbitmq.Publisher)

// PrepareOptimisticPublish builds and returns a closer function (PreparedOptimisticPublish) which should be called either
// immediately or later in the processing. The closer function may fail but the messages will remain pending in the
// outbox until FlushNextPage is called.
func PrepareOptimisticPublish[k PendingMessage](logger *zap.Logger, storage Store[k], messages []k) PreparedOptimisticPublish {
	return func(ctx context.Context, publisher hrabbitmq.Publisher) {
		OptimisticPublish(ctx, logger, storage, publisher, messages)
	}
}

// FlushNextPage will flush a page of messages from the store using the given number of goroutines for parallelism.
// If more messages are available, this method will return true and the caller can decide whether to run again
// immediately or to execute again later.
func FlushNextPage[k PendingMessage](ctx context.Context, logger *zap.Logger, storage Store[k], parallelism int, publisher hrabbitmq.Publisher) (bool, error) {
	if parallelism < 1 {
		return true, errors.New("parallelism must be > 0")
	}

	page, more, err := storage.LoadPage(ctx)
	if err != nil {
		return true, errors.Wrap(err, "failed to load a page of pending messages")
	}
	if len(page) == 0 {
		return more, nil
	}

	// convert page into a buffered channel
	messageChan := make(chan k, len(page))
	for _, message := range page {
		messageChan <- message
	}
	// close the channel so that once all messages are consumed, the goroutines will exit
	close(messageChan)

	wg := new(sync.WaitGroup)
	errChan := make(chan error, parallelism)

	for i := 0; i < parallelism; i++ {
		wg.Add(1)
		go consumerRoutine(ctx, logger, wg, storage, publisher, messageChan, errChan)
	}
	wg.Wait()

	close(errChan)
	if len(errChan) > 0 {
		errorCollection := make([]error, 0)
		for err := range errChan {
			errorCollection = append(errorCollection, err)
		}
		return false, goerrors.Join(errorCollection...)
	}

	return more, nil
}

// observedFlushMessage wraps the flush of an individual message with tracing.
// This function is provider-agnostic and works with both Datadog and OpenTelemetry backends.
func observedFlushMessage[k PendingMessage](ctx context.Context, logger *zap.Logger, storage Store[k], publisher hrabbitmq.Publisher, message k) (flushErr error) {
	span, subCtx := htelemetry.StartSpanFromContext(ctx, operationPrefix+"flush")
	defer func() {
		if flushErr != nil {
			span.Finish(htelemetry.WithError(flushErr))
		} else {
			span.Finish()
		}
	}()

	// Add trace context to logger using provider-agnostic method
	logger = hlogger.TraceScopedLoggerFromSpan(logger, span).With(zap.String("message-id", message.MessageId()))

	if len(message.MessageRoutingKeys()) == 1 {
		logger = logger.With(zap.String("routing-key", message.MessageRoutingKeys()[0]))
	} else {
		logger = logger.With(zap.Strings("routing-key", message.MessageRoutingKeys()))
	}

	if err := flushMessage(subCtx, logger, storage, publisher, message); err != nil {
		logger.Warn("failed to flush message after prepare", zap.Error(err))
		return err
	}
	return nil
}

// consumerRoutine is used inside the FlushNextPage api as a goroutine. It will flush all messages available on the msg channel
// and return an error over the err channel.
func consumerRoutine[k PendingMessage](ctx context.Context, logger *zap.Logger, wg *sync.WaitGroup, storage Store[k], publisher hrabbitmq.Publisher, msgChan chan k, errChan chan error) {
Loop:
	for {
		select {
		case m, ok := <-msgChan:
			if ok {
				if err := observedFlushMessage(ctx, logger, storage, publisher, m); err != nil {
					errChan <- err
					break Loop
				}
			} else {
				break Loop
			}
		case <-ctx.Done():
			break Loop
		}
	}
	wg.Done()
}

// FlushMessageRateLimiter is a rate limiter for the messages emitted by the background flush.
// Here we want a big burst for moments of combined
var FlushMessageRateLimiter = rate.NewLimiter(rate.Limit(10), 200)

// flushMessage sends a message to RabbitMQ and marks it as complete.
// This function is provider-agnostic and works with both Datadog and OpenTelemetry backends.
func flushMessage[k PendingMessage](ctx context.Context, logger *zap.Logger, storage Store[k], publisher hrabbitmq.Publisher, message k) error {
	{
		subCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if FlushMessageRateLimiter.Wait(subCtx) != nil {
			logger.Error("rate limit exceeded - if this message appears too frequently with the same message id over and over there may be a bug causing this to tight-loop")
			return fmt.Errorf("rate limit exceeded")
		}
	}

	options := []func(*rabbitmq.PublishOptions){
		rabbitmq.WithPublishOptionsMessageID(message.MessageId()),
		rabbitmq.WithPublishOptionsTimestamp(time.Now().UTC()),
		rabbitmq.WithPublishOptionsExchange(message.MessageExchange()),
		func(options *rabbitmq.PublishOptions) {
			// Use provider-agnostic span injection
			options.Headers = hrabbitmq.InjectSpanToTableWithProvider(ctx, logger, options.Headers)
		},
	}

	// If the message carries additional options, we can provide those
	if mok, ok := interface{}(message).(PendingMessageWithPublisherOptions); ok {
		options = append(options, mok.MessageOptions()...)
	}

	confirmations, err := publisher.PublishWithDeferredConfirmWithContext(
		ctx,
		message.MessagePayload(),
		message.MessageRoutingKeys(),
		options...,
	)
	if err != nil {
		err = errors.Wrap(err, "failed to send pending message")
		return err
	}
	for _, c := range confirmations {
		// annoyingly the amqp091 library makes it difficult to test confirmations
		ok := false
		err = nil
		done := c.Done()

		// detect if we're in a testing scenario and make a mock confirmation channel
		if done == nil {
			done2 := make(chan struct{}, 1)
			done2 <- struct{}{}
			done = done2
			ok = true
		}

		// wait for either the context to finish or the confirmation to come through
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "failed to wait for publish")
		case <-done:
			ok = ok || c.Acked()
			if !ok {
				return errors.New("failed to publish message: message was rejected")
			}
		}
	}
	if err := storage.Complete(ctx, message.MessageId()); err != nil {
		return errors.Wrap(err, "failed to complete pending message publish")
	}
	logger.Debug("successfully published pending message")
	return nil
}
