package hstandardreliableoutbox

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wagslane/go-rabbitmq"
	"go.uber.org/zap"

	"github.com/stellwerk-labs/golib/hrabbitmq"
	"github.com/stellwerk-labs/golib/hrabbitmq/reliableoutbox"
)

func Test(t *testing.T) {
	conn, err := sql.Open("postgres", "postgres://postgres:PassW0rd@localhost/postgres?sslmode=disable")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, conn.Close())
	}()

	assert.NoError(t, conn.PingContext(t.Context()))

	{
		tx, err := conn.BeginTx(t.Context(), &sql.TxOptions{Isolation: sql.LevelSerializable})
		require.NoError(t, err)
		defer func() {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				assert.NoError(t, err)
			}
		}()

		require.NoError(t, MigrateUp01(t.Context(), tx))

		require.NoError(t, tx.Commit())
	}

	exchange := "my-exchange"
	rk := fmt.Sprintf("rk-%s", rand.Text())

	received := make(chan rabbitmq.Delivery)
	var publisher hrabbitmq.Publisher
	{
		amqp, err := rabbitmq.NewConn("amqp://guest:guest@localhost:5672/")
		require.NoError(t, err)

		pub, err := rabbitmq.NewPublisher(amqp)
		require.NoError(t, err)
		pub.NotifyPublish(func(p rabbitmq.Confirmation) {
		})
		publisher = pub

		cons, err := hrabbitmq.NewConsumerWithHandlerWaiter(
			amqp,
			func(d rabbitmq.Delivery) (action rabbitmq.Action) {
				select {
				case received <- d:
				case <-t.Context().Done():
					assert.Fail(t, "timeout")
				}
				return rabbitmq.Ack
			},
			rk,
			rabbitmq.WithConsumerOptionsExchangeDeclare,
			rabbitmq.WithConsumerOptionsExchangeName(exchange),
			rabbitmq.WithConsumerOptionsExchangeKind("direct"),
			rabbitmq.WithConsumerOptionsConsumerAutoAck(true),
			rabbitmq.WithConsumerOptionsQueueAutoDelete,
			rabbitmq.WithConsumerOptionsRoutingKey(rk),
		)
		require.NoError(t, err)
		go func() {
			assert.NoError(t, cons.Run())
		}()

		defer func() {
			assert.NoError(t, cons.Close(t.Context()))
		}()
	}

	// Wait 3 seconds until we're confident the consumer is fully ready and the queue is prepared in rabbitmq
	time.Sleep(3 * time.Second)

	msgs, err := InsertPendingEventMessages(
		t.Context(),
		conn,
		[]*PendingEventMessage{{RoutingKey: rk, Exchange: exchange, Payload: []byte(`{"hello":"world"}`)}},
	)
	require.NoError(t, err)
	require.NotEmpty(t, msgs)
	msgId := msgs[0].Id

	prepped := reliableoutbox.PrepareOptimisticPublish(zap.L(), SqlContextAsReliableOutbox(conn), msgs)
	prepped(t.Context(), publisher)

	// And finally let's wait for our consumer to pick up the message!
	select {
	case d := <-received:
		assert.Equal(t, fmt.Sprint(msgId), d.MessageId)
		assert.JSONEq(t, `{"hello": "world"}`, string(d.Body))
	case <-t.Context().Done():
		require.Fail(t, "timeout")
	}

	// And check that the pending message doesn't exist any-more
	for {
		p, m, err := SqlContextAsReliableOutbox(conn).LoadPage(t.Context())
		require.NoError(t, err)
		for _, message := range p {
			assert.NotEqual(t, msgId, message.Id)
		}
		if !m {
			break
		}
	}

	{
		tx, err := conn.BeginTx(t.Context(), &sql.TxOptions{Isolation: sql.LevelSerializable})
		require.NoError(t, err)
		defer func() {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				assert.NoError(t, err)
			}
		}()

		require.NoError(t, MigrateDown01(t.Context(), tx))

		require.NoError(t, tx.Commit())
	}
}
