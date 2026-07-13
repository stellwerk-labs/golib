package hpostgresconnect

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/XSAM/otelsql"
	"github.com/lib/pq"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.uber.org/zap"

	sqltrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql"
)

var (
	otelDriverOnce     sync.Once
	otelDriverName     string
	otelDriverErr      error
	otelRecordErrorMu  sync.RWMutex
	otelRecordErrorFn  func(err error) bool
)

type Config struct {
	Logger  *zap.Logger
	ConnStr string

	ConnectRetries  int
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxIdleTime time.Duration
	ConnMaxLifetime time.Duration

	// CustomErrorTraceCheck is an optional (allowed to be nil) function used by tracing to determine whether to flag
	// a span as an error or not.
	CustomErrorTraceCheck func(err error) bool
}

// Model is the underlying type for the entire model.
type Database struct {
	*sql.DB
	logger *zap.Logger
}

const (
	// DefaultMaxOpenConns is the default maximum number of open connections the client may open. This is high by default
	// and services should work to decrease this where possible.
	DefaultMaxOpenConns = 50

	DEFAULT_MAX_IDLE_CONNS     = 2
	DEFAULT_CONNECTION_RETRIES = 6
)

// sleepWithContext waits for a delay or until the ctx is cancelled.
// Returns an error if the ctx has been cancelled.
func sleepWithContext(ctx context.Context, delay time.Duration) error {
	select {
	case <-ctx.Done():
	case <-time.After(delay):
	}

	return ctx.Err()
}

// connectionBackoff tries to run a query on the database ad if it fails, tries again after an increasing wait.
// This is useful because when deploying, it is often the case that the database only becomes accessible some time
// after deployment. (e.g. because the CloudSQL proxy takes time to establish the connection to the database.)
func (db *Database) connectionBackoff(ctx context.Context, maxAttempts int) error {
	attempt := 1

	_, err := db.DB.ExecContext(ctx, "SET timezone = 'utc'")
	for err != nil && attempt < maxAttempts {
		delay := int(1 << uint(attempt))
		db.logger.Sugar().Warn(fmt.Sprintf("Cannot connect to DB, backing off and trying again in %d seconds.", delay), "err", err)

		// Back off doubling the wait time every time. Start with a wait of 2 seconds (2**1)
		if err = sleepWithContext(ctx, time.Duration(delay)*time.Second); err != nil {
			break
		}
		attempt++
		_, err = db.DB.ExecContext(ctx, "SET timezone = 'utc'")
	}
	if attempt >= maxAttempts {
		return errors.New("connecting to database")
	}
	return err
}

// InitDatabase initialize database
func InitDatabase(ctx context.Context, config *Config) (*Database, error) {
	config.Logger.Info("Connecting to database")

	sqltrace.Register("postgres", &pq.Driver{})
	errorChecker := NewTraceErrorCheck(config.Logger, config.CustomErrorTraceCheck)
	db, err := sqltrace.Open("postgres", config.ConnStr, sqltrace.WithErrorCheck(errorChecker.Check))
	if err != nil {
		return nil, fmt.Errorf("opening database connection: %w", err)
	}
	database := &Database{db, config.Logger}

	maxOpenConns := config.MaxOpenConns
	maxIdleConns := config.MaxIdleConns
	connMaxIdleTime := config.ConnMaxIdleTime

	if maxIdleConns == 0 {
		maxIdleConns = DEFAULT_MAX_IDLE_CONNS
	}
	if maxOpenConns == 0 {
		maxOpenConns = DefaultMaxOpenConns
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxIdleTime(connMaxIdleTime)
	if config.ConnMaxLifetime != 0 {
		db.SetConnMaxLifetime(config.ConnMaxLifetime)
	}

	// Block executing while we attempt to connect to the database
	connectRetries := config.ConnectRetries
	if connectRetries == 0 {
		connectRetries = DEFAULT_CONNECTION_RETRIES
	}
	if err := database.connectionBackoff(ctx, connectRetries); err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	// now that the db is connected, notify the error checker
	errorChecker.SetConnectedSuccessfully(db)
	return database, nil
}

// InitDatabaseOTel initializes a database connection with OpenTelemetry tracing.
// This is the OTel equivalent of InitDatabase.
//
// The service should configure the global OTel TracerProvider before calling this function.
// Traces will be exported according to the configured exporter (e.g., OTLP to collector).
func InitDatabaseOTel(ctx context.Context, config *Config) (*Database, error) {
	config.Logger.Info("Connecting to database with OTel tracing")

	errorChecker := NewTraceErrorCheck(config.Logger, config.CustomErrorTraceCheck)

	// Update the package-level RecordError function so the shared driver
	// uses this caller's error checker.
	otelRecordErrorMu.Lock()
	otelRecordErrorFn = errorChecker.Check
	otelRecordErrorMu.Unlock()

	// Register the postgres driver with OTel instrumentation exactly once.
	otelDriverOnce.Do(func() {
		otelDriverName, otelDriverErr = otelsql.Register("postgres",
			otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
			otelsql.WithSpanOptions(otelsql.SpanOptions{
				Ping:     true,
				RowsNext: false, // Don't create spans for each row iteration
				RecordError: func(err error) bool {
					otelRecordErrorMu.RLock()
					fn := otelRecordErrorFn
					otelRecordErrorMu.RUnlock()
					if fn != nil {
						return fn(err)
					}
					return true
				},
			}),
		)
	})
	if otelDriverErr != nil {
		return nil, fmt.Errorf("registering OTel SQL driver: %w", otelDriverErr)
	}

	db, err := sql.Open(otelDriverName, config.ConnStr)
	if err != nil {
		return nil, fmt.Errorf("opening database connection: %w", err)
	}

	// Record database connection pool metrics
	if err := otelsql.RegisterDBStatsMetrics(db, otelsql.WithAttributes(semconv.DBSystemPostgreSQL)); err != nil {
		config.Logger.Warn("Failed to register database stats metrics with OTel", zap.Error(err))
	}

	database := &Database{db, config.Logger}

	maxOpenConns := config.MaxOpenConns
	maxIdleConns := config.MaxIdleConns
	connMaxIdleTime := config.ConnMaxIdleTime

	if maxIdleConns == 0 {
		maxIdleConns = DEFAULT_MAX_IDLE_CONNS
	}
	if maxOpenConns == 0 {
		maxOpenConns = DefaultMaxOpenConns
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxIdleTime(connMaxIdleTime)

	// Block executing while we attempt to connect to the database
	connectRetries := config.ConnectRetries
	if connectRetries == 0 {
		connectRetries = DEFAULT_CONNECTION_RETRIES
	}
	if err := database.connectionBackoff(ctx, connectRetries); err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	// Now that the db is connected, notify the error checker
	errorChecker.SetConnectedSuccessfully(db)
	return database, nil
}
