# `reliableoutbox` utilities

This library is used to provide more reliable "at-least-once" message publishing from a service that produces new RabbitMQ messages due to changes in a data-store.

How this works:

1. We FIRST write each outgoing message to a database table and assign it a unique message ID.
2. We then have 2 different routines for sending the message: IMMEDIATE and BACKGROUND.
3. The IMMEDIATE flow takes place in the request path and attempts to publish the message and remove it from the DB.
4. However, if the server has crashed, or AMQP is not available, we must revert to the BACKGROUND path which runs on some interval.
5. The BACKGROUND path runs every N seconds/minutes and attempts to publish and delete any messages pending in the DB.
6. Consumers of the message should attempt to deduplicate any affects using idempotency keys based on the message id or a cache of recently seen message ids.

This results in at-least-once behavior. Since the message can be half-published M times, fully published N times, and cleared from the datastore once.

How to use this library:

1. Add a datastore model for a pending message. Implement the `reliableoutbox.PendingMessage` interface. Also implement `PendingMessageWithPublisherOptions` if you want to send additional headers or metadata.
2. Implement the `reliableoutbox.Store` interface on your datastore layer. This will allow the `reliableoutbox` utilities to pull a page of pending messages and attempt to publish them.
3. Inside your API handler which inserts new pending messages, call `reliableoutbox.OptimisticPublish` in a goroutine after commit.
4. In a background goroutine, run `reliableoutbox.ScheduledFlushPendingMessages`. This ensures any messages that weren't published in (3) are published in the background.

We recommend you use the `delayqueues` library as well to add exponential backoff on errors in the actual rabbitMQ handlers.

## Example pending message model

Pipelines used the following table. You can probably use something similar, but you obviously don't need the `expiration_seconds` and `delay_seconds` column if you don't use them.

```
CREATE TABLE pending_event_messages (
	-- auto incrementing primary key
	id bigserial primary key,
	-- track the created at time so that we can emit metrics around lag and delay
	created_at timestamp without time zone NOT NULL,
	-- track whether the message has been published or not yet, we will garbage collect old published'
	-- messages
	is_published bool NOT NULL,
	-- the exchange to publish to
	exchange text NOT NULL,
	-- the key to publish under
	key text NOT NULL,
	-- the payload of the message, always json in our case
	payload jsonb NOT NULL,
	-- expiration seconds to apply
	expiration_seconds int NOT NULL,
	-- delay seconds to apply
	delay_seconds int NOT NULL default 0
);
```
