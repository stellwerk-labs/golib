package htelemetry

import (
	"context"
	"errors"
	"testing"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/mocktracer"
)

func TestWithError(t *testing.T) {
	testErr := errors.New("test error")
	opt := WithError(testErr)

	cfg := &finishConfig{}
	opt.applyFinish(cfg)

	if cfg.Error != testErr {
		t.Errorf("WithError: expected %v, got %v", testErr, cfg.Error)
	}
}

func TestResourceName(t *testing.T) {
	name := "test-resource"
	opt := ResourceName(name)

	cfg := &startSpanConfig{}
	opt.applyStartSpan(cfg)

	if cfg.ResourceName != name {
		t.Errorf("ResourceName: expected %q, got %q", name, cfg.ResourceName)
	}
}

func TestChildOf(t *testing.T) {
	parent := &mockSpanContext{traceID: "123", spanID: "456"}
	opt := ChildOf(parent)

	cfg := &startSpanConfig{}
	opt.applyStartSpan(cfg)

	if cfg.Parent != parent {
		t.Errorf("ChildOf: expected parent to be set")
	}
}

func TestTag(t *testing.T) {
	opt := Tag("key1", "value1")

	cfg := &startSpanConfig{}
	opt.applyStartSpan(cfg)

	if cfg.Tags == nil {
		t.Fatal("Tag: expected Tags map to be initialized")
	}
	if cfg.Tags["key1"] != "value1" {
		t.Errorf("Tag: expected Tags[key1]=%q, got %q", "value1", cfg.Tags["key1"])
	}
}

func TestTagMultiple(t *testing.T) {
	opts := []StartSpanOption{
		Tag("string", "value"),
		Tag("int", 42),
		Tag("bool", true),
	}

	cfg := applyStartOpts(opts)

	if cfg.Tags["string"] != "value" {
		t.Errorf("Tag: expected string tag")
	}
	if cfg.Tags["int"] != 42 {
		t.Errorf("Tag: expected int tag")
	}
	if cfg.Tags["bool"] != true {
		t.Errorf("Tag: expected bool tag")
	}
}

func TestApplyStartOpts(t *testing.T) {
	parent := &mockSpanContext{traceID: "123", spanID: "456"}
	opts := []StartSpanOption{
		ResourceName("my-resource"),
		ChildOf(parent),
	}

	cfg := applyStartOpts(opts)

	if cfg.ResourceName != "my-resource" {
		t.Errorf("applyStartOpts: expected ResourceName %q, got %q", "my-resource", cfg.ResourceName)
	}
	if cfg.Parent != parent {
		t.Errorf("applyStartOpts: expected Parent to be set")
	}
}

func TestApplyFinishOpts(t *testing.T) {
	testErr := errors.New("finish error")
	opts := []FinishOption{
		WithError(testErr),
	}

	cfg := applyFinishOpts(opts)

	if cfg.Error != testErr {
		t.Errorf("applyFinishOpts: expected Error %v, got %v", testErr, cfg.Error)
	}
}

func TestSetAndGetProvider(t *testing.T) {
	// Save and restore original provider
	original := GetProvider()
	defer SetProvider(original)

	// Verify we have a default provider
	if original == nil {
		t.Error("GetProvider: expected non-nil default provider")
	}

	// Verify it's a datadogProvider (the default)
	_, isDatadog := original.(*datadogProvider)
	if !isDatadog {
		t.Errorf("GetProvider: expected *datadogProvider as default, got %T", original)
	}

	// Test SetProvider with a mock provider
	mock := &mockProvider{}
	SetProvider(mock)

	got := GetProvider()
	if got != mock {
		t.Errorf("SetProvider/GetProvider: expected mock provider, got %T", got)
	}
}

func TestGlobalFunctions(t *testing.T) {
	// Test with the default datadog provider using a mock tracer
	mt := mocktracer.Start()
	defer mt.Stop()

	ctx := context.Background()

	// Test StartSpan
	span := StartSpan("test-op")
	if span == nil {
		t.Error("StartSpan: expected non-nil span")
	}

	// Test StartSpanFromContext
	span2, newCtx := StartSpanFromContext(ctx, "test-op-2")
	if span2 == nil {
		t.Error("StartSpanFromContext: expected non-nil span")
	}
	if newCtx == nil {
		t.Error("StartSpanFromContext: expected non-nil context")
	}

	// Test SpanFromContext - should find the span we just created
	foundSpan, ok := SpanFromContext(newCtx)
	if !ok {
		t.Error("SpanFromContext: expected ok=true for context with span")
	}
	if foundSpan == nil {
		t.Error("SpanFromContext: expected non-nil span")
	}

	// Test ContextWithSpan
	newCtx2 := ContextWithSpan(ctx, span)
	if newCtx2 == nil {
		t.Error("ContextWithSpan: expected non-nil context")
	}

	// Test Inject
	carrier := &mockCarrier{data: make(map[string]string)}
	spanCtx := span.Context()
	err := Inject(spanCtx, carrier)
	if err != nil {
		t.Errorf("Inject: unexpected error: %v", err)
	}

	// Test Extract
	extractedCtx, err := Extract(carrier)
	if err != nil {
		t.Errorf("Extract: unexpected error: %v", err)
	}
	if extractedCtx == nil {
		t.Error("Extract: expected non-nil span context")
	}

	// Clean up spans
	span.Finish()
	span2.Finish()
}

// mockProvider is a mock implementation of Provider for testing.
type mockProvider struct {
	startSpanCalled            int
	startSpanFromContextCalled int
	spanFromContextCalled      int
	contextWithSpanCalled      int
	injectCalled               int
	extractCalled              int
}

func (m *mockProvider) StartSpan(operationName string, opts ...StartSpanOption) Span {
	m.startSpanCalled++
	return &mockSpan{}
}

func (m *mockProvider) StartSpanFromContext(ctx context.Context, operationName string, opts ...StartSpanOption) (Span, context.Context) {
	m.startSpanFromContextCalled++
	return &mockSpan{}, ctx
}

func (m *mockProvider) SpanFromContext(ctx context.Context) (Span, bool) {
	m.spanFromContextCalled++
	return nil, false
}

func (m *mockProvider) ContextWithSpan(ctx context.Context, span Span) context.Context {
	m.contextWithSpanCalled++
	return ctx
}

func (m *mockProvider) Inject(spanCtx SpanContext, carrier TextMapCarrier) error {
	m.injectCalled++
	return nil
}

func (m *mockProvider) Extract(carrier TextMapCarrier) (SpanContext, error) {
	m.extractCalled++
	return nil, ErrSpanContextNotFound
}

// mockSpan is a mock implementation of Span for testing.
type mockSpan struct {
	tags     map[string]interface{}
	finished bool
	err      error
}

func (s *mockSpan) SetTag(key string, value interface{}) {
	if s.tags == nil {
		s.tags = make(map[string]interface{})
	}
	s.tags[key] = value
}

func (s *mockSpan) Finish(opts ...FinishOption) {
	s.finished = true
	cfg := applyFinishOpts(opts)
	s.err = cfg.Error
}

func (s *mockSpan) Context() SpanContext {
	return &mockSpanContext{traceID: "mock-trace", spanID: "mock-span"}
}

// mockSpanContext is a mock implementation of SpanContext for testing.
type mockSpanContext struct {
	traceID string
	spanID  string
}

func (c *mockSpanContext) TraceID() string     { return c.traceID }
func (c *mockSpanContext) SpanID() string      { return c.spanID }
func (c *mockSpanContext) TraceIDUint64() uint64 { return 0 }
func (c *mockSpanContext) SpanIDUint64() uint64  { return 0 }

// mockCarrier is a mock implementation of TextMapCarrier for testing.
type mockCarrier struct {
	data map[string]string
}

func (c *mockCarrier) Set(key, val string) {
	if c.data == nil {
		c.data = make(map[string]string)
	}
	c.data[key] = val
}

func (c *mockCarrier) ForeachKey(handler func(key, val string) error) error {
	for k, v := range c.data {
		if err := handler(k, v); err != nil {
			return err
		}
	}
	return nil
}
