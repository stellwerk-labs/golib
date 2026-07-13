package htelemetry

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func setupOTelProvider(t *testing.T) (*otelProvider, *tracetest.InMemoryExporter) {
	t.Helper()

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)

	// Set up propagator
	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)

	provider := &otelProvider{
		tracer:     tp.Tracer("test-tracer"),
		propagator: propagator,
	}

	return provider, exporter
}

func TestNewOTelProvider(t *testing.T) {
	// Set up a global tracer provider for this test
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	originalTP := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(originalTP)

	provider := NewOTelProvider("test-service")
	if provider == nil {
		t.Fatal("NewOTelProvider returned nil")
	}

	// Verify it's the right type
	_, ok := provider.(*otelProvider)
	if !ok {
		t.Errorf("NewOTelProvider: expected *otelProvider, got %T", provider)
	}
}

func TestNewOTelProviderWithTracer(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("custom-tracer")

	// Test with nil propagator (should use global)
	provider := NewOTelProviderWithTracer(tracer, nil)
	if provider == nil {
		t.Fatal("NewOTelProviderWithTracer returned nil")
	}

	// Test with custom propagator
	propagator := propagation.TraceContext{}
	provider2 := NewOTelProviderWithTracer(tracer, propagator)
	if provider2 == nil {
		t.Fatal("NewOTelProviderWithTracer with propagator returned nil")
	}
}

func TestOTelProvider_StartSpan(t *testing.T) {
	provider, exporter := setupOTelProvider(t)

	span := provider.StartSpan("test-operation")
	if span == nil {
		t.Fatal("StartSpan returned nil")
	}

	span.Finish()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Name != "test-operation" {
		t.Errorf("expected span name %q, got %q", "test-operation", spans[0].Name)
	}
}

func TestOTelProvider_StartSpanWithResourceName(t *testing.T) {
	provider, exporter := setupOTelProvider(t)

	span := provider.StartSpan("test-operation", ResourceName("my-resource"))
	span.Finish()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	// Check that resource.name attribute was set
	found := false
	for _, attr := range spans[0].Attributes {
		if string(attr.Key) == "resource.name" && attr.Value.AsString() == "my-resource" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected resource.name attribute to be set")
	}
}

func TestOTelProvider_StartSpanWithTag(t *testing.T) {
	provider, exporter := setupOTelProvider(t)

	span := provider.StartSpan("test-operation", Tag("env", "test"), Tag("version", 123))
	span.Finish()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	// Check that tags were set as attributes
	foundEnv := false
	foundVersion := false
	for _, attr := range spans[0].Attributes {
		if string(attr.Key) == "env" && attr.Value.AsString() == "test" {
			foundEnv = true
		}
		if string(attr.Key) == "version" && attr.Value.AsInt64() == 123 {
			foundVersion = true
		}
	}
	if !foundEnv {
		t.Error("expected env attribute to be set")
	}
	if !foundVersion {
		t.Error("expected version attribute to be set")
	}
}

func TestOTelProvider_StartSpanFromContext(t *testing.T) {
	provider, exporter := setupOTelProvider(t)

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

	spans := exporter.GetSpans()
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}
}

func TestOTelProvider_SpanFromContext(t *testing.T) {
	provider, _ := setupOTelProvider(t)

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

func TestOTelProvider_ContextWithSpan(t *testing.T) {
	provider, _ := setupOTelProvider(t)

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

func TestOTelProvider_ContextWithSpan_NonOTelSpan(t *testing.T) {
	provider, _ := setupOTelProvider(t)

	ctx := context.Background()
	mockSpan := &mockSpan{}

	// Should return original context when given non-OTel span
	newCtx := provider.ContextWithSpan(ctx, mockSpan)
	if newCtx != ctx {
		t.Error("ContextWithSpan: expected original context for non-OTel span")
	}
}

func TestOTelProvider_InjectExtract(t *testing.T) {
	provider, _ := setupOTelProvider(t)

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

func TestOTelProvider_Inject_NonOTelContext(t *testing.T) {
	provider, _ := setupOTelProvider(t)

	mockCtx := &mockSpanContext{traceID: "123", spanID: "456"}
	carrier := &mockCarrier{data: make(map[string]string)}

	err := provider.Inject(mockCtx, carrier)
	if err == nil {
		t.Error("Inject: expected error for non-OTel span context")
	}
}

func TestOTelProvider_Extract_Empty(t *testing.T) {
	provider, _ := setupOTelProvider(t)

	carrier := &mockCarrier{data: make(map[string]string)}
	_, err := provider.Extract(carrier)
	if !errors.Is(err, ErrSpanContextNotFound) {
		t.Errorf("Extract: expected ErrSpanContextNotFound, got %v", err)
	}
}

func TestOTelSpan_SetTag(t *testing.T) {
	provider, exporter := setupOTelProvider(t)

	span := provider.StartSpan("test-op")

	// Test various tag types
	span.SetTag("string-key", "string-value")
	span.SetTag("int-key", 42)
	span.SetTag("int64-key", int64(100))
	span.SetTag("float-key", 3.14)
	span.SetTag("bool-key", true)
	span.SetTag("other-key", struct{ Name string }{"test"})

	span.Finish()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	// Verify attributes were set
	attrs := spans[0].Attributes
	if len(attrs) < 5 {
		t.Errorf("expected at least 5 attributes, got %d", len(attrs))
	}
}

func TestOTelSpan_FinishWithError(t *testing.T) {
	provider, exporter := setupOTelProvider(t)

	span := provider.StartSpan("test-op")
	testErr := errors.New("test error")
	span.Finish(WithError(testErr))

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	// Check that error was recorded
	if spans[0].Status.Code.String() != "Error" {
		t.Errorf("expected span status to be Error, got %s", spans[0].Status.Code.String())
	}
}

func TestOTelSpanContext_IDs(t *testing.T) {
	provider, _ := setupOTelProvider(t)

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

	// OTel trace IDs are 32 hex chars
	if len(traceID) != 32 {
		t.Errorf("TraceID: expected 32 chars, got %d", len(traceID))
	}

	// OTel span IDs are 16 hex chars
	if len(spanID) != 16 {
		t.Errorf("SpanID: expected 16 chars, got %d", len(spanID))
	}

	// Uint64 methods should return non-zero values
	if ctx.TraceIDUint64() == 0 {
		t.Error("TraceIDUint64 returned 0")
	}
	if ctx.SpanIDUint64() == 0 {
		t.Error("SpanIDUint64 returned 0")
	}
}

func TestTraceIDToDatadog(t *testing.T) {
	tests := []struct {
		name     string
		traceID  trace.TraceID
		expected uint64
	}{
		{
			name:     "all zeros",
			traceID:  trace.TraceID{},
			expected: 0,
		},
		{
			name:     "lower 64 bits set",
			traceID:  trace.TraceID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			expected: 1,
		},
		{
			name:     "upper bits ignored",
			traceID:  trace.TraceID{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0, 0, 0, 0, 0, 0, 0, 1},
			expected: 1,
		},
		{
			name:     "max lower 64 bits",
			traceID:  trace.TraceID{0, 0, 0, 0, 0, 0, 0, 0, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			expected: 0xFFFFFFFFFFFFFFFF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TraceIDToDatadog(tt.traceID)
			if result != tt.expected {
				t.Errorf("TraceIDToDatadog: expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestDatadogToTraceID(t *testing.T) {
	tests := []struct {
		name      string
		ddTraceID uint64
		expected  trace.TraceID
	}{
		{
			name:      "zero",
			ddTraceID: 0,
			expected:  trace.TraceID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:      "one",
			ddTraceID: 1,
			expected:  trace.TraceID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		},
		{
			name:      "max",
			ddTraceID: 0xFFFFFFFFFFFFFFFF,
			expected:  trace.TraceID{0, 0, 0, 0, 0, 0, 0, 0, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DatadogToTraceID(tt.ddTraceID)
			if result != tt.expected {
				t.Errorf("DatadogToTraceID: expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDatadogToSpanID(t *testing.T) {
	tests := []struct {
		name     string
		ddSpanID uint64
		expected trace.SpanID
	}{
		{
			name:     "zero",
			ddSpanID: 0,
			expected: trace.SpanID{0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:     "one",
			ddSpanID: 1,
			expected: trace.SpanID{0, 0, 0, 0, 0, 0, 0, 1},
		},
		{
			name:     "max",
			ddSpanID: 0xFFFFFFFFFFFFFFFF,
			expected: trace.SpanID{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DatadogToSpanID(tt.ddSpanID)
			if result != tt.expected {
				t.Errorf("DatadogToSpanID: expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestParseTraceID(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    trace.TraceID
		expectError bool
	}{
		{
			name:     "valid trace ID",
			input:    "00000000000000000000000000000001",
			expected: trace.TraceID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		},
		{
			name:     "valid trace ID with values",
			input:    "0102030405060708090a0b0c0d0e0f10",
			expected: trace.TraceID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
		},
		{
			name:        "invalid hex",
			input:       "not-valid-hex-string-at-all!!!!",
			expectError: true,
		},
		{
			name:        "wrong length",
			input:       "0001",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseTraceID(tt.input)
			if tt.expectError {
				if err == nil {
					t.Error("ParseTraceID: expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("ParseTraceID: unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("ParseTraceID: expected %v, got %v", tt.expected, result)
				}
			}
		})
	}
}

func TestParseSpanID(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    trace.SpanID
		expectError bool
	}{
		{
			name:     "valid span ID",
			input:    "0000000000000001",
			expected: trace.SpanID{0, 0, 0, 0, 0, 0, 0, 1},
		},
		{
			name:     "valid span ID with values",
			input:    "0102030405060708",
			expected: trace.SpanID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		},
		{
			name:        "invalid hex",
			input:       "not-hex!",
			expectError: true,
		},
		{
			name:        "wrong length",
			input:       "0001",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseSpanID(tt.input)
			if tt.expectError {
				if err == nil {
					t.Error("ParseSpanID: expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("ParseSpanID: unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("ParseSpanID: expected %v, got %v", tt.expected, result)
				}
			}
		})
	}
}

func TestAsOTelSpan(t *testing.T) {
	provider, _ := setupOTelProvider(t)

	// Test with OTel span
	span := provider.StartSpan("test-op")
	otelSpan := AsOTelSpan(span)
	if otelSpan == nil {
		t.Error("AsOTelSpan: expected non-nil for OTel span")
	}

	// Test with non-OTel span
	mockSpan := &mockSpan{}
	otelSpan = AsOTelSpan(mockSpan)
	if otelSpan != nil {
		t.Error("AsOTelSpan: expected nil for non-OTel span")
	}
}

func TestAsOTelSpanContext(t *testing.T) {
	provider, _ := setupOTelProvider(t)

	// Test with OTel span context
	span := provider.StartSpan("test-op")
	ctx := span.Context()
	otelCtx := AsOTelSpanContext(ctx)
	if !otelCtx.IsValid() {
		t.Error("AsOTelSpanContext: expected valid span context for OTel span")
	}

	// Test with non-OTel span context
	mockCtx := &mockSpanContext{}
	otelCtx = AsOTelSpanContext(mockCtx)
	if otelCtx.IsValid() {
		t.Error("AsOTelSpanContext: expected invalid span context for non-OTel span")
	}
}

func TestWrapOTelSpan(t *testing.T) {
	// Test nil span
	wrappedNil := WrapOTelSpan(nil)
	if wrappedNil != nil {
		t.Error("WrapOTelSpan: expected nil for nil input")
	}

	// Test with actual OTel span
	provider, _ := setupOTelProvider(t)
	span := provider.StartSpan("test-op")
	otelSpan := AsOTelSpan(span)

	wrapped := WrapOTelSpan(otelSpan)
	if wrapped == nil {
		t.Error("WrapOTelSpan: expected non-nil for valid span")
	}

	// Verify the wrapped span works
	wrapped.SetTag("test", "value")
	wrapped.Finish()
}

func TestOTelSpan_Inner(t *testing.T) {
	provider, _ := setupOTelProvider(t)

	span := provider.StartSpan("test-op")
	otelSpan, ok := span.(*otelSpan)
	if !ok {
		t.Fatal("expected *otelSpan")
	}

	inner := otelSpan.Inner()
	if inner == nil {
		t.Error("Inner: expected non-nil")
	}
}

func TestOTelSpanContext_Inner(t *testing.T) {
	provider, _ := setupOTelProvider(t)

	span := provider.StartSpan("test-op")
	ctx := span.Context()
	otelCtx, ok := ctx.(*otelSpanContext)
	if !ok {
		t.Fatal("expected *otelSpanContext")
	}

	inner := otelCtx.Inner()
	if !inner.IsValid() {
		t.Error("Inner: expected valid span context")
	}
}

func TestOTelCarrierAdapter(t *testing.T) {
	carrier := &mockCarrier{data: make(map[string]string)}
	adapter := &otelCarrierAdapter{carrier: carrier}

	// Test Set
	adapter.Set("key1", "value1")
	adapter.Set("key2", "value2")

	// Test Get
	if got := adapter.Get("key1"); got != "value1" {
		t.Errorf("Get: expected %q, got %q", "value1", got)
	}
	if got := adapter.Get("nonexistent"); got != "" {
		t.Errorf("Get: expected empty string for nonexistent key, got %q", got)
	}

	// Test Keys
	keys := adapter.Keys()
	if len(keys) != 2 {
		t.Errorf("Keys: expected 2 keys, got %d", len(keys))
	}
}

func TestRoundTripConversions(t *testing.T) {
	// Test that converting DD -> OTel -> DD preserves the value
	originalDDTrace := uint64(12345678901234567890)
	otelTraceID := DatadogToTraceID(originalDDTrace)
	backToDDTrace := TraceIDToDatadog(otelTraceID)
	if backToDDTrace != originalDDTrace {
		t.Errorf("Round trip trace ID failed: original %d, got %d", originalDDTrace, backToDDTrace)
	}

	originalDDSpan := uint64(9876543210)
	otelSpanID := DatadogToSpanID(originalDDSpan)
	// Convert back manually
	var backToDDSpan uint64
	for _, b := range otelSpanID {
		backToDDSpan = (backToDDSpan << 8) | uint64(b)
	}
	if backToDDSpan != originalDDSpan {
		t.Errorf("Round trip span ID failed: original %d, got %d", originalDDSpan, backToDDSpan)
	}
}
