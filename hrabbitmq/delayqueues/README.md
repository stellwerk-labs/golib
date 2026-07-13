# `delayqueues` utilities

STOP: Before you go further with v1 of delayqueues package. Consider using the `v2` subpackage which is considerably simpler.

This package contains a useful framework for creating delay queues in RabbitMQ.

Usually when you have an error or reject a message in AMQP, the message is either dropped, or placed back on the queue
for processing by another consumer. This can cause thrashing and resource over consumption when you need to wait for an
external condition to be reached or fixed.

Instead, you can use the dead letter mechanisms to delay messages.

To delay a message by T:

1. Create a new fanout or topic exchange specifically for delayed messages. NOTE, this isn't strictly necessary but does isolate delayed messages from other services.
2. Create a new queue on this exchange for delay-T:
   3. Set the message ttl on the queue to T so each message will be "dead" after T is reached.
   4. Set the dead-letter exchange to your main exchange
   5. Set the dead-letter routing key to your consumer on the main exchange
   6. Inside your consumer, check messages for any remaining delay, and republish them back to the appropriate delay queue if there is more time to wait, otherwise, handle them.

For example using this library...

```go
package main

import (
   "context"
   "time"

   "github.com/stellwerk-labs/golib/hrabbitmq"
   "github.com/stellwerk-labs/golib/hrabbitmq/delayqueues"
   "github.com/wagslane/go-rabbitmq"
   "go.uber.org/zap"
)

func main() {
   var conn *rabbitmq.Conn
   var pub hrabbitmq.Publisher

   // we can either delay be 1s or 1m. any other delays are made up of multiple delays of 1s or 1m.
   buckets := []time.Duration{time.Second, time.Minute}

   dlc, _ := delayqueues.NewConfig("delay-exchange", "delay-by-%s", buckets, pub)

   for _, b := range buckets {
      _ = delayqueues.SetupConsumer(dlc, b, "delay-by-"+b.String(), "main-exchange", "work-queue", conn, zap.L())
   }

   consumer, _ := hrabbitmq.NewConsumerWithHandlerWaiter(
      conn,
      func(d rabbitmq.Delivery) rabbitmq.Action { 
         // check if there is still delay remaining and handle that if needed 
         if skip, err := dlc.HandleDelayContinuation(context.Background(), zap.L(), &d); err != nil {
            // send to dead letter queue
            return rabbitmq.NackRequeue
         } else if skip {
            // or we're done!
            return rabbitmq.Ack
         }

         if err := inner(&d); err != nil {
            // oh no an error from the handler, check if it's a graceful retry and handle it
            if skip, err := dlc.HandleGracefulRetryError(context.Background(), zap.L(), &d, err); err != nil || !skip {
               // if we failed to publish the handler here, send us to the dead letter queue
               return rabbitmq.NackDiscard
             }
         }

         // successfully processed on work queue
         return rabbitmq.Ack
      },
      "work-queue",
   )
   _ = consumer.Run()
}
```