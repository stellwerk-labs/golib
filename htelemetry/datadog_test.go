package htelemetry

import (
	"context"
	"errors"
	"testing"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/mocktracer"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func setupDatadogProvider(t *testing.T) (*datadogProvider, mocktracer.Tracer) {
	t.Helper()

	mt := mocktracer.Start()
	t.Cleanup(func() {
		mt.Stop()
	})

	return &datadogProvider{}, mt
}

func TestDatadogProvider_StartSpan(t *testing.T) {
	provider, mt := setupDatadogProvider(t)

	span := provider.StartSpan("test-operation")
	if span == nil {
		t.Fatal("StartSpan returned nil")
	}

	span.Finish()

	spans := mt.FinishedSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].OperationName() != "test-operation" {
		t.Errorf("expected operation name %q, got %q", "test-operation", spans[0].OperationName())
	}
}

func TestDatadogProvider_StartSpanWithResourceName(t *testing.T) {
	provider, mt := setupDatadogProvider(t)

	span := provider.StartSpan("test-operation", ResourceName("my-resource"))
	span.Finish()

	spans := mt.FinishedSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	if spans[0].Tag("resource.name") != "my-resource" {
		t.Errorf("expected resource.name %q, got %q", "my-resource", spans[0].Tag("resource.name"))
	}
}

func TestDatadogProvider_StartSpanWithTag(t *testing.T) {
	provider, mt := setupDatadogProvider(t)

	span := provider.StartSpan("test-operation", Tag("env", "test"), Tag("version", 123))
	span.Finish()

	spans := mt.FinishedSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	if spans[0].Tag("env") != "test" {
		t.Errorf("expected tag env=test, got %v", spans[0].Tag("env"))
	}
	if spans[0].Tag("version") == nil {
		t.Error("expected tag version to be set")
	}
}

func TestDatadogProvider_StartSpanFromContext(t *testing.T) {
	provider, mt := setupDatadogProvider(t)

	ctx := context.Background()
	span, newCtx := provider.StartSpanFromContext(ctx, "parent-operation")
	if span == nil {
		t.Fatal("StartSpanFromContext returned nil span")
	}
	if newCtx == nil {
		t.Fatal("StartSpanFromContext returned nil context")
	}

	// Start a child span
	childSpan, _ := provider.StartSpanFromContext(newCtx, "child-operation")
	childSpan.Finish()
	span.Finish()

	spans := mt.FinishedSpans()
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}

	// Verify parent-child relationship
	var parentSpan, childSpanFinished mocktracer.Span
	for _, s := range spans {
		if s.OperationName() == "parent-operation" {
			parentSpan = s
		} else if s.OperationName() == "child-operation" {
			childSpanFinished = s
		}
	}

	if parentSpan == nil || childSpanFinished == nil {
		t.Fatal("could not find parent or child span")
	}

	if childSpanFinished.ParentID() != parentSpan.SpanID() {
		t.Error("child span parent ID does not match parent span ID")
	}
}

func TestDatadogProvider_SpanFromContext(t *testing.T) {
	provider, _ := setupDatadogProvider(t)

	// Test with no span in context
	ctx := context.Background()
	span, ok := provider.SpanFromContext(ctx)
	if ok {
		t.Error("SpanFromContext: expected ok=false for empty context")
	}
	if span != nil {
		t.Error("SpanFromContext: expected nil span for empty context")
	}

	// Test with span in context
	_, ctxWithSpan := provider.StartSpanFromContext(ctx, "test-op")
	span, ok = provider.SpanFromContext(ctxWithSpan)
	if !ok {
		t.Error("SpanFromContext: expected ok=true for context with span")
	}
	if span == nil {
		t.Error("SpanFromContext: expected non-nil span for context with span")
	}
}

func TestDatadogProvider_ContextWithSpan(t *testing.T) {
	provider, _ := setupDatadogProvider(t)

	ctx := context.Background()
	span := provider.StartSpan("test-op")

	newCtx := provider.ContextWithSpan(ctx, span)
	if newCtx == nil {
		t.Fatal("ContextWithSpan returned nil")
	}

	// Verify we can extract the span back
	extractedSpan, ok := provider.SpanFromContext(newCtx)
	if !ok {
		t.Error("expected to extract span from context")
	}
	if extractedSpan == nil {
		t.Error("extracted span is nil")
	}
}

func TestDatadogProvider_ContextWithSpan_NonDDSpan(t *testing.T) {
	provider, _ := setupDatadogProvider(t)

	ctx := context.Background()
	mockSpan := &mockSpan{}

	// Should return original context when given non-DD span
	newCtx := provider.ContextWithSpan(ctx, mockSpan)
	if newCtx != ctx {
		t.Error("ContextWithSpan: expected original context for non-DD span")
	}
}

func TestDatadogProvider_InjectExtract(t *testing.T) {
	provider, _ := setupDatadogProvider(t)

	span := provider.StartSpan("test-op")
	spanCtx := span.Context()

	// Inject
	carrier := &mockCarrier{data: make(map[string]string)}
	err := provider.Inject(spanCtx, carrier)
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	// Verify something was injected
	if len(carrier.data) == 0 {
		t.Error("Inject: expected carrier to have data")
	}

	// Extract
	extractedCtx, err := provider.Extract(carrier)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Verify trace IDs match
	if extractedCtx.TraceID() != spanCtx.TraceID() {
		t.Errorf("Extract: trace ID mismatch: got %s, want %s", extractedCtx.TraceID(), spanCtx.TraceID())
	}
}

func TestDatadogProvider_Inject_NonDDContext(t *testing.T) {
	provider, _ := setupDatadogProvider(t)

	mockCtx := &mockSpanContext{traceID: "123", spanID: "456"}
	carrier := &mockCarrier{data: make(map[string]string)}

	err := provider.Inject(mockCtx, carrier)
	if err == nil {
		t.Error("Inject: expected error for non-DD span context")
	}
}

func TestDatadogProvider_Extract_Empty(t *testing.T) {
	provider, _ := setupDatadogProvider(t)

	carrier := &mockCarrier{data: make(map[string]string)}
	_, err := provider.Extract(carrier)
	if !errors.Is(err, ErrSpanContextNotFound) {
		t.Errorf("Extract: expected ErrSpanContextNotFound, got %v", err)
	}
}

func TestDDSpan_SetTag(t *testing.T) {
	provider, mt := setupDatadogProvider(t)

	span := provider.StartSpan("test-op")

	span.SetTag("string-key", "string-value")
	span.SetTag("int-key", 42)
	span.SetTag("bool-key", true)

	span.Finish()

	spans := mt.FinishedSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	if v, ok := spans[0].Tag("string-key").(string); !ok || v != "string-value" {
		t.Errorf("expected tag string-key=%q, got %v", "string-value", spans[0].Tag("string-key"))
	}
	if spans[0].Tag("int-key") == nil {
		t.Error("expected tag int-key to be set")
	}
	if spans[0].Tag("bool-key") == nil {
		t.Error("expected tag bool-key to be set")
	}
}

func TestDDSpan_FinishWithError(t *testing.T) {
	provider, mt := setupDatadogProvider(t)

	span := provider.StartSpan("test-op")
	testErr := errors.New("test error")
	span.Finish(WithError(testErr))

	spans := mt.FinishedSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	// Check that error was recorded
	errTag := spans[0].Tag("error")
	if errTag == nil {
		t.Error("expected error tag to be set")
	}
}

func TestDDSpanContext_IDs(t *testing.T) {
	provider, _ := setupDatadogProvider(t)

	span := provider.StartSpan("test-op")
	ctx := span.Context()

	traceID := ctx.TraceID()
	spanID := ctx.SpanID()

	if traceID == "" {
		t.Error("TraceID returned empty string")
	}
	if spanID == "" {
		t.Error("SpanID returned empty string")
	}

	// DD trace IDs are decimal strings
	traceIDUint := ctx.TraceIDUint64()
	spanIDUint := ctx.SpanIDUint64()

	if traceIDUint == 0 {
		t.Error("TraceIDUint64 returned 0")
	}
	if spanIDUint == 0 {
		t.Error("SpanIDUint64 returned 0")
	}
}

func TestAsDatadogSpan(t *testing.T) {
	provider, _ := setupDatadogProvider(t)

	// Test with DD span
	span := provider.StartSpan("test-op")
	ddSpan := AsDatadogSpan(span)
	if ddSpan == nil {
		t.Error("AsDatadogSpan: expected non-nil for DD span")
	}

	// Test with non-DD span
	mockSpan := &mockSpan{}
	ddSpan = AsDatadogSpan(mockSpan)
	if ddSpan != nil {
		t.Error("AsDatadogSpan: expected nil for non-DD span")
	}
}

func TestAsDatadogSpanContext(t *testing.T) {
	provider, _ := setupDatadogProvider(t)

	// Test with DD span context
	span := provider.StartSpan("test-op")
	ctx := span.Context()
	ddCtx := AsDatadogSpanContext(ctx)
	if ddCtx == nil {
		t.Error("AsDatadogSpanContext: expected non-nil for DD span context")
	}

	// Test with non-DD span context
	mockCtx := &mockSpanContext{}
	ddCtx = AsDatadogSpanContext(mockCtx)
	if ddCtx != nil {
		t.Error("AsDatadogSpanContext: expected nil for non-DD span context")
	}
}

func TestWrapDatadogSpan(t *testing.T) {
	// Test nil span
	wrappedNil := WrapDatadogSpan(nil)
	if wrappedNil != nil {
		t.Error("WrapDatadogSpan: expected nil for nil input")
	}

	// Test with actual DD span
	mt := mocktracer.Start()
	defer mt.Stop()

	ddSpan := tracer.StartSpan("test-op")
	wrapped := WrapDatadogSpan(ddSpan)
	if wrapped == nil {
		t.Error("WrapDatadogSpan: expected non-nil for valid span")
	}

	// Verify the wrapped span works
	wrapped.SetTag("test", "value")
	wrapped.Finish()
}

func TestDDSpan_Inner(t *testing.T) {
	provider, _ := setupDatadogProvider(t)

	span := provider.StartSpan("test-op")
	ddSpan, ok := span.(*ddSpan)
	if !ok {
		t.Fatal("expected *ddSpan")
	}

	inner := ddSpan.Inner()
	if inner == nil {
		t.Error("Inner: expected non-nil")
	}
}

func TestDDSpanContext_Inner(t *testing.T) {
	provider, _ := setupDatadogProvider(t)

	span := provider.StartSpan("test-op")
	ctx := span.Context()
	ddCtx, ok := ctx.(*ddSpanContext)
	if !ok {
		t.Fatal("expected *ddSpanContext")
	}

	inner := ddCtx.Inner()
	if inner == nil {
		t.Error("Inner: expected non-nil")
	}
}

func TestDDCarrierAdapter(t *testing.T) {
	carrier := &mockCarrier{data: make(map[string]string)}
	adapter := &ddCarrierAdapter{carrier: carrier}

	// Test Set
	adapter.Set("key1", "value1")
	adapter.Set("key2", "value2")

	if carrier.data["key1"] != "value1" {
		t.Errorf("Set: expected key1=%q, got %q", "value1", carrier.data["key1"])
	}

	// Test ForeachKey
	keys := make(map[string]string)
	err := adapter.ForeachKey(func(k, v string) error {
		keys[k] = v
		return nil
	})
	if err != nil {
		t.Errorf("ForeachKey: unexpected error: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("ForeachKey: expected 2 keys, got %d", len(keys))
	}

	// Test ForeachKey with error
	expectedErr := errors.New("handler error")
	err = adapter.ForeachKey(func(k, v string) error {
		return expectedErr
	})
	if err != expectedErr {
		t.Errorf("ForeachKey: expected error %v, got %v", expectedErr, err)
	}
}

func TestDatadogProvider_StartSpanWithParent(t *testing.T) {
	provider, mt := setupDatadogProvider(t)

	// Create parent span
	parentSpan := provider.StartSpan("parent-op")
	parentCtx := parentSpan.Context()

	// Create child span with ChildOf option
	childSpan := provider.StartSpan("child-op", ChildOf(parentCtx))
	childSpan.Finish()
	parentSpan.Finish()

	spans := mt.FinishedSpans()
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}

	// Find the child span and verify parent relationship
	var foundChild, foundParent mocktracer.Span
	for _, s := range spans {
		if s.OperationName() == "child-op" {
			foundChild = s
		} else if s.OperationName() == "parent-op" {
			foundParent = s
		}
	}

	if foundChild == nil || foundParent == nil {
		t.Fatal("could not find child or parent span")
	}

	if foundChild.ParentID() != foundParent.SpanID() {
		t.Errorf("child parent ID (%d) != parent span ID (%d)", foundChild.ParentID(), foundParent.SpanID())
	}
}
