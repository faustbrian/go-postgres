# Security policy

## Supported versions

| Release line | Status | End of support |
| --- | --- | --- |
| Latest published `v2` release | Supported once published | Not scheduled |
| Latest `v1` release | Supported | Not scheduled |

Preparing v2 on main does not retire v1 reporting support or make unreleased
source a supported release. Supported lines receive private triage and upgrade
guidance; a fix may require migration to a newer major. An advisory identifies
affected modules and versions, fixed releases, and any change to support.
Fixes use Git tags from main, including a new major for incompatible changes;
support does not promise a separate version-specific source branch or backport.

## Reporting a vulnerability

Use [GitHub private vulnerability reporting](https://github.com/faustbrian/go-postgres/security/advisories/new)
for this repository. Include a minimal reproducer, affected version, impact,
and mitigation. Do not include production DSNs, credentials, query arguments,
customer data, or certificates.

The repository maintainer owns private triage, remediation, release decisions,
and coordinated disclosure. Follow the shared
[vulnerability-management procedures](https://github.com/faustbrian/go-library-tools/blob/5f9ee29176fe07942c5ec5d504459d0ad332cec8/docs/ecosystem/security/vulnerability-management.md)
for severity, acknowledgement and remediation targets, embargo handling,
advisories, and affected-consumer reassessment. Keep reporter identity and
private evidence out of public commits, logs, and release artifacts.

## Security boundary

DSNs, certificates, hooks, SQL, arguments, PostgreSQL errors, and telemetry
exporters cross trust boundaries. The package prevents its own validation and
startup errors from echoing DSNs, never emits SQL or arguments through its
observation API, and treats server `Detail` and `Hint` fields as sensitive.

Applications remain responsible for secret storage, certificate issuance,
network policy, PostgreSQL roles, row-level security, statement policy,
migrations, backups, authentication, authorization, and workload deadlines.

See [docs/security.md](docs/security.md) and
[docs/pool-and-lifecycle.md](docs/pool-and-lifecycle.md) for the threat model
and operational boundaries.
