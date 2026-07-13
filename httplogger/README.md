# httplogger

Logs information about handled http requests in a structured way.

## Usage

* `SilencedPaths` are only logged with `LOG_LEVEL=debug` or when the response status is not 2XX.

```golang
router := muxtrace.NewRouter()
// add routes as usual
router.Use(httplogger.LoggingMiddleware(&httplogger.Config{
  Logger:        logger,
  SilencedPaths: []string{"/alive", "/health"},
}))
```
