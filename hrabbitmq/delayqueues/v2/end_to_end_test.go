package v2

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wagslane/go-rabbitmq"
	"go.uber.org/zap"

	"github.com/stellwerk-labs/golib/hrabbitmq"
)

// TestEndToEnd requires a RABBITMQ_URL. It sends 100 messages and expects to receive them successfully on another
// handler. A certain percentage of the messages are randomly failed or panic-ed.
// $ docker run -it --rm --name rabbitmq -p 5672:5672 -p 15672:15672 rabbitmq:4.0-management
// $ RABBITMQ_URL=amqp://guest:guest@localhost:5672/
func TestEndToEnd(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	zap.ReplaceGlobals(logger)

	rabbitUrl, ok := os.LookupEnv("RABBITMQ_URL")
	if !ok {
		t.Skip("RABBITMQ_URL not set")
	}

	// ====================================================================================================
	// START OF STANDARD PATTERN

	// Prepare a new rabbit mq connection. This can be a managedamqp connection if necessary with reconnect if needed.
	conn, err := rabbitmq.NewConn(rabbitUrl, rabbitmq.WithConnectionOptionsLogger(hrabbitmq.NewLogger(zap.L())))
	require.NoError(t, err)
	defer conn.Close()

	// Now we need a publisher to use for both sending our messages and to send the delay and dead-letter retries.
	pub, err := rabbitmq.NewPublisher(conn, rabbitmq.WithPublisherOptionsLogger(hrabbitmq.NewLogger(zap.L())))
	require.NoError(t, err)
	defer pub.Close()
	// Enable async confirmation
	pub.NotifyPublish(func(p rabbitmq.Confirmation) {
	})

	// Cache for deduplication
	cache := expirable.NewLRU[string, int32](10000, nil, 60*time.Second)

	// Setup the delay queues which expire the messages after the N seconds delays and then sends the message back to the common exchange
	require.NoError(t, SetupStandardDelayConsumers(conn, zap.L().With(zap.String("consumer", "delaying"))))

	// Setup the dead letter queue and consumer which pushes the messages onto the delay queues and exponential backoff.
	dlc, err := SetupStandardDeadLetterConsumer(conn, zap.L().With(zap.String("consumer", "dead-letters")), pub, cache)
	require.NoError(t, err)
	defer dlc.CloseWithContext(context.Background())

	// END OF STANDARD PATTERN
	// ====================================================================================================

	// launch the standard consumers - usually you do this in a better error-catchy kind of way
	go func() {
		if err := dlc.Run(); err != nil {
			zap.L().Error("consumer run failed", zap.Error(err))
		}
	}()

	consumerCounter := new(atomic.Uint32)
	consumerCounter.Add(rand.Uint32() / 2)
	setupConsumer := func(h HandlerFunc) (rk string, c *hrabbitmq.ConsumerWithHandlerWaiter, err error) {
		rk = fmt.Sprintf("test-%d", consumerCounter.Add(1))
		handler := WrapRepublishGracefulRetriesWithDelay(pub, cache, h)
		handler = WrapWithTimeout(time.Minute, handler)
		handler = WrapLoggerAndTracer(rk, handler)
		consumer, err := hrabbitmq.NewConsumerWithHandlerWaiter(
			conn,
			func(d rabbitmq.Delivery) (action rabbitmq.Action) {
				if err := handler(context.Background(), zap.L().With(zap.String("consumer", rk)), &d); err != nil {
					return rabbitmq.NackDiscard
				}
				return rabbitmq.Ack
			},
			rk,
			rabbitmq.WithConsumerOptionsExchangeName(DefaultExchange),
			rabbitmq.WithConsumerOptionsExchangeDeclare,
			rabbitmq.WithConsumerOptionsExchangeDurable,
			rabbitmq.WithConsumerOptionsExchangeKind("topic"),

			// we explicitly don't want to ack messages by default! only after successful processing
			rabbitmq.WithConsumerOptionsConsumerAutoAck(false),
			rabbitmq.WithConsumerOptionsConcurrency(5),

			// table required here because rabbitmq lib doesn't support all args
			rabbitmq.WithConsumerOptionsQueueArgs(rabbitmq.Table{
				// set the message ttl, if takes longer than this on a consumer, mark it failed
				"x-message-ttl": (time.Minute * 10).Milliseconds(),
				// if the message "fails" (nack, reject, or TTL timeout), send it here
				"x-dead-letter-exchange":    DefaultExchange,
				"x-dead-letter-routing-key": DeadLetterRoutingKey,
				// ensure we dead letter things correctly
				"x-dead-letter-strategy": "at-least-once",
				// ensure we reject publish if queue is full
				"x-overflow": "reject-publish",
			}),
			rabbitmq.WithConsumerOptionsQueueQuorum,
			rabbitmq.WithConsumerOptionsQueueDurable,
			rabbitmq.WithConsumerOptionsRoutingKey(rk),
		)
		return rk, consumer, err
	}

	publishCounter := new(atomic.Uint32)
	publishOnces := func(ctx context.Context, rk string, data []byte) error {
		confirms, err := pub.PublishWithDeferredConfirmWithContext(
			ctx, data, []string{rk},
			rabbitmq.WithPublishOptionsMessageID(rk+"_"+fmt.Sprint(publishCounter.Add(1))),
			rabbitmq.WithPublishOptionsTimestamp(time.Now()),
			rabbitmq.WithPublishOptionsExchange(DefaultExchange),
			rabbitmq.WithPublishOptionsContentType("application/json"),
		)
		if err != nil {
			return err
		}
		for _, confirm := range confirms {
			if !confirm.Wait() {
				return fmt.Errorf("confirm delivery failed")
			} else if !confirm.Acked() {
				return fmt.Errorf("ack delivery failed")
			}
		}
		return nil
	}

	t.Run("single message send and consume", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		signal := make(chan bool)
		rk, c, err := setupConsumer(func(ctx context.Context, logger *zap.Logger, msg *rabbitmq.Delivery) error {
			signal <- true
			return nil
		})
		require.NoError(t, err)
		defer c.CloseWithContext(ctx)
		go func() {
			require.NoError(t, c.Run())
		}()
		time.Sleep(time.Second)
		require.NoError(t, publishOnces(ctx, rk, []byte("{}")))

		select {
		case <-signal:
		case <-ctx.Done():
			require.Fail(t, "timeout")
		}
	})

	t.Run("single message send and succeed after a few attempts", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		signal := make(chan bool)
		counter := new(atomic.Int32)
		rk, c, err := setupConsumer(func(ctx context.Context, logger *zap.Logger, msg *rabbitmq.Delivery) error {
			if counter.Add(1) < 3 {
				return fmt.Errorf("unexpected error")
			}
			assert.Equal(t, 2, int(msg.Headers[RetryAttemptTrackerHeader].(int32)))
			signal <- true
			return nil
		})
		require.NoError(t, err)
		defer c.CloseWithContext(ctx)
		go func() {
			require.NoError(t, c.Run())
		}()
		time.Sleep(time.Second)
		require.NoError(t, publishOnces(ctx, rk, []byte("{}")))

		select {
		case <-signal:
		case <-ctx.Done():
			require.Fail(t, "timeout")
		}
	})

	t.Run("single message send and succeed after graceful retry", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		signal := make(chan bool)
		counter := new(atomic.Int32)
		rk, c, err := setupConsumer(func(ctx context.Context, logger *zap.Logger, msg *rabbitmq.Delivery) error {
			if counter.Add(1) < 3 {
				return NewGracefulRetryErrorWithDelay(fmt.Errorf("fizzy"), time.Second)
			}
			assert.Equal(t, 2, int(msg.Headers[RetryAttemptTrackerHeader].(int32)))
			signal <- true
			return nil
		})
		require.NoError(t, err)
		defer c.CloseWithContext(ctx)
		go func() {
			require.NoError(t, c.Run())
		}()
		time.Sleep(time.Second)
		require.NoError(t, publishOnces(ctx, rk, []byte("{}")))

		select {
		case <-signal:
		case <-ctx.Done():
			require.Fail(t, "timeout")
		}
	})

	t.Run("deduplication in the dead letter queue", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		signal := make(chan bool)
		counter := new(atomic.Int32)
		rk, c, err := setupConsumer(func(ctx context.Context, logger *zap.Logger, msg *rabbitmq.Delivery) error {
			c := counter.Add(1)
			logger.Sugar().Debugw("Consumer started", "counter", c)
			if c == 1 {
				// Send message to the dead letter queue from where it's published with attempts=1
				return fmt.Errorf("unexpected error")
			}
			if c == 2 {
				// Cache contains 1. Send message to the dead letter queue, where it's published with attempts=2
				cached, _ := cache.Get(msg.MessageId)
				assert.Equal(t, 1, int(cached))
				return fmt.Errorf("unexpected error")
			}
			if c == 3 {
				require.Equal(t, 2, int(msg.Headers[RetryAttemptTrackerHeader].(int32)))

				// Cache contains 2. Send message to the dead letter queue with attempts=1
				cached, _ := cache.Get(msg.MessageId)
				assert.Equal(t, 2, int(cached))
				originalRoutingKey := msg.RoutingKey
				msg.RoutingKey = DeadLetterRoutingKey
				msg.Headers["x-death"] = []interface{}{
					amqp091.Table{
						"count":        1,
						"exchange":     msg.Exchange,
						"queue":        originalRoutingKey,
						"reason":       "rejected",
						"routing-keys": []interface{}{originalRoutingKey},
						"time":         time.Now().Format(time.RFC3339),
					},
				}
				msg.Headers[RetryAttemptTrackerHeader] = int32(1)
				require.NoError(t, republishMessage(ctx, logger, pub, msg))
				// Reset
				delete(msg.Headers, "x-death")
				msg.RoutingKey = originalRoutingKey
				msg.Headers[RetryAttemptTrackerHeader] = int32(2)

				time.Sleep(2 * time.Second)

				// Send message to the dead letter queue, where it's published with attempts=3
				return fmt.Errorf("unexpected error")
			}
			// c == 4: if the duplicate was consumed, the number of attempts in the header is 2. Check that it's 3.
			assert.Equal(t, 3, int(msg.Headers[RetryAttemptTrackerHeader].(int32)))
			signal <- true
			return nil
		})
		require.NoError(t, err)
		defer c.CloseWithContext(ctx)
		go func() {
			require.NoError(t, c.Run())
		}()
		time.Sleep(time.Second)
		require.NoError(t, publishOnces(ctx, rk, []byte("{}")))

		select {
		case <-signal:
		case <-ctx.Done():
			require.Fail(t, "timeout")
		}
	})

	t.Run("many messages send and consume", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		var wg sync.WaitGroup
		rk, c, err := setupConsumer(func(ctx context.Context, logger *zap.Logger, msg *rabbitmq.Delivery) error {
			wg.Done()
			return nil
		})
		require.NoError(t, err)
		defer c.CloseWithContext(ctx)
		go func() {
			require.NoError(t, c.Run())
		}()
		time.Sleep(time.Second)
		for i := 0; i < 100; i++ {
			wg.Add(1)
			require.NoError(t, publishOnces(ctx, rk, []byte("{}")))
		}
		signal := make(chan bool)
		go func() {
			wg.Wait()
			close(signal)
		}()
		select {
		case <-signal:
		case <-ctx.Done():
			require.Fail(t, "timeout")
		}
	})

	t.Run("many fallible messages send and consume", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		var wg sync.WaitGroup
		rk, c, err := setupConsumer(func(ctx context.Context, logger *zap.Logger, msg *rabbitmq.Delivery) error {
			if time.Since(msg.Timestamp) < time.Second*5 {
				if rand.Float32() > 0.5 {
					panic("oh no!")
				} else {
					return NewGracefulRetryError(fmt.Errorf("fizzy"))
				}
			}
			wg.Done()
			return nil
		})
		require.NoError(t, err)
		defer c.CloseWithContext(ctx)
		go func() {
			require.NoError(t, c.Run())
		}()
		time.Sleep(time.Second)
		for i := 0; i < 100; i++ {
			wg.Add(1)
			require.NoError(t, publishOnces(ctx, rk, []byte("{}")))
		}
		signal := make(chan bool)
		go func() {
			wg.Wait()
			close(signal)
		}()
		select {
		case <-signal:
		case <-ctx.Done():
			require.Fail(t, "timeout")
		}
	})
}
