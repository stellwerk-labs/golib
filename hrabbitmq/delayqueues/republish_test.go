package delayqueues

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wagslane/go-rabbitmq"
	"go.uber.org/zap/zaptest"

	"github.com/stellwerk-labs/golib/hrabbitmq"
)

func TestRepublishMessageWithDelay_fail_after_retries(t *testing.T) {
	pub := new(hrabbitmq.NoOpPublisher)
	pub.AddPendingError(io.EOF)
	pub.AddPendingError(io.EOF)
	pub.AddPendingError(io.ErrUnexpectedEOF)

	dqc, err := NewConfig("my-exchange", "k%s", []time.Duration{time.Second}, pub, WithPublishRetry(func(i int) time.Duration {
		if i >= 3 {
			return -1
		}
		return time.Millisecond
	}))
	require.NoError(t, err)

	assert.EqualError(t, dqc.RepublishMessageWithDelay(context.Background(), zaptest.NewLogger(t), &rabbitmq.Delivery{
		Delivery: amqp091.Delivery{
			RoutingKey: "original",
			Body:       []byte{1, 2, 3},
			Headers:    map[string]interface{}{},
		},
	}, time.Second), "failed to publish message: unexpected EOF")
	assert.Empty(t, pub.Recorded)
}

func TestRepublishMessageWithDelay_success_after_retries(t *testing.T) {
	pub := new(hrabbitmq.NoOpPublisher)
	pub.AddPendingError(io.EOF)
	pub.AddPendingError(io.EOF)

	dqc, err := NewConfig("my-exchange", "k%s", []time.Duration{time.Second}, pub, WithPublishRetry(func(i int) time.Duration {
		if i >= 3 {
			return -1
		}
		return time.Millisecond
	}))
	require.NoError(t, err)

	assert.NoError(t, dqc.RepublishMessageWithDelay(context.Background(), zaptest.NewLogger(t), &rabbitmq.Delivery{
		Delivery: amqp091.Delivery{
			RoutingKey: "original",
			Body:       []byte{1, 2, 3},
			Headers:    map[string]interface{}{},
		},
	}, time.Minute))

	if assert.Len(t, pub.Recorded, 1) {
		re := pub.Recorded[0]
		assert.Equal(t, []string{"k1s"}, re.Keys)
		assert.Equal(t, rabbitmq.Table(map[string]interface{}{
			DefaultDelayEndedRoutingKeyHeader: "original",
			DefaultDelayRemainingHeader:       "59s",
		}), re.Options.Headers)
	}
}
