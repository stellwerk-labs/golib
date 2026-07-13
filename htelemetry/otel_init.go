package htelemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otellog "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"
)

// SpanExporter is an alias for the OTel SDK SpanExporter interface,
// exposed so callers can inject custom exporters (e.g. for testing)
// without importing the SDK directly.
type SpanExporter = sdktrace.SpanExporter

// LogExporter is an alias for the OTel SDK log.Exporter interface,
// exposed so callers can inject custom log exporters (e.g. for testing)
// without importing the SDK directly.
type LogExporter = sdklog.Exporter

// OTelConfig configures the OpenTelemetry SDK initialization.
type OTelConfig struct {
	// ServiceName is the name of the service (required).
	ServiceName string

	// ServiceVersion is the version of the service (optional).
	ServiceVersion string

	// Logger is used for logging OTel errors. If nil, errors are silently ignored.
	Logger *zap.Logger

	// RuntimeMetrics enables collection of Go runtime metrics.
	// Defaults to true.
	RuntimeMetrics *bool

	// RuntimeMetricsInterval is the minimum interval between memory stats reads.
	// Defaults to 1 second.
	RuntimeMetricsInterval time.Duration

	// SpanExporter allows injecting a pre-built span exporter, bypassing the
	// default OTLP gRPC exporter creation. This is useful for testing (e.g.
	// using tracetest.InMemoryExporter) or for non-gRPC export backends.
	// When set, ExporterOptions is ignored.
	SpanExporter SpanExporter

	// ExporterOptions allows customizing the OTLP gRPC exporter.
	// By default, the exporter reads configuration from environment variables:
	// - OTEL_EXPORTER_OTLP_ENDPOINT (e.g., "localhost:4317")
	// - OTEL_EXPORTER_OTLP_HEADERS (for authentication)
	// Ignored when SpanExporter is set.
	ExporterOptions []otlptracegrpc.Option

	// TracerProviderOptions allows adding custom options to the TracerProvider.
	// Use this for custom samplers, span processors, etc.
	TracerProviderOptions []sdktrace.TracerProviderOption

	// Propagator sets the text map propagator. If nil, uses W3C TraceContext and Baggage.
	Propagator propagation.TextMapPropagator

	// SetAsGlobalProvider sets this as the global htelemetry provider.
	// Defaults to true.
	SetAsGlobalProvider *bool

	// LogExporter allows injecting a pre-built log exporter, bypassing the
	// default OTLP gRPC log exporter creation. This is useful for testing.
	// When set, LogExporterOptions is ignored.
	LogExporter LogExporter

	// LogExporterOptions allows customizing the OTLP gRPC log exporter.
	// By default, the exporter reads configuration from environment variables.
	// Ignored when LogExporter is set.
	LogExporterOptions []otlploggrpc.Option

	// LoggerProviderOptions allows adding custom options to the LoggerProvider.
	LoggerProviderOptions []sdklog.LoggerProviderOption
}

// OTelResult holds the providers created by StartOTel.
type OTelResult struct {
	// LoggerProvider is the OTel LoggerProvider that can be passed to the otelzap bridge.
	LoggerProvider *sdklog.LoggerProvider
}

// StartOTel initializes OpenTelemetry tracing and logging with sensible defaults.
// It sets up the TracerProvider, LoggerProvider, propagators, and optionally runtime metrics.
//
// Returns an OTelResult with providers, and a shutdown function that should be called
// when the application exits (typically via defer). The shutdown function flushes any
// pending telemetry and releases resources.
//
// Example:
//
//	result, shutdown, err := htelemetry.StartOTel(ctx, htelemetry.OTelConfig{
//	    ServiceName:    "my-service",
//	    ServiceVersion: "1.0.0",
//	    Logger:         logger,
//	})
//	if err != nil {
//	    return err
//	}
//	defer shutdown(ctx)
//	logger = hlogger.WrapWithOTelBridge(logger, "my-service", result.LoggerProvider)
func StartOTel(ctx context.Context, cfg OTelConfig) (result *OTelResult, shutdown func(context.Context) error, err error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "unknown-service"
	}

	// Build resource with service info
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, nil, err
	}

	// Create or use the provided span exporter
	var exporter sdktrace.SpanExporter
	if cfg.SpanExporter != nil {
		exporter = cfg.SpanExporter
	} else {
		exporter, err = otlptracegrpc.New(ctx, cfg.ExporterOptions...)
		if err != nil {
			return nil, nil, err
		}
	}

	// Build TracerProvider options
	tpOpts := []sdktrace.TracerProviderOption{
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	}
	tpOpts = append(tpOpts, cfg.TracerProviderOptions...)

	// Create TracerProvider
	tp := sdktrace.NewTracerProvider(tpOpts...)

	// Set global TracerProvider
	otel.SetTracerProvider(tp)

	// Create or use the provided log exporter
	var logExp sdklog.Exporter
	if cfg.LogExporter != nil {
		logExp = cfg.LogExporter
	} else {
		logExp, err = otlploggrpc.New(ctx, cfg.LogExporterOptions...)
		if err != nil {
			return nil, nil, err
		}
	}

	// Build LoggerProvider options
	lpOpts := []sdklog.LoggerProviderOption{
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
		sdklog.WithResource(res),
	}
	lpOpts = append(lpOpts, cfg.LoggerProviderOptions...)

	// Create LoggerProvider
	lp := sdklog.NewLoggerProvider(lpOpts...)

	// Set global LoggerProvider
	otellog.SetLoggerProvider(lp)

	// Set propagator
	propagator := cfg.Propagator
	if propagator == nil {
		propagator = propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		)
	}
	otel.SetTextMapPropagator(propagator)

	// Set error handler if logger provided
	if cfg.Logger != nil {
		otel.SetErrorHandler(&otelErrorHandler{logger: cfg.Logger})
	}

	// Start runtime metrics collection
	runtimeMetrics := cfg.RuntimeMetrics == nil || *cfg.RuntimeMetrics
	if runtimeMetrics {
		interval := cfg.RuntimeMetricsInterval
		if interval == 0 {
			interval = time.Second
		}
		if err := runtime.Start(runtime.WithMinimumReadMemStatsInterval(interval)); err != nil {
			if cfg.Logger != nil {
				cfg.Logger.Warn("failed to start runtime metrics", zap.Error(err))
			}
		}
	}

	// Set as global htelemetry provider
	setGlobal := cfg.SetAsGlobalProvider == nil || *cfg.SetAsGlobalProvider
	if setGlobal {
		SetProvider(NewOTelProvider(cfg.ServiceName))
	}

	shutdownFn := func(ctx context.Context) error {
		tpErr := tp.Shutdown(ctx)
		lpErr := lp.Shutdown(ctx)
		if tpErr != nil {
			return tpErr
		}
		return lpErr
	}

	return &OTelResult{LoggerProvider: lp}, shutdownFn, nil
}

// otelErrorHandler adapts zap.Logger to OTel's ErrorHandler interface.
type otelErrorHandler struct {
	logger *zap.Logger
}

func (h *otelErrorHandler) Handle(err error) {
	h.logger.Error("opentelemetry error", zap.Error(err))
}
