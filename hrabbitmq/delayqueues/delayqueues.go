package delayqueues

import (
	"fmt"
	"slices"
	"time"

	"github.com/pkg/errors"

	"github.com/stellwerk-labs/golib/hrabbitmq"
)

const (
	// ---- common rabbit mq headers ----

	// DeathHeader is inserted by the dead letter exchange (see https://www.rabbitmq.com/docs/dlx)
	DeathHeader = "x-death"

	// ---- default custom header names for this library ----

	DefaultDelayEndedRoutingKeyHeader = "x-hrabbitmq-delay-ended-routing-key"
	DefaultDelayRemainingHeader       = "x-hrabbitmq-delay-remaining"
	DefaultLastRetryDelayHeader       = "x-hrabbitmq-last-exponential-delay"
	DefaultRetryAttemptTrackerHeader  = "x-hrabbitmq-graceful-retry-attempt"
)

type DelayQueueConfig struct {
	delayExchange           string
	delayRoutingKeyTemplate string
	durations               []time.Duration
	publisher               hrabbitmq.Publisher

	// publishRetryFunction is called when the publish request in RepublishMessageWithDelay fails. This function
	// should return a duration to wait before making another attempt or -1 to indicate no more attempts.
	publishRetryFunction func(i int) time.Duration

	// delayEndedRoutingKeyHeader holds the routing key to copy the message to when the DelayRemainingHeader has reached 0.
	delayEndedRoutingKeyHeader string
	// delayRemainingHeader holds the duration of the remaining delay on this message
	delayRemainingHeader string
	// lastExponentialDelayHeader holds the last delay used in exponential backoff
	lastExponentialDelayHeader string
	// gracefulRetryAttemptHeader holds the number of times this message has been gracefully retried. graceful retries
	// do not go via the dead letter queue.
	gracefulRetryAttemptHeader string
}

func (c *DelayQueueConfig) DelayExchange() string {
	return c.delayExchange
}

func (c *DelayQueueConfig) DelayRoutingKeyTemplate() string {
	return c.delayRoutingKeyTemplate
}

func (c *DelayQueueConfig) Durations() []time.Duration {
	return c.durations
}

func (c *DelayQueueConfig) DelayEndedRoutingKeyHeader() string {
	return c.delayEndedRoutingKeyHeader
}

func (c *DelayQueueConfig) DelayRemainingHeader() string {
	return c.delayRemainingHeader
}

func (c *DelayQueueConfig) LastExponentialDelayHeader() string {
	return c.lastExponentialDelayHeader
}

func (c *DelayQueueConfig) GracefulRetryAttemptHeader() string {
	return c.gracefulRetryAttemptHeader
}

func NewConfig(
	delayExchange string,
	delayRoutingKeyTemplate string,
	durations []time.Duration,
	publisher hrabbitmq.Publisher,
	options ...func(config *DelayQueueConfig),
) (*DelayQueueConfig, error) {
	if delayExchange == "" {
		return nil, errors.New("exchange must be non empty")
	} else if publisher == nil {
		return nil, errors.New("publisher cannot be nil")
	} else if len(durations) < 1 {
		return nil, errors.New("delay durations must have at least 1 item")
	} else if durations[0] < time.Second {
		return nil, errors.New("lowest duration must be at least 1s")
	} else {
		prev := durations[0]
		for _, d := range durations[1:] {
			if d <= prev {
				return nil, errors.New("delay durations must be strictly increasing")
			}
			prev = d
		}
	}
	_ = fmt.Sprintf(delayRoutingKeyTemplate, durations[0])
	cfg := &DelayQueueConfig{
		delayExchange:           delayExchange,
		durations:               slices.Clone(durations),
		delayRoutingKeyTemplate: delayRoutingKeyTemplate,
		publisher:               publisher,
	}
	// default options
	WithPublishRetry(nil)(cfg)
	WithOverrideHeaderNames(
		DefaultDelayEndedRoutingKeyHeader,
		DefaultDelayRemainingHeader,
		DefaultLastRetryDelayHeader,
		DefaultRetryAttemptTrackerHeader,
	)(cfg)
	for _, option := range options {
		option(cfg)
	}
	return cfg, nil
}

// MustNewConfig is the same as NewConfig but panic's if there's an error. Best for testing.
func MustNewConfig(
	delayExchange string,
	delayRoutingKeyTemplate string,
	durations []time.Duration,
	publisher hrabbitmq.Publisher,
	options ...func(config *DelayQueueConfig),
) *DelayQueueConfig {
	if cfg, err := NewConfig(delayExchange, delayRoutingKeyTemplate, durations, publisher, options...); err != nil {
		panic(err)
	} else {
		return cfg
	}
}

// WithPublishRetry allows us to modify or remove the retry function
func WithPublishRetry(f func(i int) time.Duration) func(c *DelayQueueConfig) {
	return func(c *DelayQueueConfig) {
		c.publishRetryFunction = f
	}
}

// WithOverrideHeaderNames allows you to override the default header names used to track metadata
func WithOverrideHeaderNames(
	delayEndedRoutingKeyHeader,
	delayRemainingHeader,
	lastExponentialDelayHeader,
	gracefulRetryAttemptHeader string,
) func(c *DelayQueueConfig) {
	return func(c *DelayQueueConfig) {
		c.delayEndedRoutingKeyHeader = delayEndedRoutingKeyHeader
		c.delayRemainingHeader = delayRemainingHeader
		c.lastExponentialDelayHeader = lastExponentialDelayHeader
		c.gracefulRetryAttemptHeader = gracefulRetryAttemptHeader
	}
}

// GetDelayKeyMapping returns the mapping from duration to routing key for all the durations this queue config supports.
func (c *DelayQueueConfig) GetDelayKeyMapping() map[time.Duration]string {
	out := make(map[time.Duration]string, len(c.durations))
	for _, duration := range c.durations {
		out[duration] = fmt.Sprintf(c.delayRoutingKeyTemplate, duration)
	}
	return out
}
