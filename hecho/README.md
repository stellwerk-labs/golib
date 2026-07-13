# hecho

Collection of echo middlewares and helpers.

## Usage

```golang
func (s *Server) EchoServer()*echo.Echo {
 e := hecho.DefaultEchoServer(&hecho.ServerConfig{
  AppName: s.AppName,
  Logger:  s.Logger,
 })

 s.MapRoutes(e)

 return e
}

func (s *Server) MapRoutes(e *echo.Echo) *echo.Echo {
 myStrictApiHandler := NewStrictHandler(s, []StrictMiddlewareFunc{hecho.AuthMiddleware[StrictHandlerFunc](UserIdHeaderScopes),
  hecho.EmptyBodyMiddleware[StrictHandlerFunc](skipEmptyBodyOpIds)})
 RegisterHandlers(e, myStrictApiHandler)
 RegisterHandlersForStaticRoutes(e, s)

 return e
}
```

Or with OpenAPI validation:

```golang
func (s *Server) EchoServer()*echo.Echo {
 e := hecho.DefaultEchoServerWithValidation(&hecho.ValidatedServerConfig{
  AppName:    s.AppName,
  Logger:     s.Logger,
  SchemaFile: path.Join("./fixtures/openapi.yaml"),
 })

 s.MapRoutes(e)

 return e
}

func (s *Server) MapRoutes(e *echo.Echo) *echo.Echo {
 myStrictApiHandler := NewStrictHandler(s, []StrictMiddlewareFunc{hecho.AuthMiddleware[StrictHandlerFunc](UserIdHeaderScopes)})
 RegisterHandlers(e, myStrictApiHandler)
 RegisterHandlersForStaticRoutes(e, s)

 return e
}
```

### Tracing Configuration

The server supports two tracing backends. Datadog is used by default for backward compatibility.

#### Using Datadog (default)

```golang
e := hecho.DefaultEchoServer(&hecho.ServerConfig{
    AppName: s.AppName,
    Logger:  s.Logger,
    // Tracing defaults to TracingDatadog
})
```

#### Using OpenTelemetry

```golang
e := hecho.DefaultEchoServer(&hecho.ServerConfig{
    AppName: s.AppName,
    Logger:  s.Logger,
    Tracing: hecho.TracingOTel,
})
```

The same applies to `DefaultEchoServerWithValidation`:

```golang
e, err := hecho.DefaultEchoServerWithValidation(&hecho.ValidatedServerConfig{
    AppName:    s.AppName,
    Logger:     s.Logger,
    SchemaFile: "./fixtures/openapi.yaml",
    Tracing:    hecho.TracingOTel,
})
```

### Strict handler middlewares

```golang
// when creating the handler
apiHandler := NewStrictHandler(s, []StrictMiddlewareFunc{
  hecho.AuthMiddleware[StrictHandlerFunc](UserIdHeaderScopes),
  hecho.EmptyBodyMiddleware[StrictHandlerFunc],
})

// in your code to fetch the user id
userID := hecho.GetUserID(ctx)
```
