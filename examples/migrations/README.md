# Migration job

This executable combines `postgres`, pgx's `database/sql` bridge, and
`migrations`. It plans and applies embedded migrations once as a dedicated
deployment job:

```sh
DATABASE_URL='postgres://app:secret@localhost/app' go run .
```

The example is a nested module because it composes independently released
`migrations` and `postgres` modules. Both use Go 1.27.0 for validation. The local
`replace` directive ensures CI compiles this example against the checkout under
test. Build deployment images with an explicit output path such as
`go build -o /app/migrate .`.

Do not run migrations implicitly during service startup. Supply a dedicated
DDL identity and review the plan, locks, statement timeout, and rollback
strategy before deploying a migration.
