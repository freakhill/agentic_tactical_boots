# 0119 — GitHub App repository discovery picker

Status: in progress

SCOPE: add an explicit host-side, metadata-only GitHub App installation repository discovery command and use it as searchable suggestions in the Emacs Credentials `R` assignment flow, while preserving manual entry, full profile-scope replacement confirmation, policy re-trust, and launch-time authorization.

OFF-LIMITS: no Contents/Admin/mutation permission for discovery; no ambient `gh`, PAT, user token, GitHub Enterprise, Forgejo discovery, all-link aggregation, token/list persistence, auto-fetch/poll, agent/sandbox access, profile mutation from discovery, partial-success list, new runtime dependency, live provider calls in automated tests, or weakening account-link, one-forge, network, trust, and session credential laws.

WORKTREE: `.worktrees/0119-github-repository-discovery/`

DECISIONS: `specs/research/2026-07-18-github-repository-discovery-ayo.md` and `specs/research/2026-07-18-github-repository-discovery-flo.md`.

- [x] T1 — Land the approved security and public-surface decision
  FILE:     `specs/research/2026-07-18-github-repository-discovery-ayo.md`, `specs/research/2026-07-18-github-repository-discovery-flo.md`, `specs/0119-github-repository-discovery.md`
  CHANGE:   Pin exact metadata-only authority, one-account CLI/shared-envelope contract, detached cleanup and signal behavior, complete bounded pagination, non-authoritative snapshot semantics, searchable/manual Emacs flow, residual risk, and executable task order before production edits.
  VERIFY:   `git diff --check && rg -n 'metadata:read|CREDENTIAL_REVOKE_FAILED|snapshot, not authorization|shared envelope|WORKTREE:.*0119' specs/research/2026-07-18-github-repository-discovery-{ayo,flo}.md specs/0119-github-repository-discovery.md`
  EXPECTED: Command exits 0 and the notes resolve every contested lifecycle/contract choice without values or personal account state.

- [x] T2 — Add meaningful RED provider and CLI contract tests
  FILE:     `internal/engine/creds/githubapp/repositories.go`, `internal/engine/creds/githubapp/repositories_test.go`, `internal/cli/creds_repositories.go`, `internal/cli/creds_repositories_test.go`, `internal/cli/dependencies.go`, `internal/cli/profile.go`
  CHANGE:   Introduce only the minimum compile-safe discovery signatures/stubs, then tests for exact metadata-only/no-selector minting, fixed bounded pagination/validation/sorting, one-account lookup, shared success/error envelopes, secret/ref/raw-body exclusion, command-local cancellation, independent cleanup context, and exactly one post-mint revoke. Assert the stubs fail on missing behavior, not syntax/plumbing.
  VERIFY:   `out=$(mktemp); if go test ./internal/engine/creds/githubapp ./internal/cli -run 'Repositories|RepositoryDiscovery' -count=1 >"$out" 2>&1; then cat "$out"; rm -f "$out"; exit 1; fi; cat "$out"; rg -n 'not implemented|want .* got|unexpected' "$out"; rc=$?; rm -f "$out"; exit $rc`
  EXPECTED: Wrapper exits 0 only because focused tests compile and fail for the explicit unimplemented discovery behavior.

- [x] T3 — Implement bounded GitHub discovery and lifecycle-owning CLI
  FILE:     `internal/engine/creds/githubapp/http.go`, `internal/engine/creds/githubapp/mint.go`, `internal/engine/creds/githubapp/repositories.go`, `internal/engine/creds/githubapp/repositories_test.go`, `internal/cli/creds_repositories.go`, `internal/cli/creds_repositories_test.go`, `internal/cli/dependencies.go`, `internal/cli/profile.go`, `internal/jsoncontract/*` only if an existing code/fixture needs extension
  CHANGE:   Resolve exactly one `github.com/<owner>` link, verify owner/current Contents maximum, mint exactly metadata-read with no repo selector, enumerate fixed bounded pages and validate only `full_name`, buffer candidates, and revoke once before output using a fresh ten-second cleanup context. Add command-local signal cancellation. Emit existing v1 shared envelopes; withhold candidates on every error, especially cleanup uncertainty. Reuse seams and add no dependency.
  VERIFY:   `go test ./internal/engine/creds/githubapp ./internal/cli ./internal/jsoncontract -run 'Repositories|RepositoryDiscovery|Creds.*Repositories|ErrorCode' -count=1 -v`
  EXPECTED: Command exits 0; tests prove exact authority, complete/sorted selectors, fixed value-free outcomes, cancellation-safe cleanup, and no live provider calls.

- [ ] T4 — Add meaningful RED Emacs discovery-journey tests
  FILE:     `emacs/test/safeslop-contract-test.el`, `emacs/test/safeslop-credentials-test.el`, `emacs/test/safeslop-ui-probe.el`
  CHANGE:   Add strict shared-envelope parser tests and trace the real `R` branch through visible account selection, explicit Fetch/manual choice, exact discovery argv, searchable read/write completion, capability warning, current/off-snapshot prefill, cancellation/failure/stale callback draft preservation, and unchanged final replacement confirmation. Tests fake async CLI only.
  VERIFY:   `out=$(mktemp); if emacs -Q --batch -L emacs -l ert -l emacs/test/safeslop-test.el -l emacs/test/safeslop-contract-test.el -l emacs/test/safeslop-credentials-test.el --eval '(ert-run-tests-batch-and-exit "safeslop-test-credentials-.*\(discover\|installation-repositor\)")' >"$out" 2>&1; then cat "$out"; rm -f "$out"; exit 1; fi; cat "$out"; rg -n 'FAILED|void-function|should.*:form' "$out"; rc=$?; rm -f "$out"; exit $rc`
  EXPECTED: Wrapper exits 0 only because the new journey assertions fail on the still-manual picker, not test syntax or fixture plumbing.

- [ ] T5 — Implement searchable installation suggestions in the existing `R` flow
  FILE:     `emacs/safeslop-contract.el`, `emacs/safeslop-credentials.el`, `emacs/test/safeslop-contract-test.el`, `emacs/test/safeslop-credentials-test.el`, `emacs/test/safeslop-ui-probe.el`
  CHANGE:   Strictly parse the minimum success data; only in GitHub explicit mode let the operator visibly choose a linked GitHub account and Fetch (default) or manual entry; fetch asynchronously; use repository names as searchable, manual-accepting suggestions with current read/write scopes prefilled; show the bounded Contents hint and explicitly warn on currently impossible write; reject late/dead/reused callbacks; preserve draft/current profile on all cancellation/error paths; retain the existing before/after save confirmation and re-trust message.
  VERIFY:   `emacs -Q --batch -L emacs -l ert -l emacs/test/safeslop-test.el -l emacs/test/safeslop-contract-test.el -l emacs/test/safeslop-credentials-test.el --eval '(ert-run-tests-batch-and-exit "safeslop-test-credentials-.*\(discover\|installation-repositor\|repo-picker\)")' && make test-emacs-ui-matrix`
  EXPECTED: All focused ERT and raw/Doom/Evil/Doom-Evil slots pass; no fetch occurs outside explicit `R`, and no discovery result changes authority before normal confirmation.

- [ ] T6 — Synchronize operator documentation and credential skills
  FILE:     `README.md`, `emacs/README.md`, `skills/agent-key-lifecycle/SKILL.md`, `skills/agent-sandbox-ops/SKILL.md`, `specs/0090-credential-connection-repo-picker.md`, `specs/0119-github-repository-discovery.md`
  CHANGE:   Replace the deliberate live-GitHub deferral with the exact explicit account-scoped command and Emacs workflow; document metadata-only mint/revoke/one-hour residual, complete snapshot limits, manual fallback, non-authoritative semantics, no Forgejo discovery, unchanged save/re-trust/launch gates, and current command examples/help.
  VERIFY:   `git diff --check && rg -n 'creds repositories|metadata(:|-)read|linked App|one hour|manual|re-trust|launch.*authoritative|Forgejo.*deferred|0119' README.md emacs/README.md skills/agent-{key-lifecycle,sandbox-ops}/SKILL.md specs/0090-credential-connection-repo-picker.md specs/0119-github-repository-discovery.md`
  EXPECTED: Command exits 0; docs/skills describe real commands and distinguish discovery, policy scope, account links, runtime credentials, and authorization.

- [ ] T7 — Run final review, full gates, install, and a value-free live smoke
  FILE:     whole repository; installed `~/.local/bin/safeslop` and `~/.local/share/safeslop/emacs/*.el`
  CHANGE:   Obtain blocking-only review, run focused and full checks on the accepted tree, build/install, verify installed files/version, then invoke discovery once against the already-linked healthy GitHub App while redirecting JSON to a private temp file and report only envelope status/repository count (never names/values). Mark complete only after all gates and cleanup succeed.
  VERIFY:   `git diff --check && make test-emacs-ui-matrix && make check && make build`
  EXPECTED: Command exits 0; Go/ERT/byte-compile/UI-matrix/build gates pass with no live calls in tests. The separate post-install smoke returns an ok v1 envelope and only a count is printed before the temp file is removed.
