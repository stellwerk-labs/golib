package v2

import (
	"fmt"
	"time"

	"github.com/wagslane/go-rabbitmq"
	"go.uber.org/zap"

	"github.com/stellwerk-labs/golib/hrabbitmq"
)

func setupDelayConsumer(d time.Duration, conn *rabbitmq.Conn, logger *zap.Logger) error {
	consumer, err := rabbitmq.NewConsumer(
		conn, fmt.Sprintf(DelayRoutingKeyTemplate, d),
		// dummy consumer options
		rabbitmq.WithConsumerOptionsLogger(hrabbitmq.NewLogger(logger)),
		rabbitmq.WithConsumerOptionsConcurrency(1),
		rabbitmq.WithConsumerOptionsConsumerAutoAck(false),
		rabbitmq.WithConsumerOptionsQOSPrefetch(0),

		// queue definition
		// table required here because rabbitmq lib doesn't support all args
		rabbitmq.WithConsumerOptionsQueueArgs(rabbitmq.Table{
			// although each message should have its own ttl, we must have a default here
			"x-message-ttl": d.Milliseconds(),
			// push messages right back to the main exchange after timeout
			"x-dead-letter-exchange":    DefaultExchange,
			"x-dead-letter-routing-key": DelayEndedRoutingKey,
			// ensure we dead letter things correctly
			"x-dead-letter-strategy": "at-least-once",
			// ensure we reject publish if queue is full
			"x-overflow": "reject-publish",
		}),
		rabbitmq.WithConsumerOptionsQueueQuorum,
		rabbitmq.WithConsumerOptionsQueueDurable,

		// binding to the delay routing key
		rabbitmq.WithConsumerOptionsExchangeName(DefaultExchange),
		rabbitmq.WithConsumerOptionsRoutingKey(fmt.Sprintf(DelayRoutingKeyTemplate, d)),
	)
	if err != nil {
		return fmt.Errorf("failed to setup delay consumer: %v", err)
	}
	go func() {
		if p := recover(); p != nil {
			logger.Error("fatal error in delay consumer", zap.Error(err))
		}
		if err := consumer.Run(func(d rabbitmq.Delivery) (action rabbitmq.Action) {
			go consumer.Close()
			return rabbitmq.NackRequeue
		}); err != nil {
			logger.Error("failed to spawn delay consumer", zap.Error(err))
		}
	}()
	return nil
}

func SetupStandardDelayConsumers(conn *rabbitmq.Conn, logger *zap.Logger) error {
	for _, d := range delayDurations {
		if err := setupDelayConsumer(d, conn, logger); err != nil {
			return fmt.Errorf("failed to setup delay consumer: %w", err)
		}
	}
	return nil
}
