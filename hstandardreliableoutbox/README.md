# hstandardreliableoutbox

This is a Postgres and Rabbitmq implementation of the `hrabbitmq/reliableoutbox` pattern that we use quite often.

This is built out to avoid having individual services configuring exactly the same thing over and over.

To use this, see the [e2e_test.go](e2e_test.go):

1. Apply the `MigrateUp01` migration through `goose` or `go-migrate` or your chosen migrations library.
2. Create a Postgres DB connection to interact with SQL
3. Create a RabbitMQ publisher to publish the messages
4. Use `InsertPendingEventMessages` to insert pending messages - preferably within a transaction
5. Use `hrabbitmq`'s `reliableoutbox.PrepareOptimisticPublish` for the optimistic publish path within API routes
6.And use `hrabbitmq`'s `reliableoutbox.ScheduledFlushPendingMessages` to run the background scheduling

Done!
