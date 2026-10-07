package main

import (
	"context"
	"fmt"
	"os"
	"time"

	postgres "github.com/faustbrian/go-postgres/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type dbtx interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

type queries struct {
	db dbtx
}

func (q *queries) WithTx(tx pgx.Tx) *queries {
	return &queries{db: tx}
}

func (q *queries) CurrentTime(ctx context.Context) (time.Time, error) {
	var value time.Time
	err := q.db.QueryRow(ctx, "SELECT now()").Scan(&value)

	return value, err
}

// resolveDSN deliberately opts this application into native ambient parsing.
// pgx parsing itself is synchronous; callbacks must remain cooperatively bounded.
func resolveDSN(ctx context.Context, dsn string) (*postgres.PoolConfig, error) {
	if ctx.Err() != nil {
		return nil, context.Cause(ctx)
	}
	return pgxpool.ParseConfig(dsn)
}

func main() {
	ctx := context.Background()
	pool, err := postgres.Connect(ctx, postgres.Config{DSN: os.Getenv("DATABASE_URL"), ResolveDSN: resolveDSN, StartupPolicy: postgres.StartupPing})
	if err != nil {
		panic(err)
	}
	defer func() { _ = pool.Shutdown(context.Background()) }()

	generated := &queries{db: pool.Raw()}
	err = postgres.RunTransaction(ctx, pool.Raw(), postgres.TransactionOptions{}, func(ctx context.Context, tx pgx.Tx) error {
		value, err := generated.WithTx(tx).CurrentTime(ctx)
		if err == nil {
			fmt.Println(value.UTC())
		}
		return err
	})
	if err != nil {
		panic(err)
	}
}
