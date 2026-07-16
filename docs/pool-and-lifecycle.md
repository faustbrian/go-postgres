# Pool construction and lifecycle

## Sizing

Start from the PostgreSQL connection budget, subtract administration,
migration, replication, and failover headroom, then divide the remainder among
maximum concurrently deployed application replicas. `MaxConns=10` is a finite
library default, not a universal recommendation. `MinIdleConns` reduces cold
acquisition latency but creates connections proactively.

## Timeouts

`ConnectTimeout` bounds each network connection. `AcquireTimeout` bounds queue
wait plus any connection establishment performed for acquisition. Request
deadlines earlier than these values win. `PingTimeout` applies to startup and
readiness. Query deadlines remain caller-owned and must be applied to every
request or job context.

## Startup

`StartupPing` is the default and proves DNS, transport, TLS, authentication,
server acceptance, session initialization, and one pool acquisition before the
application announces startup. `StartupLazy` defers all of that and should be
reserved for systems whose orchestrator or worker loop owns retry behavior.

## Session initialization

`SessionInit` runs once for every newly established connection, after a native
`AfterConnect` hook. A native hook error skips session initialization. Any
session initialization error rejects the connection and is preserved through
`errors.Is` and `errors.As`. Keep the hook idempotent, bounded by its context,
and free of process-external side effects.

## Saturation

When the configured acquisition deadline expires, the error matches
`ErrAcquireTimeout` and `context.DeadlineExceeded`. It also matches
`ErrPoolExhausted`, and classifies as `ErrorPoolExhaustion`, only when the
contemporaneous pool snapshot shows every slot acquired or constructing. An
earlier caller deadline or cancellation retains its context classification.
Inspect `Stats.AcquiredConns`, `IdleConns`, `EmptyAcquireCount`, and
`EmptyAcquireWaitTime`. Do not increase pool size before ruling out leaked
connections, long transactions, slow queries, and excessive concurrency.

## Shutdown

`Close` starts native shutdown exactly once. pgxpool must wait for borrowed
connections, but the wrapper stops waiting at the caller or configured
deadline and returns `ErrShutdownTimeout`. Native shutdown continues in one
background goroutine. Stop accepting work, cancel workers, wait for handlers,
then close the pool. Returning borrowed connections is mandatory.
