package hecho

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	strictecho "github.com/oapi-codegen/runtime/strictmiddleware/echo"
)

// RequestHeadersMiddleware middleware for openapi-generated echo server to store request headers in the context.
func RequestHeadersMiddleware(f strictecho.StrictEchoHandlerFunc, operationID string) strictecho.StrictEchoHandlerFunc {
	return func(ctx echo.Context, args interface{}) (interface{}, error) {
		ctx.SetRequest(ctx.Request().WithContext(context.WithValue(ctx.Request().Context(), ContextKeyHeaders, ctx.Request().Header)))
		return f(ctx, args)
	}
}

func GetHeader(ctx context.Context, key string) (string, error) {
	header, ok := ctx.Value(ContextKeyHeaders).(http.Header)
	if !ok {
		return "", errors.New("echo middleware for using request headers is not activated")
	}
	return header.Get(key), nil
}
