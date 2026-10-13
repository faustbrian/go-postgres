# Repository standards

[Documentation index](README.md) · [Repository entry](../README.md)

- `make inventory`, `make cohesion`, `make repository-check`, `make check`, and
  `make ci` are the maintained bounded entry points. `make check-full` and
  release rehearsal select the full contract explicitly.
- Production Go source satisfies GO-SAFETY-1: no `unsafe`, cgo, or
  `go:linkname`.
- Full native coverage evidence instruments module packages and includes real
  PostgreSQL; percentage acceptance follows the existing behavioral-risk policy.
- Behavioral changes use red-green-refactor; database semantic claims use
  integration tests.
- Fuzz regressions remain checked into `testdata/fuzz`.
- Documentation, changelog, compatibility, and security contracts change with
  public behavior.
- Commits are Conventional Commits with an explanatory body.

Choose the next guide in the [documentation index](README.md)
or return to the [repository entry](../README.md) for installation and ownership.
