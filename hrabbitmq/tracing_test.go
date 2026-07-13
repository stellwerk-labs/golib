package hrabbitmq

import (
	"context"
	"strconv"
	"testing"

	"github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/wagslane/go-rabbitmq"
	"go.uber.org/zap"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/mocktracer"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func TestTracing(t *testing.T) {
	mt := mocktracer.Start()
	defer mt.Stop()

	logger := &zap.Logger{}

	publishSpan, publishCtx := tracer.StartSpanFromContext(context.Background(), "publish")
	defer publishSpan.Finish()

	msg := amqp091.Publishing{
		Headers: amqp091.Table{
			"numeric": 5,
		},
	}

	InjectSpanToMessage(publishCtx, logger, &msg)

	consumeCtx := context.Background()

	traceID := strconv.FormatUint(publishSpan.Context().TraceID(), 10)

	assert.Contains(t, msg.Headers, "traceparent")
	assert.Contains(t, msg.Headers, "tracestate")
	assert.Contains(t, msg.Headers, "x-datadog-tags")
	assert.Equal(t, 5, msg.Headers["numeric"])
	assert.Equal(t, traceID, msg.Headers["x-datadog-trace-id"])
	assert.Equal(t, traceID, msg.Headers["x-datadog-parent-id"])

	consumeSpan, _ := tracer.StartSpanFromContext(consumeCtx, "consume", ExtractSpanFromMessage(logger, msg.Headers)...)
	defer consumeSpan.Finish()

	assert.Equal(t, publishSpan.Context().TraceID(), consumeSpan.Context().TraceID())
}

func TestTracingInjectTable(t *testing.T) {
	mt := mocktracer.Start()
	defer mt.Stop()

	logger := &zap.Logger{}

	publishSpan, publishCtx := tracer.StartSpanFromContext(context.Background(), "publish")
	defer publishSpan.Finish()

	traceID := strconv.FormatUint(publishSpan.Context().TraceID(), 10)

	headers := InjectSpanToTable(publishCtx, logger, rabbitmq.Table{"a": "b"})

	assert.Contains(t, headers, "traceparent")
	assert.Contains(t, headers, "tracestate")
	assert.Contains(t, headers, "x-datadog-tags")
	assert.Equal(t, "b", headers["a"])
	assert.Equal(t, traceID, headers["x-datadog-trace-id"])
	assert.Equal(t, traceID, headers["x-datadog-parent-id"])
}
