package hlogger

import (
	"fmt"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestDataDogErrorMappingCore(t *testing.T) {
	c, logs := observer.New(zapcore.InfoLevel)
	logger := DataDogErrorMappingCoreWrap(zap.New(c))
	logger.Info("foo")
	logger.With(zap.String("foo", "bar")).With(zap.Error(fmt.Errorf("hi"))).Error("blah")
	logger.With(zap.Error(errors.WithStack(fmt.Errorf("baz")))).Warn("hi")

	if assert.Equal(t, 3, logs.Len()) {

		assert.Equal(t, map[string]interface{}{}, logs.All()[0].ContextMap())
		assert.Equal(t, map[string]interface{}{
			"error": map[string]interface{}{
				"kind":    "*errors.errorString",
				"message": "hi",
			},
			"foo": "bar",
		}, logs.All()[1].ContextMap())
		assert.Contains(t, logs.All()[2].ContextMap(), "error")
		errorBits := logs.All()[2].ContextMap()["error"].(map[string]interface{})
		if assert.Contains(t, errorBits, "stack") {
			assert.Regexp(t, `^github.com/stellwerk-labs/golib/hlogger.TestDataDogErrorMappingCore[.\s]+`, errorBits["stack"].(string))
			delete(errorBits, "stack")
		}
		assert.Equal(t, map[string]interface{}{
			"kind":    "*errors.withStack",
			"message": "baz",
		}, errorBits)
	}
}
