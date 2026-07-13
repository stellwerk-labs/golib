# htelemetry

Backend-agnostic tracing abstraction supporting both Datadog and OpenTelemetry.

## Features

- Unified API for distributed tracing across both backends
- Easy provider switching at startup without code changes
- Automatic trace context propagation
- Zap logger integration
- Runtime metrics collection (OpenTelemetry)

## Usage

### Using Datadog (default)

Datadog is the default provider for backward compatibility. Simply use the tracing functions directly:

```go
import "github.com/stellwerk-labs/golib/htelemetry"

// Start Datadog tracer (as usual)
ddtrace.Start(
    ddtrace.WithServiceName("my-service"),
    ddtrace.WithServiceVersion("1.0.0"),
)
defer ddtrace.Stop()

// Use htelemetry functions - they use Datadog by default
span, ctx := htelemetry.StartSpanFromContext(ctx, "my-operation")
defer span.Finish()

span.SetTag("user.id", userID)
```

### Using OpenTelemetry

To switch to OpenTelemetry, use `StartOTel` at application startup:

```go
import (
    "github.com/stellwerk-labs/golib/htelemetry"
    "github.com/stellwerk-labs/golib/hlogger"
)

func main() {
    ctx := context.Background()

    // Initialize OpenTelemetry (tracing + logging)
    result, shutdown, err := htelemetry.StartOTel(ctx, htelemetry.OTelConfig{
        ServiceName:    "my-service",
        ServiceVersion: "1.0.0",
        Logger:         logger, // optional: for OTel error logging
    })
    if err != nil {
        log.Fatal(err)
    }
    defer shutdown(ctx)

    // Wrap zap logger with the OTel log bridge so logs are sent via OTLP
    // with native trace context correlation
    logger = hlogger.WrapWithOTelBridge(logger, "my-service", result.LoggerProvider)

    // Use htelemetry functions - they now use OpenTelemetry
    span, ctx := htelemetry.StartSpanFromContext(ctx, "my-operation")
    defer span.Finish()

    span.SetTag("user.id", userID)
}
```

The OTLP exporter is configured via environment variables (no code changes needed):

- `OTEL_EXPORTER_OTLP_ENDPOINT` - Endpoint URL with scheme (default: `https://localhost:4317`)
- `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` - Traces-specific endpoint (takes precedence over above)
- `OTEL_EXPORTER_OTLP_HEADERS` - Headers for authentication (e.g., `api-key=secret`)
- `OTEL_EXPORTER_OTLP_INSECURE` - Disable TLS (`true`/`false`, default: `false`)

Example:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT="http://otel-collector:4317"
export OTEL_EXPORTER_OTLP_HEADERS="api-key=your-api-key"
```

### Advanced OpenTelemetry Configuration

For most use cases, environment variables are sufficient. Use `ExporterOptions` only for advanced scenarios:

```go
result, shutdown, err := htelemetry.StartOTel(ctx, htelemetry.OTelConfig{
    ServiceName:    "my-service",
    ServiceVersion: "1.0.0",
    Logger:         logger,

    // Custom TracerProvider options (e.g., sampling)
    TracerProviderOptions: []sdktrace.TracerProviderOption{
        sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.1)),
    },

    // Disable runtime metrics if not needed
    RuntimeMetrics: ptr(false),

    // Custom runtime metrics interval
    RuntimeMetricsInterval: 5 * time.Second,
})
```

## Tracing API

### Starting Spans

```go
// Start a root span
span := htelemetry.StartSpan("operation-name")
defer span.Finish()

// Start a child span from context
span, ctx := htelemetry.StartSpanFromContext(ctx, "child-operation")
defer span.Finish()

// Start span with options
span := htelemetry.StartSpan("operation",
    htelemetry.ResourceName("GET /users"),  // Datadog resource grouping
    htelemetry.ChildOf(parentSpanContext),  // Explicit parent
)
```

### Setting Tags

```go
span.SetTag("http.status_code", 200)
span.SetTag("user.id", "user-123")
span.SetTag("db.rows_affected", 42)
```

### Finishing with Error

```go
if err != nil {
    span.Finish(htelemetry.WithError(err))
    return err
}
span.Finish()
```

### Extracting Span from Context

```go
span, ok := htelemetry.SpanFromContext(ctx)
if ok {
    span.SetTag("additional.info", "value")
}
```

### Context Propagation

```go
// Inject span context into carrier (e.g., HTTP headers)
carrier := &myCarrier{}
htelemetry.Inject(span.Context(), carrier)

// Extract span context from carrier
spanCtx, err := htelemetry.Extract(carrier)
if err == nil {
    span := htelemetry.StartSpan("downstream-op", htelemetry.ChildOf(spanCtx))
    defer span.Finish()
}
```

## Integration with hecho

Use with the hecho HTTP server:

```go
import (
    "github.com/stellwerk-labs/golib/hecho"
    "github.com/stellwerk-labs/golib/htelemetry"
)

func main() {
    // Initialize OTel first
    result, shutdown, _ := htelemetry.StartOTel(ctx, htelemetry.OTelConfig{
        ServiceName: "my-api",
        Logger:      logger,
    })
    defer shutdown(ctx)

    // Wrap logger with OTel log bridge
    logger = hlogger.WrapWithOTelBridge(logger, "my-api", result.LoggerProvider)

    // Create Echo server with OTel tracing
    e := hecho.DefaultEchoServer(&hecho.ServerConfig{
        AppName: "my-api",
        Logger:  logger,
        Tracing: hecho.TracingOTel,
    })
}
```

## Trace ID Formats

The providers use different trace ID formats:

| Provider | Trace ID | Span ID |
|----------|----------|---------|
| Datadog | 64-bit decimal | 64-bit decimal |
| OpenTelemetry | 128-bit hex (32 chars) | 64-bit hex (16 chars) |

Both formats are accessible via the `SpanContext` interface:

```go
ctx := span.Context()

// String format (native to each provider)
traceID := ctx.TraceID()
spanID := ctx.SpanID()

// Uint64 format (for Datadog log correlation)
ddTraceID := ctx.TraceIDUint64()
ddSpanID := ctx.SpanIDUint64()
```

### ID Conversion Utilities

For OpenTelemetry to Datadog log correlation:

```go
import "go.opentelemetry.io/otel/trace"

// Convert OTel trace ID to Datadog format
ddTraceID := htelemetry.TraceIDToDatadog(otelTraceID)

// Convert Datadog ID to OTel format
otelTraceID := htelemetry.DatadogToTraceID(ddTraceID)

// Parse hex strings
traceID, err := htelemetry.ParseTraceID("0102030405060708090a0b0c0d0e0f10")
spanID, err := htelemetry.ParseSpanID("0102030405060708")
```
