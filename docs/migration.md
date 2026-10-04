# Migration from direct pgx and database/sql

## v1 to v2 safe configuration

The next major uses `github.com/faustbrian/go-postgres/v2` on main, not a new
source directory. Its public release and clean-public-consumer verification
are pending; do not use an unpublished version in downstream modules.
Canonical and retained adapter names remain, with their existing metric scopes.

Every configuration must supply `ResolveDSN(ctx, dsn)`, returning a fresh,
exclusively transferred, valid pgx parser-created `*PoolConfig`. There is no
built-in parser or resolver. Applications explicitly own any parser environment,
passfile/service/TLS filesystem access, resolution allocations, and cooperation
with cancellation. An application that deliberately opts into native pgx
resolution can use:

```go
func resolveDSN(ctx context.Context, dsn string) (*postgres.PoolConfig, error) {
    if ctx.Err() != nil {
        return nil, context.Cause(ctx)
    }
    return pgxpool.ParseConfig(dsn) // application-owned ambient acquisition
}
```

`PrepareConfig(ctx, input)` is the single preparation owner used by `Connect`.
`ParseConfig(input)` delegates with the finite default preparation deadline.
`ConfigResolutionTimeout` defaults to five seconds; caller deadlines win.
`Configure` now takes the same preparation context before the native config.
Callbacks must cooperate; deadlines cannot preempt arbitrary application code.
Resolver errors have fixed private messages without upstream causes; trusted
Configure causes and native transaction/error identities remain inspectable.

`Config.Limits` zero fields select ceilings: 8 KiB DSN, 16 fallbacks, 128 runtime
parameters, 64 KiB aggregate native strings, and 1024 entries for each native
statement/description cache, plus 8 MiB per native protocol-message body.
Positive values only reduce these ceilings;
negative values are invalid. Aggregate bytes count each occurrence of all
native scalar strings (including original connection string), fallback hosts,
and runtime parameter keys/values. Counts precede iteration and byte subtraction
cannot overflow. Shapes are checked before typed overrides and after Configure.
Native caches may be disabled with zero capacities. MaxConns is 1..1024.
The initial native message allowance of zero selects the explicit package
allowance before Configure, rather than relying on the native frontend default.
Final zero, negative or over-budget message allowances are refused. Message
budgets can reject otherwise valid large rows; applications must account for
their query results within this finite contract. Custom BuildFrontend and its
opaque allocations remain bounded, trusted application-owned collaborators.
Connection/preparation/ping/acquisition/shutdown timeouts are positive and at
most one hour; zero typed fields select defaults. Pool lifetime/idle/health
durations are positive and at most 30 days; jitter is nonnegative and no larger
than lifetime. Final native invariants cannot be bypassed by Configure.

`StartupLazy` is now zero/default. Both final minimum connection counts must
be zero under lazy startup, including after Configure; select `StartupPing`
explicitly when startup connectivity or positive minima are required. Native
pool maintenance still has its own shutdown-owned goroutine. Later Acquire,
Ping, Raw operations and application/native callbacks may perform network I/O.

Opaque TLS certificate pools, keys, caches, callbacks, tracers and resolver
backing allocations remain trusted application-owned bounded resources. Typed
TLS defensive copying is retained, not a certificate-size admission policy.
Review these owners if untrusted data or blocking work is introduced; the
credential-shape policy does not certify arbitrary native collaborators.

Queue Control Plane and Service DSN-based Connect callers require explicit
resolution and the paired `/v2` nominal import migration. Audit/postgrestest and
other published v1 consumers remain supported by their existing release.
`examples/migrations` intentionally continues consuming published v1.1.0 and
SDK1.45; its SDK interoperability test does not certify unreleased v2 behavior.

## From direct pgxpool wiring

1. Keep existing SQL and generated queries unchanged.
2. Translate pool sizes, lifetimes, and hooks into `Config`; compare every old
   default because this module deliberately uses finite explicit values.
3. Replace pool construction with `Connect` and pass `pool.Raw()` to existing
   code.
4. Replace manual begin/defer rollback patterns with `RunTransaction` where its
   exactly-once callback contract fits.
5. Replace string matching with `Classify` and SQLSTATE predicates.
6. Add readiness, bounded shutdown, and observation adapters.
7. Run real database contention, cancellation, and failure tests before rollout.

## From database/sql

Generated and handwritten code must move to pgx/v5 types, including `pgx.Rows`,
`pgx.Row`, `pgconn.CommandTag`, codecs, batches, and copy APIs. Audit null,
timestamp, numeric, array, JSON, and custom type semantics. `database/sql`
connection lifetime and transaction behavior are similar concepts but not
identical contracts.

Roll out behind normal service canaries. Compare connection counts, acquisition
wait, error categories, transaction latency, and query results. Do not run two
large pools per replica longer than needed during migration.

## From pre-1.1 Golib APIs

`New` and `Pool.Close` remain source-compatible delegates. Prefer `Connect` and
`Pool.Shutdown` in new code. Replace `postgrestest.Start` and `Database.Close`
with `postgrestest.Open` and `Database.Shutdown`. New integrations should import
`adapters/otel` and `adapters/service`; the `otelpostgres` and
`postgresservice` paths remain deprecated compatibility facades.
