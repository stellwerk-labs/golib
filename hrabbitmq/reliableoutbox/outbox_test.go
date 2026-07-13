package reliableoutbox

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"github.com/stellwerk-labs/golib/hrabbitmq"
)

type ExamplePendingMessage struct {
	Id          string
	Exchange    string
	RoutingKeys []string
	Payload     []byte
}

func (e *ExamplePendingMessage) MessageId() string {
	return e.Id
}

func (e *ExamplePendingMessage) MessageExchange() string {
	return e.Exchange
}

func (e *ExamplePendingMessage) MessageRoutingKeys() []string {
	return e.RoutingKeys
}

func (e *ExamplePendingMessage) MessagePayload() []byte {
	return e.Payload
}

var _ PendingMessage = (*ExamplePendingMessage)(nil)

func TestPrepareFlush_send_nothing(t *testing.T) {
	storage := new(InMemoryStorage[*ExamplePendingMessage])
	closer := PrepareOptimisticPublish[*ExamplePendingMessage](zaptest.NewLogger(t), storage, []*ExamplePendingMessage{})
	publisher := new(hrabbitmq.NoOpPublisher)
	assert.NotNil(t, closer)
	closer(context.Background(), publisher)
	assert.Len(t, publisher.Recorded, 0)
}

func TestPrepareFlush_send_some(t *testing.T) {
	publisher := new(hrabbitmq.NoOpPublisher)
	storage := new(InMemoryStorage[*ExamplePendingMessage])
	messages := []*ExamplePendingMessage{
		{Id: "a", Exchange: "e", RoutingKeys: []string{"r"}},
		{Id: "b", Exchange: "e", RoutingKeys: []string{"r"}},
	}
	storage.Put(messages)
	closer := PrepareOptimisticPublish[*ExamplePendingMessage](zaptest.NewLogger(t), storage, messages)
	assert.NotNil(t, closer)

	t.Cleanup(publisher.RegisterPublishChecker(func(publish hrabbitmq.RecordedPublish) {
		assert.Equal(t, "a", publish.Options.MessageID)
	}))
	t.Cleanup(publisher.RegisterPublishChecker(func(publish hrabbitmq.RecordedPublish) {
		assert.Equal(t, "b", publish.Options.MessageID)
	}))

	ctx, waiter := InstallTestCallbackOnContext(context.Background())
	closer(ctx, publisher)
	waiter()
	assert.Len(t, publisher.Recorded, 2)

	page, more, err := storage.LoadPage(context.Background())
	if assert.NoError(t, err) {
		assert.False(t, more)
		assert.Len(t, page, 0)
	}

	more, err = FlushNextPage[*ExamplePendingMessage](context.Background(), zaptest.NewLogger(t), storage, 3, publisher)
	if assert.NoError(t, err) {
		assert.False(t, more)
	}
}

func TestPrepareFlush_PublishErrors(t *testing.T) {
	publisher := new(hrabbitmq.NoOpPublisher)
	publisher.AddPendingError(errors.New("hi"))
	publisher.AddPendingError(errors.New("bye"))
	storage := new(InMemoryStorage[*ExamplePendingMessage])

	messages := []*ExamplePendingMessage{
		{Id: "a", Exchange: "e", RoutingKeys: []string{"r"}},
	}
	storage.Put(messages)
	closer := PrepareOptimisticPublish[*ExamplePendingMessage](zaptest.NewLogger(t), storage, messages)
	assert.NotNil(t, closer)

	// should try and send the same message twice
	t.Cleanup(publisher.RegisterPublishChecker(func(publish hrabbitmq.RecordedPublish) {
		assert.Equal(t, "a", publish.Options.MessageID)
	}))
	t.Cleanup(publisher.RegisterPublishChecker(func(publish hrabbitmq.RecordedPublish) {
		assert.Equal(t, "a", publish.Options.MessageID)
	}))

	closer(context.Background(), publisher)
	assert.Len(t, publisher.GetRecorded(), 0)

	page, more, err := storage.LoadPage(context.Background())
	if assert.NoError(t, err) {
		assert.False(t, more)
		assert.Len(t, page, 1)
	}

	more, err = FlushNextPage[*ExamplePendingMessage](context.Background(), zaptest.NewLogger(t), storage, 3, publisher)
	assert.EqualError(t, err, "failed to send pending message: bye")
	assert.False(t, more)
}

func TestPrepareFlush_send_on_flush(t *testing.T) {
	publisher := new(hrabbitmq.NoOpPublisher)
	storage := new(InMemoryStorage[*ExamplePendingMessage])
	messages := []*ExamplePendingMessage{
		{Id: "1", Exchange: "e", RoutingKeys: []string{"r"}},
		{Id: "2", Exchange: "e", RoutingKeys: []string{"r"}},
	}
	storage.Put(messages)
	closer := PrepareOptimisticPublish[*ExamplePendingMessage](zaptest.NewLogger(t), storage, messages)
	assert.NotNil(t, closer)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	closer(ctx, publisher)
	assert.Len(t, publisher.GetRecorded(), 0)

	page, more, err := storage.LoadPage(context.Background())
	if assert.NoError(t, err) {
		assert.False(t, more)
		assert.Len(t, page, 2)
	}

	more, err = FlushNextPage[*ExamplePendingMessage](context.Background(), zaptest.NewLogger(t), storage, 3, publisher)
	if assert.NoError(t, err) {
		assert.False(t, more)
	}

	page, more, err = storage.LoadPage(context.Background())
	if assert.NoError(t, err) {
		assert.False(t, more)
		assert.Len(t, page, 0)
	}
}

func TestFlushNextPage_cancelled_prior(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	storage := new(InMemoryStorage[*ExamplePendingMessage])
	more, err := FlushNextPage[*ExamplePendingMessage](ctx, zaptest.NewLogger(t), storage, 1, new(hrabbitmq.NoOpPublisher))
	assert.True(t, more)
	assert.EqualError(t, err, "failed to load a page of pending messages: context canceled")
}

func TestFlushNextPage_no_workers(t *testing.T) {
	storage := new(InMemoryStorage[*ExamplePendingMessage])
	more, err := FlushNextPage[*ExamplePendingMessage](context.Background(), zaptest.NewLogger(t), storage, 0, new(hrabbitmq.NoOpPublisher))
	assert.True(t, more)
	assert.EqualError(t, err, "parallelism must be > 0")
}

func TestFlushNextPage_page_error(t *testing.T) {
	storage := new(InMemoryStorage[*ExamplePendingMessage])
	storage.AddError(fmt.Errorf("oh no!"))
	more, err := FlushNextPage[*ExamplePendingMessage](context.Background(), zaptest.NewLogger(t), storage, 1, new(hrabbitmq.NoOpPublisher))
	assert.True(t, more)
	assert.EqualError(t, err, "failed to load a page of pending messages: oh no!")
}

func TestFlushNextPage_many_messages(t *testing.T) {
	publisher := new(hrabbitmq.NoOpPublisher)
	storage := new(InMemoryStorage[*ExamplePendingMessage])
	messages := make([]*ExamplePendingMessage, 100)
	for i := 0; i < len(messages); i++ {
		messages[i] = &ExamplePendingMessage{Id: fmt.Sprint(i), Exchange: "e", RoutingKeys: []string{"r"}}
	}
	storage.Put(messages)

	more, err := FlushNextPage[*ExamplePendingMessage](context.Background(), zaptest.NewLogger(t), storage, 3, publisher)
	if assert.NoError(t, err) {
		assert.False(t, more)
	}

	assert.Len(t, publisher.GetRecorded(), 100)

	page, more, err := storage.LoadPage(context.Background())
	if assert.NoError(t, err) {
		assert.False(t, more)
		assert.Len(t, page, 0)
	}
}
