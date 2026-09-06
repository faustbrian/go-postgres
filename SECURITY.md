# Security policy

## Supported versions

| Release line | Status | End of support |
| --- | --- | --- |
| Latest `v1` release | Supported | Not scheduled |

Security fixes are applied to the latest published `v1` release. An advisory
will identify affected versions and any change to the supported release lines.

## Reporting a vulnerability

Use GitHub private vulnerability reporting for this repository. Include a
minimal reproducer, affected version, impact, and mitigation. Do not include
production DSNs, credentials, query arguments, customer data, or certificates.

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
