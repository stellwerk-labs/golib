# hpostgresconnect

Is a module which provides a method to initialize a database.

## Usage

```golang
// InitDatabase inside the app code
logger, err := hlogger.New("INFO", false, "json")

cfg := &Config{
  ConnStr: "postgres://root:PassW0rd@localhost/db?sslmode=disable",
  Logger:  logger,
  ConnectRetries: 6,
  MaxIdleConns: 2,
  ConnMaxIdleTime: 0
}

sugar := logger.Sugar()

ctx := context.Background()
db, err := hpostgres.InitDatabase(ctx, cfg)
if err != nil {
  sugar.Fatalw("Error initializing Database", "err", err)
}
```

```golang
// InitDatabase inside test code
cfg := &Config{
  ConnStr: connectionString,
  Logger:  logger,
}
db, err := InitDatabase(ctx, cfg)
assert.NoError(err)
```
