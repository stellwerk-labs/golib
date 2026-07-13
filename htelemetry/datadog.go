package htelemetry

import (
	"context"
	"fmt"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

// datadogProvider implements Provider using the Datadog dd-trace-go library.
type datadogProvider struct{}

// Ensure datadogProvider implements Provider.
var _ Provider = (*datadogProvider)(nil)

func (p *datadogProvider) StartSpan(operationName string, opts ...StartSpanOption) Span {
	cfg := applyStartOpts(opts)
	ddOpts := p.convertStartOpts(cfg)
	return &ddSpan{inner: tracer.StartSpan(operationName, ddOpts...)}
}

func (p *datadogProvider) StartSpanFromContext(ctx context.Context, operationName string, opts ...StartSpanOption) (Span, context.Context) {
	cfg := applyStartOpts(opts)
	ddOpts := p.convertStartOpts(cfg)
	span, ctx := tracer.StartSpanFromContext(ctx, operationName, ddOpts...)
	return &ddSpan{inner: span}, ctx
}

func (p *datadogProvider) SpanFromContext(ctx context.Context) (Span, bool) {
	span, ok := tracer.SpanFromContext(ctx)
	if !ok {
		return nil, false
	}
	return &ddSpan{inner: span}, true
}

func (p *datadogProvider) ContextWithSpan(ctx context.Context, span Span) context.Context {
	if ds, ok := span.(*ddSpan); ok {
		return tracer.ContextWithSpan(ctx, ds.inner)
	}
	// If it's not a DD span, we can't put it in DD's context
	return ctx
}

func (p *datadogProvider) Inject(spanCtx SpanContext, carrier TextMapCarrier) error {
	if dc, ok := spanCtx.(*ddSpanContext); ok {
		return tracer.Inject(dc.inner, &ddCarrierAdapter{carrier})
	}
	return fmt.Errorf("cannot inject non-Datadog span context")
}

func (p *datadogProvider) Extract(carrier TextMapCarrier) (SpanContext, error) {
	ctx, err := tracer.Extract(&ddCarrierAdapter{carrier})
	if err != nil {
		if err == tracer.ErrSpanContextNotFound {
			return nil, ErrSpanContextNotFound
		}
		return nil, err
	}
	return &ddSpanContext{inner: ctx}, nil
}

func (p *datadogProvider) convertStartOpts(cfg *startSpanConfig) []tracer.StartSpanOption {
	var opts []tracer.StartSpanOption

	if cfg.ResourceName != "" {
		opts = append(opts, tracer.ResourceName(cfg.ResourceName))
	}

	if cfg.Parent != nil {
		if dc, ok := cfg.Parent.(*ddSpanContext); ok {
			opts = append(opts, tracer.ChildOf(dc.inner))
		}
	}

	for k, v := range cfg.Tags {
		opts = append(opts, tracer.Tag(k, v))
	}

	return opts
}

// ddSpan wraps a Datadog span to implement the Span interface.
type ddSpan struct {
	inner ddtrace.Span
}

func (s *ddSpan) SetTag(key string, value interface{}) {
	s.inner.SetTag(key, value)
}

func (s *ddSpan) Finish(opts ...FinishOption) {
	cfg := applyFinishOpts(opts)
	if cfg.Error != nil {
		s.inner.Finish(tracer.WithError(cfg.Error))
	} else {
		s.inner.Finish()
	}
}

func (s *ddSpan) Context() SpanContext {
	return &ddSpanContext{inner: s.inner.Context()}
}

// Inner returns the underlying Datadog span for cases where direct access is needed.
// This should be used sparingly and only when the Datadog-specific API is required.
func (s *ddSpan) Inner() ddtrace.Span {
	return s.inner
}

// ddSpanContext wraps a Datadog span context to implement the SpanContext interface.
type ddSpanContext struct {
	inner ddtrace.SpanContext
}

func (c *ddSpanContext) TraceID() string {
	return fmt.Sprintf("%d", c.inner.TraceID())
}

func (c *ddSpanContext) SpanID() string {
	return fmt.Sprintf("%d", c.inner.SpanID())
}

func (c *ddSpanContext) TraceIDUint64() uint64 {
	return c.inner.TraceID()
}

func (c *ddSpanContext) SpanIDUint64() uint64 {
	return c.inner.SpanID()
}

// Inner returns the underlying Datadog span context for cases where direct access is needed.
func (c *ddSpanContext) Inner() ddtrace.SpanContext {
	return c.inner
}

// ddCarrierAdapter adapts our TextMapCarrier to Datadog's TextMapWriter/TextMapReader.
type ddCarrierAdapter struct {
	carrier TextMapCarrier
}

func (a *ddCarrierAdapter) Set(key, val string) {
	a.carrier.Set(key, val)
}

func (a *ddCarrierAdapter) ForeachKey(handler func(key, val string) error) error {
	return a.carrier.ForeachKey(handler)
}

// Ensure ddCarrierAdapter implements Datadog's interfaces.
var _ tracer.TextMapWriter = (*ddCarrierAdapter)(nil)
var _ tracer.TextMapReader = (*ddCarrierAdapter)(nil)

// AsDatadogSpan extracts the underlying Datadog span from an htelemetry.Span.
// Returns nil if the span is not a Datadog span.
func AsDatadogSpan(span Span) ddtrace.Span {
	if ds, ok := span.(*ddSpan); ok {
		return ds.inner
	}
	return nil
}

// AsDatadogSpanContext extracts the underlying Datadog span context from an htelemetry.SpanContext.
// Returns nil if the span context is not a Datadog span context.
func AsDatadogSpanContext(ctx SpanContext) ddtrace.SpanContext {
	if dc, ok := ctx.(*ddSpanContext); ok {
		return dc.inner
	}
	return nil
}

// WrapDatadogSpan wraps a Datadog span as an htelemetry.Span.
// This is useful when you have a Datadog span from external code and need to use it
// with htelemetry functions.
func WrapDatadogSpan(span ddtrace.Span) Span {
	if span == nil {
		return nil
	}
	return &ddSpan{inner: span}
}
