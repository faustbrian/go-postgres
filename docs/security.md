# Security and threat model

Model version: 2.0.0. Reviewed: 2026-10-07. Owner: repository maintainer.
Scope: the root `github.com/faustbrian/go-postgres/v2` module and its canonical
and retained telemetry adapters. This is a source-bound model, not a claim
that the pending v2 release or any application deployment is verified.

## Assets

- PostgreSQL credentials, client keys, certificates, DSNs, and network routes
- SQL arguments and rows, including tenant and personal data
- database availability, connection budget, locks, and transaction integrity
- schema names and server diagnostics

## Threats and controls

| Threat | Control | Application responsibility |
| --- | --- | --- |
| DSN disclosure | validation/startup strings omit DSNs; fuzz regression | do not log input config or unwrapped errors blindly |
| TLS downgrade or impersonation | typed all-host TLS override; TLS guide | verified roots, names, secret rotation, network policy |
| pool exhaustion | finite maximum and acquisition timeout; stats | concurrency limits, capacity budget, leak prevention |
| cancellation loss | caller contexts propagated; uncanceled rollback only | set request/job and statement deadlines |
| partial transaction cleanup | one commit or rollback path; joined errors | avoid external side effects or own retry/idempotency |
| telemetry data leak | bounded event schema; no SQL/arguments/raw errors | safe exporters, allow-listed query names |
| cardinality attack | fixed operation/outcome/kind/state values | never add tenant/input labels in adapters |
| implicit credential acquisition | required explicit resolver after finite DSN admission | bound resolver acquisition and cooperate with context |
| unsafe runtime feature | GO-SAFETY-1 scan forbids unsafe/cgo/linkname | review dependencies and vulnerability reports |

`ErrorInfo.Detail` and `Hint` intentionally preserve native diagnostic data for
authorized application policy, but are not safe log fields. `Config.Configure`
is a trusted extension boundary and can weaken TLS, install unsafe tracers, or
add hooks; review it as production code. Finite native shapes and timeout/pool
invariants are revalidated after it returns. Resolver causes are withheld.

See [v2 configuration migration](migration.md#v1-to-v2-safe-configuration) for
limits and ownership. There is no library-owned native parser/bootstrap call.
Opaque TLS/callback/tracer resources and resolver allocations must have explicit
application bounds; cooperative deadlines are not preemption guarantees.

Telemetry projects caller-supplied categories to fixed recognized values or
`unknown` at custom-observer delivery and built-in output. Exact SQLSTATE
telemetry categories are listed in [observability](observability.md); a class
prefix never retains an arbitrary suffix. This policy does not redact raw
`ErrorInfo`, `SQLState`, or returned native errors, which remain application-owned
diagnostic data.

Typed TLS overrides copy certificate pools, protocol slices, and certificate
bytes. Callback functions, private keys, session caches, randomness, clocks,
and writers remain application-owned and must be safe for concurrent use.

Native hook and tracer panics are not converted into ordinary database errors.
Treat hooks as trusted process code, keep them bounded, and return errors for
expected connection rejection. This package installs no query tracer itself,
so duplicate tracer installation is controlled entirely by the application's
single `Config.Configure` composition point.

No package can make an arbitrary transaction closure safe to retry. Network
calls and emitted messages may escape PostgreSQL rollback. The module exposes
classification only and leaves execution policy to the application.

## Release and disclosure disposition

Inspected public root releases v1.0.1 and v1.1.0 parse connection configuration
through pgx and default to startup connectivity checks. They also retain native
SQLSTATE values in observations and built-in metric attributes. The pending v2
release requires explicit resolution and finite native-shape admission, defaults
to lazy startup with caller-selected ping, and projects telemetry to finite
categories. These are security and public-contract hardening changes; this
source review has not established
an advisory-required PostgreSQL vulnerability or impact in an application
deployment. That is not a claim that every earlier integration was safe.

Both inspected v1 releases select OpenTelemetry SDK v1.44.0, within the affected
dependency range of the upstream Low-severity
[GHSA-8wmf-6v46-5gfg](https://github.com/advisories/GHSA-8wmf-6v46-5gfg).
The candidate v2 graph selects SDK v1.45.0, the upstream fix. Dependency selection
does not establish exposure in a particular PostgreSQL application, and pending
source does not fix an already published v1 artifact. Review the upstream
deployment conditions and keep application diagnostic logging restricted.

The maintainer retains private triage for supported v1 reports and must reopen
this disposition if a supported integration demonstrates protected-data
exposure, unsafe implicit acquisition or exploitable unbounded work. Record
affected and first-fixed public versions from that evidence; do not infer an
affected range or severity from empty advisory lists or unpublished source.

## Retained trust boundaries and risk ownership

These are explicit application responsibilities, not scanner exemptions or
claims that arbitrary collaborators are safe. The maintainer owns enforcement
of the library's admission and redaction controls; deployment and collaborator
owners must satisfy the constraints below before adoption.

| Boundary | Owner | Rationale | Required mitigation | Review condition |
| --- | --- | --- | --- | --- |
| `ResolveDSN`, `Configure`, hooks and tracers | application integration owner | trusted callbacks may allocate, block or panic outside library control | finite acquisition and allocation budgets, cooperative context handling, reviewed TLS and tracer policy; do not treat timeout as preemption | collaborator, configured native shape or trust domain changes |
| Native errors, `ErrorInfo`, `SQLState`, server `Detail` and `Hint` | application observability owner | authorized classification retains diagnostic data that can be sensitive | allow-list safe fields and redact before logging, tracing or exporting | logging/exporter policy or native diagnostic handling changes |
| PostgreSQL hosts, credentials, TLS and roles | application security owner | explicit connection configuration cannot replace deployment access controls | approved destinations, certificate validation, least privilege and credential rotation | destination, resolver, TLS, role or secret-acquisition policy changes |
| Transaction closures and retries | application transaction owner | PostgreSQL rollback cannot undo external effects | keep effects transaction-owned or provide explicit outbox/idempotency and compensation policy | a closure gains external effects or retry semantics change |
| Borrowed connections and workload capacity | application operations owner | finite pool limits do not ensure release of borrowed resources or bound application demand | close every borrowed resource; set workload deadlines and a capacity/concurrency budget | workload, pool limits, resource lifetime or shutdown policy changes |

The maintainer must revisit this model when public configuration defaults,
admission limits, dependency behavior, redaction, or release support changes.
Private reports follow [the security policy](../SECURITY.md); critical or high
findings cannot be accepted into a passing release verdict. A medium finding
needs its own evidence-backed, time-bounded disposition; this boundary table
does not waive a confirmed vulnerability.
