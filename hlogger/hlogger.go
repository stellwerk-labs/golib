package hlogger

import (
	"context"
	"fmt"

	"go.opentelemetry.io/contrib/bridges/otelzap"
	otellog "go.opentelemetry.io/otel/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type HLogger struct {
	Logger *zap.Logger
	Level  *zap.AtomicLevel
}

func (h *HLogger) ChangeLevel(logLevel string) error {
	l, err := zapcore.ParseLevel(logLevel)
	if err != nil {
		return err
	}
	h.Level.SetLevel(l)

	return nil
}

// New returns a plain zap.Logger
func New(logLevel string, development bool, encoding string) (*zap.Logger, error) {
	logger, err := NewHLogger(logLevel, development, encoding)
	if err != nil {
		return nil, err
	}

	return logger.Logger, nil
}

// NewHLogger returns an HLogger with a changeable logging level
func NewHLogger(logLevel string, development bool, encoding string) (*HLogger, error) {
	// Logging
	level, err := zap.ParseAtomicLevel(logLevel)
	if err != nil {
		return nil, fmt.Errorf(`parsing log level: %w`, err)
	}
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.RFC3339NanoTimeEncoder

	loggerCfg := zap.Config{
		Level:            level,
		Development:      development,
		Sampling:         nil,
		Encoding:         encoding,
		EncoderConfig:    encoderConfig,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
	logger, err := loggerCfg.Build(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return &DataDogErrorMappingCore{inner: core}
	}))
	if err != nil {
		return nil, fmt.Errorf(`building logger: %w`, err)
	}

	return &HLogger{
		Logger: logger,
		Level:  &level,
	}, nil
}

// NewLogger returns a logger generally used inside an application
func NewLogger() (*HLogger, error) {
	return NewHLogger("INFO", false, "json")
}

// NewTestLogger returns a logger usable during testing
func NewTestLogger() (*HLogger, error) {
	return NewHLogger("INFO", true, "console")
}

// WithSpanFromContext returns key-value pairs for adding trace context to a sugared logger.
// Deprecated: Use WithSpanFromCtx instead, which works with both DD and OTel providers.
func WithSpanFromContext(ctx context.Context) []interface{} {
	span, ok := tracer.SpanFromContext(ctx)
	if !ok {
		return nil
	}

	spanCTX := span.Context()
	return []interface{}{
		"trace_id", spanCTX.TraceID(),
		"dd", map[string]uint64{
			"trace_id": spanCTX.TraceID(),
			"span_id":  spanCTX.SpanID(),
		},
	}
}

// LogDetails appends trace context to a list of key-value pairs for sugared logging.
// Deprecated: Use LogDetailsFromCtx instead, which works with both DD and OTel providers.
func LogDetails(ctx context.Context, args ...interface{}) []interface{} {
	return append(args, WithSpanFromContext(ctx)...)
}

func OnExit(logger *zap.Logger) {
	// flush the remaining logs
	logger.Sync()

	// log panics using zap and exit
	if err := recover(); err != nil {
		logger.Sugar().Fatalw("panic", "err", err)
	}
}

type dataDogLogger struct {
	logger *zap.Logger
}

func (d *dataDogLogger) Log(msg string) {
	d.logger.Sugar().Infow(msg, "from", "datadog")
}

func NewDataDogLogger(logger *zap.Logger) ddtrace.Logger {
	return &dataDogLogger{
		logger: logger,
	}
}

// WrapWithOTelBridge replaces the logger's core with the otelzap bridge core,
// so that all log records are sent to the OTel LoggerProvider via OTLP.
// The DataDogErrorMappingCore is preserved as a wrapper around the bridge core.
//
// Callers should pass the LoggerProvider returned by htelemetry.StartOTel.
func WrapWithOTelBridge(logger *zap.Logger, serviceName string, lp otellog.LoggerProvider) *zap.Logger {
	bridgeCore := otelzap.NewCore(serviceName, otelzap.WithLoggerProvider(lp))
	return logger.WithOptions(zap.WrapCore(func(_ zapcore.Core) zapcore.Core {
		return &DataDogErrorMappingCore{inner: bridgeCore}
	}))
}

// TraceScopedLogger returns a new logger, based on the input one, with the datadog field set to the given span.
// Deprecated: Use TraceScopedLoggerFromSpan instead, which works with both DD and OTel providers.
func TraceScopedLogger(logger *zap.Logger, span ddtrace.Span) *zap.Logger {
	return logger.With(zap.Object("dd", &ddContext{
		span: span,
	}))
}

type ddContext struct {
	span tracer.Span
}

func (d *ddContext) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	// see https://docs.datadoghq.com/tracing/other_telemetry/connect_logs_and_traces/go/
	enc.AddUint64("trace_id", d.span.Context().TraceID())
	enc.AddUint64("span_id", d.span.Context().SpanID())
	return nil
}

// TraceScopedLoggerCtx returns a new logger with the span information extracted from the current context if it exists.
// If no span exists on the context, the original logger is returned.
// Deprecated: Use TraceScopedLoggerFromCtx instead, which works with both DD and OTel providers.
func TraceScopedLoggerCtx(logger *zap.Logger, ctx context.Context) *zap.Logger {
	span, ok := tracer.SpanFromContext(ctx)
	if !ok {
		return logger
	}

	return TraceScopedLogger(logger, span)
}
