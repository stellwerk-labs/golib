package hstandardreliableoutbox

import (
	"context"
	"database/sql"
)

func MigrateUp01(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS pending_event_messages (
    -- auto incrementing primary key
    id bigserial primary key,
    -- track the created at time so that we can emit metrics around lag and delay
    created_at timestamp without time zone NOT NULL,
    -- the exchange to publish to
    exchange text NOT NULL,
    -- the routing key to publish under
    routing_key text NOT NULL,
    -- the payload of the message, always json in our case
    payload jsonb NOT NULL,
    -- expiration seconds to apply as the per-message TTL
    expiration_seconds int NOT NULL
);
`)
	return err
}

func MigrateDown01(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`DROP TABLE IF EXISTS pending_event_messages;`)
	return err
}
