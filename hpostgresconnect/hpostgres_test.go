package hpostgresconnect

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stellwerk-labs/golib/hlogger"
	"github.com/stretchr/testify/assert"
)

var (
	connectionString = "postgres://root:PassW0rd@localhost/db?sslmode=disable"
)

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

func TestPostgresConnection_Error(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()
	logger, err := hlogger.New("INFO", false, "console")
	assert.NoError(err)

	cfg := &Config{
		ConnStr:        "postgres://root:INVALID@localhost/db?sslmode=disable",
		Logger:         logger,
		ConnectRetries: 1,
	}
	_, err = InitDatabase(ctx, cfg)
	assert.Error(err)
}

func TestPostgresConnection_ConnectionError(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()
	logger, err := hlogger.New("INFO", false, "console")
	assert.NoError(err)

	customErrorCheckerCalls := atomic.Int32{}
	cfg := &Config{
		ConnStr:        strings.Replace(connectionString, "localhost", "localhost:9", 1),
		Logger:         logger,
		ConnectRetries: 1,
		CustomErrorTraceCheck: func(err error) bool {
			customErrorCheckerCalls.Add(1)
			return true
		},
	}
	_, err = InitDatabase(ctx, cfg)
	assert.Error(err)
	// should not be called since we haven't connected successfully yet
	assert.Equal(0, int(customErrorCheckerCalls.Load()))

	cfg.ConnStr = strings.Replace(connectionString, "root", "??", 1)
	_, err = InitDatabase(ctx, cfg)
	assert.Error(err)
	// now it should be called because this is no longer an initial connection failure
	assert.Equal(1, int(customErrorCheckerCalls.Load()))
}
