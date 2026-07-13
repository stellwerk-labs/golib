package delayqueues

import (
	"context"
	"testing"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wagslane/go-rabbitmq"
	"go.uber.org/zap/zaptest"

	"github.com/stellwerk-labs/golib/hrabbitmq"
)

func TestHandleDelayContinuation_no_headers(t *testing.T) {
	pub := new(hrabbitmq.NoOpPublisher)
	dqc, err := NewConfig("my-exchange", "k%s", []time.Duration{time.Second, time.Minute}, pub)
	require.NoError(t, err)
	ok, err := dqc.HandleDelayContinuation(context.Background(), zaptest.NewLogger(t), &rabbitmq.Delivery{
		Delivery: amqp091.Delivery{
			RoutingKey: "original",
			Body:       []byte{1, 2, 3},
			Headers:    map[string]interface{}{},
		},
	})
	assert.False(t, ok)
	assert.NoError(t, err)
}

func TestHandleDelayContinuation_basic(t *testing.T) {
	pub := new(hrabbitmq.NoOpPublisher)
	dqc, err := NewConfig("my-exchange", "k%s", []time.Duration{time.Second, time.Minute}, pub)
	require.NoError(t, err)
	ok, err := dqc.HandleDelayContinuation(context.Background(), zaptest.NewLogger(t), &rabbitmq.Delivery{
		Delivery: amqp091.Delivery{
			RoutingKey: "original",
			Body:       []byte{1, 2, 3},
			Headers: map[string]interface{}{
				DefaultDelayRemainingHeader: "3s",
			},
		},
	})
	assert.True(t, ok)
	assert.NoError(t, err)
	if assert.Len(t, pub.Recorded, 1) {
		re := pub.Recorded[0]
		assert.Equal(t, []string{"k1s"}, re.Keys)
		assert.Equal(t, rabbitmq.Table(map[string]interface{}{
			DefaultDelayEndedRoutingKeyHeader: "original",
			DefaultDelayRemainingHeader:       "2s",
		}), re.Options.Headers)
	}
}

func TestHandleDelayContinuation_stuck_old(t *testing.T) {
	pub := new(hrabbitmq.NoOpPublisher)
	dqc, err := NewConfig("my-exchange", "k%s", []time.Duration{time.Second, time.Minute}, pub)
	require.NoError(t, err)
	ok, err := dqc.HandleDelayContinuation(context.Background(), zaptest.NewLogger(t), &rabbitmq.Delivery{
		Delivery: amqp091.Delivery{
			RoutingKey: "original",
			Body:       []byte{1, 2, 3},
			Headers: map[string]interface{}{
				DefaultDelayRemainingHeader: "3s",
				DeathHeader: amqp091.Table{
					"time": time.Now().Add(-time.Minute),
				},
			},
		},
	})
	assert.False(t, ok)
	assert.NoError(t, err)
}
