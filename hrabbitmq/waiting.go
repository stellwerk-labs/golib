package hrabbitmq

import (
	"context"
	"fmt"
	"sync"

	"github.com/wagslane/go-rabbitmq"
)

// HandlerWaiter is a structure that can be used to wait (see Wait) until all wrapped (see Wrap) handlers have finished
// and are no longer processing messages. These inflight messages will still be re-queued automatically since the
// consumer was cancelled, but hopefully they are idempotent and can finish safely.
type HandlerWaiter struct {
	lock sync.RWMutex
}

// Wrap returns a modified handler that is coordinated around the waiting lock. If we are currently Wait-ing then the
// handler will be skipped in order to prevent deadlocks.
func (hw *HandlerWaiter) Wrap(original rabbitmq.Handler) rabbitmq.Handler {
	if hw == nil {
		panic("waiter is nil")
	}
	return func(d rabbitmq.Delivery) rabbitmq.Action {
		if hw.lock.TryRLock() {
			defer hw.lock.RUnlock()
			return original(d)
		}
		return rabbitmq.NackRequeue
	}
}

// Wait until the handler is no longer processing messages. Or until the context is cancelled or times-out.
func (hw *HandlerWaiter) Wait(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	} else if ctx.Err() != nil {
		return ctx.Err()
	}
	c := make(chan struct{})
	go func() {
		hw.lock.Lock()
		defer hw.lock.Unlock()
		close(c)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c:
		return nil
	}
}

// ConsumerWithHandlerWaiter is the struct returned by NewConsumerWithHandlerWaiter
type ConsumerWithHandlerWaiter struct {
	*rabbitmq.Consumer
	rabbitmq.Handler
	*HandlerWaiter

	once       sync.Once
	closeError error
}

func (c *ConsumerWithHandlerWaiter) Run() error {
	return c.Consumer.Run(c.Handler)
}

func (c *ConsumerWithHandlerWaiter) Close(ctx context.Context) error {
	c.once.Do(func() {
		c.Consumer.Close()
		c.closeError = c.Wait(ctx)
	})
	return c.closeError
}

// NewConsumerWithHandlerWaiter returns a consumer with a close function that also waits for the goroutines to complete.
// The consumer MUST still be started with Run().
func NewConsumerWithHandlerWaiter(conn *rabbitmq.Conn, handler rabbitmq.Handler, queue string, options ...func(*rabbitmq.ConsumerOptions)) (*ConsumerWithHandlerWaiter, error) {
	if handler == nil {
		return nil, fmt.Errorf("handler function is nil")
	}
	waiter := new(HandlerWaiter)
	consumer, err := rabbitmq.NewConsumer(conn, queue, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to setup consumer: %w", err)
	}
	return &ConsumerWithHandlerWaiter{Consumer: consumer, HandlerWaiter: waiter, Handler: waiter.Wrap(handler)}, nil
}
