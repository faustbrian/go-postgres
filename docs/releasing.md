# Release process

[Documentation index](README.md) · [Repository entry](../README.md)

The root's current major is `github.com/faustbrian/go-postgres/v3`, tagged `v3.x.y`
from main without a version-specific source directory. Earlier API baselines
remain in Git history. Publish the root before updating independently released
consumers to the public v3 module. `examples/migrations` is a separate module
that still consumes published v1.1.0 and go-migrations v2; it moves to v3 and
go-migrations v4 together once both are published.

1. Confirm the intended SemVer and update `CHANGELOG.md` with a dated version.
2. Review public API, pgx release notes, supported Go/PostgreSQL matrix,
   security findings, and migration guidance.
3. Run `make check-full`, `go mod tidy -diff`, `git diff --check`, and actionlint.
4. Push the release commit and verify every CI, integration, Security, fuzz,
   benchmark, and compatibility job on that exact SHA with no required skip.
5. Create a signed or protected `vMAJOR.MINOR.PATCH` tag on a commit reachable
   from `main`.
6. Deliberately publish the source archive, checksums and changelog-derived
   release notes through the coordinator's authorized release operation. This
   repository has CI only, not a tag-triggered publication workflow.

The CI `release_dry_run` selector performs structural release validation and
invokes `golib release dry-run` through the verified public release pinned in
`.golib.yaml`; it does not build the verifier from source. That
executed rehearsal exercises the selected release contract without publishing
tags or assets. Neither structural validation nor rehearsal proves actual
public proxy consumption; deliberate publication and public verification remain
separate boundaries.

Ordinary and scheduled CI use the shared bounded route, not the full campaign
or a PostgreSQL-major matrix. Select supported PostgreSQL versions and full
qualification explicitly at the applicable release boundary. Local
`golib release dry-run --module .` is also available; see
[contributor verification](../CONTRIBUTING.md#verification) for public-tool
installation and disposable-cache sessions.

Never move or force-update a published tag. A pgx or PostgreSQL support change
requires explicit compatibility evidence before release.

Choose the next guide in the [documentation index](README.md)
or return to the [repository entry](../README.md) for installation and ownership.
