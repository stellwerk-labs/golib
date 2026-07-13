package hecho

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestDefaultOAIValidationSkipper(t *testing.T) {
	testCases := []struct {
		name   string
		method string
		path   string
		skip   bool
	}{
		{
			name: "true for included",
			path: "/alive",
			skip: true,
		},
		{
			name: "false for others",
			path: "/resource",
			skip: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			c := e.NewContext(nil, nil)
			c.SetPath(tc.path)

			assert.Equal(t, tc.skip, DefaultOAIValidationSkipper(c))
		})
	}
}

func TestOapiValidationPatternError(t *testing.T) {
	v, err := oapiValidatorFromYaml([]byte(`openapi: 3.0.0
info:
  title: ""
  version: ""
paths:
  /{x}/{y}:
    get:
      parameters:
        - name: x
          in: path
          schema:
            type: string
            pattern: ^bar$
            x-pattern-error: must be "bar"
        - name: y
          in: path
          schema:
            type: string
            pattern: ^foo$
      responses:
        "200": {}
`), func(c echo.Context) bool {
		return false
	})
	assert.NoError(t, err)
	e := echo.New()
	e.GET("/:x/:y", func(c echo.Context) error {
		return nil
	})
	e.Use(v)

	t.Run("no error when valid", func(t *testing.T) {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/bar/foo", nil))
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("default error pattern", func(t *testing.T) {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/bar/fuzz", nil))
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, `{"message":"parameter \"y\" in path has an error: string doesn't match the regular expression \"^foo$\""}
`, rec.Body.String())
	})

	t.Run("custom error pattern", func(t *testing.T) {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/fuzz/foo", nil))
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, `{"message":"parameter \"x\" in path has an error: must be \"bar\""}
`, rec.Body.String())
	})

}
