package hstandardreliableoutbox

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/wagslane/go-rabbitmq"
	"go.uber.org/zap"

	"github.com/stellwerk-labs/golib/hlogger"
	"github.com/stellwerk-labs/golib/hrabbitmq/reliableoutbox"
)

// MessageIdPrefix allows a unique prefix to be prepended to message ids for cases where independent copies of the
// standard reliable outbox are operating with overlapping primary key spaces. So here we add a prefix.
var MessageIdPrefix = ""

type PendingEventMessage struct {
	Id         int64
	CreatedAt  time.Time
	Exchange   string
	RoutingKey string
	Payload    json.RawMessage
	Expiration time.Duration
}

func (p *PendingEventMessage) MessageOptions() []func(*rabbitmq.PublishOptions) {
	return []func(opts *rabbitmq.PublishOptions){
		func(opts *rabbitmq.PublishOptions) {
			if p.Expiration > time.Millisecond {
				opts.Expiration = strconv.Itoa(int(p.Expiration / time.Millisecond))
			}
		},
	}
}

func (p *PendingEventMessage) MessageId() string {
	return MessageIdPrefix + strconv.FormatInt(p.Id, 10)
}

func (p *PendingEventMessage) MessageExchange() string {
	return p.Exchange
}

func (p *PendingEventMessage) MessageRoutingKeys() []string {
	return []string{p.RoutingKey}
}

func (p *PendingEventMessage) MessagePayload() []byte {
	return p.Payload
}

var _ reliableoutbox.PendingMessage = (*PendingEventMessage)(nil)
var _ reliableoutbox.PendingMessageWithPublisherOptions = (*PendingEventMessage)(nil)

type SqlContextOutbox struct {
	SqlContext
}

func SqlContextAsReliableOutbox(sc SqlContext) *SqlContextOutbox {
	return &SqlContextOutbox{sc}
}

func (m *SqlContextOutbox) Complete(ctx context.Context, messageId string) error {
	logger := hlogger.TraceScopedLoggerFromCtx(zap.L(), ctx)
	if id, err := strconv.ParseInt(strings.TrimPrefix(messageId, MessageIdPrefix), 10, 64); err != nil {
		return fmt.Errorf("failed to parse message id '%s' as int64: %w", messageId, err)
	} else if _, err := m.ExecContext(ctx, `DELETE FROM pending_event_messages WHERE id = $1`, id); err != nil {
		return fmt.Errorf("failed to execute delete query: %w", err)
	}
	logger.Debug("deleted pending event message", zap.String("messageId", messageId))
	return nil
}

func (m *SqlContextOutbox) LoadPage(ctx context.Context) ([]*PendingEventMessage, bool, error) {
	logger := hlogger.TraceScopedLoggerFromCtx(zap.L(), ctx)
	const n = 10
	if rs, err := m.QueryContext(ctx, `SELECT id, created_at, exchange, routing_key, payload, expiration_seconds FROM pending_event_messages ORDER BY id LIMIT $1`, n+1); err != nil {
		return nil, true, fmt.Errorf("failed to execute list query: %w", err)
	} else {
		defer func() {
			if err := rs.Close(); err != nil {
				zap.L().Error("failed to close", zap.Error(err))
			}
		}()
		out := make([]*PendingEventMessage, 0, n)
		var more bool
		for rs.Next() {
			var pem PendingEventMessage
			var rawJson []byte
			var rawExpirationSeconds int
			if err = rs.Scan(&pem.Id, &pem.CreatedAt, &pem.Exchange, &pem.RoutingKey, &rawJson, &rawExpirationSeconds); err != nil {
				return out, true, fmt.Errorf("failed while scanning pending event message: %w", err)
			} else if len(out) >= n {
				// if we've already read N messages (the sql limit is N+1) then we know we can return "MORE" in the output.
				more = true
				break
			} else {
				// otherwise we add this message to the outgoing set
				pem.Payload = rawJson
				pem.Expiration = time.Duration(rawExpirationSeconds) * time.Second
				pem.CreatedAt = pem.CreatedAt.UTC()
				out = append(out, &pem)
			}
		}
		if err = rs.Err(); err != nil {
			return out, true, fmt.Errorf("failed while scanning pending event messages: %w", err)
		}
		logger.Debug("returning page of pending messages", zap.Int("n", len(out)), zap.Bool("more", more))
		return out, more, nil
	}
}
