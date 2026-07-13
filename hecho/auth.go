package hecho

import (
	"context"
	"errors"
	"net/http"

	"github.com/stellwerk-labs/golib/herrors"
	"github.com/labstack/echo/v4"
	strictecho "github.com/oapi-codegen/runtime/strictmiddleware/echo"
)

var ErrUnauthorized = errors.New("unauthorized")

type contextKey int

const (
	ContextKeyUserID contextKey = iota
	ContextKeyOperationID
	ContextKeyHeaders
)

func GetUserID(ctx context.Context) string {
	if userID := ctx.Value(ContextKeyUserID); userID == nil {
		panic(errors.New("try to access userId for an endpoint which has no security header specified, did you miss it?"))
	} else {
		return userID.(string)
	}
}

// AuthMiddleware will ensure that requests have been authenticated and contain an HTTP 'From' header with a user id. The userIdHeaderScopes should be
// the name of the parameter or scope which contains the value.
func AuthMiddleware(userIdHeaderScopes string) strictecho.StrictEchoMiddlewareFunc {
	return func(f strictecho.StrictEchoHandlerFunc, operationID string) strictecho.StrictEchoHandlerFunc {
		return func(ctx echo.Context, args interface{}) (interface{}, error) {
			if ctx.Get(userIdHeaderScopes) != nil {
				userID := ctx.Request().Header.Get("From")
				if userID == "" {
					return nil, herrors.NewWithStatus(http.StatusUnauthorized, ErrUnauthorized.Error(), nil)
				}
				ctx.SetRequest(ctx.Request().WithContext(context.WithValue(ctx.Request().Context(), ContextKeyUserID, userID)))

			}
			return f(ctx, args)
		}
	}
}
