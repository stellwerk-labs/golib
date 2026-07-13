package hpostgresconnect

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"sync/atomic"

	"go.uber.org/zap"
)

// TraceErrorCheck is a sqltrace span error checker which observes each traced database operation and determines
// whether to mark the entire span as failed when a database error is seen. This implementation provides special
// behavior to avoid marking connection failures that happen while the sql proxy is still starting up, and also to
// dump database stats when an operation fails due to a network error
type TraceErrorCheck struct {
	logger            *zap.Logger
	next              func(err error) bool
	connectedDatabase atomic.Value
}

// NewTraceErrorCheck constructs a TraceErrorCheck. logger is required, the extra checker function is optional.
func NewTraceErrorCheck(logger *zap.Logger, extra func(err error) bool) *TraceErrorCheck {
	if logger == nil {
		panic("logger must be defined")
	}
	return &TraceErrorCheck{logger: logger, next: extra}
}

func (t *TraceErrorCheck) SetConnectedSuccessfully(db *sql.DB) {
	t.connectedDatabase.Store(db)
}

func (t *TraceErrorCheck) Check(err error) bool {
	var dumpStats bool
	db := t.connectedDatabase.Load()

	// if we get net errors, and it's our initial connect attempts - these are expected and shouldn't be alerted on
	var opError *net.OpError
	if errors.As(err, &opError) {
		if db == nil {
			return false
		}
		dumpStats = true
	}

	// otherwise other kinds of errors can expose a bad connection pool
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, driver.ErrBadConn) {
		dumpStats = true
	}

	if dumpStats && db != nil {
		stats := db.(*sql.DB).Stats()
		t.logger.Info(
			"logging db stats due to database error",
			zap.String("db-stats", fmt.Sprintf(
				"%d in use + %d idle = %d open (max %d)", stats.InUse, stats.Idle, stats.OpenConnections, stats.MaxOpenConnections,
			)),
		)
	}

	// if customer defined another error checker, then call that
	if t.next != nil {
		return t.next(err)
	}
	return true
}
