package hecho

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stellwerk-labs/golib/herrors"
	"github.com/stellwerk-labs/golib/hlogger"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/mocktracer"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func executeTestRequest(e *echo.Echo, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	return w
}

func getDefaultEchoServer(skip func(*http.Request) bool) *echo.Echo {
	logger, _ := hlogger.NewTestLogger()
	conf := &ServerConfig{
		AppName:                  "test-app",
		Logger:                   logger.Logger,
		SkipJSONContentTypeCheck: skip,
	}
	e := DefaultEchoServer(conf)
	e.POST("/test", func(c echo.Context) error { return c.JSON(http.StatusOK, nil) })
	return e
}

func getDefaultEchoServerWithValidation(assert *assert.Assertions) *echo.Echo {
	logger, _ := hlogger.NewTestLogger()
	conf := &ValidatedServerConfig{
		AppName:                          "test-app",
		Logger:                           logger.Logger,
		SchemaFile:                       path.Join("./fixtures/openapi.yaml"),
		DefaultJSONInMultipartFormFields: []string{"metadata"},
	}
	e, err := DefaultEchoServerWithValidation(conf)
	assert.NoError(err)

	e.POST("/test", func(c echo.Context) error { return c.JSON(http.StatusOK, nil) })
	e.POST("/test-file", func(c echo.Context) error { return c.JSON(http.StatusOK, nil) })
	return e
}

func TestDefaultEchoServer_XMLPayload(t *testing.T) {
	assert := assert.New(t)
	body := "<request> <parameters> <email>test@test.com</email> <password>test</password> </parameters> </request>"
	req, err := http.NewRequestWithContext(context.TODO(), "POST", "/test", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Errorf("creating request: %v", err)
	}
	req.Header.Add("Content-Type", "text/xml; charset=utf-8")
	e := getDefaultEchoServer(nil)
	resp := executeTestRequest(e, req)
	assert.Equal(http.StatusBadRequest, resp.Code)
}

func TestDefaultEchoServerWithValidation_XMLPayload(t *testing.T) {
	assert := assert.New(t)
	body := "<request> <parameters> <email>test@test.com</email> <password>test</password> </parameters> </request>"
	req, err := http.NewRequestWithContext(context.TODO(), "POST", "/test", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Errorf("creating request: %v", err)
	}
	req.Header.Add("Content-Type", "text/xml; charset=utf-8")
	e := getDefaultEchoServerWithValidation(assert)
	resp := executeTestRequest(e, req)
	assert.Equal(http.StatusBadRequest, resp.Code)
	assert.Equal(`{"error":"HTTP-400","message":"request body has an error: header Content-Type has unexpected value \"text/xml; charset=utf-8\""}`, strings.TrimSpace(resp.Body.String()))
}

func TestDefaultEchoServerWithValidation_FormData(t *testing.T) {
	assert := assert.New(t)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file part
	filePart, err := writer.CreateFormFile("file", "stub.txt")
	assert.NoError(err)
	_, err = filePart.Write([]byte("stub"))
	assert.NoError(err)

	// Add JSON part
	metadataJSON, err := json.Marshal(map[string]interface{}{"name": "value"})
	assert.NoError(err)
	jsonPart, err := writer.CreateFormField("metadata")
	assert.NoError(err)
	_, err = jsonPart.Write(metadataJSON)
	assert.NoError(err)

	assert.NoError(writer.Close())

	req, err := http.NewRequestWithContext(context.TODO(), "POST", "/test-file", body)
	assert.NoError(err)
	req.Header.Add("Content-Type", writer.FormDataContentType())
	e := getDefaultEchoServerWithValidation(assert)
	resp := executeTestRequest(e, req)
	assert.Equal(http.StatusOK, resp.Code)
}

func TestDefaultEchoServerWithValidation_FormDataWithoutFileContentType(t *testing.T) {
	assert := assert.New(t)

	body := strings.Replace(`--NO_CONTENT_TYPE
Content-Disposition: form-data; name="file"; filename="stub.txt"

stub
--NO_CONTENT_TYPE
Content-Disposition: form-data; name="metadata"

{"name":"value"}
--NO_CONTENT_TYPE--`, "\n", "\r\n", -1)

	req, err := http.NewRequestWithContext(context.TODO(), "POST", "/test-file", bytes.NewReader([]byte(body)))
	assert.NoError(err)
	req.Header.Add("Content-Type", "multipart/form-data; boundary=NO_CONTENT_TYPE")
	e := getDefaultEchoServerWithValidation(assert)
	resp := executeTestRequest(e, req)
	assert.Equal(http.StatusOK, resp.Code)
}

func TestDefaultEchoServer_XMLPayload_Skipped(t *testing.T) {
	assert := assert.New(t)
	body := "<request> <parameters> <email>test@test.com</email> <password>test</password> </parameters> </request>"
	req, err := http.NewRequestWithContext(context.TODO(), "POST", "/test", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Errorf("creating request: %v", err)
	}
	req.Header.Add("Content-Type", "text/xml; charset=utf-8")
	skip := func(req *http.Request) bool {
		return req.URL.Path == "/test" && req.Method == "POST"
	}
	e := getDefaultEchoServer(skip)
	resp := executeTestRequest(e, req)
	assert.Equal(http.StatusOK, resp.Code)
}

func TestDefaultEchoServer_JSONPayload(t *testing.T) {
	assert := assert.New(t)
	body := `{"key": "value"}`
	req, err := http.NewRequestWithContext(context.TODO(), "POST", "/test", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Errorf("creating request: %v", err)
	}
	req.Header.Add("Content-Type", "application/json")
	e := getDefaultEchoServer(nil)

	resp := executeTestRequest(e, req)
	assert.Equal(http.StatusOK, resp.Code)
}

func TestDefaultEchoServerWithValidation_JSONPayload(t *testing.T) {
	assert := assert.New(t)
	body := `{"key": "value"}`
	req, err := http.NewRequestWithContext(context.TODO(), "POST", "/test", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Errorf("creating request: %v", err)
	}
	req.Header.Add("Content-Type", "application/json")
	e := getDefaultEchoServerWithValidation(assert)
	resp := executeTestRequest(e, req)
	assert.Equal(http.StatusOK, resp.Code)
}

func TestDefaultEchoServer_EmptyPayloadAllowed(t *testing.T) {
	assert := assert.New(t)
	req, err := http.NewRequestWithContext(context.TODO(), "POST", "/test", nil)
	if err != nil {
		t.Errorf("creating request: %v", err)
	}
	e := getDefaultEchoServer(nil)

	resp := executeTestRequest(e, req)
	assert.Equal(http.StatusOK, resp.Code)
}

func TestDefaultEchoServerWithValidation_EmptyPayload(t *testing.T) {
	assert := assert.New(t)
	req, err := http.NewRequestWithContext(context.TODO(), "POST", "/test", nil)
	if err != nil {
		t.Errorf("creating request: %v", err)
	}
	req.Header.Add("Content-Type", "application/json")
	e := getDefaultEchoServerWithValidation(assert)
	resp := executeTestRequest(e, req)
	assert.Equal(http.StatusBadRequest, resp.Code)
	assert.Equal(`{"error":"HTTP-400","message":"request body has an error: value is required but missing"}`, strings.TrimSpace(resp.Body.String()))
}

func TestDefaultEchoServerWithValidation_DefaultSkipped(t *testing.T) {
	assert := assert.New(t)
	req, err := http.NewRequestWithContext(context.TODO(), "GET", "/alive", nil)
	if err != nil {
		t.Errorf("creating request: %v", err)
	}
	e := getDefaultEchoServerWithValidation(assert)
	e.GET("/alive", func(c echo.Context) error {
		return nil
	})
	resp := executeTestRequest(e, req)
	assert.Equal(http.StatusOK, resp.Code)
	fmt.Println(resp.Body.String())
}

func TestCustomHTTPErrorHandler(t *testing.T) {
	assert := assert.New(t)
	req, err := http.NewRequestWithContext(context.TODO(), "GET", "/test", nil)
	if err != nil {
		t.Errorf("creating request: %v", err)
	}
	e := getDefaultEchoServer(nil)
	e.GET("/test", func(c echo.Context) error {
		return &herrors.PlatformOrchestratorError{
			StatusCode: http.StatusConflict,
			Message:    "conflict",
		}
	})

	resp := executeTestRequest(e, req)
	assert.Equal(http.StatusConflict, resp.Code)
	assert.Equal(`{"error":"HTTP-409","message":"conflict"}`, strings.TrimSpace(resp.Body.String()))
}

func TestCustomHTTPErrorHandler_MethodNotAllowed(t *testing.T) {
	assert := assert.New(t)
	req, err := http.NewRequestWithContext(context.TODO(), "PATCH", "/test", nil)
	if err != nil {
		t.Errorf("creating request: %v", err)
	}
	e := getDefaultEchoServer(nil)
	e.GET("/test", func(c echo.Context) error {
		return nil
	})

	resp := executeTestRequest(e, req)
	assert.Equal(http.StatusMethodNotAllowed, resp.Code)
	assert.Equal(`{"error":"HTTP-405","message":"Method Not Allowed"}`, strings.TrimSpace(resp.Body.String()))
}

func TestCustomHTTPErrorHandler_NoMessage(t *testing.T) {
	assert := assert.New(t)
	req, err := http.NewRequestWithContext(context.TODO(), "GET", "/test", nil)
	if err != nil {
		t.Errorf("creating request: %v", err)
	}
	e := getDefaultEchoServer(nil)
	e.GET("/test", func(c echo.Context) error {
		return &herrors.PlatformOrchestratorError{
			StatusCode: http.StatusInternalServerError,
		}
	})

	resp := executeTestRequest(e, req)
	assert.Equal(http.StatusInternalServerError, resp.Code)
	assert.Equal(`{"error":"HTTP-500","message":"Unexpected error"}`, strings.TrimSpace(resp.Body.String()))
}

func TestCustomHTTPErrorHandler_InvalidJSON(t *testing.T) {
	assert := assert.New(t)
	req, err := http.NewRequestWithContext(context.TODO(), "POST", "/test", bytes.NewReader([]byte("{ -- }")))
	if err != nil {
		t.Errorf("creating request: %v", err)
	}
	e := getDefaultEchoServer(nil)
	e.POST("/test", func(c echo.Context) error {
		u := map[string]interface{}{}
		if err := c.Bind(u); err != nil {
			return err
		}
		return nil
	})

	resp := executeTestRequest(e, req)
	assert.Equal(http.StatusBadRequest, resp.Code)
	assert.Equal(`{"error":"HTTP-400","message":"Syntax error: offset=3, error=invalid character '-' looking for beginning of object key string"}`, strings.TrimSpace(resp.Body.String()))
}

func TestRequestLogger(t *testing.T) {
	assert := assert.New(t)
	observer, logs := observer.New(zap.InfoLevel)
	conf := &ServerConfig{
		AppName: "test-app",
		Logger:  zap.New(observer),
	}
	e := DefaultEchoServer(conf)
	e.GET("/test", func(c echo.Context) error {
		ids, _ := hlogger.EnsurePlatformOrchestratorIdsOnCtx(c.Request().Context())
		ids.OrgId = "test-org"
		return c.JSON(http.StatusOK, nil)
	})

	req, err := http.NewRequestWithContext(context.TODO(), "GET", "/test", nil)
	if err != nil {
		t.Errorf("creating request: %v", err)
	}

	resp := executeTestRequest(e, req)
	assert.Equal(http.StatusOK, resp.Code)

	logged := logs.All()
	assert.Len(logged, 1)

	log := logged[0]
	assert.Equal("handled", log.Message)
	logCtx := log.ContextMap()
	assert.Equal("test-org", logCtx["po-org-id"])
	assert.Equal(logCtx["http"], map[string]interface{}{
		"method":        "GET",
		"response_size": int64(5),
		"status_code":   200,
		"url":           "",
		"useragent":     "",
		"version":       "HTTP/1.1",
	})
	assert.Greater(logCtx["duration"], int64(1))
}

func TestRequestLogger_PlainError(t *testing.T) {
	assert := assert.New(t)
	observer, logs := observer.New(zap.InfoLevel)
	conf := &ServerConfig{
		AppName: "test-app",
		Logger:  zap.New(observer),
	}
	e := DefaultEchoServer(conf)
	e.GET("/test", func(c echo.Context) error { return errors.New("plain error") })

	req, err := http.NewRequestWithContext(context.TODO(), "GET", "/test", nil)
	if err != nil {
		t.Errorf("creating request: %v", err)
	}

	resp := executeTestRequest(e, req)
	assert.Equal(http.StatusInternalServerError, resp.Code)

	logged := logs.All()
	assert.Len(logged, 1)

	log := logged[0]
	assert.Equal("handled", log.Message)
	logCtx := log.ContextMap()
	assert.Equal(map[string]interface{}{
		"method":        "GET",
		"response_size": int64(50),
		"status_code":   500,
		"url":           "",
		"useragent":     "",
		"version":       "HTTP/1.1",
	}, logCtx["http"])
	assert.Equal(map[string]interface{}{
		"err": "plain error",
	}, logCtx["err"])
	assert.Greater(logCtx["duration"], int64(1))
}

func TestRequestLogger_PlatformOrchestratorError(t *testing.T) {
	assert := assert.New(t)
	observer, logs := observer.New(zap.InfoLevel)
	conf := &ServerConfig{
		AppName: "test-app",
		Logger:  zap.New(observer),
	}
	e := DefaultEchoServer(conf)
	e.GET("/test", func(c echo.Context) error {
		return &herrors.PlatformOrchestratorError{
			StatusCode: http.StatusInternalServerError,
			Err:        errors.New("plain error"),
			Message:    "platform-orchestrator error",
		}
	})

	req, err := http.NewRequestWithContext(context.TODO(), "GET", "/test", nil)
	if err != nil {
		t.Errorf("creating request: %v", err)
	}

	resp := executeTestRequest(e, req)
	assert.Equal(http.StatusInternalServerError, resp.Code)

	logged := logs.All()
	assert.Len(logged, 1)

	log := logged[0]
	assert.Equal("handled", log.Message)
	logCtx := log.ContextMap()
	assert.Equal(map[string]interface{}{
		"method":        "GET",
		"response_size": int64(61),
		"status_code":   500,
		"url":           "",
		"useragent":     "",
		"version":       "HTTP/1.1",
	}, logCtx["http"])
	assert.Equal(map[string]interface{}{
		"err": "HTTP-500: platform-orchestrator error: plain error\ncode=500, message=Internal Server Error",
	}, logCtx["err"])
	assert.Greater(logCtx["duration"], int64(1))
}

func TestRequestLogger_EchoHTTPError(t *testing.T) {
	assert := assert.New(t)
	observer, logs := observer.New(zap.InfoLevel)
	conf := &ServerConfig{
		AppName: "test-app",
		Logger:  zap.New(observer),
	}
	e := DefaultEchoServer(conf)
	echoErr := &echo.HTTPError{
		Code:    http.StatusInternalServerError,
		Message: "unexpected error",
	}
	e.GET("/test", func(c echo.Context) error {
		return echoErr
	})

	req, err := http.NewRequestWithContext(context.TODO(), "GET", "/test", nil)
	if err != nil {
		t.Errorf("creating request: %v", err)
	}

	resp := executeTestRequest(e, req)
	assert.Equal(http.StatusInternalServerError, resp.Code)
	var humErr *herrors.PlatformOrchestratorError
	err = json.Unmarshal(resp.Body.Bytes(), &humErr)
	assert.NoError(err)
	expectedHumErr, err := herrorToJson(herrors.NewWithStatus(echoErr.Code, echoErr.Message.(string), echoErr))
	assert.NoError(err)
	assert.Equal(expectedHumErr, humErr)

	logged := logs.All()
	assert.Len(logged, 1)

	log := logged[0]
	assert.Equal("handled", log.Message)
	logCtx := log.ContextMap()
	assert.Equal(map[string]interface{}{
		"method":        "GET",
		"response_size": int64(50),
		"status_code":   500,
		"url":           "",
		"useragent":     "",
		"version":       "HTTP/1.1",
	}, logCtx["http"])
	assert.Equal(map[string]interface{}{
		"err": "code=500, message=unexpected error",
	}, logCtx["err"])
	assert.Greater(logCtx["duration"], int64(1))
}

func TestRequestLogger_PlatformOrchestratorError_ForbiddenTrace403InSpan(t *testing.T) {
	assert := assert.New(t)
	mt := mocktracer.Start()
	defer mt.Stop()
	logger, _ := hlogger.NewTestLogger()

	conf := &ServerConfig{
		AppName: "test-app",
		Logger:  logger.Logger,
	}
	e := DefaultEchoServer(conf)

	humErr := &herrors.PlatformOrchestratorError{
		Code:       "RES-105",
		Err:        errors.New("forbidden"),
		Message:    "platform-orchestrator error",
		StatusCode: 403,
	}
	e.GET("/test", func(c echo.Context) error {
		return humErr
	})

	root := tracer.StartSpan("root")
	req, err := http.NewRequestWithContext(context.TODO(), "GET", "/test", nil)
	if err != nil {
		t.Errorf("creating request: %v", err)
	}
	err = tracer.Inject(root.Context(), tracer.HTTPHeadersCarrier(req.Header))
	assert.Nil(err)

	resp := executeTestRequest(e, req)
	assert.Equal(http.StatusForbidden, resp.Code)
	respHumErr := &herrors.PlatformOrchestratorError{}
	err = json.Unmarshal(resp.Body.Bytes(), respHumErr)
	assert.NoError(err)
	expectedHumErr, err := herrorToJson(humErr.WithConventions())
	assert.NoError(err)
	assert.Equal(expectedHumErr, respHumErr)

	spans := mt.FinishedSpans()
	assert.Len(spans, 1)

	span := spans[0]
	assert.Equal("http.request", span.OperationName())
	assert.Equal(ext.SpanTypeWeb, span.Tag(ext.SpanType))
	assert.Equal("test-app", span.Tag(ext.ServiceName))
	assert.Contains(span.Tag(ext.ResourceName), "/test")
	// Every non echo http error is treated as 500
	// https://github.com/DataDog/dd-trace-go/blob/main/contrib/labstack/echo.v4/echotrace.go
	assert.Equal("403", span.Tag(ext.HTTPCode))
	assert.Equal("GET", span.Tag(ext.HTTPMethod))
	assert.Equal(root.Context().SpanID(), span.ParentID())
	assert.Equal("labstack/echo.v4", span.Tag(ext.Component))
	assert.Equal(ext.SpanKindServer, span.Tag(ext.SpanKind))
}

func TestRequestLogger_PlatformOrchestratorErrorWrapped_ForbiddenTrace403InSpan(t *testing.T) {
	assert := assert.New(t)
	mt := mocktracer.Start()
	defer mt.Stop()
	logger, _ := hlogger.NewTestLogger()

	conf := &ServerConfig{
		AppName: "test-app",
		Logger:  logger.Logger,
	}
	e := DefaultEchoServer(conf)

	wrappedHumErr := &herrors.PlatformOrchestratorError{
		Code:    "RES-105",
		Err:     errors.New("forbidden"),
		Message: "platform-orchestrator error",
	}
	e.GET("/test", func(c echo.Context) error {
		return &echo.HTTPError{
			Code:     http.StatusForbidden,
			Message:  "jwt not provided",
			Internal: wrappedHumErr,
		}
	})

	root := tracer.StartSpan("root")
	req, err := http.NewRequestWithContext(context.TODO(), "GET", "/test", nil)
	if err != nil {
		t.Errorf("creating request: %v", err)
	}
	err = tracer.Inject(root.Context(), tracer.HTTPHeadersCarrier(req.Header))
	assert.Nil(err)

	resp := executeTestRequest(e, req)
	assert.Equal(http.StatusForbidden, resp.Code)
	var humErr *herrors.PlatformOrchestratorError
	err = json.Unmarshal(resp.Body.Bytes(), &humErr)
	assert.NoError(err)
	expectedHumErr, err := herrorToJson(wrappedHumErr.WithConventions())
	assert.NoError(err)
	assert.Equal(expectedHumErr, humErr)

	spans := mt.FinishedSpans()
	assert.Len(spans, 1)

	span := spans[0]
	assert.Equal("http.request", span.OperationName())
	assert.Equal(ext.SpanTypeWeb, span.Tag(ext.SpanType))
	assert.Equal("test-app", span.Tag(ext.ServiceName))
	assert.Contains(span.Tag(ext.ResourceName), "/test")
	// Every non echo http error is treated as 500
	// https://github.com/DataDog/dd-trace-go/blob/main/contrib/labstack/echo.v4/echotrace.go
	assert.Equal("403", span.Tag(ext.HTTPCode))
	assert.Equal("GET", span.Tag(ext.HTTPMethod))
	assert.Equal(root.Context().SpanID(), span.ParentID())
	assert.Equal("labstack/echo.v4", span.Tag(ext.Component))
	assert.Equal(ext.SpanKindServer, span.Tag(ext.SpanKind))
}

func TestRequestLogger_ClientCanceled(t *testing.T) {
	assert := assert.New(t)
	observer, logs := observer.New(zap.InfoLevel)
	conf := &ServerConfig{
		AppName: "test-app",
		Logger:  zap.New(observer),
	}
	e := DefaultEchoServer(conf)
	e.GET("/test", func(c echo.Context) error {
		ctx := c.Request().Context()
		select {
		case <-ctx.Done(): //context cancelled
		case <-time.After(1 * time.Second): //timeout
		}

		return ctx.Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, "GET", "/test", nil)
	if err != nil {
		t.Errorf("creating request: %v", err)
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	resp := executeTestRequest(e, req)
	assert.Equal(499, resp.Code)

	logged := logs.All()
	assert.Len(logged, 1)

	log := logged[0]
	assert.Equal("handled", log.Message)
	logCtx := log.ContextMap()
	assert.Equal(map[string]interface{}{
		"method":        "GET",
		"response_size": int64(55),
		"status_code":   499,
		"url":           "",
		"useragent":     "",
		"version":       "HTTP/1.1",
	}, logCtx["http"])
	assert.Equal(map[string]interface{}{
		"err": "HTTP-499: Client Closed Request: context canceled\ncode=499, message=",
	}, logCtx["err"])
	assert.Greater(logCtx["duration"], int64(1))
}

func herrorToJson(humErr *herrors.PlatformOrchestratorError) (*herrors.PlatformOrchestratorError, error) {
	bytes, err := json.Marshal(humErr)
	if err != nil {
		return nil, err
	}
	var result *herrors.PlatformOrchestratorError
	err = json.Unmarshal(bytes, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

type fakePgErr struct {
	text string
}

func (s *fakePgErr) SQLState() string {
	return ""
}

func (s *fakePgErr) Error() string {
	return "pq: " + s.text
}

func TestDefaultEchoServer_OTelTracing(t *testing.T) {
	assert := assert.New(t)

	// Set up an in-memory OTel tracer provider
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	origTP := otel.GetTracerProvider()
	origProp := otel.GetTextMapPropagator()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	defer func() {
		otel.SetTracerProvider(origTP)
		otel.SetTextMapPropagator(origProp)
	}()

	logger, _ := hlogger.NewTestLogger()
	conf := &ServerConfig{
		AppName: "test-app-otel",
		Logger:  logger.Logger,
		Tracing: TracingOTel,
	}
	e := DefaultEchoServer(conf)
	e.GET("/test", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	e.GET("/health", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	// Normal request should produce a span
	req, err := http.NewRequestWithContext(context.TODO(), "GET", "/test", nil)
	assert.NoError(err)
	resp := executeTestRequest(e, req)
	assert.Equal(http.StatusOK, resp.Code)

	spans := exporter.GetSpans()
	assert.NotEmpty(spans, "expected at least one span for /test")

	// Health endpoint should be skipped
	exporter.Reset()
	req, err = http.NewRequestWithContext(context.TODO(), "GET", "/health", nil)
	assert.NoError(err)
	resp = executeTestRequest(e, req)
	assert.Equal(http.StatusOK, resp.Code)

	spans = exporter.GetSpans()
	assert.Empty(spans, "expected no spans for /health (skipped)")
}

func TestDefaultEchoServerPgCatch(t *testing.T) {
	e := DefaultEchoServer(&ServerConfig{Logger: zaptest.NewLogger(t)})
	e.GET("/other", func(c echo.Context) error {
		return &fakePgErr{text: "blah"}
	})
	e.GET("/utf8", func(c echo.Context) error {
		return &fakePgErr{text: "invalid byte sequence for encoding \"UTF8\": 0x00"}
	})
	resp := executeTestRequest(e, httptest.NewRequest(http.MethodGet, "/other", nil))
	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "{\"error\":\"HTTP-500\",\"message\":\"Unexpected error\"}\n", resp.Body.String())
	resp = executeTestRequest(e, httptest.NewRequest(http.MethodGet, "/utf8", nil))
	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Equal(t, "{\"error\":\"HTTP-400\",\"message\":\"invalid utf-8 byte during request\"}\n", resp.Body.String())
}
