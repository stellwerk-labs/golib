package hecho

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"io"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestEnsureParamsInUTF8Middleware(t *testing.T) {
	es := echo.New()
	es.Use(EnsureParamsInUTF8Middleware)
	es.HTTPErrorHandler = CustomHTTPErrorHandler

	es.Router().Add(http.MethodGet, "/o/:orgId/e/:envId/a/:appId/:pipelineId", func(c echo.Context) error {
		return nil
	})

	t.Run("invalid character in params", func(t *testing.T) {
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/o/\xf0/e/my-env/a/my-app/my-pip", nil)
		es.ServeHTTP(resp, req)
		assert.Equal(t, 400, resp.Code)
		body, err := io.ReadAll(resp.Body)
		assert.Nil(t, err)
		assert.Equal(t, "{\"error\":\"HTTP-400\",\"message\":\"invalid UTF-8 in param: \\ufffd\"}\n", string(body))
	})

	t.Run("invalid character in params", func(t *testing.T) {
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/o/my-org/e/my-env/a/my-app/my-pip?arg=\xf0", nil)
		es.ServeHTTP(resp, req)
		assert.Equal(t, 400, resp.Code)
		body, err := io.ReadAll(resp.Body)
		assert.Nil(t, err)
		assert.Equal(t, "{\"error\":\"HTTP-400\",\"message\":\"invalid UTF-8 value in param: arg=\\ufffd\"}\n", string(body))
	})

	t.Run("valid params", func(t *testing.T) {
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/o/my-org/e/my-env/a/my-app/my-pip?arg=valid", nil)
		es.ServeHTTP(resp, req)
		assert.Equal(t, 200, resp.Code)
	})
}
