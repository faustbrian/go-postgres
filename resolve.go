package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ResolveNativeDSN is an explicit opt-in Config.ResolveDSN that parses the
// admitted DSN with pgx's native parser, so selecting it is the application's
// choice of ambient credential acquisition. The parser consults PG* environment
// variables and reads the passfile, service file and TLS certificate, key and
// root files, both those named by the DSN or environment and pgx's
// home-directory defaults (~/.pgpass, ~/.pg_service.conf, ~/.postgresql/*). It
// may also look up the OS user account. A nil ctx returns ErrContextRequired and
// a done ctx returns its cause before parsing; parsing takes no context, so its
// file reads can block past ctx's deadline. Errors returned directly come from
// pgx and may carry the connection string; PrepareConfig withholds them.
func ResolveNativeDSN(ctx context.Context, dsn string) (*PoolConfig, error) {
	if ctx == nil {
		return nil, ErrContextRequired
	}
	if ctx.Err() != nil {
		return nil, context.Cause(ctx)
	}
	return pgxpool.ParseConfig(dsn)
}
