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
6. Let the release workflow re-run gates, build a deterministic source archive,
   publish checksums, and create release notes from the changelog.

Never move or force-update a published tag. A pgx or PostgreSQL support change
requires explicit compatibility evidence before release.
