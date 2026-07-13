package v2

import (
	"context"
	"fmt"
	"maps"
	"math"
	"math/rand/v2"
	"time"

	"github.com/pkg/errors"
	"github.com/rabbitmq/amqp091-go"
	"github.com/wagslane/go-rabbitmq"
	"go.uber.org/zap"

	"github.com/stellwerk-labs/golib/hrabbitmq"
)

// CalculateNextDelayRoutingKey returns the routing key to use for delaying a message by at most "remaining" time.
// The only caveat is that the delay will always be at least the smallest bucket.
func calculateNextDelayRoutingKey(buckets []time.Duration, remaining time.Duration) (string, time.Duration) {
	if len(buckets) == 0 {
		panic("at least one time bucket must be provided")
	}
	match := buckets[0]
	for _, duration := range buckets {
		if remaining >= duration {
			match = duration
		}
	}
	newRemaining := remaining - match
	if newRemaining < 0 {
		newRemaining = 0
	}
	return fmt.Sprintf(DelayRoutingKeyTemplate, match), newRemaining
}

// GenerateDelayHeadersAndKey will generate the delay queue headers and new routing key for an existing message.
// If headers is nil, a new table will be generated and returned.
func generateDelayHeadersAndKey(routingKey string, period time.Duration, headers rabbitmq.Table) (string, rabbitmq.Table) {
	if headers == nil {
		headers = make(rabbitmq.Table)
	}
	key, nextRemaining := calculateNextDelayRoutingKey(delayDurations, period)
	if nextRemaining > 0 {
		headers[DelayEndedRemainingHeader] = nextRemaining.String()
	}
	headers[DelayEndedNextRoutingKeyHeader] = routingKey
	return key, headers
}

func calculateNextDelay(lastDelay, min, max time.Duration, jitter float64) time.Duration {
	var currentDelay = min
	if lastDelay >= min {
		currentDelay = time.Duration(float64(lastDelay) * 2.0)
	}
	currentDelay = time.Duration(math.Ceil(float64(currentDelay) * (1 - jitter*rand.Float64())))
	if currentDelay > max {
		currentDelay = max
	} else if currentDelay < min {
		currentDelay = min
	}
	return currentDelay
}

// RepublishMessageWithExponentialBackoff republishes the message with exponential backoff: if this is the first time
// then it backs of by the lowest interval, if it's a subsequent time, then it performs an exponential delay up to
// a maximum.
func republishMessageWithExponentialBackoff(
	ctx context.Context,
	logger *zap.Logger,
	pub hrabbitmq.Publisher,
	d *rabbitmq.Delivery,
) error {
	var lastDelay time.Duration
	if raw, ok := d.Headers[LastRetryDelayHeader]; ok {
		lastDelay, _ = time.ParseDuration(raw.(string))
	}
	delay := calculateNextDelay(lastDelay, delayDurations[0], time.Hour, 0.1)
	if d.Headers == nil {
		d.Headers = make(amqp091.Table, 1)
	}
	d.Headers[LastRetryDelayHeader] = delay.String()
	return republishWithFixedDelay(ctx, logger, pub, d, delay)
}

// RepublishMessageWithDelay can be called by any handler to republish the given message with a fixed delay. The
// message will be put on the appropriate delay queue with headers that indicate how much delay remains.
func republishWithFixedDelay(
	ctx context.Context,
	logger *zap.Logger,
	pub hrabbitmq.Publisher,
	d *rabbitmq.Delivery,
	period time.Duration,
) error {
	headers := maps.Clone(rabbitmq.Table(d.Headers))
	d.RoutingKey, headers = generateDelayHeadersAndKey(d.RoutingKey, period, headers)
	d.Headers = amqp091.Table(headers)
	return republishMessage(ctx, logger, pub, d)
}

func republishMessage(
	ctx context.Context,
	logger *zap.Logger,
	pub hrabbitmq.Publisher,
	d *rabbitmq.Delivery,
) error {
	headers := hrabbitmq.InjectSpanToTable(ctx, logger, maps.Clone(rabbitmq.Table(d.Headers)))
	// We've seen issues where rabbit may reject published messages due to networking or load. We will generally retry
	// these using the dead letter queue, but it's also reasonable for us to retry to the network request until we
	// get the publisher confirmations returned.
	var confirmations rabbitmq.PublisherConfirmation
	var err error
RetryLoop:

	for i := 0; true; i++ {
		if err != nil {
			delay := time.Duration(-1)
			if i <= 5 {
				delay = time.Duration(float64(time.Millisecond*200) * math.Pow(float64(2), float64(i-1)))
			}
			if delay <= 0 {
				break RetryLoop
			}
			logger.Warn("retrying message publish due to error", zap.Error(err))
			t := time.NewTimer(delay)
			select {
			case <-t.C:
			case <-ctx.Done():
				t.Stop()
				break RetryLoop
			}
		}
		confirmations, err = pub.PublishWithDeferredConfirmWithContext(
			ctx,
			d.Body,
			[]string{d.RoutingKey},
			// copy attributes from the message
			rabbitmq.WithPublishOptionsContentType(d.ContentType),
			rabbitmq.WithPublishOptionsContentEncoding(d.ContentEncoding),
			rabbitmq.WithPublishOptionsAppID(d.AppId),
			rabbitmq.WithPublishOptionsMessageID(d.MessageId),
			rabbitmq.WithPublishOptionsTimestamp(d.Timestamp),
			rabbitmq.WithPublishOptionsCorrelationID(d.CorrelationId),
			rabbitmq.WithPublishOptionsPriority(d.Priority),
			rabbitmq.WithPublishOptionsReplyTo(d.ReplyTo),
			rabbitmq.WithPublishOptionsUserID(d.UserId),
			rabbitmq.WithPublishOptionsHeaders(headers),
			rabbitmq.WithPublishOptionsExchange(d.Exchange),
		)
		if err == nil {
			break RetryLoop
		}
	}
	if err != nil {
		return errors.Wrap(err, "failed to publish message")
	}
	for _, c := range confirmations {
		if ok, err := c.WaitContext(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for publish confirmation")
		} else if !ok {
			return errors.New("publish failed with unknown error")
		}
	}
	return nil
}
