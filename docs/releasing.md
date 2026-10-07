# Release process

The root's next major is `github.com/faustbrian/go-postgres/v2`, tagged `v2.x.y`
from main without a version-specific source directory. The v1 API archive is
historical; the v2 archive characterizes the new nominal boundary. Publish the
root before updating independently released consumers to actual public v2.
`examples/migrations` stays pinned to released root v1.1.0 until that adoption.

1. Confirm the intended SemVer and update `CHANGELOG.md` with a dated version.
2. Review public API, pgx release notes, supported Go/PostgreSQL matrix,
   security findings, and migration guidance.
3. Run `make check`, `go mod tidy -diff`, `git diff --check`, and actionlint.
4. Push the release commit and verify every CI, integration, Security, fuzz,
   benchmark, and compatibility job on that exact SHA with no required skip.
5. Create a signed or protected `vMAJOR.MINOR.PATCH` tag on a commit reachable
   from `main`.
6. Deliberately publish the source archive, checksums and changelog-derived
   release notes through the coordinator's authorized release operation. This
   repository has CI only, not a tag-triggered publication workflow.

The CI `release_dry_run` selector runs structural release validation and the
ordinary check contract. It is not an executed CLI release rehearsal or proof
of public proxy consumption. Those release checks and publication remain
separate prerequisites.

Never move or force-update a published tag. A pgx or PostgreSQL support change
requires explicit compatibility evidence before release.
