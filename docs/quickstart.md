# Quickstart

Install the module and create one application-owned pool:

```sh
go get github.com/faustbrian/go-postgres/v2@v2 # after public v2 publication
```

For a complete compiler-built program, use the
[`examples/service`](../examples/service/main.go) entry point. From a repository
checkout with a development PostgreSQL instance available:

```sh
DATABASE_URL='postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable' \
  go run ./examples/service
```

The package-level construction flow is:

```go
pool, err := postgres.Connect(ctx, postgres.Config{
	ResolveDSN:       resolveDSN, // application resolver from the migration guide
	StartupPolicy:    postgres.StartupPing,
    DSN:             os.Getenv("DATABASE_URL"),
    MaxConns:        20,
    MinIdleConns:    2,
    AcquireTimeout:  2 * time.Second,
    PingTimeout:     time.Second,
    ShutdownTimeout: 10 * time.Second,
    SessionInit: func(ctx context.Context, conn *pgx.Conn) error {
        _, err := conn.Exec(ctx, "SET statement_timeout = '5s'")
        return err
    },
})
if err != nil {
    return err
}
defer pool.Shutdown(context.Background())
```

`Connect` admits the DSN before explicit application resolution and constructs
the native pool. The example explicitly requests a startup ping. The default
`StartupLazy` requires zero final minima and performs no proactive connection.
See [resolver ownership and migration](migration.md#v1-to-v2-safe-configuration).

Use native pgx methods through `pool.Raw()`:

```go
var count int
err := pool.Raw().QueryRow(ctx, "SELECT count(*) FROM jobs").Scan(&count)
```

Use the transaction runner when cleanup and error composition matter:

```go
err := postgres.RunTransaction(ctx, pool.Raw(), postgres.TransactionOptions{},
    func(ctx context.Context, tx pgx.Tx) error {
        _, err := tx.Exec(ctx, "UPDATE jobs SET claimed_at = now() WHERE id = $1", id)
        return err
    },
)
```

Read [TLS](tls.md), [pool lifecycle](pool-and-lifecycle.md), and
[transactions](transactions.md) before production deployment. Additional
compiler-built examples cover [workers](../examples/worker/main.go),
[sqlc](../examples/sqlc/main.go), and a dedicated
[migration job](../examples/migrations/README.md).
