# Repository standards

- `make inventory`, `make cohesion`, `make repository-check`, `make check`, and
  `make ci` are the maintained local and CI entry points.
- Production Go source satisfies GO-SAFETY-1: no `unsafe`, cgo, or
  `go:linkname`.
- Exact coverage instruments every module package and includes real PostgreSQL.
- Behavioral changes use red-green-refactor; database semantic claims use
  integration tests.
- Fuzz regressions remain checked into `testdata/fuzz`.
- Documentation, changelog, compatibility, and security contracts change with
  public behavior.
- Commits are Conventional Commits with an explanatory body.
