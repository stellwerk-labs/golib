package hecho

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	strictecho "github.com/oapi-codegen/runtime/strictmiddleware/echo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testHeaderHandler strictecho.StrictEchoHandlerFunc = func(ctx echo.Context, request interface{}) (response interface{}, err error) {
	return GetHeader(ctx.Request().Context(), "Accept")
}

func TestGetHeaders(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "application/v2+json")
	require.NoError(t, err)
	resp := httptest.NewRecorder()
	eCtx := echo.New().NewContext(req, resp)
	h := RequestHeadersMiddleware(testHeaderHandler, "op-id")
	r, err := h(eCtx, req)
	assert.NoError(t, err)
	assert.Equal(t, "application/v2+json", r)
}

func TestGetHeaders_NoMiddlewareError(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "application/v2+json")
	require.NoError(t, err)
	resp := httptest.NewRecorder()
	eCtx := echo.New().NewContext(req, resp)
	_, err = testHeaderHandler(eCtx, req)
	assert.EqualError(t, err, "echo middleware for using request headers is not activated")
}
