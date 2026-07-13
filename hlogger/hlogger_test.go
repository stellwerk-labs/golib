package hlogger

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func TestNew(t *testing.T) {
	assert := assert.New(t)
	logger, err := New("INFO", false, "json")
	assert.NoError(err)

	OnExit(logger)
}

func TestNewLogger(t *testing.T) {
	assert := assert.New(t)
	logger, err := NewLogger()
	assert.NoError(err)

	OnExit(logger.Logger)
}

func TestChangeLevel(t *testing.T) {
	assert := assert.New(t)
	logger, err := NewLogger()
	assert.NoError(err)

	assert.NoError(logger.ChangeLevel("DEBUG"))
}

func TestNewTestLogger(t *testing.T) {
	assert := assert.New(t)
	logger, err := NewTestLogger()
	assert.NoError(err)

	OnExit(logger.Logger)
}

func TestTraceScopedLoggerCtx_none(t *testing.T) {
	assert := assert.New(t)
	tracer.Start(tracer.WithServiceName("test-service"))
	defer tracer.Stop()
	logger, err := NewTestLogger()
	assert.NoError(err)
	l := TraceScopedLoggerCtx(logger.Logger, context.Background())
	assert.Equal(logger.Logger, l)
}

func TestTraceScopedLoggerCtx_with_span(t *testing.T) {
	assert := assert.New(t)
	tracer.Start(tracer.WithServiceName("test-service"))
	defer tracer.Stop()
	logger, err := NewTestLogger()
	assert.NoError(err)
	ctx := context.Background()
	_, ctx = tracer.StartSpanFromContext(ctx, "foo")
	l := TraceScopedLoggerCtx(logger.Logger, ctx)
	assert.NotEqual(logger.Logger, l)
}
