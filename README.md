# Stellwerk Shared Go Module Library

* [hconfig](./hconfig#readme) Common config loading
* [hdbmigrate](./hdbmigrate#readme) Database migration (based on Goose with sql and code)
* [hecho](./hecho#readme) HTTP based stack based on echo and code-generation
* [henvflags](./henvflags#readme) In-memory feature flags
* [herrors](./herrors#readme) Common Platform Orchestrator API errors
* [hlogger](./hlogger#readme) A common logger and logging helpers
* [hpostgres](./hpostgres#readme) A common postgres initialization and migrate method (based on golang-migrate, sql only)
* [hpostgresconnect](./hpostgresconnect#readme) A common postgres initialization method
* [hrabbitmq](./hrabbitmq#readme) rabbitmq helper functions
* [htemplate](./htemplate#readme) A collection of [sprig](https://masterminds.github.io/sprig/) functions that should be safe to use
* [httplogger](./httplogger#readme) http logging middleware
* [hpagination](./hpagination#readme) Helper functions to construct and decode page tokens
* [hretrier](./hretrier) http retry wrapper
* [hservicejwt](./hservicejwt#readme) Utilities for encoding, decoding, and working with service JWTs passed between services
* [hstandardreliableoutbox](./hstandardreliableoutbox#readme) Postgres and RabbitMQ implementation of the reliable outbox pattern
* [htelemetry](./htelemetry#readme) Backend-agnostic tracing abstraction supporting both Datadog and OpenTelemetry
* [hvaultapi](./hvaultapi#readme) Vault login and token lifecycle management
* [utils/release](./utils/release#readme) CLI tool to release golib libraries in dependency order

## Adding additional modules

Additional modules can be added by:

1. Creating another directory with a `go.mod` and `go.sum` inside
1. Add directory to `go.work`
1. Add a line in this file
1. Add the module to this README

Please remember that only functionality used by a majority of service should be extracted and not something, which
is only used by a few.
