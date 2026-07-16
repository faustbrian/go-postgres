# Changelog

All notable changes are documented here. The project follows Semantic
Versioning and keeps an Unreleased section until a release is tagged.

## [Unreleased]

## [1.0.0] - 2026-07-16

### Added

- finite typed pgxpool configuration with secret-safe validation and panic
  containment for malformed DSNs
- fail-fast or lazy startup, bounded acquire, health, statistics, liveness, and
  shutdown operations with direct native pool access
- context-aware transaction and savepoint runners with panic-safe rollback and
  preserved callback, commit, and rollback errors
- SQLSTATE, constraint, cancellation, timeout, connectivity, saturation, and
  retry-advisory classification without flattening original errors
- bounded lifecycle observations, safe `slog`, and optional OpenTelemetry
  metrics that omit SQL, arguments, DSNs, and raw database errors
- Testcontainers support plus PostgreSQL 14-18 integration evidence for
  contention, deadlock, serialization, cancellation, server timeouts,
  constraints, transaction loss, stop/restart, saturation, shutdown, and cleanup
- executable sqlc, go-migrations, service, and worker adoption examples
- exact production coverage, race, leak, fuzz, benchmark, safety, lint,
  vulnerability, documentation, compatibility, and release automation

[Unreleased]: https://github.com/faustbrian/go-postgres/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/faustbrian/go-postgres/releases/tag/v1.0.0
