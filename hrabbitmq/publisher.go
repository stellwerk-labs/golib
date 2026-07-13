package hrabbitmq

import (
	"context"

	"github.com/wagslane/go-rabbitmq"
)

type Publisher interface {
	PublishWithDeferredConfirmWithContext(
		ctx context.Context,
		data []byte,
		routingKeys []string,
		options ...func(*rabbitmq.PublishOptions),
	) (rabbitmq.PublisherConfirmation, error)
}

var _ Publisher = (*rabbitmq.Publisher)(nil)

// FuncPublisher is a function that implements Publisher
type FuncPublisher func(
	ctx context.Context,
	data []byte,
	routingKeys []string,
	options ...func(*rabbitmq.PublishOptions),
) (rabbitmq.PublisherConfirmation, error)

func (f FuncPublisher) PublishWithDeferredConfirmWithContext(ctx context.Context, data []byte, routingKeys []string, options ...func(*rabbitmq.PublishOptions)) (rabbitmq.PublisherConfirmation, error) {
	return f(ctx, data, routingKeys, options...)
}

var _ Publisher = FuncPublisher(nil)
