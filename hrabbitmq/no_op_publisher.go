package hrabbitmq

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"github.com/wagslane/go-rabbitmq"
)

// NoOpPublisher is used for testing. It silently records events without waiting for confirmation.
type NoOpPublisher struct {
	Recorded        []RecordedPublish
	pendingErrors   []error
	pendingWatchers []chan RecordedPublish
	lock            sync.Mutex
}

type RecordedPublish struct {
	Keys    []string
	Data    []byte
	Options *rabbitmq.PublishOptions
}

var _ Publisher = (*NoOpPublisher)(nil)

func (p *NoOpPublisher) AddPendingError(err error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.pendingErrors == nil {
		p.pendingErrors = []error{err}
	} else {
		p.pendingErrors = append(p.pendingErrors, err)
	}
}

func (p *NoOpPublisher) PublishWithDeferredConfirmWithContext(
	ctx context.Context,
	data []byte,
	routingKeys []string,
	options ...func(*rabbitmq.PublishOptions),
) (rabbitmq.PublisherConfirmation, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	// assemble recorded message
	opts := &rabbitmq.PublishOptions{}
	for _, optionFunc := range options {
		optionFunc(opts)
	}
	rec := RecordedPublish{
		Keys:    routingKeys,
		Data:    data,
		Options: opts,
	}

	// if something is watching for a message, call it
	if len(p.pendingWatchers) > 0 {
		next := p.pendingWatchers[0]
		p.pendingWatchers = p.pendingWatchers[1:]
		next <- rec
	}

	if len(p.pendingErrors) > 0 {
		err := p.pendingErrors[0]
		p.pendingErrors = p.pendingErrors[1:]
		return nil, err
	}
	if p.Recorded == nil {
		p.Recorded = make([]RecordedPublish, 0)
	}
	p.Recorded = append(p.Recorded, rec)

	return []*amqp091.DeferredConfirmation{}, nil
}

// GetRecorded returns thread-safe access to Recorded.
func (p *NoOpPublisher) GetRecorded() []RecordedPublish {
	p.lock.Lock()
	defer p.lock.Unlock()
	return slices.Clone(p.Recorded)
}

func (p *NoOpPublisher) RegisterPublishChecker(checker func(publish RecordedPublish)) func() {
	c := make(chan RecordedPublish, 1)
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.pendingWatchers == nil {
		p.pendingWatchers = make([]chan RecordedPublish, 0, 1)
	}
	p.pendingWatchers = append(p.pendingWatchers, c)
	return func() {
		select {
		case m := <-c:
			checker(m)
		case <-time.After(time.Minute):
			panic("timeout")
		}
	}
}
