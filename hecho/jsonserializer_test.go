package hecho

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

type CustomPayload struct {
	Field string `json:"field"`
}

func TestDefaultJSONSerializer(t *testing.T) {
	assert := assert.New(t)

	d := DefaultJSONSerializer{}
	w := httptest.NewRecorder()

	payload := &CustomPayload{}
	req, err := http.NewRequest("POST", "/test", bytes.NewReader([]byte(`{"Field": "test"}`)))
	assert.NoError(err)

	assert.NoError(d.Deserialize(echo.New().NewContext(req, w), payload))
	assert.Equal(&CustomPayload{
		Field: "test",
	}, payload)
}

func TestDefaultJSONSerializer_UnknownField(t *testing.T) {
	assert := assert.New(t)

	d := DefaultJSONSerializer{}
	w := httptest.NewRecorder()

	payload := &CustomPayload{}
	req, err := http.NewRequest("POST", "/test", bytes.NewReader([]byte(`{"UnknownField": "test"}`)))
	assert.NoError(err)

	assert.Error(d.Deserialize(echo.New().NewContext(req, w), payload))
}

func TestDefaultJSONSerializer_UnknownField_with_callback_false(t *testing.T) {
	d := DefaultJSONSerializer{
		UnknownFieldCallback: func(err error) (stop bool) {
			assert.Error(t, err)
			return false
		},
	}
	w := httptest.NewRecorder()

	payload := &CustomPayload{}
	req, err := http.NewRequest("POST", "/test", bytes.NewReader([]byte(`{"UnknownField": "test"}`)))
	assert.NoError(t, err)
	assert.NoError(t, d.Deserialize(echo.New().NewContext(req, w), payload))
}

func TestDefaultJSONSerializer_UnknownField_with_callback_true(t *testing.T) {
	d := DefaultJSONSerializer{
		UnknownFieldCallback: func(err error) (stop bool) {
			assert.Error(t, err)
			return true
		},
	}
	w := httptest.NewRecorder()

	payload := &CustomPayload{}
	req, err := http.NewRequest("POST", "/test", bytes.NewReader([]byte(`{"UnknownField": "test"}`)))
	assert.NoError(t, err)
	assert.Error(t, d.Deserialize(echo.New().NewContext(req, w), payload))
}

func TestDefaultJSONSerializer_Serialize(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	assert := assert.New(t)
	assert.Equal(e, c.Echo())
	assert.NotNil(c.Request())
	assert.NotNil(c.Response())

	enc := new(DefaultJSONSerializer)

	err := enc.Serialize(c, CustomPayload{"echo 'hello world' > my_file"}, "")
	if assert.NoError(err) {
		assert.Equal(`{"field":"echo 'hello world' > my_file"}`+"\n", rec.Body.String())
	}
}

func TestDefaultJSONSerializer_null_check(t *testing.T) {
	d := DefaultJSONSerializer{}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", bytes.NewReader([]byte(`{"Field": "test\u0000"}`)))
	assert.EqualError(t, d.Deserialize(echo.New().NewContext(req, w), &CustomPayload{}), "API-000: request body contains NUL char")
}

func TestDefaultJSONSerializer_disable_null_check(t *testing.T) {
	d := DefaultJSONSerializer{DisableRequestBodyNullCheck: func(c echo.Context) bool {
		return true
	}}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", bytes.NewReader([]byte(`{"Field": "test\u0000"}`)))
	payload := &CustomPayload{}
	assert.NoError(t, d.Deserialize(echo.New().NewContext(req, w), &payload))
	assert.Equal(t, "test\x00", payload.Field)
}
