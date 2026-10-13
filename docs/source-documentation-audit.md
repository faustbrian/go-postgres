# Source documentation audit

[Documentation index](README.md) · [Repository entry](../README.md)

## Scope and reproducible inventory

Baseline: `b24e5e40d470fa3f2f454de0c79666e639c16413`. Tier A: source comments
and necessary consumer-facing clarification only; executable behavior, module
inputs and external API type baselines remain unchanged. The owner is this
standalone repository, not the coordination directory.

Reproduce with `git ls-files '*.go'`, `git ls-files '*go.mod'`,
`golib source-docs inventory --json`, and `rg -n 'go:build|go:embed|go:generate|Code generated|Deprecated:' --glob '*.go'`.
The inventory contains 48 tracked Go files, two modules and 12 package
directories, including the independently resolved published-v1 migration
consumer. No vendored or generated Go source is rewritten. The only build
constraint is `integration`, on the integration test and benchmark. Embedded
migration SQL is an input, not a generated source artifact. There is no local
Go generator. Deprecation and analyzer directives remain intact.

| File | Module directory | Package | Kind | Function declarations |
| --- | --- | --- | --- | ---: |
| `adapters/otel/observer.go` | `.` | `postgresotel` | source | 2 |
| `adapters/otel/observer_test.go` | `.` | `postgresotel` | test | 11 |
| `adapters/service/example_test.go` | `.` | `postgresservice_test` | test | 1 |
| `adapters/service/service.go` | `.` | `postgresservice` | source | 11 |
| `adapters/service/service_test.go` | `.` | `postgresservice_test` | test | 14 |
| `benchmark_test.go` | `.` | `postgres` | test | 6 |
| `config.go` | `.` | `postgres` | source | 13 |
| `config_admission_boundaries_test.go` | `.` | `postgres` | test | 4 |
| `config_admission_test.go` | `.` | `postgres` | test | 23 |
| `config_limits.go` | `.` | `postgres` | source | 3 |
| `config_test.go` | `.` | `postgres` | test | 20 |
| `doc.go` | `.` | `postgres` | source | 0 |
| `documentation_test.go` | `.` | `postgres` | test | 2 |
| `errors.go` | `.` | `postgres` | source | 18 |
| `errors_test.go` | `.` | `postgres` | test | 8 |
| `examples/migrations/main.go` | `examples/migrations` | `main` | source | 4 |
| `examples/migrations/main_test.go` | `examples/migrations` | `main` | test | 1 |
| `examples/migrations/telemetry_consumer_test.go` | `examples/migrations` | `main` | test | 1 |
| `examples/service/main.go` | `.` | `main` | source | 2 |
| `examples/sqlc/main.go` | `.` | `main` | source | 4 |
| `examples/worker/main.go` | `.` | `main` | source | 2 |
| `fuzz_test.go` | `.` | `postgres` | test | 6 |
| `integration_benchmark_test.go` | `.` | `postgres_test` | test | 3 |
| `integration_test.go` | `.` | `postgres_test` | test | 24 |
| `internal/oteladapter/observer.go` | `.` | `oteladapter` | source | 2 |
| `internal/telemetry/categories.go` | `.` | `telemetry` | source | 5 |
| `internal/telemetry/categories_test.go` | `.` | `telemetry_test` | test | 1 |
| `leak_test.go` | `.` | `postgres` | test | 1 |
| `observe.go` | `.` | `postgres` | source | 4 |
| `otelpostgres/observer.go` | `.` | `otelpostgres` | source | 2 |
| `otelpostgres/observer_test.go` | `.` | `otelpostgres` | test | 11 |
| `otelpostgres/telemetry_admission_test.go` | `.` | `otelpostgres` | test | 1 |
| `pool.go` | `.` | `postgres` | source | 23 |
| `pool_test.go` | `.` | `postgres` | test | 24 |
| `postgresservice/compatibility_internal_test.go` | `.` | `postgresservice` | test | 1 |
| `postgresservice/example_test.go` | `.` | `postgresservice_test` | test | 1 |
| `postgresservice/postgresservice.go` | `.` | `postgresservice` | source | 10 |
| `postgresservice/postgresservice_test.go` | `.` | `postgresservice_test` | test | 15 |
| `postgrestest/database.go` | `.` | `postgrestest` | source | 16 |
| `postgrestest/database_test.go` | `.` | `postgrestest` | test | 17 |
| `postgrestest/transaction.go` | `.` | `postgrestest` | source | 4 |
| `postgrestest/transaction_test.go` | `.` | `postgrestest` | test | 20 |
| `resolver_admission_hosted_test.go` | `.` | `postgres_test` | test | 1 |
| `slog.go` | `.` | `postgres` | source | 2 |
| `slog_test.go` | `.` | `postgres` | test | 3 |
| `telemetry_admission_test.go` | `.` | `postgres` | test | 2 |
| `transaction.go` | `.` | `postgres` | source | 7 |
| `transaction_test.go` | `.` | `postgres` | test | 30 |

## Gaps and rationale questions before source edits

Missing useful public field/member comments occur in configuration, statistics,
health, observations, error metadata and adapter/test-support contracts. Package
entry documentation is especially thin at the root and executable examples.
Names alone do not explain borrowed handles, cleanup ownership, blocking
callbacks, privacy boundaries or transaction failure windows.

Questions to resolve from source and tests rather than infer: which operations
reject nil contexts; does a shutdown deadline bound cleanup or only the caller's
wait; who releases acquired connections; what happens after returned commit
errors versus commit panic; which metadata is sanitized; can observer blocking
be preempted; when service constructors transfer ownership; and which test
cleanup failures remain inspectable? Remaining design questions must not be
turned into invented guarantees or behavioral fixes.

High-risk files: `config.go`, `config_limits.go`, `pool.go`, `transaction.go`,
`observe.go`, `errors.go`, service adapter and compatibility facade,
`postgrestest/database.go` and `postgrestest/transaction.go`. Review their private
state transitions and rationale alongside public entry points. Native pgx,
OpenTelemetry providers and application callbacks retain their own contracts.

All test helpers, both service examples, five fuzz targets, six root
microbenchmarks, two no-op OpenTelemetry microbenchmarks and two tagged
PostgreSQL benchmarks, integration, leak, admission, privacy and adapter tests
are audited as source evidence. No live database,
container, cancellation-race or performance qualification is implied by this
prose pass. Existing SQLc-style methods and migration filesystem methods are
private implementation members, not new public API obligations.

## Selected checks

Use pinned Go 1.27.2 after the serialized slot is granted. Compare all tracked
Go executable tokens, comment-free ASTs and compiler/analyzer directives with
the baseline; check native formatting and render every package's documentation.
Inspect `.golib.yaml`, `scripts/check-api.sh` and `documentation_test.go`: the
native API oracle is root `apidiff` against `api/v2.export`, not a full-prose
Go documentation snapshot. No new export baseline is warranted for comments.
Run the source checker and the native documentation/example selector without
live integration tags; inspect local links, generation inputs, debt markers,
changed-content secrets and the complete final diff. Do not run broad unchanged
runtime, container, race, fuzz or benchmark campaigns. Exact task-owned Go
caches and temporary rendering storage are removed after the bounded session.

## Contract inventory and resolved questions

The native AST inventory covers 267 public declarations/members, including
embedded fields, interface obligations, constants and deprecated variants.
External pgx alias members retain their upstream contract; this pass does not
rewrite dependency source. Reproduce the rendered inventory with `go doc -all`
in each of the 12 directories listed in `modules.json`.

| Package surface | Audited types and contracts |
| --- | --- |
| Root configuration | Config, ConfigError, ConfigLimits, PoolConfig, TLSMode/TLSConfig, StartupPolicy; constants, fields, PrepareConfig/ParseConfig and Error/Unwrap |
| Root pool | Pool, Stats, Health; sentinel errors, Connect/New, Raw, Acquire, Ping, Readiness, Liveness, Stats, Shutdown/Close |
| Root transactions | Beginner.BeginTx, TransactionOptions, SavepointOptions; RunTransaction, RunSavepoint and RunSavepointWithOptions |
| Root diagnostics/observation | ErrorKind/ErrorInfo, Classify, SQLState, IsKind and all kind/retry predicates; Operation, Outcome, Observation, Observer.Observe, ObserverFunc.Observe, NewSlogObserver |
| Canonical and retained service adapters | Resource.Ping/Close, Constructor, Options, OptionsError.Error/Unwrap, StartupError.Error/Unwrap, Adapter and New/Component/Pool/Readiness; retained type/error identities and migration paragraphs |
| Canonical and retained OpenTelemetry adapters | Config.MeterProvider, Observer, New and Observe; distinct instrumentation scopes and borrowed provider lifecycle |
| Test support | Config, Database and Open/Start/DSN/Container/Shutdown/Close; IsolationBeginner.Begin, RunIsolated and cleanup/callback sentinels |
| Internal/executable packages | Finite telemetry projection, shared observer instruments, application-owned pool/HTTP/worker lifecycles and independent published-v1 migration consumer |

- Preparation/Connect and owned Shutdown explicitly reject nil contexts;
  acquisition, probes and transaction helpers require usable contexts rather
  than promising uniform nil refusal. Resolver causes are withheld; trusted
  hook/native error graphs are not globally redacted.
- Raw pools, acquired connections, callback transactions, adapter Pool and
  Container handles are borrowed. Release connections, do not finalize callback
  transactions, and do not independently close owned resources.
- Pool shutdown bounds each caller's wait, not native cleanup. Container
  termination uses one detached cooperative cleanup context; caller wait and
  terminal cleanup outcome are separate. Channel ownership is adjacent to code.
- Successful callback return requests commit. A returned commit failure is
  terminal without another rollback or a durability guarantee; commit panic
  leaves rollback armed. Cleanup detaches caller cancellation/deadlines but
  preserves values. Negative transaction cleanup durations expire immediately.
- Observers execute synchronously before results are returned, may be concurrent,
  and have no recursion guard. Ordinary panic recovery does not isolate blocking
  or goroutine termination. One directly contradicted observability sentence was
  narrowed accordingly; no runtime repair was bundled.
- Constructors own failed partial acquisitions. Successful service resources
  transfer; failed startup ping attempts owned cleanup. Shared pools are never
  closed, and lifecycle snapshots are not leases against shutdown. The adapter
  is for one ordered lifecycle, not a restartable or concurrent Start factory.
- Raw diagnostic server fields remain sensitive. Telemetry is a separate finite
  categorical projection; no observer provider/exporter lifecycle is acquired.
- No external specification edition is pinned in the catalog. Native pgx/v5
  v5.11.0 source confirms statistics meanings; PostgreSQL server semantics,
  isolation and dialect claims remain the integration suite's responsibility.
- No actionable debt/note/warning marker was found in tracked Go comments.
  Eight retained service-facade deprecation markers needed separate paragraphs;
  migration text and all compiler/analyzer/embed/build directives were preserved.
  Handwritten SQLc-style code is explicitly not labeled generated.

## Selected verification and outcome

- Go 1.27.2 native formatting passed for all 48 files. A disposable stdlib
  parser/scanner compared executable tokens (including inserted semicolons and
  literal spellings), normalized comment-free ASTs, and compiler/analyzer
  directives with the exact baseline: identical across all files.
- `go doc -all` rendered all 12 catalog package directories; rendered member
  contracts were inspected, final affected packages rerendered, and root output
  reproduced twice byte-stably. No tracked prose snapshot exists. The configured
  and native API selection is root apidiff against `api/v2.export`; that baseline
  remains unchanged. The export analyzer is not claimed as locally executed.
- The native documentation/example selector passed:

  ```sh
  go test ./... \
    -run 'TestExportedSymbolsHaveGoDocumentation|^Example' \
    -count=1 -timeout=3m
  ```

  The two unchanged service examples execute with fake Resources and
  do not claim PostgreSQL qualification.
- In `examples/migrations`, `go mod tidy -diff` produced no diff and
  `go test . -count=1 -timeout=3m` passed the embedded-source and published
  telemetry SDK consumer checks. These checks do not acquire a database.
- `golib source-docs inventory --json` and `golib source-docs check --json`
  cover all tracked files and report no structural findings. Shared tooling/CI
  adoption remains coordinator-owned, not a delivered-CI claim from this pass.
- Native local Markdown path validation and Unreleased extraction passed.
  Embedded SQL inputs, module/workspace manifests/sums and API export baseline
  are unchanged; no generated output needs regeneration.
- Pinned Gitleaks v8.30.1 scans changed-current full files with the owned policy,
  inline allow suppression disabled, an empty ignore file and full redaction:
  zero findings. Changed-content inspection and ordinary complete Tier A review
  found no executable changes or unresolved source-documentation finding.
- All three exact task-owned Go caches, parser helper, rendered copies and secret
  snapshots/reports were removed after the bounded session. The detached
  committed worktree is preserved for coordinator-owned delivery.

No unresolved rationale question required an invented guarantee. Live database,
container, hosted-only admission, server-major matrix, race/leak, active fuzz,
benchmark, external CI and publication were not executed for this unchanged
runtime. Unchanged behavior evidence remains bounded to its original producers.

## Objective Enforcement

The checksum-pinned public Tools v2.4.0 verifier now supplies
`make source-docs-check` as a prerequisite of the existing local and CI Make
routes. Ordinary shared CI selects strict source-documentation checking and
requires its result. Native prerequisites, operation budgets, security and
release controls are retained. Hosted exact-head evidence is recorded by the
coordination ledger; configuration alone does not establish green delivery.

Choose the next guide in the [documentation index](README.md)
or return to the [repository entry](../README.md) for installation and ownership.
