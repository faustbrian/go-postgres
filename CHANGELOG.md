# Changelog

All notable changes are documented here. The project follows Semantic
Versioning and keeps an Unreleased section until a release is tagged.

## [Unreleased]

## [3.0.0] - 2026-10-22

### Security

- Build with Go 1.27.3 to address GO-2026-6809 in `crypto/x509`.
  Consumers should use Go 1.27.3 or newer.

### Removed

- Remove the `github.com/faustbrian/go-postgres/v2/postgresservice` and
  `github.com/faustbrian/go-postgres/v2/otelpostgres` import paths.
- Remove the Service v1 `Component`/`Readiness` binding and the
  `ComponentV2`/`ReadinessV2` methods from `adapters/service`; its
  `Component` and `Readiness` now return Service v3 types.
- Remove `New`, `Pool.Close`, `ParseConfig`, `RunSavepointWithOptions`,
  `postgrestest.Start` and `Database.Close`. Each had a single canonical
  replacement; version 3 keeps only that owner.

### Changed

- Move the module to `github.com/faustbrian/go-postgres/v3`.
- Bind `adapters/service` directly to `github.com/faustbrian/go-service/v3`.
  `Resource` now requires `Ping` and `Shutdown`, which `*Pool` satisfies.
- Merge the savepoint entry points: `RunSavepoint` takes `SavepointOptions`.
- Report OpenTelemetry metrics under the
  `github.com/faustbrian/go-postgres/v3/adapters/otel` instrumentation scope.
  Metric names, units and bounded attributes are unchanged.

## [2.1.0] - 2026-10-15

### Added

- Add `ComponentV2` and `ReadinessV2` to the canonical service adapter for
  native Service v2 composition. Existing Service v1 bindings remain supported;
  both bindings share the same pool ownership, readiness and cleanup lifecycle.

### Changed

- Clarify native pool borrowing and caller-bounded shutdown, transaction commit
  failure windows and detached rollback, synchronous observer privacy/liveness
  limits, adapter ownership transfer and real-fixture cleanup in source docs.
  Public types and executable behavior are unchanged.

## [2.0.1] - 2026-10-08

### Changed

- Adopt Moby API v1.56.1 for the PostgreSQL test-container transport and the
  independently resolved migration example. Preserve fixture startup, stable
  loopback port binding, restart, and cleanup with Testcontainers v0.44.0 and
  Moby client v0.5.0; owned pool and transaction contracts remain unchanged.

- Adopt Testcontainers core and PostgreSQL modules at v0.44.0 for the public
  `postgrestest` helper and retain the separate published-v1 migration example.
  Native container logging now uses stderr by default; upstream session-ID
  configuration and buffered exec-output behavior remain native contracts.
  Pool APIs, startup/readiness, shutdown ownership, and PostgreSQL support
  remain unchanged.
- Update OpenTelemetry API and metric modules to v1.47.0 while retaining the
  v1.45.0 SDK and PostgreSQL instrumentation scopes and metric contracts.
- Align the standalone migrations example with pgx v5.11.0, already used by
  the root library, while preserving its separately published v1 dependency.
  Connection URLs now follow libpq rules: `+` stays literal and the last
  repeated query parameter wins. Text-format timestamps with time zones use
  the client scan location, matching the binary format. See the
  [pgx v5.11.0 changes](https://github.com/jackc/pgx/releases/tag/v5.11.0).
- Adopt stable HTTP metric conventions in the Docker client used by test
  containers. Dashboards consuming dependency-provided HTTP metrics must use
  `http.client.request.duration` and `http.client.request.body.size`.
  The legacy `http.client.response.size` instrument is no longer emitted.
  HTTP request duration is measured in seconds instead of milliseconds.

## [2.0.0] - 2026-10-07

### Changed

- Prepare the v2 module/import path on main. Require an explicit application
  `ResolveDSN` collaborator, add context-aware `PrepareConfig` and `Configure`,
  admit DSN/native credential shapes and cache capacities before owned work,
  and revalidate hook results. Resolver errors omit untrusted causes.
- Default startup to lazy without proactive minimum connections; request
  `StartupPing` explicitly. Lazy positive minima and out-of-policy native
  timeouts/pool settings are rejected rather than silently changed. See the
  migration guide; published v1 consumers remain a separate baseline.

### Fixed

- Upgrade root pgx to v5.11.0 to fix quoted connection-value bounds handling.
  No package panic recovery hides resolver failures. Admit its new native
  message-body allowance to an explicit 8 MiB default/ceiling with positive
  reductions; post-Configure zero, negative and over-budget values are refused.

- Upgrade the OpenTelemetry SDK and aligned API modules to v1.45.0, fixing
  GHSA-8wmf-6v46-5gfg's disclosure of exporter configuration through verbose
  internal diagnostics. Keep unrelated Docker and HTTP instrumentation pins.

- Project telemetry categories to fixed recognized values or `unknown` before
  custom observer delivery and built-in slog/OpenTelemetry emission, including
  direct canonical and retained adapter calls. Native error classification and
  SQLSTATE inspection remain unchanged.

## [1.1.0] - 2026-09-09

### Added

- Add context-first `Connect` and `postgrestest.Open` acquisition APIs,
  caller-independent `Pool.Shutdown` and `Database.Shutdown` lifecycle APIs,
  and canonical `adapters/otel` and `adapters/service` integration paths.

### Changed

- Preserve `New`, `Pool.Close`, `postgrestest.Start`, `Database.Close`,
  `otelpostgres`, and `postgresservice` as compatibility delegates while new
  code adopts explicit acquisition, shutdown, and target-oriented adapters.

- Publish schema-v2 cohesion metadata and versioned Golib ecosystem navigation
  for the PostgreSQL module, its lifecycle and telemetry adapters, and its test
  companion.

- Adopt the checksum-verified `go-library-tools` v1.4.0 CLI, add the local
  `make cohesion` validation entry point, and pin reusable-workflow cohesion
  enforcement and public-first module resolution to its immutable revision.

- Resolve Golib dependencies from their public immutable releases instead of
  bootstrap-shadowed module archives.

- Adopt the versioned shared repository contract while retaining package-owned
  verification evidence and PostgreSQL-backed quality gates.

### Fixed

- Upgrade `golang.org/x/crypto` to v0.56.0 so PostgreSQL test-container
  shutdown paths are not exposed to the SSH channel deadlocks described by
  GO-2026-6354 and GO-2026-6355.

- Keep embedded SQL fixtures outside the migration binary's default output
  path so standard module builds complete without a filesystem collision.

### Documentation

- Complete the stable-v1 entry path with canonical installation, executable
  examples, package and policy navigation, root-tag guidance, and live support
  and private security-reporting routes.

- Replace pre-v1 security guidance with the supported `v1` release line and
  correct the migration example's Go version requirements.

- Link the README to the immutable v1.4.0 Persistence and durability family
  guidance.

- Replace archived monorepo links and completed execution artifacts with a
  standalone, human-oriented documentation structure.

## [1.0.1] - 2026-08-25

### Fixed

- Reconcile the migration example with the final standalone root archive
  after documentation generation changed its module checksum.

### Changed

- Upgrade `moby/go-archive` and `golang.org/x/crypto` to their current
  security-fixed releases and reconcile the resulting indirect dependency
  graph.

- Exclude intentional nested modules from root local-proxy archives so local,
  bootstrap, CI, and public module checksums describe the same source
  boundary.

- Track the pinned documentation-tool lockfile so clean CI checkouts install
  the exact validated cspell dependency.

- Reconcile standalone dependency checksums against deterministic current
  module archives so CI, local verification, and release consumers resolve
  identical content.

- Harden standalone documentation validation with deterministic spelling and
  link checks, package-specific documentation gates, and repository-local
  contributor guidance.

### Documentation

- Correct stale package, standalone, and authoritative-source links in public
  documentation.

### Documentation

- Replace obsolete standalone-repository links and workflow claims with
  monorepo-canonical targets and current release guidance.
- Document the package's initial stable `v1.0.1` scope, security policy,
  roadmap, and hardening guidance.

- Link the package README to package-owned documentation.

### Added

- A `postgresservice` lifecycle adapter for constructor-created or existing
  pools, optional startup validation and readiness, and explicit close
  ownership.
- `postgrestest.RunIsolated` for bounded always-rollback integration tests
  that preserve callback errors and panic values
- pinned fail-closed API compatibility checks against the committed prospective
  v1 module baseline

### Changed

- Publish the module from its standalone `github.com/faustbrian/go-postgres` identity while preserving its documented API and behavior.
- Replace obsolete owned-module pseudo-version pins with the monorepo's local
  `v0.0.0` source-proxy coordinates; release tooling continues to emit exact
  `v1.0.0` dependency versions.
- Remove unused CLI-related indirect dependencies from canonical module
  metadata.
- Pin owned sibling modules to exact resolvable main pseudo-versions so
  standalone and clean external consumers use immutable dependency content.

- Run the pinned exported API compatibility check as part of the complete
  module gate.
- API compatibility tooling now runs against the isolated module graph so owned
  dependency source changes cannot conflict with release checksums.
- hardening documentation records exact hosted release-candidate proof
- security support and roadmap documentation reflect the stable v1 state
- hardening evidence now covers the full transaction-mode matrix, representative
  DSN forms, native pool hooks, authentication redaction, and strict TLS refusal
- nested savepoint evidence now proves inner rollback and outer persistence
- startup evidence now covers wrong-protocol endpoints and explicit native DNS
  ownership alongside stable-endpoint recovery
- performance documentation now records a dated allocation baseline for every
  required helper and real-PostgreSQL benchmark
- API compatibility checks now use the repository-pinned current `apidiff`
  revision

### Fixed

- Migration example checks no longer retain mutable checksums for local
  zero-version workspace modules.
- transaction, savepoint, and isolated-test callbacks that terminate their
  goroutine now roll back exactly once; observed operations report `aborted`
- rollback panics can no longer replace a callback panic or convert
  `runtime.Goexit` into a different terminal failure
- commit and savepoint-release panics now attempt bounded rollback before the
  original panic propagates, preventing stranded pooled transactions
- rollback panics can no longer replace returned transaction or isolation
  callback errors
- panicking Testcontainers setup hooks now clean up the owned container before
  preserving the original panic, including when termination also panics
- Testcontainers cleanup panics can no longer replace a returned setup error
- Testcontainers termination panics can no longer replace connection-string
  startup failures
- Testcontainers setup hooks that call `testing.T.FailNow` now clean up the
  owned container before terminating the test goroutine
- typed TLS overrides copy certificate pools and mutable protocol and
  certificate data before pool construction
- successful isolated callbacks no longer hide a rollback panic as success
- `golang.org/x/text` now uses the latest fixed release, removing
  `GO-2026-5970` from reachable pgx pool construction paths

### v1.0.1 scope

The following initial scope is included in `v1.0.1`.

#### Added

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
- executable sqlc, migrations, service, and worker adoption examples
- exact production coverage, race, leak, fuzz, benchmark, safety, lint,
  vulnerability, documentation, compatibility, and release automation

[Unreleased]: https://github.com/faustbrian/go-postgres/compare/v3.0.0...HEAD
[3.0.0]: https://github.com/faustbrian/go-postgres/compare/v2.1.0...v3.0.0
[2.1.0]: https://github.com/faustbrian/go-postgres/compare/v2.0.1...v2.1.0
[2.0.1]: https://github.com/faustbrian/go-postgres/compare/v2.0.0...v2.0.1
[2.0.0]: https://github.com/faustbrian/go-postgres/compare/v1.1.0...v2.0.0
[1.1.0]: https://github.com/faustbrian/go-postgres/compare/v1.0.1...v1.1.0
[1.0.1]: https://github.com/faustbrian/go-postgres/releases/tag/v1.0.1
