# hlogger

Is a small wrapper around [zap](https://github.com/uber-go/zap).

## Usage

```golang
// Inside the app code
logw, err := hlogger.NewLogger()
if err != nil {
  log.Fatalf("Error building logger: %v (%s)", err)
}
defer hlogger.OnExit(logw.Logger)

// Change the log level of the existing logger
logw.ChangeLevel("ERROR")
// use logw.Logger
```

```golang
// With datadog
ddtrace.Start(ddtrace.WithServiceVersion(version.Version), ddtrace.WithLogger(hlogger.NewDataDogLogger(logger)))
```

```golang
// Scope a logger to a trace so all future log lines are connected in datadog
logger = hlogger.TraceScopedLogger(logger, span)
```


```golang
// Inside test code
logw, err := hlogger.NewTestLogger()
assert.NoError(err)
// use logw.Logger
```

## Setting Platform Orchestrator IDs

The basics:

```go
ids, ctx := hlogger.EnsurePlatformOrchestratorIdsOnCtx(ctx)
ids.OrgId = "test-org"
ids.AppId = "test-app"

logger = logger.With(ids.AsLogFields()...)
logger.Info("hello!")
```

This example shows more complex use. The same ids are tracked across the context tree so that changes inside a child
context will be available to the parent context that first initialized or reset the platformOrchestratorIds object.

```go
ids, ctx := hlogger.EnsurePlatformOrchestratorIdsOnCtx(ctx)

ctx, cancel := context.WithCancel(ctx)
defer cancel()

extractPlatformOrchestratorFields(ctx)

logger = logger.With(ids.AsLogFields()...)
logger.Info("hello!")
```
