# 0120 — Promote ad-hoc session to profile

Status: approved for implementation

SCOPE: add a reviewed, snapshot-bound workflow that promotes an ad-hoc session into a **new**, ordinary project profile and maps only explicitly selected session grants to exact durable `persistentEgress` rules for future sessions. Provide a value-free CLI preview/apply plan, a dedicated bounded promotion protocol model plus independently authored Go reducer and graph conformance, and an explicit Emacs review flow.

OFF-LIMITS: do not mutate a source session, its lifecycle, grants, acknowledgements, proxy/runtime authority, credentials, or trust state; do not overwrite an existing project profile or mutate a builtin; do not infer/select all grants, copy observations or traffic, include secrets/credential refs/request data/staged paths/runtime internals in output, copy process/container state, auto-trust, auto-launch, add a daemon/runtime dependency, or extend `formal/session/SessionBoundary.tla`.

WORKTREE: `.worktrees/0120-ad-hoc-session-promotion/`

DECISIONS: `specs/research/2026-07-18-ad-hoc-session-promotion-{ayo,flo}.md`.

## Pinned contract

```text
safeslop profile promote preview NAME [safeslop.cue] \
  --session-id ID [--grant-id G]... --plan PLAN --output json
safeslop profile promote apply --plan PLAN --output json
```

`preview` consumes an ad-hoc-session snapshot only. It creates a candidate for a new project profile, never replaces a project/builtin profile, and defaults the repeatable `--grant-id` selection to empty. Each selected current grant ID contributes exactly its normalized FQDN:80/443 destination to `persistentEgress`; no other session data becomes durable authority. The candidate retains normalized agent/environment/network and the recorded canonical absolute workspace. Complete resolved identity metadata emits the exact identity packages with `BareAgent=true`; absent metadata retains the synthetic ad-hoc default behavior; partial/contradictory metadata fails closed.

Preview writes a versioned, bounded mode-`0600` plan after complete rendering and schema validation. The plan binds source session identity plus promotion-relevant ETag/snapshot, public grant revision plus complete grant set, target name, target state (`absent` or exact byte hash), selected grant IDs, candidate policy hash, and renderer/schema versions. It records no secret, CUE body, ref, request, private staged path, or runtime detail. Its JSON review output is value-free and includes source lifecycle/binding, target state, fidelity, logical new-profile delta, selected destinations, candidate hash, session-unchanged statement, and untrusted-result statement.

Apply rereads authoritative source and target state inside a target-policy transaction. It permits source status `created` or `stopped`; preview may read `running`. A `running -> stopped` transition alone is promotion-compatible when all promotion-relevant source evidence still matches. Any other source/policy drift, invalid candidate, existing/builtin name, malformed/stale plan, or known pre-commit failure fails closed. Complete candidate validation precedes atomic replacement. Commit uncertainty is reported as uncertainty, never success. Known success leaves the CUE untrusted and affects only future sessions.

The promotion model has its own `formal/promotion/` relation and pure Go reducer/kernel. Its only data effects are source read, policy read, candidate validation, exact replacement, and report. It covers one source, one target, two grant IDs/all subsets, two source versions, absent/two target versions, target present/absent, eligible/ineligible source, valid/invalid candidate, source/policy drift, and known-failed/known-committed/unknown commit outcomes. The TLA+ and Go relations are independently authored and graph-equal in both directions over those bounds. It explicitly excludes CUE correctness, filesystem crash durability, cryptographic hashes, arbitrary bounds, and package reproducibility.

Emacs presents `Promote to new profile…` only for ad-hoc sessions. It fetches fresh value-free session/grant data, prompts for a new name, renders all grants unchecked with `session-only → profile-persistent / future sessions`, invokes preview, shows the pinned review facts, requires a second confirmation, then applies the exact plan. It keeps draft/review state on cancellation, stale/validation failure, and commit uncertainty. Known success routes to normal file review/trust; no path auto-trusts, stops, or launches.

- [x] T1 — Add RED promotion-kernel tests and an independent reducer
  FILE: `internal/engine/promotion/promotion.go`, `internal/engine/promotion/promotion_test.go`
  CHANGE: Define data-only source/policy snapshots, plan intent, finite events/effects/outcomes, normalization, `Prepare`, `CheckApply`, and deterministic `Step`. Write RED tests first for every pinned invariant, including empty selection, exact selected-ID mapping, no session/runtime write effect, source/policy matching, create-only/builtin refusal, validation-before-replace, and unknown-commit routing.
  VERIFY: `go test ./internal/engine/promotion -run 'Promotion|Prepare|Apply|Invariant' -count=1 -v`
  EXPECTED: Focused tests prove the pure kernel rejects all unsafe transitions and emits only the five data-only effects.

- [x] T2 — Add the bounded promotion TLA+ relation, mutants, and bidirectional graph conformance
  FILE: `formal/promotion/Promotion.tla`, `formal/promotion/Promotion.cfg`, `formal/promotion/mutants/*.cfg`, `formal/promotion/README.md`, `internal/engine/promotion/tla_*_test.go`, `internal/engine/promotion/testdata/*`, `scripts/check-tla-promotion.sh`, `Makefile`
  CHANGE: Independently author the finite TLA+ relation and strict graph parser/comparator integration. Require positive invariant/deadlock checks and named failures for mutants removing the empty-selection, exact-source, complete-grant-revision, policy-hash, create-only, validation, and source-consistency guards or treating unknown commit as success. Add bootstrap/model/conformance targets and include promotion conformance in `make check` without adding a binary dependency.
  VERIFY: `make check-tla-promotion && TLA_OFFLINE=1 make check-tla-promotion && go test ./internal/engine/promotion -run 'TLA|Graph|Mutation' -count=1 -v`
  EXPECTED: Positive finite relation and Go reducer have exact normalized initial/state/edge equality; every named mutant fails its declared law/action.

- [x] T3 — Build a shared locked atomic policy replacement primitive with classified outcomes
  FILE: `internal/engine/policy/transaction.go`, `internal/engine/policy/transaction_test.go`, `internal/cli/dependencies.go`, affected policy-write callers/tests`
  CHANGE: TDD a per-policy `0600` lock and complete render/validate-before-replace primitive with no-replace/create-only support, exact target-hash comparison, durable temp/write/sync/rename flow, and typed known-precommit versus commit-uncertain outcomes. Refactor only existing policy mutations that can safely consume the primitive; preserve their public contracts.
  VERIFY: `go test ./internal/engine/policy ./internal/cli -run 'PolicyTransaction|Atomic|CommitUncertain|ProfileEgress|ProfileCredentials' -count=1 -v`
  EXPECTED: Contending writers serialize; stale/no-replace/validation/pre-commit failures leave target unchanged; uncertainty is never represented as committed success.

- [ ] T4 — Add promotion plan codec, source snapshot adapter, and candidate renderer fidelity tests
  FILE: `internal/engine/promotion/plan.go`, `internal/engine/promotion/plan_test.go`, `internal/cli/profile_promote.go`, `internal/cli/profile_promote_test.go`, `internal/engine/session/*_test.go`, `internal/jsoncontract/testdata/*`
  CHANGE: TDD bounded `0600` plan encoding/decoding and source-snapshot extraction from authoritative session records. Render an ordinary self-contained profile into the full target CUE while preserving unrelated policy semantics. Enforce ad-hoc-only eligibility; canonical workspace fidelity; resolved-metadata all-or-nothing handling; grant-revision/complete-set binding; one-to-one selected durable destinations; value-free public data; and no session mutation. Add golden success/error envelopes without CUE/secret content.
  VERIFY: `go test ./internal/engine/promotion ./internal/engine/session ./internal/cli ./internal/jsoncontract -run 'Promote|Promotion|AdHoc|Plan|Fidelity|ValueFree|PersistentEgress' -count=1 -v`
  EXPECTED: Preview leaves session/policy unchanged and writes a private plan; malformed, profile-backed, partial-metadata, duplicate/stale, and value-leaking paths fail closed.

- [ ] T5 — Wire the preview/apply Cobra contract and exact transaction behavior
  FILE: `internal/cli/profile.go`, `internal/cli/profile_promote.go`, `internal/cli/profile_promote_test.go`, `internal/cli/cli_help_iw3_test.go`, `internal/jsoncontract/testdata/*`
  CHANGE: Register `profile promote preview|apply` with exact required/repeatable flags and output-only JSON. Preview validates/render/binds/writes plan. Apply re-reads authoritative source and target under the policy transaction, accepts only created/stopped source (while allowing compatible running-to-stopped evidence), invokes reducer decisions, and maps stale, conflict, validation, known failure, and unknown commit to distinct value-free envelopes. Do not trust, run, or write a session.
  VERIFY: `go test ./internal/cli ./internal/jsoncontract -run 'ProfilePromote|PromotionPlan|PromoteHelp|ValueFree|CommitUncertain' -count=1 -v`
  EXPECTED: Exact CLI help/argv and JSON goldens pass; apply changes only a newly absent target on known commit and leaves it untrusted.

- [ ] T6 — Add the explicit Emacs promote review flow and ERT contracts
  FILE: `emacs/safeslop-egress.el`, `emacs/safeslop-session.el`, `emacs/safeslop-portal.el`, `emacs/test/safeslop-test.el`, `emacs/test/safeslop-contract-test.el`, `emacs/test/safeslop-ui-probe.el`
  CHANGE: Add portal/detail action visibility only for ad-hoc sessions; fetch current status plus grant snapshot asynchronously; build an unchecked checkbox selection UI; construct exact preview/apply argv; render only safe review fields and the untrusted/future-session statements; require a second confirmation. Guard stale callbacks and retain the buffer/draft for cancellation and all failed/uncertain outcomes; on success open ordinary policy review/trust guidance only.
  VERIFY: `make test-emacs EMACS="$(command -v emacs)" && make test-emacs-ui-matrix`
  EXPECTED: ERT proves unchecked defaults, exact argv, two confirmations, no implicit grant selection/stop/trust/launch, safe rendering, callback guarding, and retained draft on failure.

- [ ] T7 — Synchronize documentation, operator skill, acceptance notes, and full verification
  FILE: `README.md`, `skills/agent-sandbox-ops/SKILL.md`, `formal/promotion/README.md`, `specs/0120-ad-hoc-session-promotion.md`
  CHANGE: Document the CLI, preview/apply drift semantics, created/stopped apply rule, explicit unchecked grants, source-to-future lifetime change, untrusted outcome, no auto-stop/trust/launch, transaction uncertainty, formal bounds/limits, and exact Emacs workflow. Mark tasks only after their verification evidence is recorded.
  VERIFY: `git diff --check && make check && make build`
  EXPECTED: Formatting, Go vet/tests, both finite model gates, strict Emacs tests, docs/skill consistency checks, and static binary build pass.
