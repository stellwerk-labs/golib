package hrabbitmq

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wagslane/go-rabbitmq"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestLogger(t *testing.T) {
	logger := &zap.Logger{}

	rabbitmq.WithConsumerOptionsLogger(NewLogger(logger))
}

func TestLoggerFormat(t *testing.T) {
	assert := assert.New(t)

	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	observedLogger := zap.New(observedZapCore)

	rabbitmqLogger := NewLogger(observedLogger)

	rabbitmqLogger.Infof("plain")
	rabbitmqLogger.Infof("with %s", "value")

	assert.Equal(2, observedLogs.Len())
	assert.Equal("plain", observedLogs.All()[0].Message)
	assert.Equal("with value", observedLogs.All()[1].Message)
}
