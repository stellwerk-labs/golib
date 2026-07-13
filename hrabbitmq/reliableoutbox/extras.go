package reliableoutbox

import (
	"context"
	"slices"
	"sync"
	"time"
)

// InMemoryStorage is an example implementation of the message storage. This is used in the unit tests here but may also
// be used in unit tests in service code that consumes this type.
type InMemoryStorage[k PendingMessage] struct {
	lock          sync.Mutex
	messages      []k
	pendingErrors chan error
}

func (i *InMemoryStorage[k]) Put(messages []k) {
	i.lock.Lock()
	defer i.lock.Unlock()
	if i.messages == nil {
		i.messages = make([]k, 0, len(messages))
	}
	for _, message := range messages {
		i.messages = append(i.messages, message)
	}
}

func (i *InMemoryStorage[k]) Complete(ctx context.Context, messageId string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	i.lock.Lock()
	defer i.lock.Unlock()
	select {
	case err := <-i.pendingErrors:
		return err
	default:
		i.messages = slices.DeleteFunc(i.messages, func(m k) bool {
			return m.MessageId() == messageId
		})
		return nil
	}
}

func (i *InMemoryStorage[k]) LoadPage(ctx context.Context) ([]k, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	i.lock.Lock()
	defer i.lock.Unlock()
	select {
	case err := <-i.pendingErrors:
		return nil, true, err
	default:
		output := make([]k, len(i.messages))
		copy(output, i.messages)
		return output, false, nil
	}
}

func (i *InMemoryStorage[k]) AddError(err error) {
	i.lock.Lock()
	defer i.lock.Unlock()
	if i.pendingErrors == nil {
		i.pendingErrors = make(chan error, 1000)
	}
	select {
	case i.pendingErrors <- err:
	default:
		panic("pending errors are full")
	}
}

// InstallTestCallbackOnContextWithTimeout is a testing utility that will return the amended context along with a
// waiting function. The waiting function will block until it receives the callback or will panic.
func InstallTestCallbackOnContextWithTimeout(ctx context.Context, timeout time.Duration) (context.Context, func()) {
	c := make(chan bool, 1)
	return context.WithValue(ctx, testCallbackContextKey, c), func() {
		select {
		case <-c:
		case <-time.After(timeout):
			panic("timeout waiting for callback")
		}
	}
}

// InstallTestCallbackOnContext is InstallTestCallbackOnContextWithTimeout with a default 10s timeout.
func InstallTestCallbackOnContext(ctx context.Context) (context.Context, func()) {
	return InstallTestCallbackOnContextWithTimeout(ctx, time.Second*10)
}
