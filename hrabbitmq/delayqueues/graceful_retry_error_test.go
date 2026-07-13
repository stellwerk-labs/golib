package delayqueues

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wagslane/go-rabbitmq"
	"go.uber.org/zap/zaptest"

	"github.com/stellwerk-labs/golib/hrabbitmq"
)

func TestWrap(t *testing.T) {
	w := NewGracefulRetryErrorWithDelay(fmt.Errorf("%w: blah", os.ErrClosed), time.Minute)
	assert.Equal(t, "graceful retry after 1m0s: file already closed: blah", w.Error())
	assert.Equal(t, os.ErrClosed, errors.Unwrap(errors.Unwrap(w)))

	var egr *GracefulRetryError
	if assert.True(t, errors.As(w, &egr)) {
		assert.Equal(t, time.Minute, egr.delay)
	}
}

func TestWrapZero(t *testing.T) {
	w := NewGracefulRetryError(fmt.Errorf("%w: blah", os.ErrClosed))
	assert.Equal(t, "graceful retry: file already closed: blah", w.Error())
	assert.Equal(t, os.ErrClosed, errors.Unwrap(errors.Unwrap(w)))

	var egr *GracefulRetryError
	if assert.True(t, errors.As(w, &egr)) {
		assert.Equal(t, time.Duration(0), egr.delay)
	}
}

func TestWrapNil(t *testing.T) {
	w := NewGracefulRetryErrorWithDelay(nil, time.Minute)
	assert.Equal(t, "graceful retry after 1m0s", w.Error())
	assert.Equal(t, nil, errors.Unwrap(w))

	var egr *GracefulRetryError
	if assert.True(t, errors.As(w, &egr)) {
		assert.Equal(t, time.Minute, egr.delay)
	}
}

func TestHandleGracefulRetryError_no_match(t *testing.T) {
	pub := new(hrabbitmq.NoOpPublisher)
	dqc, err := NewConfig("my-exchange", "k%s", []time.Duration{time.Second, time.Minute}, pub)
	require.NoError(t, err)
	handled, err := dqc.HandleGracefulRetryError(context.Background(), zaptest.NewLogger(t), &rabbitmq.Delivery{}, io.EOF)
	assert.False(t, handled)
	assert.EqualError(t, err, io.EOF.Error())
}

func TestHandleGracefulRetryError_fixed_delay(t *testing.T) {
	pub := new(hrabbitmq.NoOpPublisher)
	dqc, err := NewConfig("my-exchange", "k%s", []time.Duration{time.Second, time.Minute}, pub)
	require.NoError(t, err)
	handled, err := dqc.HandleGracefulRetryError(context.Background(), zaptest.NewLogger(t), &rabbitmq.Delivery{
		Delivery: amqp091.Delivery{
			RoutingKey: "original",
			Body:       []byte{1, 2, 3},
			Headers: map[string]interface{}{
				DefaultRetryAttemptTrackerHeader: 2,
			},
		},
	}, NewGracefulRetryErrorWithDelay(io.EOF, time.Minute*2))
	assert.True(t, handled)
	assert.NoError(t, err)
	if assert.Len(t, pub.Recorded, 1) {
		re := pub.Recorded[0]
		assert.Equal(t, []string{"k1m0s"}, re.Keys)
		assert.Equal(t, rabbitmq.Table(map[string]interface{}{
			DefaultDelayEndedRoutingKeyHeader: "original",
			DefaultDelayRemainingHeader:       "1m0s",
			DefaultRetryAttemptTrackerHeader:  3,
		}), re.Options.Headers)
	}
}

func TestHandleGracefulRetryError_exponential_delay(t *testing.T) {
	pub := new(hrabbitmq.NoOpPublisher)
	dqc, err := NewConfig("my-exchange", "k%s", []time.Duration{time.Second, time.Minute}, pub)
	require.NoError(t, err)
	handled, err := dqc.HandleGracefulRetryError(context.Background(), zaptest.NewLogger(t), &rabbitmq.Delivery{
		Delivery: amqp091.Delivery{
			RoutingKey: "original",
			Body:       []byte{1, 2, 3},
			Headers: map[string]interface{}{
				DefaultRetryAttemptTrackerHeader: 2,
			},
		},
	}, NewGracefulRetryError(io.EOF))
	assert.True(t, handled)
	assert.NoError(t, err)
	if assert.Len(t, pub.Recorded, 1) {
		re := pub.Recorded[0]
		assert.Equal(t, []string{"k1s"}, re.Keys)
		assert.Equal(t, rabbitmq.Table(map[string]interface{}{
			DefaultDelayEndedRoutingKeyHeader: "original",
			DefaultLastRetryDelayHeader:       "1s",
			DefaultRetryAttemptTrackerHeader:  3,
		}), re.Options.Headers)
	}
}
