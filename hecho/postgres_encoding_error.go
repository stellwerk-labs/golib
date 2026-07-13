package hecho

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/stellwerk-labs/golib/herrors"
	"github.com/stellwerk-labs/golib/hlogger"
)

// possiblePgErr matches the pgx/pg.Err type without us importing it here.
type possiblePgErr interface {
	error
	SQLState() string
}

const (
	pgUtf8EncodingErrPrefix = `pq: invalid byte sequence for encoding "UTF8":`
)

// TranslatePostgresUtf8ByteErrorsToHttp400 catches errors that would otherwise be a 500 and returns them as a more
// useful 400 message. This is a universal thing that can happen if users pass weird request body or query param data
// and can be difficult to validate on an individual field by field basis. This middleware helps us catch this
// for all services. The main culprit of this bug, is when users insert a null byte \x00 into a string field.
func TranslatePostgresUtf8ByteErrorsToHttp400(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if err := next(c); err != nil {
			if ppe := (possiblePgErr)(nil); errors.As(err, &ppe) {
				if strings.HasPrefix(ppe.Error(), pgUtf8EncodingErrPrefix) {
					// Do our best to log the error. We use the global zap logger here which should always exist and has hopefully been
					// overridden to point to the customised logger.
					hlogger.TraceScopedLoggerFromCtx(zap.L(), c.Request().Context()).Warn("catching invalid utf8 encoding error", zap.Error(err))
					return herrors.NewWithStatus(http.StatusBadRequest, "invalid utf-8 byte during request", nil)
				}
			}
			return err
		}
		return nil
	}
}
