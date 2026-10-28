# Migration from direct pgx and database/sql

[Documentation index](README.md) · [Repository entry](../README.md)

## v2 to v3 clean replacement

Version 3 keeps one owner for each mechanism and removes the superseded entry
points instead of retaining aliases or forwarding packages:

| Removed in v3 | Replacement |
| --- | --- |
| `New(ctx, config)` | `Connect(ctx, config)` |
| `Pool.Close(ctx)` | `Pool.Shutdown(ctx)` |
| `ParseConfig(config)` | `PrepareConfig(ctx, config)` with the caller's context |
| `RunSavepoint(ctx, tx, cleanupTimeout, fn)` | `RunSavepoint(ctx, tx, SavepointOptions{CleanupTimeout: d}, fn)` |
| `RunSavepointWithOptions` | `RunSavepoint` |
| `postgrestest.Start`, `Database.Close` | `postgrestest.Open`, `Database.Shutdown` |
| `postgresservice` package | `adapters/service` |
| `otelpostgres` package | `adapters/otel` |
| Service v1 `Component`/`Readiness`, `ComponentV2`/`ReadinessV2` | Service v3 `Component`/`Readiness` |

Update imports to `github.com/faustbrian/go-postgres/v3` and move service
composition to `github.com/faustbrian/go-service/v3` in the same change.
`adapters/service.Resource` now requires `Ping` and `Shutdown`; `*Pool`
satisfies it, and custom resources rename their `Close` method.

The OpenTelemetry instrumentation scope becomes
`github.com/faustbrian/go-postgres/v3/adapters/otel`. Metric names, units and
bounded attributes are unchanged; update dashboards or selectors that filter
on the old `github.com/faustbrian/go-postgres/adapters/otel` or
`github.com/faustbrian/go-postgres/otelpostgres` scopes when the consumer
adopts v3. Roll back by reverting the consumer revision as a whole.

## v1 to v2 safe configuration

Published v2.0.0 used `github.com/faustbrian/go-postgres/v2`. Downstream
adoption must still verify each affected application.

Every configuration must supply `ResolveDSN(ctx, dsn)`, returning a fresh,
exclusively transferred, valid pgx parser-created `*PoolConfig`. Nothing is
resolved by default. Applications explicitly own any parser environment,
passfile/service/TLS filesystem access, resolution allocations, and cooperation
with cancellation. Since v3.1.0 an application that deliberately opts into
native pgx resolution, including its ambient environment and files, passes
`postgres.ResolveNativeDSN`:

```go
pool, err := postgres.Connect(ctx, postgres.Config{
    DSN:        dsn,
    ResolveDSN: postgres.ResolveNativeDSN, // explicit ambient acquisition
})
```

`PrepareConfig(ctx, input)` is the single preparation owner used by `Connect`.
`ConfigResolutionTimeout` defaults to five seconds; caller deadlines win.
`Configure` now takes the same preparation context before the native config.
Callbacks must cooperate; deadlines cannot preempt arbitrary application code.
Resolver errors have fixed private messages without upstream causes; trusted
Configure causes and native transaction/error identities remain inspectable.

`Config.Limits` zero fields select defaults: 8 KiB DSN, 16 fallbacks, 128 runtime
parameters, 64 KiB aggregate native strings, and 1024 entries for each native
statement/description cache, plus 8 MiB per native protocol-message body.
Positive values only reduce these ceilings, except the message-body budget,
which may be raised up to `MaximumProtocolMessageBodyCeiling` when an
application reads rows larger than 8 MiB;
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
than lifetime. Final native invariants cannot be bypassed by Configure itself,
but a `BeforeConnect` hook it installs receives a per-connection copy after
admission and is trusted not to relax them.

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

DSN-based Connect callers require explicit resolution.

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

Choose the next guide in the [documentation index](README.md)
or return to the [repository entry](../README.md) for installation and ownership.
