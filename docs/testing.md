# Testing with Testcontainers

Use `postgrestest` when behavior depends on SQLSTATE, isolation, locking,
constraints, cancellation, session state, pool saturation, or connection loss.
A fake is not equivalent evidence.

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
defer cancel()

database, err := postgrestest.Start(ctx, postgrestest.Config{
    Image: "postgres:18-alpine",
    Setup: func(ctx context.Context, dsn string) error {
        pool, err := pgxpool.New(ctx, dsn)
        if err != nil {
            return err
        }
        defer pool.Close()
        _, err = pool.Exec(ctx, schemaSQL)
        return err
    },
})
if err != nil {
    t.Fatal(err)
}
t.Cleanup(func() { _ = database.Close(context.Background()) })
```

The repository integration suite selects its image with `POSTGRES_VERSION` and
CI runs 14 through 18. Keep tests deterministic: use isolated tables or
databases, explicit timeouts, channel synchronization for contention, and
server-side primitives rather than arbitrary sleeps where possible.

`CleanupTimeout` bounds container termination even after the setup context is
canceled. A failed `Close` may be retried; successful cleanup is idempotent.
Set `HostPort` only when a stop/start test requires a stable loopback endpoint,
and ensure the selected port is isolated from concurrent jobs.

Transaction-isolated tests are appropriate only when code does not commit,
open additional connections, use non-transactional DDL, or depend on session
state outside the test transaction. Otherwise create isolated schema/database
fixtures and clean them explicitly.
