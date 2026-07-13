package hecho

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
)

func TestBuildContextTimeoutMiddleware(t *testing.T) {
	mw := BuildContextTimeoutMiddleware(middleware.ContextTimeoutConfig{
		Timeout: time.Minute,
		Skipper: func(c echo.Context) bool {
			return strings.Contains(c.Path(), "fizz")
		},
		ErrorHandler: func(err error, c echo.Context) error {
			if strings.Contains(c.Path(), "buzz") {
				return fmt.Errorf("was a buzz: %w", err)
			}
			return err
		},
	})

	handler := mw(func(ctx echo.Context, request interface{}) (response interface{}, err error) {
		if _, hasDeadline := ctx.Request().Context().Deadline(); hasDeadline {
			return nil, context.DeadlineExceeded
		}
		return 1, nil
	}, "my-op")
	echoServer := echo.New()

	t.Run("fizz - no timeout", func(t *testing.T) {
		ctx := echoServer.AcquireContext()
		defer echoServer.ReleaseContext(ctx)
		ctx.SetRequest(&http.Request{})
		ctx.SetPath("/fizz/")
		resp, err := handler(ctx, 2)
		if assert.NoError(t, err) {
			assert.Equal(t, 1, resp)
		}
	})

	t.Run("buzz - wrapped timeout", func(t *testing.T) {
		ctx := echoServer.AcquireContext()
		defer echoServer.ReleaseContext(ctx)
		ctx.SetRequest(&http.Request{})
		ctx.SetPath("/buzz/")
		resp, err := handler(ctx, 2)
		assert.EqualError(t, err, "was a buzz: context deadline exceeded")
		assert.Nil(t, resp)
	})

	t.Run("banana - bare timeout", func(t *testing.T) {
		ctx := echoServer.AcquireContext()
		defer echoServer.ReleaseContext(ctx)
		ctx.SetRequest(&http.Request{})
		ctx.SetPath("/banana/")
		resp, err := handler(ctx, 2)
		assert.EqualError(t, err, "context deadline exceeded")
		assert.Nil(t, resp)
	})

}

func TestBuildContextTimeoutMiddleware_minimal_config(t *testing.T) {
	mw := BuildContextTimeoutMiddlewareWithDuration(time.Minute)
	handler := mw(func(ctx echo.Context, request interface{}) (response interface{}, err error) {
		if _, hasDeadline := ctx.Request().Context().Deadline(); hasDeadline {
			return nil, context.DeadlineExceeded
		}
		return 1, nil
	}, "my-op")
	echoServer := echo.New()
	ctx := echoServer.AcquireContext()
	defer echoServer.ReleaseContext(ctx)
	ctx.SetRequest(&http.Request{})
	ctx.SetPath("/banana/")
	resp, err := handler(ctx, 2)
	assert.EqualError(t, err, "context deadline exceeded")
	assert.Nil(t, resp)
}
