package hlogger

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/stellwerk-labs/golib/htelemetry"
)

// TraceScopedLoggerFromCtx returns a logger with trace context using the global telemetry provider.
// This works with both Datadog and OpenTelemetry backends.
//
// For Datadog backend: adds string trace_id/span_id and dd.trace_id/dd.span_id (uint64) for log correlation.
// For OpenTelemetry backend: adds only dd.* fields — the otelzap bridge handles native trace context.
//
// If no span exists in the context, the original logger is returned unchanged.
func TraceScopedLoggerFromCtx(logger *zap.Logger, ctx context.Context) *zap.Logger {
	span, ok := htelemetry.SpanFromContext(ctx)
	if !ok {
		return logger
	}
	return TraceScopedLoggerFromSpan(logger, span)
}

// TraceScopedLoggerFromSpan adds trace context from an htelemetry.Span to the logger.
// This works with both Datadog and OpenTelemetry spans.
//
// For Datadog spans: adds string trace_id/span_id and dd.trace_id/dd.span_id (uint64) for log correlation.
// For OpenTelemetry spans: adds only dd.* fields — the otelzap bridge handles native trace context.
func TraceScopedLoggerFromSpan(logger *zap.Logger, span htelemetry.Span) *zap.Logger {
	spanCtx := span.Context()

	// When using Datadog provider, add string trace_id/span_id fields for log correlation.
	// When using OTel provider, the otelzap bridge sets native TraceID/SpanID on the log record.
	if !htelemetry.IsOTel() {
		logger = logger.With(
			zap.String("trace_id", spanCtx.TraceID()),
			zap.String("span_id", spanCtx.SpanID()),
		)
	}

	// Add dd.* fields for Datadog log correlation.
	// For DD provider: uses the native uint64 values.
	// For OTel provider: uses the lower 64 bits of the 128-bit trace ID (compatible with DD's OTLP ingestion).
	traceID := spanCtx.TraceIDUint64()
	spanID := spanCtx.SpanIDUint64()
	if traceID != 0 || spanID != 0 {
		logger = logger.With(zap.Object("dd", zapcore.ObjectMarshalerFunc(func(enc zapcore.ObjectEncoder) error {
			enc.AddUint64("trace_id", traceID)
			enc.AddUint64("span_id", spanID)
			return nil
		})))
	}

	return logger
}

// WithSpanFromCtx returns key-value pairs for adding trace context to a sugared logger.
//
// Example usage:
//
//	logger.Sugar().Infow("message", hlogger.WithSpanFromCtx(ctx)...)
func WithSpanFromCtx(ctx context.Context) []interface{} {
	span, ok := htelemetry.SpanFromContext(ctx)
	if !ok {
		return nil
	}

	spanCtx := span.Context()
	var result []interface{}

	// When using Datadog provider, add string trace_id/span_id fields.
	// When using OTel provider, the otelzap bridge handles native trace context.
	if !htelemetry.IsOTel() {
		result = append(result,
			"trace_id", spanCtx.TraceID(),
			"span_id", spanCtx.SpanID(),
		)
	}

	// Add dd.* for Datadog log correlation
	traceID := spanCtx.TraceIDUint64()
	spanID := spanCtx.SpanIDUint64()
	if traceID != 0 || spanID != 0 {
		result = append(result, "dd", map[string]uint64{
			"trace_id": traceID,
			"span_id":  spanID,
		})
	}

	return result
}

// LogDetailsFromCtx appends trace context to a list of key-value pairs for sugared logging.
//
// Example usage:
//
//	logger.Sugar().Infow("message", hlogger.LogDetailsFromCtx(ctx, "key", "value")...)
func LogDetailsFromCtx(ctx context.Context, args ...interface{}) []interface{} {
	return append(args, WithSpanFromCtx(ctx)...)
}
