# herrors

`herrors` provides a common structure for packaging http errors returned over our APIs. Methods are provided
to ensure the result matches our API Guidelines for error responses.

## Usage examples

### Construction

```golang
herr := NewWithStatus(404, "resource not found", nil)

herr = NewWithStatusAndDetails(409, "a conflict occurred", nil, map[string]interface{}{
    "other_resource_id": "foo",
})

// The error code can be overridden after construction to return custom error codes with more semantic meaning
herr.Code = "RES-999"

herr = NewInternalError(errors.New("panic!"))

// deprecated form (doesn't allow status code to be set)
herr = New("HTTP-123", "More details", map[string]interface{}{}, errors.New("foo"))
```

### HTTP Server

```golang
func handle(w http.ResponseWriter, r *http.Request) {
    logger := hlogger.TraceScopedLoggerCtx(s.Logger, ctx).Sugar()

    herrors.NewInternalError(errors.New("blah")).WriteToHttpWithErrLogger(w, logger.Errorw)
}
```

Would render as

```
{
  "error": "HTTP-500",
  "message": "Unexpected error",
}
```

### Tracing errors

The struct carries an `Err` attribute which points to the underlying error that caused this http-level error. Since
this field is internal and not rendered in the output it's useful to ensure that this is logged somewhere before the
request finishes.

When using the `hecho` utilities, this should be done automatically using the default request logger, but if you're
only reliant on the Go standard library HTTP handlers, you should use `PlatformOrchestratorError.WriteToHttpWithErrLogger` to
ensure the response is marshalled correctly and that the `Err` is captured.
