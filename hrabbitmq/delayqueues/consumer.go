package delayqueues

import (
	"time"

	"github.com/pkg/errors"
	"github.com/wagslane/go-rabbitmq"
	"go.uber.org/zap"

	"github.com/stellwerk-labs/golib/hrabbitmq"
)

// setupConsumerInner is an inner function used to test SetupConsumer
func setupConsumerInner(
	cfg *DelayQueueConfig,
	d time.Duration, returnExchange string, returnRoutingKey string,
	logger *zap.Logger,
	extraOptions ...func(options *rabbitmq.ConsumerOptions),
) (string, []func(*rabbitmq.ConsumerOptions)) {
	rk, ok := cfg.GetDelayKeyMapping()[d]
	if !ok {
		panic("non-configured duration")
	}
	options := append([]func(*rabbitmq.ConsumerOptions){
		rabbitmq.WithConsumerOptionsExchangeName(cfg.delayExchange),
		rabbitmq.WithConsumerOptionsExchangeDeclare,
		rabbitmq.WithConsumerOptionsExchangeDurable,
		rabbitmq.WithConsumerOptionsExchangeKind("topic"),

		// dummy consumer options
		rabbitmq.WithConsumerOptionsLogger(&hrabbitmq.Logger{Logger: logger.Sugar()}),
		rabbitmq.WithConsumerOptionsConcurrency(1),
		rabbitmq.WithConsumerOptionsConsumerAutoAck(false),
		rabbitmq.WithConsumerOptionsQOSPrefetch(0),

		// queue definition
		// table required here because rabbitmq lib doesn't support all args
		rabbitmq.WithConsumerOptionsQueueArgs(rabbitmq.Table{
			// although each message should have its own ttl, we must have a default here
			"x-message-ttl": d.Milliseconds(),
			// push messages right back to the main exchange after timeout
			"x-dead-letter-exchange":    returnExchange,
			"x-dead-letter-routing-key": returnRoutingKey,
			// ensure we dead letter things correctly
			"x-dead-letter-strategy": "at-least-once",
			// ensure we reject publish if queue is full
			"x-overflow": "reject-publish",
		}),
		rabbitmq.WithConsumerOptionsQueueQuorum,
		rabbitmq.WithConsumerOptionsQueueDurable,

		// binding to the delay routing key
		rabbitmq.WithConsumerOptionsRoutingKey(rk),
	}, extraOptions...)
	return rk, options
}

// SetupConsumer binds a dummy consumer queue on the target rabbitmq connection. Note that this consumer doesn't
// actually have consuming routines, and intentionally relies on the AMQP dead-letter mechanism to timeout a message
// and bounce it back on the return queue for the continue-delay handler.
// The delay exchange and return exchange can _technically_ be the same exchange if needed.
func SetupConsumer(
	cfg *DelayQueueConfig,
	d time.Duration,
	delayQueue string,
	returnExchange string,
	returnRoutingKey string,
	conn *rabbitmq.Conn,
	logger *zap.Logger,
	extraOptions ...func(options *rabbitmq.ConsumerOptions),
) error {
	logger = logger.Named("delay-consumer-" + d.String())
	rk, options := setupConsumerInner(cfg, d, returnExchange, returnRoutingKey, logger, extraOptions...)
	logger.Info("binding dummy delay consumer", zap.Duration("delay", d), zap.String("routing-key", rk))
	consumer, err := rabbitmq.NewConsumer(conn, delayQueue, options...)
	if err != nil {
		return errors.Wrap(err, "failed to setup consumer")
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
