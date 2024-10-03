package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rnovatorov/pgxlisten"
)

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", os.Args[0], err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("new pool: %w", err)
	}
	defer pool.Close()

	groupID := os.Getenv("GROUP_ID")
	if groupID == "" {
		groupID = "default"
	}

	memberID := os.Getenv("MEMBER_ID")
	if memberID == "" {
		memberID = randomID()
	}

	errc := make(chan error, 2)
	go func() {
		errc <- runHeartbeat(ctx, pool, groupID, memberID)
		cancel()
	}()
	go func() {
		errc <- runWatch(ctx, pool, groupID)
		cancel()
	}()
	return errors.Join(<-errc, <-errc)
}

func runHeartbeat(
	ctx context.Context, pool *pgxpool.Pool, groupID, memberID string,
) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		if err := heartbeat(ctx, pool, groupID, memberID); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func runWatch(ctx context.Context, pool *pgxpool.Pool, groupID string) error {
	listener := pgxlisten.StartListener(pool, pgxlisten.WithContext(ctx))
	defer listener.Stop()

	channel := listener.Listen(fmt.Sprintf("members.%s", groupID))
	defer channel.Unlisten()

	members := make(memberlist)

	for {
		select {
		case <-ctx.Done():
			return nil
		case n := <-channel.Notifications():
			if n.ConnectionReset {
				mems, err := listMembers(ctx, pool, groupID)
				if err != nil {
					return fmt.Errorf("list members: %w", err)
				}
				members.truncate()
				members.add(mems...)
				fmt.Printf("members: %v\n", members)
				continue
			}

			var event struct {
				Type string `json:"type"`
				ID   string `json:"id"`
			}
			if err := json.Unmarshal([]byte(n.Payload), &event); err != nil {
				return fmt.Errorf("unmarshal payload: %w", err)
			}
			switch event.Type {
			case "inserted":
				members.add(event.ID)
				fmt.Printf("members: %v\n", members)
			case "deleted":
				members.delete(event.ID)
				fmt.Printf("members: %v\n", members)
			}
		}
	}
}

func heartbeat(
	ctx context.Context, pool *pgxpool.Pool, groupID, memberID string,
) error {
	const ttl = 10 * time.Second

	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			DELETE
			FROM members
			WHERE group_id = $1 AND expires_at <= now()
		`, groupID); err != nil {
			return fmt.Errorf("delete: %w", err)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO members (group_id, id, expires_at)
			VALUES ($1, $2, now() + $3::interval)
			ON CONFLICT (group_id, id) DO UPDATE SET
				expires_at = now() + $3::interval
		`, groupID, memberID, ttl); err != nil {
			return fmt.Errorf("insert: %w", err)
		}

		return nil
	})
}

func listMembers(
	ctx context.Context, pool *pgxpool.Pool, groupID string,
) ([]string, error) {
	rows, _ := pool.Query(ctx, `
		SELECT id
		FROM members
		WHERE group_id = $1
	`, groupID)
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (string, error) {
		return pgx.RowTo[string](row)
	})
}

type memberlist map[string]struct{}

func (m memberlist) add(ids ...string) {
	for _, id := range ids {
		m[id] = struct{}{}
	}
}

func (m memberlist) delete(id string) {
	delete(m, id)
}

func (m memberlist) truncate() {
	for id := range m {
		delete(m, id)
	}
}

func randomID() string {
	buf := make([]byte, 16)
	_, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}
