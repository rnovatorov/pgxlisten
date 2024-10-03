# pgxlisten

pgxlisten implements a client for the `LISTEN`/`NOTIFY` feature of PostgreSQL on
top of the beautiful `github.com/jackc/pgx` library.

## Usage

```go
pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
if err != nil {
	panic(err)
}
defer pool.Close()

listener := pgxlisten.StartListener(pool)
defer listener.Stop()

channel := listener.Listen("my_channel")
defer channel.Unlisten()

for notification := range channel.Notifications() {
	if notification.ConnectionReset {
		// handle reconnect
		continue
	}
	// process `notification.Payload`
}
```

For more something more realistic check out [examples](examples).

## Alternatives

1. Directly using `WaitForNotification`.
2. Using https://github.com/jackc/pgxlisten.

The API of this library is similar to that of https://github.com/jackc/pgxlisten. The difference is outlined below:

1. Listener uses `pgxpool` instead of a connection factory function, which makes it slightly easier to instantiate `Listener`. Should any custom connection hooks be needed, they may be added as `Listener` options (this is not implemented at the moment).
2. Handlers may be added and removed dynamically, although with a penalty of reacquiring the connection.
3. Notifications are sent using 1-buffered channels so handlers process them concurrently. Should a handler block for a long time, other handlers of course will be blocked as well.
4. The need for backlog processing is signaled using a field in the `Notification` struct. This way the same handler function can process both backlog and new messages.
5. The goroutine responsible for dispatching notifications is spawned inside `Listener`, which removes the burden of managing it from the caller:

```diff
- listenerCtx, listenerCtxCancel := context.WithCancel(ctx)
- defer listenerCtxCancel()
- listenerDoneChan := make(chan struct{})
-
- go func() {
- 	listener.Listen(listenerCtx)
- 	close(listenerDoneChan)
- }()
-
- // do other stuff
-
- listenerCtxCancel()
- <-listenerDoneChan

+ listener := StartListener(...)
+ defer listener.Stop()
+
+ // do other stuff
```

Take a look at [this thread](https://github.com/jackc/pgx/issues/1121) for more details.
