package hecho

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/stellwerk-labs/golib/herrors"
	"github.com/labstack/echo/v4"
	strictecho "github.com/oapi-codegen/runtime/strictmiddleware/echo"
)

func DefaultJSONContentTypeMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if c.Request().Header.Get(echo.HeaderContentType) == "" {
			c.Request().Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		}

		return next(c)
	}
}

func toLookupMap(slice []string) map[string]bool {
	m := make(map[string]bool, len(slice))
	for _, id := range slice {
		m[id] = true
	}

	return m
}

func ensureJSONContentTypeInFormParts(reader *multipart.Reader, boundary string, filenames map[string]bool) ([]byte, error) {
	newBody := &bytes.Buffer{}
	writer := multipart.NewWriter(newBody)
	writer.SetBoundary(boundary)

	defer writer.Close()

	for {
		srcPart, err := reader.NextRawPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read next part: %w", err)
		}

		if filenames[srcPart.FormName()] && srcPart.Header.Get(echo.HeaderContentType) == "" {
			srcPart.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		}

		targetPart, err := writer.CreatePart(srcPart.Header)
		if err != nil {
			return nil, fmt.Errorf("failed to create target part: %w", err)
		}

		if _, err := io.Copy(targetPart, srcPart); err != nil {
			return nil, fmt.Errorf("failed to copy part content: %w", err)
		}
		if err := srcPart.Close(); err != nil {
			return nil, fmt.Errorf("failed to close source part: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}

	return newBody.Bytes(), nil
}

// DefaultJSONInMultipartFormMiddleware ensures that all parts of a multipart form request have a JSON content type,
// which is required to handle legacy requests that do not set the content type.
func DefaultJSONInMultipartFormMiddleware(fieldNameSlice []string) echo.MiddlewareFunc {
	fieldNames := toLookupMap(fieldNameSlice)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()

			reqContentType := req.Header.Get(echo.HeaderContentType)
			if !strings.HasPrefix(reqContentType, echo.MIMEMultipartForm) || req.Body == http.NoBody || req.Body == nil {
				return next(c)
			}

			var err error

			d, params, err := mime.ParseMediaType(reqContentType)
			if err != nil || !(d == "multipart/form-data") {
				return http.ErrNotMultipart
			}
			boundary, ok := params["boundary"]
			if !ok {
				return http.ErrMissingBoundary
			}

			newBody, err := ensureJSONContentTypeInFormParts(multipart.NewReader(req.Body, boundary), boundary, fieldNames)
			if err != nil {
				return fmt.Errorf("ensureJSONContentTypeInFormParts: %w", err)
			}

			req.Body = io.NopCloser(bytes.NewReader(newBody))
			req.ContentLength = int64(len(newBody))

			return next(c)
		}
	}
}

func RejectUnexpectedContentTypeMiddleware(skip func(req *http.Request) bool, contentType string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			if skip == nil || !skip(req) {
				reqContentType := req.Header.Get(echo.HeaderContentType)
				if !strings.HasPrefix(reqContentType, contentType) {
					return herrors.NewWithStatus(
						http.StatusBadRequest,
						fmt.Sprintf("wrong content type, %s expected: %s", reqContentType, contentType),
						nil,
					)
				}
			}
			return next(c)
		}
	}
}

func RejectNoJsonContentTypeMiddleware(skip func(req *http.Request) bool) echo.MiddlewareFunc {
	return RejectUnexpectedContentTypeMiddleware(skip, echo.MIMEApplicationJSON)
}

func EmptyBodyMiddleware(skipOperationIDs []string) strictecho.StrictEchoMiddlewareFunc {
	skip := toLookupMap(skipOperationIDs)

	return func(f strictecho.StrictEchoHandlerFunc, operationID string) strictecho.StrictEchoHandlerFunc {
		return func(ctx echo.Context, args interface{}) (interface{}, error) {
			if skip[operationID] {
				return f(ctx, args)
			}

			req := ctx.Request()
			if req.Method != "POST" && req.Method != "PUT" && req.Method != "PATCH" {
				return f(ctx, args)
			}

			if req.Body == nil {
				return nil, herrors.NewWithStatus(http.StatusBadRequest, "empty request body", nil)
			}
			return f(ctx, args)
		}
	}
}
