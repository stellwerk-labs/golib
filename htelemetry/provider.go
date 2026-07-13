// Package htelemetry provides a backend-agnostic abstraction for distributed tracing.
// It supports both Datadog and OpenTelemetry backends, allowing services to choose
// their telemetry provider at startup without changing library code.
//
// By default, the Datadog provider is used for backward compatibility.
// To switch to OpenTelemetry, call SetProvider with an OTel provider at startup.
package htelemetry

import (
	"context"
	"errors"
	"sync/atomic"
)

// ErrSpanContextNotFound is returned when no span context is found in a carrier.
var ErrSpanContextNotFound = errors.New("span context not found")

// Span represents a backend-agnostic tracing span.
type Span interface {
	// SetTag sets a key-value tag on the span.
	SetTag(key string, value interface{})

	// Finish completes the span with optional finish options.
	Finish(opts ...FinishOption)

	// Context returns the span's context for propagation.
	Context() SpanContext
}

// SpanContext provides trace and span identifiers for propagation and logging.
type SpanContext interface {
	// TraceID returns the trace ID as a string.
	// For Datadog, this is the decimal representation of a 64-bit uint.
	// For OpenTelemetry, this is a 32-character hex string (128-bit).
	TraceID() string

	// SpanID returns the span ID as a string.
	SpanID() string

	// TraceIDUint64 returns the trace ID as a uint64.
	// For Datadog, this is the native 64-bit trace ID.
	// For OpenTelemetry, this returns the lower 64 bits of the 128-bit trace ID,
	// which is compatible with Datadog log correlation.
	TraceIDUint64() uint64

	// SpanIDUint64 returns the span ID as a uint64.
	// Both Datadog and OpenTelemetry use 64-bit span IDs natively.
	SpanIDUint64() uint64
}

// FinishOption modifies span finish behavior.
type FinishOption interface {
	applyFinish(*finishConfig)
}

type finishConfig struct {
	Error error
}

type finishOptionFunc func(*finishConfig)

func (f finishOptionFunc) applyFinish(c *finishConfig) { f(c) }

// WithError marks the span as having an error.
func WithError(err error) FinishOption {
	return finishOptionFunc(func(c *finishConfig) {
		c.Error = err
	})
}

// StartSpanOption configures a new span.
type StartSpanOption interface {
	applyStartSpan(*startSpanConfig)
}

type startSpanConfig struct {
	ResourceName string
	Parent       SpanContext
	Tags         map[string]interface{}
}

type startSpanOptionFunc func(*startSpanConfig)

func (f startSpanOptionFunc) applyStartSpan(c *startSpanConfig) { f(c) }

// ResourceName sets the resource name for the span (used by Datadog for grouping).
func ResourceName(name string) StartSpanOption {
	return startSpanOptionFunc(func(c *startSpanConfig) {
		c.ResourceName = name
	})
}

// ChildOf creates a child span of the given parent context.
func ChildOf(parent SpanContext) StartSpanOption {
	return startSpanOptionFunc(func(c *startSpanConfig) {
		c.Parent = parent
	})
}

// Tag sets a tag on the span at creation time.
func Tag(key string, value interface{}) StartSpanOption {
	return startSpanOptionFunc(func(c *startSpanConfig) {
		if c.Tags == nil {
			c.Tags = make(map[string]interface{})
		}
		c.Tags[key] = value
	})
}

// TextMapCarrier is the interface for injecting/extracting trace context.
type TextMapCarrier interface {
	// Set sets a key-value pair.
	Set(key, val string)

	// ForeachKey iterates over all key-value pairs.
	ForeachKey(handler func(key, val string) error) error
}

// Provider defines the interface for a telemetry backend.
type Provider interface {
	// StartSpan starts a new span with the given operation name.
	StartSpan(operationName string, opts ...StartSpanOption) Span

	// StartSpanFromContext starts a new span as a child of any span in the context.
	StartSpanFromContext(ctx context.Context, operationName string, opts ...StartSpanOption) (Span, context.Context)

	// SpanFromContext extracts a span from the context.
	SpanFromContext(ctx context.Context) (Span, bool)

	// ContextWithSpan returns a new context with the span attached.
	ContextWithSpan(ctx context.Context, span Span) context.Context

	// Inject injects the span context into a carrier for propagation.
	Inject(spanCtx SpanContext, carrier TextMapCarrier) error

	// Extract extracts a span context from a carrier.
	Extract(carrier TextMapCarrier) (SpanContext, error)
}

// providerHolder wraps a Provider so that atomic.Value always stores the same
// concrete type, regardless of the underlying Provider implementation.
type providerHolder struct {
	p Provider
}

// globalProvider holds the current provider (default: Datadog).
// Stored in an atomic.Value to prevent data races when reading/writing concurrently.
var globalProvider atomic.Value

func init() {
	globalProvider.Store(providerHolder{p: &datadogProvider{}})
}

// SetProvider sets the global telemetry provider.
// This should be called once at application startup before any tracing operations.
// Passing nil will panic.
func SetProvider(p Provider) {
	globalProvider.Store(providerHolder{p: p})
}

// GetProvider returns the current global telemetry provider.
func GetProvider() Provider {
	return globalProvider.Load().(providerHolder).p
}

// IsOTel returns true if the global telemetry provider is OpenTelemetry-based.
func IsOTel() bool {
	_, ok := GetProvider().(*otelProvider)
	return ok
}

// StartSpan starts a new span using the global provider.
func StartSpan(operationName string, opts ...StartSpanOption) Span {
	return GetProvider().StartSpan(operationName, opts...)
}

// StartSpanFromContext starts a new span as a child of any span in the context.
func StartSpanFromContext(ctx context.Context, operationName string, opts ...StartSpanOption) (Span, context.Context) {
	return GetProvider().StartSpanFromContext(ctx, operationName, opts...)
}

// SpanFromContext extracts a span from the context using the global provider.
func SpanFromContext(ctx context.Context) (Span, bool) {
	return GetProvider().SpanFromContext(ctx)
}

// ContextWithSpan returns a new context with the span attached.
func ContextWithSpan(ctx context.Context, span Span) context.Context {
	return GetProvider().ContextWithSpan(ctx, span)
}

// Inject injects the span context into a carrier for propagation.
func Inject(spanCtx SpanContext, carrier TextMapCarrier) error {
	return GetProvider().Inject(spanCtx, carrier)
}

// Extract extracts a span context from a carrier.
func Extract(carrier TextMapCarrier) (SpanContext, error) {
	return GetProvider().Extract(carrier)
}

// applyStartOpts applies all start span options to a config.
func applyStartOpts(opts []StartSpanOption) *startSpanConfig {
	cfg := &startSpanConfig{}
	for _, opt := range opts {
		opt.applyStartSpan(cfg)
	}
	return cfg
}

// applyFinishOpts applies all finish options to a config.
func applyFinishOpts(opts []FinishOption) *finishConfig {
	cfg := &finishConfig{}
	for _, opt := range opts {
		opt.applyFinish(cfg)
	}
	return cfg
}
