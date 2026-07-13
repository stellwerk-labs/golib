package hecho

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/stellwerk-labs/golib/herrors"
)

func newTestEchoWithLogger(core zapcore.Core) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = CustomHTTPErrorHandler
	loggerConfig := &LoggerConfig{Logger: zap.New(core)}
	e.Use(middleware.RequestLoggerWithConfig(GetRequestLoggerConfig(loggerConfig)))
	return e
}

func TestGetRequestLoggerConfig_LogLevels(t *testing.T) {
	tests := []struct {
		name          string
		handler       func(c echo.Context) error
		expectedLevel zapcore.Level
		expectedMsg   string
		hasErr        bool
	}{
		{
			name: "200 without error logs info",
			handler: func(c echo.Context) error {
				return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
			},
			expectedLevel: zap.InfoLevel,
			expectedMsg:   "handled",
		},
		{
			name: "201 without error logs info",
			handler: func(c echo.Context) error {
				return c.JSON(http.StatusCreated, map[string]string{"status": "created"})
			},
			expectedLevel: zap.InfoLevel,
			expectedMsg:   "handled",
		},
		{
			name: "404 without error logs warn",
			handler: func(c echo.Context) error {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
			},
			expectedLevel: zap.WarnLevel,
			expectedMsg:   "handled",
		},
		{
			name: "400 without error logs warn",
			handler: func(c echo.Context) error {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "bad request"})
			},
			expectedLevel: zap.WarnLevel,
			expectedMsg:   "handled",
		},
		{
			name: "500 without error logs error",
			handler: func(c echo.Context) error {
				c.Response().WriteHeader(http.StatusInternalServerError)
				return nil
			},
			expectedLevel: zap.ErrorLevel,
			expectedMsg:   "handled",
		},
		{
			name: "403 with PlatformOrchestratorError logs warn",
			handler: func(c echo.Context) error {
				return &herrors.PlatformOrchestratorError{
					StatusCode: http.StatusForbidden,
					Message:    "forbidden",
				}
			},
			expectedLevel: zap.WarnLevel,
			expectedMsg:   "handled",
			hasErr:        true,
		},
		{
			name: "500 with plain error logs error",
			handler: func(c echo.Context) error {
				return errors.New("unexpected failure")
			},
			expectedLevel: zap.ErrorLevel,
			expectedMsg:   "handled",
			hasErr:        true,
		},
		{
			name: "500 with PlatformOrchestratorError logs error",
			handler: func(c echo.Context) error {
				return &herrors.PlatformOrchestratorError{
					StatusCode: http.StatusInternalServerError,
					Message:    "internal error",
				}
			},
			expectedLevel: zap.ErrorLevel,
			expectedMsg:   "handled",
			hasErr:        true,
		},
		{
			name: "409 with PlatformOrchestratorError logs warn",
			handler: func(c echo.Context) error {
				return &herrors.PlatformOrchestratorError{
					StatusCode: http.StatusConflict,
					Message:    "conflict",
				}
			},
			expectedLevel: zap.WarnLevel,
			expectedMsg:   "handled",
			hasErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zap.DebugLevel)
			e := newTestEchoWithLogger(core)
			e.GET("/test", tt.handler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			e.ServeHTTP(w, req)

			entries := logs.All()
			assert.NotEmpty(t, entries, "expected at least one log entry")

			// Find the "handled" log entry (skip other potential logs from error handler)
			var logEntry *observer.LoggedEntry
			for i := range entries {
				if entries[i].Message == tt.expectedMsg {
					logEntry = &entries[i]
					break
				}
			}
			if assert.NotNil(t, logEntry, "expected a log entry with message %q", tt.expectedMsg) {
				assert.Equal(t, tt.expectedLevel, logEntry.Level, "unexpected log level")

				logCtx := logEntry.ContextMap()
				if tt.hasErr {
					assert.Contains(t, logCtx, "err", "expected err field in log context")
				}
			}
		})
	}
}

func TestGetRequestLoggerConfig_SilencedPaths(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)

	logger := zap.New(core)
	e := echo.New()
	e.HTTPErrorHandler = CustomHTTPErrorHandler
	loggerConfig := &LoggerConfig{
		Logger:        logger,
		SilencedPaths: []string{"/health"},
	}
	e.Use(middleware.RequestLoggerWithConfig(GetRequestLoggerConfig(loggerConfig)))
	e.GET("/health", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)

	entries := logs.All()
	assert.NotEmpty(t, entries)

	var logEntry *observer.LoggedEntry
	for i := range entries {
		if entries[i].Message == "handled silenced path" {
			logEntry = &entries[i]
			break
		}
	}
	if assert.NotNil(t, logEntry, "expected a silenced path log entry") {
		assert.Equal(t, zap.DebugLevel, logEntry.Level)
	}
}
