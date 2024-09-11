package pgxlisten

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Listener asynchronously acquires a connection from the connection pool,
// issues a `LISTEN` command for each listened to channel, waits for
// notifications to arrive and dispatches them to the listened to channels.
//
// Note: For each Listen call the currently open connection is closed and
// released and a new one is acquired. For this reason Listener is better
// suited for a mode of operation where a set of listened to channels is static
// rather than dynamic.
//
// See also:
//   - https://www.postgresql.org/docs/current/sql-listen.html
//   - https://www.postgresql.org/docs/current/sql-notify.html
type Listener struct {
	pool            *pgxpool.Pool
	logger          logger
	retryInterval   time.Duration
	channelsUpdated chan struct{}
	cancel          context.CancelFunc
	stopped         chan struct{}
	mu              sync.Mutex
	channels        map[string]*channel
}

// StartListener starts a new Listener.
//
// The caller of Start is responsible for calling Stop when the Listener
// instance is no longer needed.
func StartListener(pool *pgxpool.Pool, opts ...listenerOption) *Listener {
	config := newListenerConfig(opts...)

	ctx, cancel := context.WithCancel(config.context)

	l := &Listener{
		pool:            pool,
		logger:          config.logger,
		retryInterval:   config.retryInterval,
		channelsUpdated: make(chan struct{}, 1),
		cancel:          cancel,
		stopped:         make(chan struct{}),
		channels:        make(map[string]*channel),
	}

	go func() {
		defer close(l.stopped)
		defer l.cancel()
		l.run(ctx)
	}()

	return l
}

// Stop stops the Listener.
func (l *Listener) Stop() {
	l.cancel()
	<-l.stopped
}

// Listen creates a channel to receive notifications from PostgreSQL.
//
// Does not immediately cause Listener to issue `LISTEN` command, rather
// schedules it to be done asynchronously.
//
// Once upon creation and once after each reconnect a channel will receive a
// notification with ConnectionReset flag set to true. This may be used to
// avoid losing notifications.
//
// The caller of Listen is responsible for calling Unlisten on the returned
// Channel when it is no longer needed.
//
// Panics if the same channel is attempted to be listened to twice.
func (l *Listener) Listen(name string) Channel {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, ok := l.channels[name]; ok {
		panic("listen to channel twice")
	}

	unlisten := func() {
		l.mu.Lock()
		defer l.mu.Unlock()

		delete(l.channels, name)
		l.log("unlisten %q", name)

		select {
		case l.channelsUpdated <- struct{}{}:
		default:
		}
	}
	c := newChannel(unlisten)

	l.channels[name] = c
	l.log("listen %q", name)

	select {
	case l.channelsUpdated <- struct{}{}:
	default:
	}

	return c
}

func (l *Listener) run(ctx context.Context) {
	for {
		disp := startDispatcher(ctx, l.pool, l.copyChannels())

		select {
		case <-ctx.Done():
			disp.Stop()
			return
		case <-disp.Stopped():
			if err := disp.Err(); err != nil {
				l.log("dispatcher failed: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(l.retryInterval):
			}
		case <-l.channelsUpdated:
			disp.Stop()
		}
	}
}

func (l *Listener) copyChannels() map[string]*channel {
	l.mu.Lock()
	defer l.mu.Unlock()

	channels := make(map[string]*channel, len(l.channels))
	for name, c := range l.channels {
		channels[name] = c
	}

	return channels
}

func (l *Listener) log(format string, v ...any) {
	l.logger.Printf("pgxlisten: "+format, v...)
}
