package hecho

import (
	"fmt"
	"net/http"
	"unicode/utf8"

	"github.com/stellwerk-labs/golib/herrors"
	"github.com/labstack/echo/v4"
)

// EnsureParamsInUTF8Middleware ensures that params and form params are in UTF-8
func EnsureParamsInUTF8Middleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		params := ctx.ParamValues()
		for _, param := range params {
			if !utf8.ValidString(param) {
				return herrors.NewWithStatus(http.StatusBadRequest, fmt.Sprintf("invalid UTF-8 in param: %s", param), nil)
			}
		}

		if err := ctx.Request().ParseForm(); err != nil && err.Error() != "missing form body" {
			return herrors.NewWithStatus(http.StatusBadRequest, fmt.Sprintf("invalid form: %s", err.Error()), nil)
		}
		formParams := ctx.Request().Form
		for key, params := range formParams {
			if !utf8.ValidString(key) {
				return herrors.NewWithStatus(http.StatusBadRequest, fmt.Sprintf("invalid UTF-8 in param key: %s", key), nil)
			}
			for _, param := range params {
				if !utf8.ValidString(param) {
					return herrors.NewWithStatus(http.StatusBadRequest, fmt.Sprintf("invalid UTF-8 value in param: %s=%s", key, param), nil)
				}
			}
		}

		return next(ctx)
	}
}
