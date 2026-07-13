package hecho

import (
	"context"

	"github.com/labstack/echo/v4"
	strictecho "github.com/oapi-codegen/runtime/strictmiddleware/echo"
)

// OperationIdCollectorMiddleware middle ware for use with openapi generated echo servers to store the operation id on
// the context so that the http logger may extract it.
func OperationIdCollectorMiddleware(f strictecho.StrictEchoHandlerFunc, operationID string) strictecho.StrictEchoHandlerFunc {
	return func(ctx echo.Context, args interface{}) (interface{}, error) {
		ctx.SetRequest(ctx.Request().WithContext(context.WithValue(ctx.Request().Context(), ContextKeyOperationID, operationID)))
		return f(ctx, args)
	}
}

// GetOperationId retrieves the operation id if defined by the OperationIdCollectorMiddleware
func GetOperationId(ctx context.Context) (string, bool) {
	if op, ok := ctx.Value(ContextKeyOperationID).(string); ok {
		return op, true
	}
	return "", false
}
