package htelemetry

import (
	"context"
	"encoding/hex"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// otelProvider implements Provider using the OpenTelemetry SDK.
type otelProvider struct {
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
}

// Ensure otelProvider implements Provider.
var _ Provider = (*otelProvider)(nil)

// NewOTelProvider creates a new OpenTelemetry provider.
// The tracerName should be a unique identifier for your service/library (e.g., "my-service").
// This uses the global OTel TracerProvider and TextMapPropagator, so you should configure
// those before calling this function.
func NewOTelProvider(tracerName string) Provider {
	return &otelProvider{
		tracer:     otel.Tracer(tracerName),
		propagator: otel.GetTextMapPropagator(),
	}
}

// NewOTelProviderWithTracer creates a new OpenTelemetry provider with a specific tracer.
// This is useful when you want to use a custom TracerProvider instead of the global one.
func NewOTelProviderWithTracer(tracer trace.Tracer, propagator propagation.TextMapPropagator) Provider {
	if propagator == nil {
		propagator = otel.GetTextMapPropagator()
	}
	return &otelProvider{
		tracer:     tracer,
		propagator: propagator,
	}
}

func (p *otelProvider) StartSpan(operationName string, opts ...StartSpanOption) Span {
	cfg := applyStartOpts(opts)
	ctx := context.Background()

	// If there's a parent, inject it into the context so the tracer picks it up.
	if cfg.Parent != nil {
		if oc, ok := cfg.Parent.(*otelSpanContext); ok {
			ctx = trace.ContextWithSpanContext(ctx, oc.inner)
		}
	}

	otelOpts := p.convertStartOpts(cfg)
	ctx, span := p.tracer.Start(ctx, operationName, otelOpts...)
	return &otelSpan{span: span, ctx: ctx}
}

func (p *otelProvider) StartSpanFromContext(ctx context.Context, operationName string, opts ...StartSpanOption) (Span, context.Context) {
	cfg := applyStartOpts(opts)

	// If an explicit parent is provided via ChildOf, it takes precedence over
	// any span already in the context — matching the Datadog implementation.
	if cfg.Parent != nil {
		if oc, ok := cfg.Parent.(*otelSpanContext); ok {
			ctx = trace.ContextWithSpanContext(ctx, oc.inner)
		}
	}

	otelOpts := p.convertStartOpts(cfg)
	ctx, span := p.tracer.Start(ctx, operationName, otelOpts...)
	return &otelSpan{span: span, ctx: ctx}, ctx
}

func (p *otelProvider) SpanFromContext(ctx context.Context) (Span, bool) {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return nil, false
	}
	return &otelSpan{span: span, ctx: ctx}, true
}

func (p *otelProvider) ContextWithSpan(ctx context.Context, span Span) context.Context {
	if os, ok := span.(*otelSpan); ok {
		return trace.ContextWithSpan(ctx, os.span)
	}
	return ctx
}

func (p *otelProvider) Inject(spanCtx SpanContext, carrier TextMapCarrier) error {
	if oc, ok := spanCtx.(*otelSpanContext); ok {
		ctx := trace.ContextWithSpanContext(context.Background(), oc.inner)
		p.propagator.Inject(ctx, &otelCarrierAdapter{carrier})
		return nil
	}
	return fmt.Errorf("cannot inject non-OpenTelemetry span context")
}

func (p *otelProvider) Extract(carrier TextMapCarrier) (SpanContext, error) {
	ctx := p.propagator.Extract(context.Background(), &otelCarrierAdapter{carrier})
	spanCtx := trace.SpanContextFromContext(ctx)
	if !spanCtx.IsValid() {
		return nil, ErrSpanContextNotFound
	}
	return &otelSpanContext{inner: spanCtx}, nil
}

func (p *otelProvider) convertStartOpts(cfg *startSpanConfig) []trace.SpanStartOption {
	var opts []trace.SpanStartOption

	// Resource name in OTel is typically set as an attribute
	if cfg.ResourceName != "" {
		opts = append(opts, trace.WithAttributes(attribute.String("resource.name", cfg.ResourceName)))
	}

	for k, v := range cfg.Tags {
		opts = append(opts, trace.WithAttributes(toOTelAttribute(k, v)))
	}

	return opts
}

// toOTelAttribute converts a key-value pair to an OTel attribute.
func toOTelAttribute(key string, value interface{}) attribute.KeyValue {
	switch v := value.(type) {
	case string:
		return attribute.String(key, v)
	case int:
		return attribute.Int(key, v)
	case int64:
		return attribute.Int64(key, v)
	case float64:
		return attribute.Float64(key, v)
	case bool:
		return attribute.Bool(key, v)
	default:
		return attribute.String(key, fmt.Sprint(v))
	}
}

// otelSpan wraps an OpenTelemetry span to implement the Span interface.
type otelSpan struct {
	span trace.Span
	ctx  context.Context
}

func (s *otelSpan) SetTag(key string, value interface{}) {
	s.span.SetAttributes(toOTelAttribute(key, value))
}

func (s *otelSpan) Finish(opts ...FinishOption) {
	cfg := applyFinishOpts(opts)
	if cfg.Error != nil {
		s.span.RecordError(cfg.Error)
		s.span.SetStatus(codes.Error, cfg.Error.Error())
	}
	s.span.End()
}

func (s *otelSpan) Context() SpanContext {
	return &otelSpanContext{inner: s.span.SpanContext()}
}

// Inner returns the underlying OpenTelemetry span for cases where direct access is needed.
func (s *otelSpan) Inner() trace.Span {
	return s.span
}

// otelSpanContext wraps an OpenTelemetry span context to implement the SpanContext interface.
type otelSpanContext struct {
	inner trace.SpanContext
}

func (c *otelSpanContext) TraceID() string {
	return c.inner.TraceID().String()
}

func (c *otelSpanContext) SpanID() string {
	return c.inner.SpanID().String()
}

// TraceIDUint64 returns the lower 64 bits of the 128-bit trace ID.
// This can be used for Datadog log correlation when sending OTel traces to Datadog.
// For pure OTel backends, use TraceID() string instead.
func (c *otelSpanContext) TraceIDUint64() uint64 {
	// OTel trace ID is 128 bits (16 bytes), DD uses lower 64 bits (last 8 bytes)
	traceID := c.inner.TraceID()
	bytes := traceID[8:16] // Lower 64 bits
	var result uint64
	for _, b := range bytes {
		result = (result << 8) | uint64(b)
	}
	return result
}

// SpanIDUint64 returns the span ID as a uint64.
// OTel span IDs are 64 bits, same as Datadog.
func (c *otelSpanContext) SpanIDUint64() uint64 {
	spanID := c.inner.SpanID()
	var result uint64
	for _, b := range spanID {
		result = (result << 8) | uint64(b)
	}
	return result
}

// Inner returns the underlying OpenTelemetry span context for cases where direct access is needed.
func (c *otelSpanContext) Inner() trace.SpanContext {
	return c.inner
}

// otelCarrierAdapter adapts our TextMapCarrier to OTel's TextMapCarrier interface.
type otelCarrierAdapter struct {
	carrier TextMapCarrier
}

func (a *otelCarrierAdapter) Get(key string) string {
	var result string
	a.carrier.ForeachKey(func(k, v string) error {
		if k == key {
			result = v
		}
		return nil
	})
	return result
}

func (a *otelCarrierAdapter) Set(key, val string) {
	a.carrier.Set(key, val)
}

func (a *otelCarrierAdapter) Keys() []string {
	var keys []string
	a.carrier.ForeachKey(func(k, _ string) error {
		keys = append(keys, k)
		return nil
	})
	return keys
}

// Ensure otelCarrierAdapter implements OTel's TextMapCarrier.
var _ propagation.TextMapCarrier = (*otelCarrierAdapter)(nil)

// AsOTelSpan extracts the underlying OpenTelemetry span from an htelemetry.Span.
// Returns nil if the span is not an OpenTelemetry span.
func AsOTelSpan(span Span) trace.Span {
	if os, ok := span.(*otelSpan); ok {
		return os.span
	}
	return nil
}

// AsOTelSpanContext extracts the underlying OpenTelemetry span context from an htelemetry.SpanContext.
// Returns an invalid span context if not an OpenTelemetry span context.
func AsOTelSpanContext(ctx SpanContext) trace.SpanContext {
	if oc, ok := ctx.(*otelSpanContext); ok {
		return oc.inner
	}
	return trace.SpanContext{}
}

// WrapOTelSpan wraps an OpenTelemetry span as an htelemetry.Span.
func WrapOTelSpan(span trace.Span) Span {
	if span == nil {
		return nil
	}
	return &otelSpan{span: span, ctx: context.Background()}
}

// TraceIDToDatadog converts an OTel 128-bit trace ID to Datadog's 64-bit format.
// This extracts the lower 64 bits of the trace ID.
func TraceIDToDatadog(traceID trace.TraceID) uint64 {
	bytes := traceID[8:16]
	var result uint64
	for _, b := range bytes {
		result = (result << 8) | uint64(b)
	}
	return result
}

// DatadogToTraceID converts a Datadog 64-bit trace ID to OTel's 128-bit format.
// The upper 64 bits are set to zero.
func DatadogToTraceID(ddTraceID uint64) trace.TraceID {
	var traceID trace.TraceID
	// Set lower 64 bits
	for i := 15; i >= 8; i-- {
		traceID[i] = byte(ddTraceID & 0xff)
		ddTraceID >>= 8
	}
	return traceID
}

// DatadogToSpanID converts a Datadog 64-bit span ID to OTel's span ID format.
func DatadogToSpanID(ddSpanID uint64) trace.SpanID {
	var spanID trace.SpanID
	for i := 7; i >= 0; i-- {
		spanID[i] = byte(ddSpanID & 0xff)
		ddSpanID >>= 8
	}
	return spanID
}

// ParseTraceID parses a hex string trace ID into an OTel TraceID.
func ParseTraceID(s string) (trace.TraceID, error) {
	var traceID trace.TraceID
	bytes, err := hex.DecodeString(s)
	if err != nil {
		return traceID, err
	}
	if len(bytes) != 16 {
		return traceID, fmt.Errorf("invalid trace ID length: expected 16 bytes, got %d", len(bytes))
	}
	copy(traceID[:], bytes)
	return traceID, nil
}

// ParseSpanID parses a hex string span ID into an OTel SpanID.
func ParseSpanID(s string) (trace.SpanID, error) {
	var spanID trace.SpanID
	bytes, err := hex.DecodeString(s)
	if err != nil {
		return spanID, err
	}
	if len(bytes) != 8 {
		return spanID, fmt.Errorf("invalid span ID length: expected 8 bytes, got %d", len(bytes))
	}
	copy(spanID[:], bytes)
	return spanID, nil
}
