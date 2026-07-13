package hstandardreliableoutbox

import (
	"context"
	"fmt"

	"github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/stellwerk-labs/golib/hlogger"
)

func InsertPendingEventMessages(ctx context.Context, sqlContext SqlContext, messages []*PendingEventMessage) ([]*PendingEventMessage, error) {
	logger := hlogger.TraceScopedLoggerFromCtx(zap.L(), ctx)

	if len(messages) == 0 {
		return nil, nil
	}
	routingKeys := make([]string, 0, len(messages))
	exchanges := make([]string, 0, len(messages))
	payloads := make([]string, 0, len(messages))
	expirations := make([]int32, 0, len(messages))
	for i, msg := range messages {
		// validate
		if msg.Id != 0 {
			return nil, fmt.Errorf("message %d: id should not be set", i)
		} else if !msg.CreatedAt.IsZero() {
			return nil, fmt.Errorf("message %d: created-at must not be set", i)
		} else if msg.RoutingKey == "" {
			return nil, fmt.Errorf("message %d: routing key must be set", i)
		} else if msg.Exchange == "" {
			return nil, fmt.Errorf("message %d: exchange must be set", i)
		} else if msg.Payload == nil {
			return nil, fmt.Errorf("message %d: payload cannot be nil", i)
		} else if msg.Expiration < 0 {
			return nil, fmt.Errorf("message %d: ttl cannot be < 0", i)
		}
		routingKeys = append(routingKeys, msg.RoutingKey)
		exchanges = append(exchanges, msg.Exchange)
		payloads = append(payloads, string(msg.Payload))
		expirations = append(expirations, int32(msg.Expiration.Seconds()))
	}

	if rs, err := sqlContext.QueryContext(
		ctx,
		`
INSERT INTO pending_event_messages (created_at, routing_key, exchange, payload, expiration_seconds)
SELECT timezone('UTC', now()), a, b, c::jsonb, d FROM unnest($1::text[], $2::text[], $3::text[], $4::integer[]) AS x(a,b,c, d)
RETURNING id, created_at
		`,
		pq.StringArray(routingKeys), pq.StringArray(exchanges), pq.StringArray(payloads), pq.Int32Array(expirations),
	); err != nil {
		return nil, fmt.Errorf("failed to insert pending event messages: %w", err)
	} else {
		defer func() {
			if err := rs.Close(); err != nil {
				logger.Error("failed to close", zap.Error(err))
			}
		}()
		out := make([]*PendingEventMessage, 0, len(messages))
		for i := 0; rs.Next(); i += 1 {
			m := messages[i]
			if err = rs.Scan(&m.Id, &m.CreatedAt); err != nil {
				return nil, fmt.Errorf("failed while scanning pending event message: %w", err)
			}
			out = append(out, m)
		}
		if err = rs.Err(); err != nil {
			return nil, fmt.Errorf("failure while scanning inserted messages: %w", err)
		}
		logger.Debug("inserted pending event messages", zap.Int("n", len(messages)))
		return out, nil
	}
}
