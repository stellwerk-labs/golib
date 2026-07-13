package delayqueues

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wagslane/go-rabbitmq"
	"go.uber.org/zap/zaptest"

	"github.com/stellwerk-labs/golib/hrabbitmq"
)

func TestSetupConsumer(t *testing.T) {
	pub := new(hrabbitmq.NoOpPublisher)
	dqc := MustNewConfig("my-exchange", "k%s", []time.Duration{time.Second, time.Minute}, pub)
	rk, opts := setupConsumerInner(dqc, time.Second, "return-exchange", "return-key", zaptest.NewLogger(t), func(options *rabbitmq.ConsumerOptions) {
		options.RabbitConsumerOptions.Name = "example"
	})
	assert.Equal(t, "k1s", rk)
	cs := &rabbitmq.ConsumerOptions{}
	for _, opt := range opts {
		opt(cs)
	}
	assert.NotNil(t, cs.Logger)
	cs.Logger = nil
	assert.Equal(t, rabbitmq.ConsumerOptions{
		RabbitConsumerOptions: rabbitmq.RabbitConsumerOptions{
			Name: "example",
		},
		QueueOptions: rabbitmq.QueueOptions{
			Durable: true,
			Args: map[string]interface{}{
				"x-dead-letter-exchange":    "return-exchange",
				"x-dead-letter-routing-key": "return-key",
				"x-dead-letter-strategy":    "at-least-once",
				"x-message-ttl":             int64(1000),
				"x-overflow":                "reject-publish",
				"x-queue-type":              "quorum",
			},
		},
		ExchangeOptions: []rabbitmq.ExchangeOptions{
			{
				Name:    "my-exchange",
				Kind:    "topic",
				Declare: true,
				Durable: true,
				Args:    map[string]interface{}{},
				Bindings: []rabbitmq.Binding{
					{RoutingKey: "k1s", BindingOptions: rabbitmq.BindingOptions{
						Declare: true, Args: map[string]interface{}{},
					}},
				},
			},
		},
		Concurrency: 1,
	}, *cs)
}
