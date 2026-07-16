# Hardening evidence

This report maps production risks to executable evidence. Hosted workflow
status for a release must be checked on the exact tagged commit; local results
alone are not a release verdict.

| ID | Risk | Evidence | Disposition |
| --- | --- | --- | --- |
| GP-001 | DSN parser panic | fuzz corpus `e756db0e3795a10e`, `FuzzParseConfig` | contained with secret-safe error |
| GP-002 | DSN/password disclosure | config and startup tests, safe error types | resolved |
| GP-003 | plaintext TLS fallback | typed TLS primary/fallback tests | resolved when adopter selects strict policy |
| GP-004 | unbounded connection wait | unit and saturation integration tests | resolved |
| GP-005 | shutdown waits forever | canceled close and borrowed-connection tests | resolved for caller wait; native cleanup continues |
| GP-006 | transaction not finalized | success/error/panic/cancel/savepoint and network-loss integration tests | resolved |
| GP-007 | rollback failure hidden | joined callback/rollback tests, including competing classified causes | resolved on returned-error paths |
| GP-008 | unsafe automatic retry | transaction callback runs once; connectivity requires `pgconn.SafeToRetry` evidence | resolved |
| GP-009 | SQLSTATE flattened | wrapped/joined metadata, live constraints, and ambiguous timeout-state tests | resolved |
| GP-010 | telemetry leaks SQL/data | bounded schema and slog/OTel tests | resolved |
| GP-011 | lock/failure behavior assumed | live lock/statement/idle timeouts, deadlock, serialization, cancellation, transaction loss, stop/restart | resolved |
| GP-012 | connection/goroutine leak | stats stress test and goleak test | resolved |
| GP-013 | version claim untested | PostgreSQL 14-18 and Go/OS workflow matrices | pending hosted CI per commit |
| GP-014 | coverage gaming | `coverpkg=./...` plus real PostgreSQL exact 100% gate | resolved locally |
| GP-015 | unsafe/cgo/linkname | GO-SAFETY-1 script and CI | resolved locally |
| GP-016 | test container leak | bounded failure cleanup and retryable termination tests | resolved |
| GP-017 | missing adoption proof | executable sqlc, migration, service, and worker examples | resolved |

Local evidence includes unit, integration, race, exact coverage, fuzz, benchmark,
vet, golangci-lint, actionlint, documentation, and vulnerability gates. The
first `v1.0.0` tag remains blocked until all hosted checks pass for the release
commit and the compatibility matrix has no skipped PostgreSQL major.

Trusted boundaries that remain intentionally application-owned include SQL and
argument policy, hook behavior, TLS roots and identities, role permissions,
query/statement deadlines, migration ordering, exporter availability, retry
idempotency, and external side effects.
