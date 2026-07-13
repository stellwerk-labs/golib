package hlogger

import (
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// DataDogErrorMappingCore calls reworkFields over any fields added to the logger. This ensures any errors are
// translated into the expected format for datadog. This is because the default zap.Error object doesn't provide
// the {message, kind, stack} attributes that datadog expects.
type DataDogErrorMappingCore struct {
	inner zapcore.Core
}

func (e *DataDogErrorMappingCore) Enabled(level zapcore.Level) bool {
	return e.inner.Enabled(level)
}

func (e *DataDogErrorMappingCore) With(fields []zapcore.Field) zapcore.Core {
	reworkFields(fields)
	return &DataDogErrorMappingCore{inner: e.inner.With(fields)}
}

func (e *DataDogErrorMappingCore) Check(entry zapcore.Entry, entry2 *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	return e.inner.Check(entry, entry2)
}

func (e *DataDogErrorMappingCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	reworkFields(fields)
	return e.inner.Write(entry, fields)
}

func (e *DataDogErrorMappingCore) Sync() error {
	return e.inner.Sync()
}

type stackTracer interface {
	StackTrace() errors.StackTrace
}

func reworkFields(fields []zapcore.Field) {
	for i, field := range fields {
		if field.Key == "error" && field.Type == zapcore.ErrorType {
			if field.Interface != nil {
				err := field.Interface.(error)
				fields[i] = zap.Object("error", zapcore.ObjectMarshalerFunc(func(encoder zapcore.ObjectEncoder) error {
					encoder.AddString("message", err.Error())
					encoder.AddString("kind", reflect.TypeOf(err).String())
					s, ok := err.(stackTracer)
					if ok {
						stack := fmt.Sprintf("%+v", s.StackTrace())
						if len(stack) > 0 && stack[0] == '\n' {
							stack = stack[1:]
						}
						encoder.AddString("stack", stack)
					}
					return nil
				}))
			}
		}
	}
}

var _ zapcore.Core = (*DataDogErrorMappingCore)(nil)

// DataDogErrorMappingCoreWrap wraps the given logger in the DataDogErrorMappingCore.
// Deprecated since this is now done by default in NewHLogger.
func DataDogErrorMappingCoreWrap(logger *zap.Logger) *zap.Logger {
	return zap.New(logger.Core(), zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return &DataDogErrorMappingCore{inner: core}
	}))
}
