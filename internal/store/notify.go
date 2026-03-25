package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Listener subscribes to PostgreSQL LISTEN/NOTIFY channels (RF03.9, RNF04.3).
// When a notification arrives, the callback is executed.
type Listener struct {
	pool    *pgxpool.Pool
	channel string
}

// NewListener creates a LISTEN/NOTIFY subscriber.
func NewListener(pool *pgxpool.Pool, channel string) *Listener {
	return &Listener{
		pool:    pool,
		channel: channel,
	}
}

// Start begins listening for notifications in a background goroutine.
// Calls onNotify whenever a notification is received on the channel.
func (l *Listener) Start(ctx context.Context, onNotify func(payload string)) {
	go func() {
		conn, err := l.pool.Acquire(ctx)
		if err != nil {
			fmt.Printf("LISTEN/NOTIFY: failed to acquire connection: %v\n", err)
			return
		}
		defer conn.Release()

		_, err = conn.Exec(ctx, fmt.Sprintf("LISTEN %s", l.channel))
		if err != nil {
			fmt.Printf("LISTEN/NOTIFY: failed to listen on %s: %v\n", l.channel, err)
			return
		}

		fmt.Printf("LISTEN/NOTIFY: subscribed to channel '%s'\n", l.channel)

		for {
			notification, err := conn.Conn().WaitForNotification(ctx)
			if err != nil {
				if ctx.Err() != nil {
					// Context cancelled — shutting down
					return
				}
				fmt.Printf("LISTEN/NOTIFY: error waiting: %v\n", err)
				return
			}

			fmt.Printf("LISTEN/NOTIFY: received on '%s': %s\n",
				notification.Channel, notification.Payload)

			onNotify(notification.Payload)
		}
	}()
}

// NotifyChannel sends a NOTIFY to a PostgreSQL channel.
// Called by the API when config changes (blocklists, policies, etc).
func NotifyChannel(ctx context.Context, pool *pgxpool.Pool, channel string, payload string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf("NOTIFY %s, '%s'", channel, payload))
	if err != nil {
		return fmt.Errorf("store: notify %s: %w", channel, err)
	}
	return nil
}
