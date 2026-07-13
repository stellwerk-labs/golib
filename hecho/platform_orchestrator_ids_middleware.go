package hecho

import (
	"github.com/labstack/echo/v4"

	"github.com/stellwerk-labs/golib/hlogger"
	"github.com/stellwerk-labs/golib/htelemetry"
)

// PlatformOrchestratorIdsMiddleware extracts well known path elements from the request url and populates well known context keys.
// It works with both Datadog and OpenTelemetry backends via htelemetry.
func PlatformOrchestratorIdsMiddleware(f echo.HandlerFunc) echo.HandlerFunc {
	return platformOrchestratorIdsMiddlewareImpl(f)
}

// platformOrchestratorIdsMiddlewareImpl is the shared implementation for platform-orchestrator IDs middleware.
func platformOrchestratorIdsMiddlewareImpl(f echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		ids, subCtx := hlogger.EnsurePlatformOrchestratorIdsOnCtx(ctx.Request().Context())
		ctx.SetRequest(ctx.Request().WithContext(subCtx))
		// if there's a tracing span, make sure we set the standard ids using the provider-agnostic method
		defer ids.SetOnCtxSpan(subCtx)

		// loop through all the path params and apply them if the param name is known
		values := ctx.ParamValues()
		for i, name := range ctx.ParamNames() {
			var value string
			// since customers control this value, let's only include it if it is within a reasonable byte limit
			if i < len(values) && len(values[i]) <= 100 {
				value = values[i]
			}

			// match it to a standard id
			switch name {
			case "orgId":
				ids.OrgId = value
			case "envId":
				ids.EnvId = value
			case "deployId":
				ids.DeployId = value
			}
		}

		// for any of the standard fields extracted from the path, lets add them to the span context as well
		// using the provider-agnostic htelemetry interface
		span, ok := htelemetry.SpanFromContext(ctx.Request().Context())
		if ok {
			for _, field := range ids.AsLogFields() {
				span.SetTag(field.Key, field.String)
			}
		}

		return f(ctx)
	}
}
