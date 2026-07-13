package hecho

import (
	"context"
	"errors"

	"github.com/stellwerk-labs/golib/herrors"
	"github.com/labstack/echo/v4"
)

// ContextCanceled is a middleware that will return a custom error if the client canceled the request.
func ContextCanceled(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if err := next(c); err != nil {
			// if the context is canceled, we will return a custom error
			if errors.Is(err, c.Request().Context().Err()) && errors.Is(err, context.Canceled) {
				return herrors.NewWithStatus(499, "Client Closed Request", err)
			} else {
				return err
			}
		}
		return nil
	}
}

func CustomHTTPErrorHandler(err error, c echo.Context) {
	// if this is an echo http error, we will wrap it as a platform-orchestrator HTTP Error
	httpErr := &echo.HTTPError{}
	isEchoHTTPError := errors.As(err, &httpErr)
	if isEchoHTTPError {
		var message string
		if m, ok := httpErr.Message.(string); ok {
			message = m
		}

		internalHe := &herrors.PlatformOrchestratorError{}
		if httpErr.Internal != nil && errors.As(httpErr.Internal, &internalHe) {
			if httpErr.Code != 0 {
				internalHe.StatusCode = httpErr.Code
			}
			err = internalHe
		} else {
			err = herrors.NewWithStatus(httpErr.Code, message, httpErr)
		}
	}

	// if this is a platform-orchestrator HTTP Error then we will ensure defaults are set, otherwise we wrap it as an internal
	// error.
	he := &herrors.PlatformOrchestratorError{}
	if !errors.As(err, &he) {
		he = herrors.NewInternalError(err)
	} else {
		// Every non echo http error is treated as 500 by the tracing, so need to inject an echo http error here if not present
		// https://github.com/DataDog/dd-trace-go/blob/main/contrib/labstack/echo.v4/echotrace.go
		if !isEchoHTTPError {
			he.Err = errors.Join(he.Err, echo.NewHTTPError(he.StatusCode))
		}

		he = he.WithConventions()
	}

	if !c.Response().Committed {
		_ = c.JSON(he.StatusCode, he)
	}
}
