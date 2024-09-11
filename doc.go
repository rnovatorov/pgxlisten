// pgxlisten implements a client for the LISTEN/NOTIFY feature of PostgreSQL on
// top of the beautiful `github.com/jackc/pgx` library.
//
// Usage:
//
//	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
//	if err != nil {
//		panic(err)
//	}
//	defer pool.Close()
//
//	listener := pgxlisten.StartListener(pool)
//	defer listener.Stop()
//
//	channel := listener.Listen("my_channel")
//	defer channel.Unlisten()
//
//	for notification := range channel.Notifications() {
//		if notification.ConnectionReset {
//			// handle reconnect
//			continue
//		}
//		// process `notification.Payload`
//	}
package pgxlisten
