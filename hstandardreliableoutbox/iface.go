package hstandardreliableoutbox

import (
	"context"
	"database/sql"
)

type SqlContext interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
}
