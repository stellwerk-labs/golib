package v2

import (
	"context"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/rabbitmq/amqp091-go"
	"github.com/wagslane/go-rabbitmq"
	"go.uber.org/zap"

	"github.com/stellwerk-labs/golib/hrabbitmq"
)

func SetupStandardDeadLetterConsumer(conn *rabbitmq.Conn, logger *zap.Logger, pub hrabbitmq.Publisher, cache *expirable.LRU[string, int32]) (*hrabbitmq.ConsumerWithHandlerWaiter, error) {
	handler := func(ctx context.Context, logger *zap.Logger, msg *rabbitmq.Delivery) error {
		if isDuplicate(msg, cache) {
			logger.Debug("message is a duplicate - dropping message")
			return nil
		}

		if msg.RoutingKey == DelayEndedRoutingKey {
			if v, ok := msg.Headers[DelayEndedNextRoutingKeyHeader]; ok {
				delete(msg.Headers, DelayEndedNextRoutingKeyHeader)
				if key, ok := v.(string); ok {
					msg.RoutingKey = key
				}
			}
			if msg.RoutingKey == DelayEndedRoutingKey {
				logger.Error("found delay_ended cycle - dropping message")
				return nil
			}

			// now check if it has any remaining time still waiting
			var remainingDelay time.Duration
			if raw, ok := msg.Headers[DelayEndedRemainingHeader]; ok {
				delete(msg.Headers, DelayEndedRemainingHeader)
				remainingDelay, _ = time.ParseDuration(raw.(string))
			}

			// if this is a redelivered delay, and it's older than the max delay then we treat it as though it has finished
			// the remaining delay and prevent putting undue load on rabbit.
			if msg.Redelivered && !msg.Timestamp.IsZero() && time.Since(msg.Timestamp.UTC()) > time.Hour*2 {
				remainingDelay = 0
			}

			// shortcut any remaining delay if we've already been in the queue for long enough
			if deathTable, ok := msg.Headers["x-death"].(amqp091.Table); ok {
				if deathTime, ok := deathTable["time"].(time.Time); ok {
					if remainingDelay > 0 && time.Since(deathTime.UTC()) > remainingDelay {
						remainingDelay = 0
					}
				}
			}

			// drop the deaths table for delay-ended since we don't care about tracking this further
			delete(msg.Headers, "x-death")

			// if it does, then delay it again
			if remainingDelay > 0 {
				logger.Debug("republishing message with remaining delay")
				return republishWithFixedDelay(ctx, logger, pub, msg, remainingDelay)
			}
			logger.Info("republishing delay-ended message to original routing key")
			return republishMessage(ctx, logger, pub, msg)
		}

		if deaths, ok := msg.Headers["x-death"].([]interface{}); ok && len(deaths) > 0 {
			dt, _ := deaths[len(deaths)-1].(amqp091.Table)
			rk, _ := dt["routing-keys"].([]interface{})
			msg.RoutingKey, _ = rk[0].(string)
		}
		// drop the deaths table for delay-ended since we don't care about tracking this further
		delete(msg.Headers, "x-death")

		if msg.RoutingKey == DeadLetterRoutingKey {
			logger.Error("found dead-letter cycle - dropping message")
			return nil
		}

		// increment the retry attempts
		msg.Headers = incrementRetryAttempts(msg.Headers)

		logger.Info("republishing dead-lettered message to exponential backoff", zap.String("next-routing-key", msg.RoutingKey))
		err := republishMessageWithExponentialBackoff(ctx, logger, pub, msg)
		if err == nil {
			rememberRetryAttempts(msg, cache)
		}
		return err
	}
	handler = WrapWithTimeout(time.Minute, handler)
	handler = WrapLoggerAndTracer("dead-letter", handler)
	return hrabbitmq.NewConsumerWithHandlerWaiter(
		conn,
		func(d rabbitmq.Delivery) (action rabbitmq.Action) {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("panic", zap.Any("panic", r))
					action = rabbitmq.NackDiscard
					return
				}
			}()
			if err := handler(context.Background(), logger, &d); err != nil {
				time.Sleep(time.Second)
				return rabbitmq.NackRequeue
			}
			return rabbitmq.Ack
		},
		DeadLetterQueueName,
		// consumer flags
		rabbitmq.WithConsumerOptionsLogger(hrabbitmq.NewLogger(logger)),
		rabbitmq.WithConsumerOptionsConsumerAutoAck(false),
		rabbitmq.WithConsumerOptionsConcurrency(5),
		rabbitmq.WithConsumerOptionsQueueQuorum,
		rabbitmq.WithConsumerOptionsQueueDurable,
		rabbitmq.WithConsumerOptionsExchangeName(DefaultExchange),
		rabbitmq.WithConsumerOptionsRoutingKey(DeadLetterRoutingKey),
		rabbitmq.WithConsumerOptionsRoutingKey(DelayEndedRoutingKey),
	)
}
