package hlogger

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stellwerk-labs/golib/htelemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/mocktracer"
)

func newTestLogger() (*zap.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	encoder := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	core := zapcore.NewCore(encoder, zapcore.AddSync(buf), zapcore.DebugLevel)
	return zap.New(core), buf
}

// setupDatadog sets up Datadog mock tracer and returns a cleanup function.
func setupDatadog(t *testing.T) func() {
	t.Helper()
	mt := mocktracer.Start()
	return func() { mt.Stop() }
}

// setupOTel sets up OpenTelemetry provider and returns a cleanup function.
func setupOTel(t *testing.T) func() {
	t.Helper()
	originalProvider := htelemetry.GetProvider()
	originalTP := otel.GetTracerProvider()
	originalPropagator := otel.GetTextMapPropagator()

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagator)

	otelProvider := htelemetry.NewOTelProvider("test-service")
	htelemetry.SetProvider(otelProvider)

	return func() {
		htelemetry.SetProvider(originalProvider)
		otel.SetTracerProvider(originalTP)
		otel.SetTextMapPropagator(originalPropagator)
	}
}

func validateDatadogTraceFields(t *testing.T, logEntry map[string]interface{}) {
	t.Helper()
	assert.NotEmpty(t, logEntry["trace_id"], "trace_id should be present for Datadog")
	assert.NotEmpty(t, logEntry["span_id"], "span_id should be present for Datadog")

	dd, ok := logEntry["dd"].(map[string]interface{})
	require.True(t, ok, "dd field should be a map")
	assert.NotZero(t, dd["trace_id"], "dd.trace_id should be non-zero")
	assert.NotZero(t, dd["span_id"], "dd.span_id should be non-zero")
}

func validateOTelTraceFields(t *testing.T, logEntry map[string]interface{}) {
	t.Helper()
	// trace_id/span_id string fields are not emitted for OTel — the otelzap bridge handles them
	assert.Nil(t, logEntry["trace_id"], "trace_id string field should not be present for OTel")
	assert.Nil(t, logEntry["span_id"], "span_id string field should not be present for OTel")

	dd, ok := logEntry["dd"].(map[string]interface{})
	require.True(t, ok, "dd field should be a map")
	assert.NotZero(t, dd["trace_id"], "dd.trace_id should be non-zero")
	assert.NotZero(t, dd["span_id"], "dd.span_id should be non-zero")
}

func TestTraceScopedLoggerFromCtx(t *testing.T) {
	tests := []struct {
		name            string
		setup           func(t *testing.T) func()
		createSpan      bool
		expectNewLogger bool
		validateLog     func(t *testing.T, logEntry map[string]interface{})
	}{
		{
			name:            "no span in context",
			setup:           nil,
			createSpan:      false,
			expectNewLogger: false,
			validateLog:     nil,
		},
		{
			name:            "with Datadog span",
			setup:           setupDatadog,
			createSpan:      true,
			expectNewLogger: true,
			validateLog:     validateDatadogTraceFields,
		},
		{
			name:            "with OTel span",
			setup:           setupOTel,
			createSpan:      true,
			expectNewLogger: true,
			validateLog:     validateOTelTraceFields,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				cleanup := tt.setup(t)
				defer cleanup()
			}

			logger, buf := newTestLogger()
			ctx := context.Background()

			if tt.createSpan {
				span, spanCtx := htelemetry.StartSpanFromContext(ctx, "test-operation")
				defer span.Finish()
				ctx = spanCtx
			}

			result := TraceScopedLoggerFromCtx(logger, ctx)

			if tt.expectNewLogger {
				require.NotEqual(t, logger, result, "should return new logger with trace context")

				result.Info("test message")

				var logEntry map[string]interface{}
				err := json.Unmarshal(buf.Bytes(), &logEntry)
				require.NoError(t, err)

				if tt.validateLog != nil {
					tt.validateLog(t, logEntry)
				}
			} else {
				assert.Equal(t, logger, result, "should return original logger when no span in context")
			}
		})
	}
}

func TestTraceScopedLoggerFromSpan(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) func()
		validateLog func(t *testing.T, logEntry map[string]interface{})
	}{
		{
			name:  "with Datadog span",
			setup: setupDatadog,
			validateLog: func(t *testing.T, logEntry map[string]interface{}) {
				assert.NotEmpty(t, logEntry["trace_id"], "trace_id should be present")
				assert.NotEmpty(t, logEntry["span_id"], "span_id should be present")
				assert.NotNil(t, logEntry["dd"])
			},
		},
		{
			name:  "with OTel span",
			setup: setupOTel,
			validateLog: func(t *testing.T, logEntry map[string]interface{}) {
				assert.Nil(t, logEntry["trace_id"], "trace_id string should not be present for OTel")
				assert.Nil(t, logEntry["span_id"], "span_id string should not be present for OTel")
				assert.NotNil(t, logEntry["dd"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setup(t)
			defer cleanup()

			logger, buf := newTestLogger()

			span := htelemetry.StartSpan("test-operation")
			defer span.Finish()

			result := TraceScopedLoggerFromSpan(logger, span)
			require.NotEqual(t, logger, result)

			result.Info("test message")

			var logEntry map[string]interface{}
			err := json.Unmarshal(buf.Bytes(), &logEntry)
			require.NoError(t, err)

			tt.validateLog(t, logEntry)
		})
	}
}

func TestWithSpanFromCtx(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T) func()
		createSpan bool
		validate   func(t *testing.T, result []interface{})
	}{
		{
			name:       "no span in context",
			setup:      nil,
			createSpan: false,
			validate: func(t *testing.T, result []interface{}) {
				assert.Nil(t, result, "should return nil when no span in context")
			},
		},
		{
			name:       "with Datadog span",
			setup:      setupDatadog,
			createSpan: true,
			validate: func(t *testing.T, result []interface{}) {
				require.NotNil(t, result)

				resultMap := make(map[string]interface{})
				for i := 0; i < len(result); i += 2 {
					key := result[i].(string)
					resultMap[key] = result[i+1]
				}

				assert.NotEmpty(t, resultMap["trace_id"], "trace_id should be present")
				assert.NotEmpty(t, resultMap["span_id"], "span_id should be present")

				dd, ok := resultMap["dd"].(map[string]uint64)
				require.True(t, ok, "dd should be a map[string]uint64")
				assert.NotZero(t, dd["trace_id"], "dd.trace_id should be non-zero")
				assert.NotZero(t, dd["span_id"], "dd.span_id should be non-zero")
			},
		},
		{
			name:       "with OTel span",
			setup:      setupOTel,
			createSpan: true,
			validate: func(t *testing.T, result []interface{}) {
				require.NotNil(t, result)

				resultMap := make(map[string]interface{})
				for i := 0; i < len(result); i += 2 {
					key := result[i].(string)
					resultMap[key] = result[i+1]
				}

				// trace_id/span_id string fields are not emitted for OTel
				assert.Nil(t, resultMap["trace_id"], "trace_id string should not be present for OTel")
				assert.Nil(t, resultMap["span_id"], "span_id string should not be present for OTel")

				dd, ok := resultMap["dd"].(map[string]uint64)
				require.True(t, ok, "dd should be a map[string]uint64")
				assert.NotZero(t, dd["trace_id"], "dd.trace_id should be non-zero")
				assert.NotZero(t, dd["span_id"], "dd.span_id should be non-zero")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				cleanup := tt.setup(t)
				defer cleanup()
			}

			ctx := context.Background()
			if tt.createSpan {
				span, spanCtx := htelemetry.StartSpanFromContext(ctx, "test-operation")
				defer span.Finish()
				ctx = spanCtx
			}

			result := WithSpanFromCtx(ctx)
			tt.validate(t, result)
		})
	}
}

func TestLogDetailsFromCtx(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T) func()
		createSpan bool
		args       []interface{}
		validate   func(t *testing.T, result []interface{})
	}{
		{
			name:       "no span in context",
			setup:      nil,
			createSpan: false,
			args:       []interface{}{"key1", "value1"},
			validate: func(t *testing.T, result []interface{}) {
				assert.Len(t, result, 2)
				assert.Equal(t, "key1", result[0])
				assert.Equal(t, "value1", result[1])
			},
		},
		{
			name:       "with Datadog span",
			setup:      setupDatadog,
			createSpan: true,
			args:       []interface{}{"key1", "value1"},
			validate: func(t *testing.T, result []interface{}) {
				assert.Greater(t, len(result), 2, "should have more than just the original args")
				assert.Equal(t, "key1", result[0])
				assert.Equal(t, "value1", result[1])
			},
		},
		{
			name:       "with OTel span",
			setup:      setupOTel,
			createSpan: true,
			args:       []interface{}{"key1", "value1"},
			validate: func(t *testing.T, result []interface{}) {
				assert.Greater(t, len(result), 2, "should have more than just the original args")
				assert.Equal(t, "key1", result[0])
				assert.Equal(t, "value1", result[1])
			},
		},
		{
			name:       "with multiple args and span",
			setup:      setupDatadog,
			createSpan: true,
			args:       []interface{}{"key1", "value1", "key2", 42},
			validate: func(t *testing.T, result []interface{}) {
				assert.Greater(t, len(result), 4, "should have more than just the original args")
				assert.Equal(t, "key1", result[0])
				assert.Equal(t, "value1", result[1])
				assert.Equal(t, "key2", result[2])
				assert.Equal(t, 42, result[3])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				cleanup := tt.setup(t)
				defer cleanup()
			}

			ctx := context.Background()
			if tt.createSpan {
				span, spanCtx := htelemetry.StartSpanFromContext(ctx, "test-operation")
				defer span.Finish()
				ctx = spanCtx
			}

			result := LogDetailsFromCtx(ctx, tt.args...)
			tt.validate(t, result)
		})
	}
}
