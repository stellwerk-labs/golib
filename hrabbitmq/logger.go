package hrabbitmq

import (
	"github.com/wagslane/go-rabbitmq"
	"go.uber.org/zap"
)

var _ rabbitmq.Logger = (*Logger)(nil)

type Logger struct {
	Logger *zap.SugaredLogger
}

func NewLogger(logger *zap.Logger) *Logger {
	return &Logger{
		Logger: logger.Named("rabbitmq").Sugar(),
	}
}

func (l Logger) Fatalf(format string, v ...interface{}) {
	l.Logger.Fatalf(format, v...)
}

func (l Logger) Errorf(format string, v ...interface{}) {
	l.Logger.Errorf(format, v...)
}

func (l Logger) Warnf(format string, v ...interface{}) {
	l.Logger.Warnf(format, v...)
}

func (l Logger) Infof(format string, v ...interface{}) {
	l.Logger.Infof(format, v...)
}

func (l Logger) Debugf(format string, v ...interface{}) {
	l.Logger.Debugf(format, v...)
}

func (l Logger) Tracef(format string, v ...interface{}) {}
