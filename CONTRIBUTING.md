# Contributing

Thanks for contributing.

## First Read

Before editing code or docs, read:

1. `AGENTS.md`
2. `README.md`
3. Relevant `specs/` files for the area you are changing
4. Relevant skill files in `skills/`

## Go Engine

`safeslop` is a single signed Go binary. Engine and CLI code live in
`cmd/safeslop` and `internal/engine/*`; the policy schema is embedded through
Go. There is no external policy compiler required at runtime.

- Format with `gofmt`.
- Keep `go vet ./...` clean.
- Put tests next to the code and keep them hermetic.
- Do not call live forges, credential providers, registries, or cloud APIs from
  unit tests. Use fakes and local HTTP test servers.
- Preserve safe defaults: `network: "deny"` unless a policy opts into more
  authority. `environment` is required (host/container) — there is no default
  tier (specs/0053 and later container-only cleanup removed historical tiers).

## Docs, Skills, and Tests Sync Policy

When command behavior, policy schema, defaults, or safety guarantees change,
update all relevant docs and tests in the same change:

- `README.md`
- Related skill files under `skills/`
- Go tests for changed behavior or error paths
- Specs/checklists when executing a written plan

## Verification

Run at least (Java 17+ is a development/CI prerequisite for the bounded TLA+
gate; GitHub CI uses Temurin 21):

```bash
make check
make build
```

`make check` includes asset/catalog drift checks, npm package-lock/SRI and
proxy-image lock checks, active-surface drift, host-helper and hostpath denylist
gates, the finite session model/mutants and bidirectional TLA+/Go graph check,
`go vet`, `gofmt` verification, `go test ./...`, and strict Emacs tests.
Container-image work must also run `make test-container-images`; progressive
egress work must run the opt-in Docker gate `make test-progressive-egress-smoke`.
For targeted work, also run the narrower package tests that prove the changed
behavior.

## Bounded Session Protocol Model

The TLA+ checker is development-only and never enters the Go binary dependency
closure. Bootstrap its SHA-256-pinned official Tools jar once, then verify an
offline run:

```bash
make bootstrap-tla-session
make check-tla-session
TLA_OFFLINE=1 make check-tla-session
```

The safety laws and frozen behavior characterization are the review authority.
`formal/session/SessionBoundary.tla` and the pure Go reducer are independent peer
implementations—neither is generated from the other—and their normalized
initial/state/labelled-edge graphs must match both ways over the reviewed finite
bounds. If the positive model, a mutant, or graph conformance reports a
counterexample, classify it before editing: checker/parser issue, model defect,
reducer/refactor defect, or pre-existing behavior defect. Translate the shortest
witness through the documented concrete field map and add a manual RED Go
regression. A behavior defect is a separate approved RED→GREEN change; never
silently rewrite characterization to make model and code agree. Full assumptions,
epistemic quotient limits, tokenless/external-runtime exclusions, pin-update
procedure, and retained artifact paths are in `formal/session/README.md`.

## Network and File-Sharing Guardrails

- Keep deny-by-default egress and explicit allowlists.
- Do not broaden allowlist domains without rationale.
- Keep the workspace boundary policy-relative, canonical, existing, and separate
  from the private runtime stage; exactly one read-write host bind is allowed.
- Accept hostile-but-valid path text through typed Compose/YAML quoting, but reject
  controls/format characters, non-directories, missing paths, and workspace-stage
  overlap.
- Never expose host credential directories to containers.
- Keep staged credentials short-lived, scoped, value-free in public output, and
  wiped on exit or session cleanup.
