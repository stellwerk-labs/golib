# hdbmigrate

Wrapper around [goose](https://github.com/pressly/goose) to work with database migrations.

## Usage

### Inside the main process

```go
// Run migrations
if err := hdbmigrate.Migrate(ctx, []string{"up"}, cfg.DBConnStr(), logger); err != nil {
  sugar.Fatalw("Error migrating Database", "err", err)
}
```

### As a separate util cmd

```go
func main() {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatalf("Error reading config: %v", err)
	}

	// Logging
	logger, err := hlogger.New(cfg.LogLevel, false, "json")
	if err != nil {
		log.Fatalf("Error building logger: %v", err)
	}

	args := os.Args[1:]

	ctx := context.Background()
	if err := hdbmigrate.Migrate(ctx, args, cfg.DBConnStr(), logger); err != nil {
		log.Fatalf("migrate err: %v", err)
	}
}
```

Which can be used like:

* Create new migration `go run ./cmd/migrate create SOME_NAME go`
* Manually run migrations `go run ./cmd/migrate up`
