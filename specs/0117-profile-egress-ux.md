# 0117 — Buildable profile composition and prominent progressive-egress review

Status: implemented and accepted

SCOPE: make profile creation offer only buildable catalog selections with an engine-owned availability explanation; ensure conventional HTTP clients send deny-tier traffic through the inspected proxy; make denied egress prominently and continuously visible in a live Emacs terminal, with an explicit keyboard route to the existing review/Allow-now/Keep-denied flow.

OFF-LIMITS: do not add `network:"ask"` or `network:"progressive"`; do not auto-open a review, display a modal, or change authority because of agent traffic; do not weaken deny topology, raw-DNS denial, hard non-grantable destinations, policy-byte trust, session-scoped grant lifetime, proxy ACK requirements, or value-free output. Do not make unavailable catalog entries silently disappear or make host/`network:"allow"` sessions appear enforceably reviewable. Do not complete or source unreviewed package build recipes in this change.

WORKTREE: `.worktrees/0117-profile-egress-ux/`

## Design

### Problem

The catalog composer renders selections whose resolved image cannot be built, then `profile create` fails only at recipe resolution. Separately, deny-tier Compose config provides only uppercase proxy environment variables. curl deliberately ignores uppercase `HTTP_PROXY` for HTTP URLs, so `curl google.com` attempts raw DNS, which deny mode correctly blocks but cannot represent as a proxy observation. The existing progressive review is available only in Session Detail, leaving a live terminal without an obvious denied-egress indicator or shortcut.

### Pinned behavior

1. `catalog list` returns additive availability metadata for every package and bundle. Availability is engine-owned and has only `ready` or `unavailable`, plus a bounded catalog-safe explanation. A bundle is ready only if its complete resolved closure has a reviewed image recipe. Existing package/bundle fields and defaults remain unchanged.
2. Composer rows remain visible. Unavailable rows are prominently marked with the engine explanation and cannot be newly selected. An unavailable row already selected by a cloned/legacy draft may be deselected, but cannot be retained through preview/save. A ready default bundle is still selected and locked; the engine test proves defaults are ready.
3. Deny-tier agent containers receive both lowercase and uppercase `http_proxy`/`https_proxy` plus matching `no_proxy` values. This changes no topology: direct external DNS remains loopback-pinned, and only proxy-observed HTTP(S) requests can become observations.
4. A running container-deny terminal periodically performs the existing read-only observation query. Its header and mode line show a literal pending-denial count and the same review shortcut. It never displays destinations in terminal chrome, never focuses/pops a buffer, never prompts, and never changes authority.
5. `C-c C-v` is bound in both supported terminal backends to open the existing session review for that terminal. The review retains explicit `a` Allow now and `k` Keep denied actions. A user must retry the original request after an allowed grant.

### Success criteria

- A new composer cannot submit an unavailable bundle/package, and it names the reason before an operator spends a preview/save attempt.
- `curl http://…` uses the proxy in deny-tier containers rather than attempting raw DNS; a denied proxied request is observable under the existing exact FQDN:port rules.
- A live Emacs terminal visibly reports pending denied destinations without an agent-triggered prompt and provides a discoverable, working review shortcut.
- Existing public catalog fields, the egress grant protocol, and all fail-closed safety behavior remain compatible.

## Tasks

- [x] T1 — Add engine-owned catalog selection availability
  FILE:     `internal/engine/container/identity.go`, `internal/engine/container/identity_test.go`, `internal/cli/cli_catalog.go`, `internal/cli/cli_catalog_test.go`
  CHANGE:   Factor the current recipe readiness checks into an exported, deterministic availability helper that classifies a resolved catalog selection as `ready` or `unavailable` with a bounded catalog-safe reason; it must not expose asset paths, command output, or host data. Add additive `availability` objects to `catalog list` package and bundle rows. Resolve each bundle through its full package/require closure, preserve all existing fields/defaults/order, and require every catalog default bundle to be ready.
  VERIFY:   `go test ./internal/engine/container ./internal/cli -run 'Recipe|Buildability|CatalogList' -count=1 -v`
  EXPECTED: Built selections (`personal`, `base-tools`, defaults) are ready; sentinel/unwired selections are unavailable with a stable safe reason; legacy JSON fields remain present.

- [x] T2 — Make the profile composer availability-aware
  FILE:     `emacs/safeslop-profiles.el`, `emacs/safeslop-profile-compose.el`, `emacs/test/safeslop-profiles-test.el`
  CHANGE:   Preserve catalog availability in the composer indexes and row state. Render every unavailable bundle/package with a prominent unavailable marker plus reason, show it in help, refuse a new selection locally, and allow an existing unavailable selection only to be removed. Reject preview before dispatch if an unavailable selection remains. Keep ordinary and default/inherited selection semantics, command argv, row/context preservation, and catalog-refresh behavior unchanged for ready selections.
  VERIFY:   `make test-emacs EMACS=$(command -v emacs)`
  EXPECTED: ERT proves unavailable rows remain legible but unselectable, selected legacy rows can be removed, preview sends no CLI command for an unavailable selection, and ready/default rows retain their existing behavior.

- [x] T3 — Route conventional HTTP clients through deny-tier proxy observation
  FILE:     `internal/engine/container/assets/compose.yml.tmpl`, `internal/engine/container/compose_test.go`, `internal/engine/container/container_images_live_test.go` (only if an existing opt-in Docker fixture can extend it), `README.md`, `skills/agent-sandbox-ops/SKILL.md`
  CHANGE:   Set lowercase `http_proxy`, `https_proxy`, and `no_proxy` alongside the existing uppercase variables to the same typed proxy values. Retain loopback DNS and the internal-only agent network. Add hermetic Compose-render assertions; extend an existing opt-in container gate only when it can demonstrate that HTTP curl sends an explicit deny through the proxy without leaking a live request.
  VERIFY:   `go test ./internal/engine/container -run 'Compose.*Proxy|DenyTier|DNS' -count=1 -v && make check-assets`
  EXPECTED: Rendered deny Compose has matching upper/lower proxy variables, raw DNS remains pinned, and no open egress path is added.

- [x] T4 — Add live terminal egress visibility and review shortcut
  FILE:     `emacs/safeslop-session-terminal.el`, `emacs/safeslop-egress.el` only if a narrow terminal-facing helper is required, `emacs/test/safeslop-test.el`
  CHANGE:   Add a buffer-local, bounded read-only observation monitor for live container-deny terminals. Refresh terminal header/mode-line count asynchronously without stealing focus; prevent concurrent requests; cancel its timer when the process/buffer exits; ignore late callbacks for dead/reused buffers. Include a literal discoverable `C-c C-v` review hint whenever the session is eligible. Bind that key in both term and eat backends to invoke the existing explicit session egress review with the terminal session id/snapshot. Never invoke review, grant, dismiss, persistent preview, or a prompt from monitor callbacks.
  VERIFY:   `make test-emacs EMACS=$(command -v emacs)`
  EXPECTED: ERT proves eligible terminals render/update the count and shortcut, non-eligible terminals do not poll, callbacks never pop or mutate authority, `C-c C-v` opens review only on explicit keypress, and timers are cleaned up.

- [x] T5 — Synchronize operator contract and verify
  FILE:     `README.md`, `skills/agent-sandbox-ops/SKILL.md`, `specs/0117-profile-egress-ux.md`
  CHANGE:   Document unavailable composer selections, HTTP-client proxy behavior, terminal egress indicator/review shortcut, and the explicit non-modal grant rule. Tick every task only after its verification succeeds and set the spec status to complete only after final verification.
  VERIFY:   `git diff --check && make check && make build`
  EXPECTED: Formatting, catalog/asset checks, TLA conformance, Go/Emacs tests, and build all pass; documentation matches the shipped controls and does not promise agent-triggered authorization.
