package v2

import (
	"context"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/rabbitmq/amqp091-go"
	"github.com/wagslane/go-rabbitmq"
	"go.uber.org/zap"
)

var DefaultExchange = "organization.common"

const (
	DeadLetterQueueName            = "hrabbitmq.dead-letters"
	DeadLetterRoutingKey           = DeadLetterQueueName
	DelayRoutingKeyTemplate        = "hrabbitmq.delay-%v"
	DelayEndedRoutingKey           = "hrabbitmq.delay-ended"
	DelayEndedNextRoutingKeyHeader = "x-hrabbitmq-next-routing-key"
	DelayEndedRemainingHeader      = "x-hrabbitmq-delay-remaining"
	LastRetryDelayHeader           = "x-hrabbitmq-last-exponential-delay"
	RetryAttemptTrackerHeader      = "x-hrabbitmq-graceful-retry-attempt"
)

var delayDurations = []time.Duration{
	time.Second * 2,
	time.Second * 10,
	time.Second * 30,
	time.Minute * 2,
	time.Minute * 10,
	time.Minute * 30,
}

type HandlerFunc = func(context.Context, *zap.Logger, *rabbitmq.Delivery) error

// incrementRetryAttempts is a helper function for tracking an incrementing retry attempt header.
func incrementRetryAttempts(headers amqp091.Table) amqp091.Table {
	if attempts, ok := headers[RetryAttemptTrackerHeader]; ok {
		headers[RetryAttemptTrackerHeader] = attempts.(int32) + 1
	} else {
		if headers == nil {
			headers = make(amqp091.Table, 1)
		}
		headers[RetryAttemptTrackerHeader] = int32(1)
	}
	return headers
}

// isDuplicate checks if this attempt of the message has been already published
func isDuplicate(msg *rabbitmq.Delivery, cache *expirable.LRU[string, int32]) bool {
	if cache != nil {
		attempts, ok := msg.Headers[RetryAttemptTrackerHeader].(int32)
		if !ok {
			attempts = 0
		}
		if cached, ok := cache.Get(msg.MessageId); ok && (cached > attempts || cached < 0) {
			// Attempt is incremental, previous attempts have been already published, skip.
			// Negative value in cache means that it needs to be skipped disregarding on attempts.
			return true
		}
	}
	return false
}

// rememberRetryAttempt stores the number of retry attempts in the cache
func rememberRetryAttempts(msg *rabbitmq.Delivery, cache *expirable.LRU[string, int32]) {
	if cache != nil {
		attempts, ok := msg.Headers[RetryAttemptTrackerHeader].(int32)
		if !ok {
			attempts = 0
		}
		if cached, ok := cache.Get(msg.MessageId); !ok || cached < attempts {
			cache.Add(msg.MessageId, attempts)
		}
	}
}
