# Package map

[Documentation index](README.md) · [Repository entry](../README.md)

This map follows the current source modules, not a synchronized release.
Use the repository README for published installation versions; an unpublished
source major is not installable merely because its package appears here.
Internal packages and non-releasable fixtures are intentionally excluded.

## github.com/faustbrian/go-postgres/v2

`postgres` is focused production infrastructure for PostgreSQL applications built directly on `pgx/v5`. It standardizes finite pool configuration, bounded lifecycle operations, transaction cleanup, SQLSTATE classification, health, safe telemetry, and real PostgreSQL testing without hiding native pgx types.

| Import path | Responsibility |
| --- | --- |
| [`github.com/faustbrian/go-postgres/v2`](../doc.go) | Use for finite pgx pool configuration, bounded lifecycle, transaction cleanup, SQLSTATE classification, health, and safe observations. |
| [`github.com/faustbrian/go-postgres/v2/otelpostgres`](../otelpostgres/observer.go) | Deprecated compatibility path for adapters/otel. |
| [`github.com/faustbrian/go-postgres/v2/postgresservice`](../postgresservice/postgresservice.go) | Deprecated compatibility path for adapters/service. |
| [`github.com/faustbrian/go-postgres/v2/postgrestest`](../postgrestest/database.go) | Use in tests for a real PostgreSQL container and always-rollback transaction isolation. |
| [`github.com/faustbrian/go-postgres/v2/adapters/otel`](../adapters/otel/observer.go) | Use to adapt PostgreSQL observations to a caller-owned OpenTelemetry meter provider. |
| [`github.com/faustbrian/go-postgres/v2/adapters/service`](../adapters/service/service.go) | Use to adapt a PostgreSQL pool into an explicit go-service lifecycle component. |

Choose the next guide in the [documentation index](README.md)
or return to the [repository entry](../README.md) for installation and ownership.
