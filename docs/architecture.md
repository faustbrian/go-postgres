# Architecture and ownership

The module sits above pgx and below application repositories or generated
queries:

```text
application / sqlc generated queries
              |
go-postgres lifecycle, transactions, errors, health, observations
              |
pgx / pgxpool / pgconn
              |
PostgreSQL
```

The application owns SQL, statement naming, retries, external side effects,
migrations, schema, authorization, and business transactions. pgx owns the
wire protocol, codecs, connection implementation, query execution, and native
pool. `go-postgres` owns finite policy defaults and repeatable cleanup around
those primitives.

There is no global pool. Construct dependencies at process startup and inject
`pool.Raw()`, a narrow application interface, or generated `sqlc.Queries`.
Configuration hooks intentionally expose native pgx so this package never has
to mirror every upstream extension point.
