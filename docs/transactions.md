# Transactions and savepoints

`RunTransaction` accepts any native pgx `BeginTx` implementation and embeds
`pgx.TxOptions`, including isolation, access mode, and deferrability.

| Path | Finalization |
| --- | --- |
| callback returns nil | commit once |
| callback returns error | rollback once; join rollback failure |
| callback panics | rollback once; re-panic original value |
| callback context canceled and returns error | rollback with a bounded context derived using `context.WithoutCancel` |
| begin fails | callback is not invoked |
| commit fails | preserve commit error; pgx defines the transaction as closed or failed |

The closure runs exactly once. The package never retries it because HTTP calls,
messages, file writes, or other external effects cannot be rolled back with
PostgreSQL. Use `IsSerializationFailure`, `IsDeadlock`, and `IsRetryable` to
drive an application-owned retry around a closure proven to contain only safe
effects.

`RunSavepoint` calls `pgx.Tx.Begin`, which creates a pseudo-nested transaction
using `SAVEPOINT`, `RELEASE SAVEPOINT`, and `ROLLBACK TO SAVEPOINT`.
`RunSavepointWithOptions` adds bounded observation without changing those
semantics. A savepoint can recover part of a transaction, but it does not
create independent commit durability and cannot undo external effects.

Keep transactions short. Never wait for user input or remote services while
holding locks. Always pass the operation context to every pgx call.
