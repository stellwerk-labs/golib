package hecho

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestEmptyBodyMiddleware(t *testing.T) {
	assert := assert.New(t)

	middleware := EmptyBodyMiddleware([]string{"skipped"})

	fn := func(ctx echo.Context, args interface{}) (interface{}, error) {
		return args, nil
	}

	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/test", bytes.NewReader([]byte(`no empty`)))
	assert.NoError(err)

	_, err = middleware(fn, "test")(echo.New().NewContext(req, w), "test")
	assert.NoError(err)
}

func TestEmptyBodyMiddleware_EmptyBody(t *testing.T) {
	assert := assert.New(t)

	middleware := EmptyBodyMiddleware([]string{"skipped"})

	fn := func(ctx echo.Context, args interface{}) (interface{}, error) {
		return args, nil
	}

	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/test", nil)
	assert.NoError(err)

	_, err = middleware(fn, "test")(echo.New().NewContext(req, w), "test")
	assert.Error(err)
}

func TestEmptyBodyMiddleware_EmptyBodySkipped(t *testing.T) {
	assert := assert.New(t)

	middleware := EmptyBodyMiddleware([]string{"skipped"})

	fn := func(ctx echo.Context, args interface{}) (interface{}, error) {
		return args, nil
	}

	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/test", nil)
	assert.NoError(err)

	_, err = middleware(fn, "skipped")(echo.New().NewContext(req, w), "test")
	assert.NoError(err)
}

func TestEnsureJSONContentTypeInFormParts(t *testing.T) {
	assert := assert.New(t)

	input := `--BOUNDARY
Content-Disposition: form-data; name="file"; filename="stub.txt"

stub
--BOUNDARY
Content-Disposition: form-data; name="metadata"

{"name":"value"}
--BOUNDARY--`

	res, err := ensureJSONContentTypeInFormParts(multipart.NewReader(strings.NewReader(input), "BOUNDARY"), "BOUNDARY", toLookupMap([]string{"metadata"}))
	assert.NoError(err)

	expected := strings.Replace(`--BOUNDARY
Content-Disposition: form-data; name="file"; filename="stub.txt"

stub
--BOUNDARY
Content-Disposition: form-data; name="metadata"
Content-Type: application/json

{"name":"value"}
--BOUNDARY--
`, "\n", "\r\n", -1)

	assert.Equal(expected, string(res))
}
