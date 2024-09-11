package pgxlisten

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type dispatcher struct {
	pool     *pgxpool.Pool
	channels map[string]*channel
	cancel   context.CancelFunc
	stopped  chan struct{}
	err      error
}

func startDispatcher(
	ctx context.Context, pool *pgxpool.Pool, channels map[string]*channel,
) *dispatcher {
	ctx, cancel := context.WithCancel(ctx)

	d := &dispatcher{
		pool:     pool,
		channels: channels,
		cancel:   cancel,
		stopped:  make(chan struct{}),
	}

	go func() {
		defer close(d.stopped)
		defer d.cancel()
		d.err = d.run(ctx)
	}()

	return d
}

func (d *dispatcher) Stop() {
	d.cancel()
	<-d.stopped
}

func (d *dispatcher) Stopped() <-chan struct{} {
	return d.stopped
}

func (d *dispatcher) Err() error {
	if d.err != nil && errors.Is(d.err, context.Canceled) {
		return nil
	}
	return d.err
}

func (d *dispatcher) run(ctx context.Context) error {
	conn, err := d.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer func() {
		_ = conn.Conn().Close(ctx)
		conn.Release()
	}()

	for name, c := range d.channels {
		if _, err := conn.Exec(
			ctx, `LISTEN `+pgx.Identifier{name}.Sanitize(),
		); err != nil {
			return fmt.Errorf("listen channel: %q: %w", name, err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.done:
		case c.notifications <- Notification{ConnectionReset: true}:
		}
	}

	for {
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return fmt.Errorf("wait for notification: %w", err)
		}

		c, ok := d.channels[notification.Channel]
		if !ok {
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.done:
		case c.notifications <- Notification{Payload: notification.Payload}:
		}
	}
}
