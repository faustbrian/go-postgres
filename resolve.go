package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ResolveNativeDSN is an explicit opt-in Config.ResolveDSN that parses the
// admitted DSN with pgx's native parser. The parser consults the process
// environment (PG* variables), passfile, service file and TLS files named by
// the DSN or that environment, so selecting it is the application's choice of
// ambient credential acquisition. It returns the context cause when ctx is
// already done and does not interrupt a parse that has started.
func ResolveNativeDSN(ctx context.Context, dsn string) (*PoolConfig, error) {
	if ctx == nil {
		return nil, ErrContextRequired
	}
	if ctx.Err() != nil {
		return nil, context.Cause(ctx)
	}
	return pgxpool.ParseConfig(dsn)
}
