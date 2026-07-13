package hpostgresconnect

import (
	"database/sql"
	"errors"
	"net"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

func TestTraceErrorCheck_Check(t *testing.T) {
	logger := zaptest.NewLogger(t)
	extraCount := atomic.Int32{}
	tc := NewTraceErrorCheck(logger, func(err error) bool {
		extraCount.Add(1)
		return true
	})
	// custom errors are always traced
	assert.True(t, tc.Check(errors.New("custom")))
	// and trigger the extra error checker if provided
	assert.Equal(t, 1, int(extraCount.Load()))
	// net errors are not traced by default
	assert.False(t, tc.Check(&net.OpError{}))
	// until the database has been successfully connected for the first time
	tc.SetConnectedSuccessfully(&sql.DB{})
	assert.True(t, tc.Check(&net.OpError{}))
}
