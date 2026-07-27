# 0121 — Dependency maintenance repair

Status: approved for implementation

SCOPE: replace Forgejo Renovate PRs #103, #105, #106, and #109 with one reviewable maintenance transaction from current `origin/main`; repair pre-existing Go and npm artifact drift; incorporate the proposed YAML, x/sys, Pi, and checkout updates only when repository policy and real verification establish they are safe; then supersede the incomplete PRs.

OFF-LIMITS: no runtime/CLI/CUE contract or safety-default change; no dependency beyond the four proposals and the already-selected but unsummed `cuelang.org/go` and `golang.org/x/term` versions on main; no generated checksum or lockfile hand-edit; no Pi minor bump that violates the locked version-selection/soak/security policy; no merge with a red `make check` or `make build`; no live credential/provider calls in tests.

WORKTREE: `/Users/jojo/workspace/worktrees/safeslop/0121-dependency-maintenance-repair/`

Baseline evidence (2026-07-27, `origin/main@ff4cc46`): `make check` stops in `check-npm-locks` because the selected `cuelang.org/go@v0.17.1` and `golang.org/x/term@v0.45.0` lack current `go.sum` entries. Pi is split across catalog/locks `0.80.7` and both package manifests `0.82.0`. PRs #103, #105, and #109 each report failed `renovate/artifacts`; #106 reports no status.

- [x] T1 — Reproduce and classify the broken baseline
  FILE:     `go.mod`, `go.sum`, `internal/engine/policy/catalog.{cue,json}`, `library/layer/container/npm-locks/pi/{package.json,package-lock.json}`, mirrored embedded npm locks, `.github/workflows/*.yml`
  CHANGE:   Run the authoritative gates before mutation and record the exact missing Go sums plus Pi manifest/catalog/lock drift. Confirm each open PR head and status so the replacement has an auditable baseline.
  VERIFY:   `out=$(mktemp); if make check >"$out" 2>&1; then cat "$out"; rm -f "$out"; exit 1; fi; cat "$out"; rg -n 'missing go.sum entry' "$out"; rc=$?; rm -f "$out"; exit $rc`
  EXPECTED: The wrapper exits 0 only because current main deterministically fails on stale generated dependency artifacts, not because of environment or test plumbing.

- [x] T2 — Repair the Go module transaction and apply #109/#103
  FILE:     `go.mod`, `go.sum`
  CHANGE:   Use the repository Go toolchain to select exactly `go.yaml.in/yaml/v3@v3.0.5` and `golang.org/x/sys@v0.47.0`, then run `go mod tidy` to generate authoritative sums for those proposals and the already-selected `cuelang.org/go@v0.17.1` and `golang.org/x/term@v0.45.0`. Review the graph and reject unrelated version movement; retain only indirect movement proven necessary by those already-selected direct modules, and remove direct/indirect entries only when `go mod why -m` proves they are unused.
  VERIFY:   `before=$(shasum go.mod go.sum); go mod tidy; test "$before" = "$(shasum go.mod go.sum)" && go mod verify && go test ./... -run '^$' && go test ./internal/engine/policy ./internal/engine/exec && go test ./internal/engine/container -run 'TestCompose(MountPlanHasExactlyOneReadWriteWorkspace|RejectsWorkspaceStructureInjection|PreservesHostileValidWorkspaceAsOneScalar)' -count=1`
  EXPECTED: A second tidy is byte-stable, the exact four selected direct versions have valid sums, only their required indirect closure moves, unused stale entries disappear, every package compiles, and the directly affected policy/exec/YAML paths pass. The full suite remains in T5 after T3 repairs the independently broken npm assets.

- [ ] T3 — Resolve Pi #105 under catalog and npm integrity policy
  FILE:     `internal/engine/policy/catalog.{cue,json}`, `library/layer/container/npm-locks/pi/{package.json,package-lock.json}`, `internal/engine/container/assets/npm-locks/pi/{package.json,package-lock.json}`, `specs/0121-dependency-maintenance-repair.md`
  CHANGE:   First restore the independently drifted Claude and pnpm manifests to their existing catalog/lock pins; these are reversions of incomplete already-merged Renovate edits, not new upgrades. Verify Pi upstream publication, changelog, package metadata, and any claimed security advisory. If `0.82.1` satisfies the locked selection plus soak/security rules, use catalog tooling, npm's lockfile-only install with scripts disabled, and `make sync-container-assets` to update catalog→manifest→transitive SRI atomically. Otherwise restore both drifted Pi manifests to the approved `0.80.7` pin and record #105 as deferred. Never hand-edit package-lock integrity.
  VERIFY:   `make check-catalog-sync check-assets check-npm-locks && go test ./internal/engine/policy ./internal/engine/container -run 'Catalog|NPM|Tool|Recipe|BuildContext' -count=1 -v`
  EXPECTED: Authored/rendered catalogs and both lock copies agree on one policy-approved Pi version; npm ci/SRI and the closed binary/script policy pass hermetically; the spec records whether #105 landed or was deferred and why.

- [ ] T4 — Apply and statically validate checkout #106
  FILE:     `.github/workflows/go.yml`, `.github/workflows/container-images.yml`
  CHANGE:   Update only `actions/checkout@v4` to official `@v7`; verify the upstream action uses Node 24, GitHub-hosted runner compatibility, workflow syntax, default checkout inputs, and no widened token permissions. Do not change other actions or workflow behavior.
  VERIFY:   `git diff --check && ruby -e 'require "yaml"; ARGV.each { |f| YAML.parse_file(f) }' .github/workflows/go.yml .github/workflows/container-images.yml && test "$(rg -l 'actions/checkout@v7' .github/workflows/*.yml | wc -l | tr -d ' ')" = 2 && ! rg -n 'actions/checkout@v4' .github/workflows`
  EXPECTED: Both active workflows parse, use only checkout v7, retain their existing jobs/permissions/commands, and the diff contains no unrelated workflow edits.

- [ ] T5 — Obtain independent review and run authoritative gates
  FILE:     whole repository
  CHANGE:   Have independent reviewers inspect the complete `origin/main..HEAD` diff for dependency scope, generated-artifact integrity, catalog-policy compliance, action-runner compatibility, and accidental behavior changes. Resolve every blocker/high finding, then rerun the exact repository gates from a clean tree.
  VERIFY:   `git diff --check origin/main...HEAD && make check && make build && git status --short --branch`
  EXPECTED: Independent review has no unresolved blocker/high; formatting, catalog/assets, npm SRI, models, Go/Emacs tests, and static build all exit 0; only the expected branch commits are present.

- [ ] T6 — Publish, merge, and supersede incomplete Renovate PRs
  FILE:     Forgejo branch/PR metadata; no additional source files
  CHANGE:   Push the verified branch, open a Forgejo PR that links #103/#105/#106/#109 and records any deliberately deferred Pi proposal, re-check head/base/status immediately before merge, merge only the reviewed head, then close remaining superseded Renovate PRs. Fast-forward the reference checkout and verify the remote result without installing artifacts.
  VERIFY:   `git fetch origin main && test "$(git rev-parse origin/main)" = "$(git rev-parse HEAD)" && git status --short --branch`
  EXPECTED: `origin/main` is exactly the verified replacement tip, the replacement PR is merged, incomplete Renovate PRs are closed as superseded, and both worktree and reference checkout remain clean.
