package reliableoutbox

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wagslane/go-rabbitmq"

	"github.com/stellwerk-labs/golib/hrabbitmq"
)

func TestDefaultScheduledFlushPeriodFunc(t *testing.T) {
	for i := 0; i < 100; i++ {
		p := DefaultScheduledFlushPeriodFunc()
		if !assert.GreaterOrEqual(t, p, time.Minute) || !assert.LessOrEqual(t, p, time.Second*90) {
			break
		}
	}
}

func TestScheduledFlushPendingMessages(t *testing.T) {

	t.Run("return immediately when cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		ScheduledFlushPendingMessages[*ExamplePendingMessage](ctx, nil, nil, nil)
	})

	t.Run("flushes two set of pages until cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		store := new(InMemoryStorage[*ExamplePendingMessage])
		sentCount := new(atomic.Int32)
		pub := hrabbitmq.FuncPublisher(func(ctx context.Context, data []byte, routingKeys []string, options ...func(*rabbitmq.PublishOptions)) (rabbitmq.PublisherConfirmation, error) {
			sentCount.Add(1)
			return nil, nil
		})

		wg := new(sync.WaitGroup)
		wg.Add(1)
		go func() {
			defer wg.Done()
			ScheduledFlushPendingMessages[*ExamplePendingMessage](ctx, store, pub, func() time.Duration {
				if sentCount.Load() <= 2 {
					store.Put([]*ExamplePendingMessage{
						{Id: "a", Exchange: "e", RoutingKeys: []string{"a"}},
						{Id: "b", Exchange: "e", RoutingKeys: []string{"b"}},
					})
					return 0
				}
				cancel()
				return time.Minute
			})
		}()

		wg.Wait()
		assert.Equal(t, 4, int(sentCount.Load()))
	})

	t.Run("don't retry on publish errors", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		store := new(InMemoryStorage[*ExamplePendingMessage])
		store.Put([]*ExamplePendingMessage{{Id: "a", Exchange: "e", RoutingKeys: []string{"a"}}})

		sendAttempts := new(atomic.Int32)
		pub := hrabbitmq.FuncPublisher(func(ctx context.Context, data []byte, routingKeys []string, options ...func(*rabbitmq.PublishOptions)) (rabbitmq.PublisherConfirmation, error) {
			if sendAttempts.Add(1) <= 2 {
				return nil, fmt.Errorf("oh no!")
			}
			return nil, nil
		})

		wg := new(sync.WaitGroup)
		wg.Add(1)
		go func() {
			defer wg.Done()
			ScheduledFlushPendingMessages[*ExamplePendingMessage](ctx, store, pub, func() time.Duration {
				if sendAttempts.Load() <= 2 {
					return 0
				}
				cancel()
				return time.Minute
			})
		}()

		wg.Wait()
		assert.Equal(t, 3, int(sendAttempts.Load()))
	})
}
