package httplogger

import (
	"net/http"

	"github.com/felixge/httpsnoop"
	"github.com/stellwerk-labs/golib/hlogger"
	"go.uber.org/zap"
)

type Config struct {
	Logger        *zap.Logger
	SilencedPaths []string
}

func LoggingMiddleware(config *Config) func(next http.Handler) http.Handler {
	silencedPathsMap := make(map[string]bool, len(config.SilencedPaths))
	for _, ignoredPath := range config.SilencedPaths {
		silencedPathsMap[ignoredPath] = true
	}
	logger := config.Logger

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// ensure we're tracking platform-orchestrator ids
			ids, ctx := hlogger.EnsurePlatformOrchestratorIdsOnCtx(r.Context())
			r = r.WithContext(ctx)

			m := httpsnoop.CaptureMetricsFn(w, func(w http.ResponseWriter) {
				// Call the next handler, which can be another middleware in the chain, or the final handler.
				next.ServeHTTP(w, r)
			})

			// See https://docs.datadoghq.com/logs/log_configuration/attributes_naming_convention/#http-requests for the naming
			httpDetails := map[string]interface{}{
				"url":           r.URL.String(),
				"status_code":   m.Code,
				"method":        r.Method,
				"useragent":     r.Header.Get("User-Agent"),
				"version":       r.Proto,
				"response_size": m.Written,
			}

			// Duration needs to be nanoseconds https://docs.datadoghq.com/logs/log_configuration/attributes_naming_convention/#performance
			logger := logger.With(ids.AsLogField(), zap.Any("http", httpDetails), zap.Int64("duration", m.Duration.Nanoseconds()))
			args := hlogger.LogDetailsFromCtx(r.Context())

			if silencedPathsMap[r.URL.Path] && m.Code >= 200 && m.Code < 300 {
				logger.Sugar().Debugw("handled silenced path", args...)
				return
			}

			switch {
			case m.Code < 100 || m.Code >= 500:
				logger.Sugar().Errorw("handled", args...)
			case m.Code >= 400:
				logger.Sugar().Warnw("handled", args...)
			default:
				logger.Sugar().Infow("handled", args...)
			}
		})
	}
}
