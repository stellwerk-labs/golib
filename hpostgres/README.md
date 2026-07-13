# hpostgres

Is a module which provides a method to initialize a database and another one to perform migration.

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

// Migrate inside the app code
ctx := context.Background()
logger, err := hlogger.New("INFO", false, "json")

cfg := &Config{
  ConnStr: "postgres://root:PassW0rd@localhost/db?sslmode=disable",
  Logger:  logger,
}
db, err := InitDatabase(ctx, cfg)
if err != nil {
  sugar.Fatalw("Error initializing Database", "err", err)
}

err = db.Migrate(ctx, "migrations")
if err != nil {
  sugar.Fatalw("Error applying migrations", "err", err)
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

// Migrate inside test code
 assert := assert.New(t)
ctx := context.Background()
logger, err := hlogger.New("INFO", false, "console")
assert.NoError(err)

cfg := &Config{
  ConnStr: connectionString,
  Logger:  logger,
}
db, err := InitDatabase(ctx, cfg)
assert.NoError(err)

err = db.Migrate(ctx, "migrations_a")
assert.NoError(err)
```
