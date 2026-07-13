package hrabbitmq

import (
	"context"
	"errors"

	"github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/stellwerk-labs/golib/htelemetry"
)

// AMQPHeadersCarrier wraps an amqp.Table as a TextMapWriter and TextMapReader.
// It implements both DD tracer and htelemetry interfaces for trace context propagation.
type AMQPHeadersCarrier amqp091.Table

// Ensure AMQPHeadersCarrier implements required interfaces.
var _ tracer.TextMapWriter = (*AMQPHeadersCarrier)(nil)
var _ tracer.TextMapReader = (*AMQPHeadersCarrier)(nil)
var _ htelemetry.TextMapCarrier = (*AMQPHeadersCarrier)(nil)

// Set implements TextMapWriter and htelemetry.TextMapCarrier.
func (c AMQPHeadersCarrier) Set(key, val string) {
	c[key] = val
}

// ForeachKey implements TextMapReader and htelemetry.TextMapCarrier.
func (c AMQPHeadersCarrier) ForeachKey(handler func(key, val string) error) error {
	for k, v := range c {
		s, ok := v.(string)
		if !ok { // ignore non-string properties
			continue
		}
		if err := handler(k, s); err != nil {
			return err
		}
	}
	return nil
}

// ExtractSpanFromMessage extracts span context from AMQP headers using Datadog tracer.
// Deprecated: Use ExtractSpanOptionsFromMessage instead, which works with both DD and OTel.
func ExtractSpanFromMessage(logger *zap.Logger, header amqp091.Table) []tracer.StartSpanOption {
	if header == nil {
		return nil
	}
	spanCTX, err := tracer.Extract(AMQPHeadersCarrier(header))
	if err != nil {
		if !errors.Is(err, tracer.ErrSpanContextNotFound) {
			logger.Sugar().Warnw("failed reading span from message", "err", err)
		}
		return nil
	}
	return []tracer.StartSpanOption{tracer.ChildOf(spanCTX)}
}

// InjectSpanToMessage injects span context into AMQP message headers using Datadog tracer.
// Deprecated: Use InjectSpanToMessageWithProvider instead, which works with both DD and OTel.
func InjectSpanToMessage(ctx context.Context, logger *zap.Logger, msg *amqp091.Publishing) {
	msg.Headers = InjectSpanToTable(ctx, logger, msg.Headers)
}

// InjectSpanToTable injects span context into a map using Datadog tracer.
// Deprecated: Use InjectSpanToTableWithProvider instead, which works with both DD and OTel.
func InjectSpanToTable[K ~map[string]interface{}](ctx context.Context, logger *zap.Logger, table K) K {
	if table == nil {
		table = K{}
	}

	span, found := tracer.SpanFromContext(ctx)
	if found {
		if err := tracer.Inject(span.Context(), AMQPHeadersCarrier(table)); err != nil {
			logger.Sugar().Warnw("failed inject span into message", "err", err)
		}
	}

	return table
}

// --- Provider-agnostic functions (work with both DD and OTel) ---

// ExtractSpanContextFromMessage extracts span context from AMQP headers using the global telemetry provider.
// Works with both Datadog and OpenTelemetry.
// Returns the span context and a boolean indicating if extraction was successful.
func ExtractSpanContextFromMessage(logger *zap.Logger, header amqp091.Table) (htelemetry.SpanContext, bool) {
	if header == nil {
		return nil, false
	}
	spanCtx, err := htelemetry.Extract(AMQPHeadersCarrier(header))
	if err != nil {
		if !errors.Is(err, htelemetry.ErrSpanContextNotFound) {
			logger.Sugar().Warnw("failed reading span from message", "err", err)
		}
		return nil, false
	}
	return spanCtx, true
}

// ExtractSpanOptionsFromMessage extracts span context from AMQP headers and returns StartSpanOptions.
// Works with both Datadog and OpenTelemetry.
func ExtractSpanOptionsFromMessage(logger *zap.Logger, header amqp091.Table) []htelemetry.StartSpanOption {
	spanCtx, ok := ExtractSpanContextFromMessage(logger, header)
	if !ok {
		return nil
	}
	return []htelemetry.StartSpanOption{htelemetry.ChildOf(spanCtx)}
}

// InjectSpanToMessageWithProvider injects span context into AMQP message headers using the global telemetry provider.
// Works with both Datadog and OpenTelemetry.
func InjectSpanToMessageWithProvider(ctx context.Context, logger *zap.Logger, msg *amqp091.Publishing) {
	msg.Headers = InjectSpanToTableWithProvider(ctx, logger, msg.Headers)
}

// InjectSpanToTableWithProvider injects span context into a map using the global telemetry provider.
// Works with both Datadog and OpenTelemetry.
func InjectSpanToTableWithProvider[K ~map[string]interface{}](ctx context.Context, logger *zap.Logger, table K) K {
	if table == nil {
		table = K{}
	}

	span, found := htelemetry.SpanFromContext(ctx)
	if found {
		if err := htelemetry.Inject(span.Context(), AMQPHeadersCarrier(table)); err != nil {
			logger.Sugar().Warnw("failed inject span into message", "err", err)
		}
	}

	return table
}
