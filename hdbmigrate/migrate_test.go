package hdbmigrate

import (
	"context"
	"testing"

	"github.com/stellwerk-labs/golib/hlogger"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"

	_ "github.com/stellwerk-labs/golib/hdbmigrate/migrations"
)

func TestGenerateAdvisoryLockId(t *testing.T) {
	assert := assert.New(t)

	aid1, err := generateAdvisoryLockId("test1")
	assert.NoError(err)
	assert.Equal("378809238", aid1)

	aid2, err := generateAdvisoryLockId("test2")
	assert.NoError(err)
	assert.Equal("3290860872", aid2)
}

var (
	connectionString = "postgres://root:PassW0rd@localhost/db?sslmode=disable"
)

func TestMigrate(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()
	logger, err := hlogger.New("INFO", false, "console")
	assert.NoError(err)

	assert.NoError(Migrate(ctx, []string{"up"}, connectionString, logger))

	db, err := goose.OpenDBWithDriver("postgres", connectionString)
	assert.NoError(err)

	_, err = db.QueryContext(ctx, "SELECT * FROM example")
	assert.NoError(err)

	assert.NoError(Migrate(ctx, []string{"down"}, connectionString, logger))

	_, err = db.QueryContext(ctx, "SELECT * FROM example")
	assert.ErrorContains(err, "relation \"example\" does not exist")
}
