package hecho

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/stellwerk-labs/golib/herrors"
)

var nulChar = "\\u0000"

// Adopted from https://github.com/labstack/echo/blob/master/json.go

// DefaultJSONSerializer implements JSON encoding using encoding/json.
type DefaultJSONSerializer struct {
	// UnknownFieldCallback can be used to log errors related to unknown fields in the request objects rather than
	// halting the json deserialization. Note that this double decodes the data. The return value indicates whether
	// the decoder should throw the error or ignore it.
	UnknownFieldCallback func(error) (stop bool)
	// DisableRequestBodyNullCheck can disable the legacy check for an encoded null char in the request body. An encoded
	// UTF-8 null "\u0000" can be correctly decoded by Go's json unmarshal and so can be supported here.
	DisableRequestBodyNullCheck func(c echo.Context) bool
}

// Serialize converts an interface into a json and writes it to the response.
// You can optionally use the indent parameter to produce pretty JSONs.
func (d DefaultJSONSerializer) Serialize(c echo.Context, i interface{}, indent string) error {
	enc := json.NewEncoder(c.Response())
	if indent != "" {
		enc.SetIndent("", indent)
	}
	enc.SetEscapeHTML(false)
	return enc.Encode(i)
}

// Deserialize reads a JSON from a request body and converts it into an interface.
func (d DefaultJSONSerializer) Deserialize(c echo.Context, i interface{}) error {
	req := c.Request()

	if req.Body == nil {
		return &herrors.PlatformOrchestratorError{
			StatusCode: http.StatusBadRequest,
			Message:    "empty request body",
			Code:       "API-000",
		}
	}

	raw, err := io.ReadAll(req.Body)
	if err != nil {
		return &herrors.PlatformOrchestratorError{
			StatusCode: http.StatusBadRequest,
			Message:    "failed to read request body",
			Code:       "API-000",
		}
	}

	// This is an old check in place from circa 2023. Postgres will reject any text fields that contain a \x00 byte with
	// an encoding error. This is weird and unexpected behavior because \x00 is as valid as any other control or
	// extended ascii character. To protect our services from this, we have a check here to reject any request bodies
	// that contain a \u- encoded null (why not the un-encoded byte, I don't know) since request bodies most likely
	// contain text that we will attempt to persist in the database. However, this encoding error will also happen for
	// sql text arguments, which this check does not protect against. We've introduced DisableRequestBodyNullCheck in
	// an attempt to replace this with more meaningful and useful protections.
	if (d.DisableRequestBodyNullCheck == nil || !d.DisableRequestBodyNullCheck(c)) && bytes.Contains(raw, []byte(nulChar)) {
		return &herrors.PlatformOrchestratorError{
			StatusCode: http.StatusBadRequest,
			Message:    "request body contains NUL char",
			Code:       "API-000",
		}
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()

	err = dec.Decode(i)
	if err != nil && strings.Contains(err.Error(), "json: unknown field") && d.UnknownFieldCallback != nil {
		if !d.UnknownFieldCallback(err) {
			dec := json.NewDecoder(bytes.NewReader(raw))
			err = dec.Decode(i)
		}
	}
	if err != nil {
		if ute := (*json.UnmarshalTypeError)(nil); errors.As(err, &ute) {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unmarshal type error: expected=%v, got=%v, field=%v, offset=%v", ute.Type, ute.Value, ute.Field, ute.Offset)).SetInternal(err)
		} else if se := (*json.SyntaxError)(nil); errors.As(err, &se) {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Syntax error: offset=%v, error=%v", se.Offset, se.Error())).SetInternal(err)
		} else {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Parsing error: error=%v", err.Error())).SetInternal(err)
		}
	}

	return nil
}
