# Compatibility matrix

[Documentation index](README.md) · [Repository entry](../README.md)

| Component | Supported | Verification |
| --- | --- | --- |
| Go | 1.27.0 | repository, race, and clean-consumer gates |
| pgx | 5.11.x for root v3 | pinned module build, tests, API review |
| PostgreSQL | 14, 15, 16, 17, 18 | Testcontainers matrix covering failure semantics and every transaction-mode combination |
| OpenTelemetry Go | API/metric 1.47.0; SDK 1.45.0 | `adapters/otel` SDK collection tests |
| Testcontainers Go | 0.44.x | integration and lifecycle tests |
| Service | v3 `Component`/`Readiness` | native lifecycle, ownership, readiness and cleanup composition tests |

The PostgreSQL range follows the upstream pgx policy at the time of the first
release. Changing the minimum or maximum supported major requires a changelog
entry, compatibility documentation, matrix update, real integration run, and
review of SQLSTATE/session behavior.

Changing pgx minor versions requires reviewing release notes and public
interfaces, then running `make check` and every PostgreSQL major job. The
module exposes native pgx types, so upstream additions are available directly;
upstream removals or behavioral changes may still require a major release here.
pgx 5.11 adds `Rows.TypeMap() *pgtype.Map`; callers implementing the native Rows
interface must implement that method. The root module adopts this upstream contract.
The current pgx hook audit covers `BeforeConnect`, `AfterConnect`,
`PrepareConn`, and `AfterRelease`; other hooks retain native pgx ownership.

Only PostgreSQL is supported. CockroachDB, YugabyteDB, proxies, and poolers may
implement overlapping protocols but require their own explicit evidence and
are not implied by this matrix.

Choose the next guide in the [documentation index](README.md)
or return to the [repository entry](../README.md) for installation and ownership.
