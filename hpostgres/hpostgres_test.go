package hpostgres

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/stellwerk-labs/golib/hlogger"
	"github.com/stretchr/testify/assert"
)

var (
	tables = []string{
		"migration_test_table",
		"schema_migrations",
	}
	connectionString = "postgres://root:PassW0rd@localhost/db?sslmode=disable"
)

func cleanDatabase(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	for _, table := range tables {
		if _, err = tx.ExecContext(ctx, fmt.Sprintf(("TRUNCATE TABLE %s RESTART IDENTITY CASCADE;"), table)); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func getSchemaMigrationRow(ctx context.Context, db *Database) (int, bool, error) {
	row := db.QueryRowContext(ctx, "SELECT version,dirty FROM schema_migrations")
	var version int
	var dirty bool
	if err := row.Scan(&version, &dirty); err != nil {
		return 0, false, err
	}

	return version, dirty, nil
}

func TestPostgresConnection(t *testing.T) {
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

	err = db.PingContext(ctx)
	assert.NoError(err)

	err = db.Close()
	assert.NoError(err)
}

func TestMigration(t *testing.T) {
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

	_, err = db.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s", tables[0]))
	assert.NoError(err)

	err = cleanDatabase(ctx, db.DB)
	assert.NoError(err)
}

func TestMissingMigrationFile(t *testing.T) {
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

	version, dirty, err := getSchemaMigrationRow(ctx, db)
	assert.NoError(err)
	assert.Equal(2, version)
	assert.False(dirty)

	err = db.Migrate(ctx, "migrations_b")
	assert.NoError(err)
	version, dirty, err = getSchemaMigrationRow(ctx, db)
	assert.NoError(err)
	assert.Equal(2, version)
	assert.False(dirty)

	err = cleanDatabase(ctx, db.DB)
	assert.NoError(err)
}
