# hrabbitmq

Helpers for https://github.com/wagslane/go-rabbitmq.

## logger

`hrabbitmq` provides a logger you can use with `go-rabbitmq`.

```go
consumer, err := rabbitmq.NewConsumer(
  connStr, rabbitmq.Config{},
  rabbitmq.WithConsumerOptionsLogger(hrabbitmq.NewLogger(logger)),
)
```

## delay queues

See [delayqueues/README.md](delayqueues/README.md) for more info on these.

## reliable outbox

See [reliableoutbox/README.md](reliableoutbox/README.md) for more info on reliably flushing pending messages to RabbitMQ.

## handler waiter

When you close a consumer, it doesn't wait for the currently executing messages to be completed. It is a good idea to gracefully shut these down.

We recommend you use the `HandlerWaiter` from this package to wrap your existing handler and then wait for a shutdown.

```
conn, err := # connect to rabbit
defer conn.Close()

waiter := &HandlerWaiter{IncludeAck: true}
gracefulContext, cancel := context.WithTimeout(context.Background(), time.Minute)
defer cancel()
defer waiter.Close(gracefulContext)

consumer, err := rabbitmq.NewConsumer(
  conn, 
  waiter.Wrap(handler),
  ...
)
defer consumer.Close()
```

This ensures that the shutdown order is:

1. Close the consumer, don't accept new messages
2. Wait until messages are finished being handled, ack/nack them as they complete
3. Close the rabbitmq connection manager

## tracing

`hrabbitmq` provides helpers to inject and extract tracing information into rabbitmq to allow end-to-end tracing.

### inject spans

Using `amqp091`

```go
msg := amqp091.Publishing{}
hrabbitmq.InjectSpanToMessage(ctx, logger, msg)
```

Or using `rabbitmq`

```go
if err := n.publisher.Publish(
  // ...
  rabbitmq.WithPublishOptionsHeaders(hrabbitmq.InjectSpanToTable(ctx, logger, rabbitmq.Table{})),
); err != nil {
  // ...
}
```

### extract spans

```go
span, ctx := tracer.StartSpanFromContext(context.Background(), "message", hrabbitmq.ExtractSpanFromMessage(logger, d.Headers)...)
defer span.Finish()
```
