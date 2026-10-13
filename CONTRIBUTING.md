# Contributing

## Before Editing

1. Read [`AGENTS.md`](AGENTS.md) and the affected package's docs.
2. Run the affected package's deterministic tests as described below.
3. Identify owned dependencies and reverse dependants in `modules.json`.
4. Preserve unrelated work and generated/corpus provenance.

## Changes

Keep commits focused and conventional. Update every affected changelog with
the behavior and migration impact. Public API changes require compatibility
evidence and documentation. Specification behavior requires a decision record,
fixture coverage, and interoperability evidence.

New direct dependencies and dependency updates must follow the
[dependency governance policy](AGENTS.md#dependencies-and-supply-chain). Package-local
update bots are forbidden; the root policy owns every module and action update.

Specification-backed changes must follow the
[specification governance contract](AGENTS.md#design), update
the affected stable decision entries, and complete the Specification Decisions
section of the pull request template. An unresolved interpretation or stale
source pin is release-blocking; peer behavior cannot silently select policy.

Selected full/release mutation gates must finish with zero surviving viable
mutants; mutation is not an ordinary local or pull-request prerequisite.

Do not add package-local workflows, permanent replacements, machine-specific
paths, bypass flags, broad mutation exclusions, or aggregate quality metrics
that hide a failing package.

## Verification

Classify changes under [proportional assurance](AGENTS.md#scope-and-verification).
The root module collects complete native coverage in explicit evidence mode.
Test failures, missing packages and invalid or absent profiles remain errors;
universal exact-percentage acceptance is replaced by behavioral risk review.
The final cancellation guards remain intact even when their precise race
windows lack deterministic public synchronization. Full/release selection
retains mutation and applicable aggregate checks; ordinary checks retain the
complete security scanner contract.

Use Go 1.27.2 for verification; the module/API minimum remains Go 1.27.0.
An isolated checkout needs no sibling repositories or coordination goal files.
Begin with deterministic tests and one active target when its hostile
configuration boundary changes, using a disposable cache for each session:

```bash
(
  set -eu
  GOCACHE="$(mktemp -d)"
  export GOCACHE
  trap 'rm -rf "${GOCACHE}"' EXIT
  GOWORK=off go test ./...
  GOWORK=off go test . -run '^$' -fuzz '^FuzzParseConfig$' -fuzztime=2s -timeout=2m
)
```

The deterministic command executes fuzz seeds/corpus, not active fuzzing.
It omits the `integration` tag and does not qualify PostgreSQL behavior.
Install the verified public v2.4.0 binary archive for your platform from
[go-library-tools releases](https://github.com/faustbrian/go-library-tools/releases/tag/v2.4.0),
checking `checksums.txt` against `.golib.yaml` and the archive against that
checksum set. Put the extracted `golib` binary on `PATH`, or select its path
through `GOLIB`; do not build Tools from source. CI verifies this same public
release while checking out its pinned reusable workflow source.

```bash
make inventory
make check
make ci
make specification-check
golib mutation --module . --package .
```

Run adjacent Make commands in one disposable-cache session as shown above.
`make check` and `make ci` run the bounded contract with root PostgreSQL
fixtures and the `integration` tag; `examples/migrations` also declares
PostgreSQL but no test tag. They are not Docker- or database-free. The
intentional example module keeps its existing public dependency baseline.
CI also validates repository/cohesion structure. Offline specification checking
has an explicit Make target. The root race runs once with a two-minute timeout
and proves one underlying cleanup across independent caller cancellation,
then rejects acquire/ping/readiness before any backend work after shutdown.
These stub-pool assertions are not live SQL/isolation/locking proof.
Shared local safety checks retain the owned prohibition of unsafe, cgo and
go:linkname. Base-free checks run seeds/corpus; changed-source CI selects causal
active fuzz smoke. Optional mutation is limited to one package, one worker
and a twenty-minute deadline.

Select full qualification and release rehearsal explicitly:

```bash
make check-full
make -f verification/package.mk docs
make specification-check-online
golib api check
golib release dry-run --module .
```

Full checks retain all five declared fuzz targets at 10000x/20m each, the
100ms benchmark operation, native coverage evidence policy, documentation and
example-module checks and all original module gates. The scheduled carrier
uses the shared bounded route, not an automatic full or PostgreSQL-major-matrix
campaign. Select real integration versions explicitly when those contracts
change. Online specification checking fetches authority/errata sources when
applicable. Release rehearsal can run locally. Report unavailable or failing
commands; bounded checks alone do not establish release qualification.

## Adding A Module

Follow [repository structure policy](AGENTS.md#repository-structure). New modules
require an explicit purpose, ownership boundary, dependency review, package
catalog entry, full quality gates, documentation, changelog, license, security
policy, compatibility plan, and release dry-run.

## Source Documentation

Install the checksum-pinned public verifier declared in `.golib.yaml`.
`make source-docs-check` validates objective Go comment, marker, link and
generated-producer structure; `make check`, `make check-full` and `make ci`
require it before their existing checks. Shared CI runs the same strict
contract and includes its result in Required. This is structural enforcement,
not a substitute for source-truth review or behavioral verification.
