package hecho

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	"go.uber.org/zap"

	ddtraceecho "gopkg.in/DataDog/dd-trace-go.v1/contrib/labstack/echo.v4"

	"github.com/stellwerk-labs/golib/herrors"
)

// TracingProvider specifies which tracing backend to use.
type TracingProvider int

const (
	// TracingDatadog uses Datadog's dd-trace-go for tracing (default).
	TracingDatadog TracingProvider = iota
	// TracingOTel uses OpenTelemetry for tracing.
	TracingOTel
)

// ServerConfig configures the Echo server.
type ServerConfig struct {
	AppName                  string
	Logger                   *zap.Logger
	SkipJSONContentTypeCheck func(req *http.Request) bool
	// Tracing specifies which tracing provider to use. Defaults to TracingDatadog.
	Tracing TracingProvider
}

// ValidatedServerConfig configures an Echo server with OpenAPI validation.
type ValidatedServerConfig struct {
	AppName string
	Logger  *zap.Logger
	// Tracing specifies which tracing provider to use. Defaults to TracingDatadog.
	Tracing TracingProvider
	// SchemaFile may contain a filepath from which to read the raw yaml/json OpenAPI schema into OpenAPIRawSchema.
	SchemaFile string
	// OpenAPIRawSchema may contain the raw yaml/json OpenAPI schema that you want to validate against.
	// This is useful when the openapi schema is embedded in the binary rather than read from the file system.
	OpenAPIRawSchema []byte
	// OpenAPISkipperFn returns true for paths that should not validate input structure against the OpenAPIRawSchema.
	OpenAPISkipperFn                 middleware.Skipper
	DefaultJSONInMultipartFormFields []string
}

// BodyBinder binds data only from echo Request Body
// Note: we may need to override echo's Default Binder as it has an issue with request objects of map type:
// https://github.com/labstack/echo/issues/2552
type BodyBinder struct{}

func (b *BodyBinder) Bind(i interface{}, c echo.Context) (err error) {
	return (&echo.DefaultBinder{}).BindBody(c, i)
}

// echoServerBase creates the base Echo server with common middleware.
func echoServerBase(appName string, logger *zap.Logger, tracing TracingProvider) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = CustomHTTPErrorHandler
	e.JSONSerializer = DefaultJSONSerializer{}

	loggerConfig := &LoggerConfig{
		Logger:        logger,
		SilencedPaths: []string{"/health", "/alive"},
	}

	// Add tracing middleware BEFORE the request logger.
	// otelecho restores the original context on defer, so the request logger
	// must be an inner middleware to see the span in the context when LogValuesFunc runs.
	switch tracing {
	case TracingOTel:
		e.Use(otelecho.Middleware(appName,
			otelecho.WithSkipper(func(c echo.Context) bool {
				path := c.Request().URL.Path
				return path == "/health" || path == "/alive"
			}),
		))
	default: // TracingDatadog
		e.Use(ddtraceecho.Middleware(
			ddtraceecho.WithServiceName(appName),
			ddtraceecho.WithErrorTranslator(func(err error) (*echo.HTTPError, bool) {
				if he := new(echo.HTTPError); errors.As(err, &he) {
					return he, true
				} else if he := new(herrors.PlatformOrchestratorError); errors.As(err, &he) {
					return &echo.HTTPError{Code: he.StatusCode}, true
				}
				return nil, false
			}),
		))
	}

	e.Use(middleware.RequestLoggerWithConfig(GetRequestLoggerConfig(loggerConfig)))

	e.Use(ContextCanceled)
	e.Use(PlatformOrchestratorIdsMiddleware)
	e.Use(EnsureParamsInUTF8Middleware)
	e.Use(DefaultJSONContentTypeMiddleware)
	e.Use(TranslatePostgresUtf8ByteErrorsToHttp400)

	e.Binder = &BodyBinder{}

	return e
}

// DefaultEchoServer creates a default Echo server.
// Uses Datadog tracing by default. Set Tracing field to TracingOTel for OpenTelemetry.
func DefaultEchoServer(c *ServerConfig) *echo.Echo {
	e := echoServerBase(c.AppName, c.Logger, c.Tracing)
	e.Use(DefaultJSONContentTypeMiddleware, RejectNoJsonContentTypeMiddleware(c.SkipJSONContentTypeCheck))
	return e
}

// DefaultEchoServerWithValidation creates an Echo server with OpenAPI validation.
// Uses Datadog tracing by default. Set Tracing field to TracingOTel for OpenTelemetry.
func DefaultEchoServerWithValidation(c *ValidatedServerConfig) (*echo.Echo, error) {
	e := echoServerBase(c.AppName, c.Logger, c.Tracing)

	if len(c.DefaultJSONInMultipartFormFields) > 0 {
		e.Use(DefaultJSONInMultipartFormMiddleware(c.DefaultJSONInMultipartFormFields))
	}

	if (c.SchemaFile == "") == (c.OpenAPIRawSchema == nil) {
		return nil, fmt.Errorf("either schema file or raw schema must be provided but not both")
	} else if c.SchemaFile != "" {
		data, err := os.ReadFile(c.SchemaFile)
		if err != nil {
			return nil, fmt.Errorf("error reading schema file from %s: %s", c.SchemaFile, err)
		}
		c.OpenAPIRawSchema = data
	}

	m, err := oapiValidatorFromYaml(c.OpenAPIRawSchema, c.OpenAPISkipperFn)
	if err != nil {
		return nil, err
	}

	e.Use(m)

	return e, nil
}
