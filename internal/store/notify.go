package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Listener subscribes to PostgreSQL LISTEN/NOTIFY channels (RF03.9, RNF04.3).
// Automatically reconnects on failure with exponential backoff.
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
// Reconnects automatically on failure (B02 fix).
func (l *Listener) Start(ctx context.Context, onNotify func(payload string)) {
	go func() {
		backoff := 1 * time.Second
		maxBackoff := 60 * time.Second

		for {
			err := l.listen(ctx, onNotify)
			if ctx.Err() != nil {
				// Context cancelled — shutting down
				return
			}

			fmt.Printf("LISTEN/NOTIFY: connection lost: %v, reconnecting in %v\n", err, backoff)
			time.Sleep(backoff)

			// Exponential backoff with cap
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}()
}

// listen runs a single LISTEN session. Returns error when connection is lost.
func (l *Listener) listen(ctx context.Context, onNotify func(payload string)) error {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, fmt.Sprintf("LISTEN %s", l.channel))
	if err != nil {
		return fmt.Errorf("listen on %s: %w", l.channel, err)
	}

	fmt.Printf("LISTEN/NOTIFY: subscribed to channel '%s'\n", l.channel)

	// Reset backoff on successful connection
	for {
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return fmt.Errorf("wait notification: %w", err)
		}

		fmt.Printf("LISTEN/NOTIFY: received on '%s': %s\n",
			notification.Channel, notification.Payload)

		onNotify(notification.Payload)
	}
}

// NotifyChannel sends a NOTIFY to a PostgreSQL channel.
func NotifyChannel(ctx context.Context, pool *pgxpool.Pool, channel string, payload string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf("NOTIFY %s, '%s'", channel, payload))
	if err != nil {
		return fmt.Errorf("store: notify %s: %w", channel, err)
	}
	return nil
}
