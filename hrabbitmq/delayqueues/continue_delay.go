package delayqueues

import (
	"context"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"github.com/wagslane/go-rabbitmq"
	"go.uber.org/zap"
)

// HandleDelayContinuation can be called in a handler on the "delay-ended" routing key to continue the delay on a
// message.
func (c *DelayQueueConfig) HandleDelayContinuation(ctx context.Context, logger *zap.Logger, d *rabbitmq.Delivery) (bool, error) {
	// now check if it has any remaining time still waiting
	var remainingDelay time.Duration
	if raw, ok := d.Headers[c.delayRemainingHeader]; ok {
		remainingDelay, _ = time.ParseDuration(raw.(string))
	}
	delete(d.Headers, c.delayRemainingHeader)

	// if this is a redelivered delay and it's older than the max delay then we treat it as though it has finished
	// the remaining delay and prevent putting undue load on rabbit.
	if d.Redelivered && !d.Timestamp.IsZero() && time.Since(d.Timestamp.UTC()) > time.Hour*2 {
		remainingDelay = 0
	}

	// shortcut any remaining delay if we've already been in the queue for long enough
	if deathTable, ok := d.Headers[DeathHeader].(amqp091.Table); ok {
		if deathTime, ok := deathTable["time"].(time.Time); ok {
			if remainingDelay > 0 && time.Since(deathTime.UTC()) > remainingDelay {
				remainingDelay = 0
			}
		}
	}

	// if it does, then delay it again
	if remainingDelay > 0 {
		return true, c.RepublishMessageWithDelay(ctx, logger, d, remainingDelay)
	}

	// otherwise return it for the next handler to process
	return false, nil
}
