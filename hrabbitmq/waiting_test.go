package hrabbitmq

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wagslane/go-rabbitmq"
)

func TestWaiter(t *testing.T) {
	c := make(chan struct{})
	spawnGroup := sync.WaitGroup{}

	inner := rabbitmq.Handler(func(d rabbitmq.Delivery) (action rabbitmq.Action) {
		spawnGroup.Done()
		<-c
		return rabbitmq.Ack
	})
	waiter := new(HandlerWaiter)
	outer := waiter.Wrap(inner)
	spawnGroup.Add(10)
	for i := 0; i < 10; i++ {
		go outer(rabbitmq.Delivery{})
	}
	spawnGroup.Wait()

	// with deadline
	ctx1, cancel1 := context.WithTimeout(context.Background(), time.Millisecond*10)
	defer cancel1()
	assert.EqualError(t, waiter.Wait(ctx1), context.DeadlineExceeded.Error())

	// close the channel will unblock the handlers
	close(c)

	// now everything should stop cleanly
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Minute)
	defer cancel2()
	assert.NoError(t, waiter.Wait(ctx2))
}
