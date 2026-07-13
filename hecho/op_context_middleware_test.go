package hecho

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	strictecho "github.com/oapi-codegen/runtime/strictmiddleware/echo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOperationId(t *testing.T) {
	var h strictecho.StrictEchoHandlerFunc = func(ctx echo.Context, request interface{}) (response interface{}, err error) {
		op, ok := GetOperationId(ctx.Request().Context())
		if !ok {
			return nil, fmt.Errorf("no value")
		}
		return nil, fmt.Errorf("value: %v", op)
	}
	h = OperationIdCollectorMiddleware(h, "my-op")

	req, err := http.NewRequest(http.MethodGet, "/", nil)
	require.NoError(t, err)
	resp := httptest.NewRecorder()
	eCtx := echo.New().NewContext(req, resp)
	_, err = h(eCtx, nil)
	assert.EqualError(t, err, "value: my-op")
}
