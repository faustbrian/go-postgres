package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	postgres "github.com/faustbrian/go-postgres/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// resolveDSN deliberately opts this application into native ambient parsing.
// pgx parsing itself is synchronous; callbacks must remain cooperatively bounded.
func resolveDSN(ctx context.Context, dsn string) (*postgres.PoolConfig, error) {
	if ctx.Err() != nil {
		return nil, context.Cause(ctx)
	}
	return pgxpool.ParseConfig(dsn)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Connect(ctx, postgres.Config{
		ResolveDSN:      resolveDSN,
		StartupPolicy:   postgres.StartupPing,
		DSN:             os.Getenv("DATABASE_URL"),
		MaxConns:        8,
		AcquireTimeout:  time.Second,
		ShutdownTimeout: 10 * time.Second,
	})
	if err != nil {
		slog.Error("database startup failed", "error", err)
		os.Exit(1)
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if err := pool.Shutdown(context.Background()); err != nil {
				slog.Error("database shutdown failed", "error", err)
			}
			return
		case <-ticker.C:
			err := postgres.RunTransaction(ctx, pool.Raw(), postgres.TransactionOptions{}, func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `
					UPDATE jobs
					SET claimed_at = now()
					WHERE id = (
						SELECT id FROM jobs
						WHERE claimed_at IS NULL
						FOR UPDATE SKIP LOCKED
						LIMIT 1
					)
				`)
				return err
			})
			if err != nil && !postgres.IsCancellation(err) {
				slog.Error("claim job failed", "kind", postgres.Classify(err).Kind)
			}
		}
	}
}
