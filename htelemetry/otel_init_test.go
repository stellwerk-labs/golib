package htelemetry

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/zap/zaptest"
)

// noopLogExporter is a no-op log exporter for tests.
type noopLogExporter struct{}

func (n *noopLogExporter) Export(_ context.Context, _ []sdklog.Record) error { return nil }
func (n *noopLogExporter) Shutdown(_ context.Context) error                  { return nil }
func (n *noopLogExporter) ForceFlush(_ context.Context) error                { return nil }

// newTestConfig returns an OTelConfig that uses an in-memory exporter,
// making the test hermetic (no OTLP endpoint required).
func newTestConfig(t *testing.T, name string) (OTelConfig, *tracetest.InMemoryExporter) {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	return OTelConfig{
		ServiceName:  name,
		SpanExporter: exp,
		LogExporter:  &noopLogExporter{},
	}, exp
}

// saveAndRestoreGlobals saves the global provider and tracer provider, restoring
// them when the test completes.
func saveAndRestoreGlobals(t *testing.T) {
	t.Helper()
	origProvider := GetProvider()
	origTP := otel.GetTracerProvider()
	t.Cleanup(func() {
		SetProvider(origProvider)
		otel.SetTracerProvider(origTP)
	})
}

func TestStartOTel_DefaultConfig(t *testing.T) {
	saveAndRestoreGlobals(t)

	cfg, _ := newTestConfig(t, "test-service")
	cfg.ServiceVersion = "1.0.0"

	ctx := context.Background()
	_, shutdown, err := StartOTel(ctx, cfg)
	if err != nil {
		t.Fatalf("StartOTel failed: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected shutdown function, got nil")
	}

	// Verify global provider was set
	provider := GetProvider()
	if _, ok := provider.(*otelProvider); !ok {
		t.Errorf("expected global provider to be *otelProvider, got %T", provider)
	}

	if err := shutdown(ctx); err != nil {
		t.Errorf("shutdown error: %v", err)
	}
}

func TestStartOTel_WithLogger(t *testing.T) {
	saveAndRestoreGlobals(t)

	cfg, _ := newTestConfig(t, "test-service-with-logger")
	cfg.Logger = zaptest.NewLogger(t)

	ctx := context.Background()
	_, shutdown, err := StartOTel(ctx, cfg)
	if err != nil {
		t.Fatalf("StartOTel failed: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected shutdown function, got nil")
	}

	if err := shutdown(ctx); err != nil {
		t.Errorf("shutdown error: %v", err)
	}
}

func TestStartOTel_DisableRuntimeMetrics(t *testing.T) {
	saveAndRestoreGlobals(t)

	cfg, _ := newTestConfig(t, "test-service-no-metrics")
	runtimeMetrics := false
	cfg.RuntimeMetrics = &runtimeMetrics

	ctx := context.Background()
	_, shutdown, err := StartOTel(ctx, cfg)
	if err != nil {
		t.Fatalf("StartOTel failed: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected shutdown function, got nil")
	}

	if err := shutdown(ctx); err != nil {
		t.Errorf("shutdown error: %v", err)
	}
}

func TestStartOTel_DisableGlobalProvider(t *testing.T) {
	saveAndRestoreGlobals(t)

	// Set a known provider first
	SetProvider(&datadogProvider{})

	cfg, _ := newTestConfig(t, "test-service-no-global")
	setGlobal := false
	cfg.SetAsGlobalProvider = &setGlobal

	ctx := context.Background()
	_, shutdown, err := StartOTel(ctx, cfg)
	if err != nil {
		t.Fatalf("StartOTel failed: %v", err)
	}

	// Verify global provider was NOT changed
	provider := GetProvider()
	if _, ok := provider.(*datadogProvider); !ok {
		t.Errorf("expected global provider to remain *datadogProvider, got %T", provider)
	}

	if err := shutdown(ctx); err != nil {
		t.Errorf("shutdown error: %v", err)
	}
}

func TestStartOTel_CustomPropagator(t *testing.T) {
	saveAndRestoreGlobals(t)
	originalProp := otel.GetTextMapPropagator()
	t.Cleanup(func() { otel.SetTextMapPropagator(originalProp) })

	cfg, _ := newTestConfig(t, "test-service-custom-propagator")
	cfg.Propagator = propagation.TraceContext{}

	ctx := context.Background()
	_, shutdown, err := StartOTel(ctx, cfg)
	if err != nil {
		t.Fatalf("StartOTel failed: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected shutdown function, got nil")
	}

	if err := shutdown(ctx); err != nil {
		t.Errorf("shutdown error: %v", err)
	}
}

func TestStartOTel_CustomTracerProviderOptions(t *testing.T) {
	saveAndRestoreGlobals(t)

	cfg, _ := newTestConfig(t, "test-service-custom-tp")
	cfg.TracerProviderOptions = []sdktrace.TracerProviderOption{
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	}

	ctx := context.Background()
	_, shutdown, err := StartOTel(ctx, cfg)
	if err != nil {
		t.Fatalf("StartOTel failed: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected shutdown function, got nil")
	}

	if err := shutdown(ctx); err != nil {
		t.Errorf("shutdown error: %v", err)
	}
}

func TestStartOTel_EmptyServiceName(t *testing.T) {
	saveAndRestoreGlobals(t)

	exp := tracetest.NewInMemoryExporter()
	cfg := OTelConfig{
		// No service name - should default to "unknown-service"
		SpanExporter: exp,
		LogExporter:  &noopLogExporter{},
	}

	ctx := context.Background()
	_, shutdown, err := StartOTel(ctx, cfg)
	if err != nil {
		t.Fatalf("StartOTel failed: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected shutdown function, got nil")
	}

	if err := shutdown(ctx); err != nil {
		t.Errorf("shutdown error: %v", err)
	}
}

func TestStartOTel_CustomRuntimeMetricsInterval(t *testing.T) {
	saveAndRestoreGlobals(t)

	cfg, _ := newTestConfig(t, "test-service-custom-interval")
	cfg.RuntimeMetricsInterval = 5 * time.Second

	ctx := context.Background()
	_, shutdown, err := StartOTel(ctx, cfg)
	if err != nil {
		t.Fatalf("StartOTel failed: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected shutdown function, got nil")
	}

	if err := shutdown(ctx); err != nil {
		t.Errorf("shutdown error: %v", err)
	}
}

func TestOtelErrorHandler(t *testing.T) {
	logger := zaptest.NewLogger(t)
	handler := &otelErrorHandler{logger: logger}

	// This should not panic
	handler.Handle(context.DeadlineExceeded)
}

func TestStartOTel_SpanCreation(t *testing.T) {
	saveAndRestoreGlobals(t)

	cfg, _ := newTestConfig(t, "test-service-spans")

	ctx := context.Background()
	_, shutdown, err := StartOTel(ctx, cfg)
	if err != nil {
		t.Fatalf("StartOTel failed: %v", err)
	}

	// Verify that spans can be created end-to-end via the global provider
	span, spanCtx := StartSpanFromContext(ctx, "test-operation")
	if span == nil {
		t.Fatal("expected non-nil span")
	}
	span.SetTag("test.key", "test-value")

	spanContext := span.Context()
	if spanContext.TraceID() == "" {
		t.Error("expected non-empty trace ID")
	}
	if spanContext.SpanID() == "" {
		t.Error("expected non-empty span ID")
	}

	span.Finish()

	if err := shutdown(spanCtx); err != nil {
		t.Errorf("shutdown error: %v", err)
	}
}
