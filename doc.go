// Package postgres provides finite pgx pool configuration, lifecycle helpers,
// transaction cleanup, error classification and bounded observations without
// hiding native pgx types. It is not a driver, ORM, migration engine or retry loop.
//
// PrepareConfig requires an application ResolveDSN collaborator, such as the
// explicit opt-in ResolveNativeDSN, and transfers
// its fresh native configuration through typed defaults and admission checks.
// Connect owns the resulting pool; StartupLazy does not prove connectivity.
// Release every acquired connection and arrange Shutdown after users finish.
// Raw is a borrowed escape hatch, not an independently owned pool.
//
// RunTransaction and RunSavepoint invoke application bodies once. Applications
// own SQL, operation contexts and any external effects; rollback cannot undo
// those effects. Observers are synchronous trusted collaborators. Diagnostic
// errors retain native data, while package telemetry projects finite categories.
// Optional service and OpenTelemetry adapters live under adapters; postgrestest
// provides separately acquired real PostgreSQL fixtures for integration tests.
package postgres
