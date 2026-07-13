package reliableoutbox

import (
	"context"

	"github.com/wagslane/go-rabbitmq"
)

// PendingMessage is the structure representing a pending message in the system, the implementation of the message must
// support at least the defined attributes, but may contain more and may specialize it's publish options via the use
// of Outbox.WithExtraOption to provide things like expiration, delays, or other custom headers.
type PendingMessage interface {
	// MessageId should return a unique id that can be used as a reference to "complete" the given message by id.
	MessageId() string
	// MessageExchange should return the exchange that the message should be sent over
	MessageExchange() string
	// MessageRoutingKeys should return all the routing keys that this message should be sent through
	MessageRoutingKeys() []string
	// MessagePayload should return the raw data for this message. It is recommended to use json.RawMessage here.
	MessagePayload() []byte
}

// PendingMessageWithPublisherOptions allows a message to expose additional publish options
// WARNING - this should be used carefully as it can easily result in head-of-line-blocking if options are invalid
// or cause continuous errors.
type PendingMessageWithPublisherOptions interface {
	// MessageOptions should return the additional options that are applied AFTER all the default options have been
	// applied.
	MessageOptions() []func(*rabbitmq.PublishOptions)
}

// Store provides an implementation of a store for storing the pending message.
type Store[k PendingMessage] interface {
	// Complete marks the message with the given id as published.
	Complete(ctx context.Context, messageId string) error
	// LoadPage returns a page of pending messages and returns whether there are more messages available or an error
	// occurred.
	LoadPage(ctx context.Context) ([]k, bool, error)
}
