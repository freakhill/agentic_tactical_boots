# 0116 — TLA+ session-boundary safety and Go conformance

Status: approved; implementation pending

SCOPE: establish a bounded TLA+ safety model for one transactional session record, two competing exact owners, old/new/uncertain durable commits, and direction-aware egress apply/inspect/recovery/teardown; refactor deterministic production decisions into one pure Go reducer; mechanically require bidirectional equality of normalized TLA+/Go initial states, reachable states, and labelled edges.

OFF-LIMITS: no public v1 JSON/JSONL/CUE, CLI, error, ordering, side-effect, default, trust, credential, workspace, mount, runtime-selection, UI, or legacy behavior change; no fourth persisted lifecycle value; no generated production code/DSL; no production trace logging; no Java/TLA runtime dependency; no proof claim for CUE, SHA-256, POSIX durability, process liveness, Docker/Squid, external runtime tampering, tokenless legacy identity, liveness, or unbounded grants/revisions; no silent fix when model and characterization disagree.

WORKTREE: `.worktrees/0116-tla-session-boundary/`

Decision notes: `specs/research/2026-07-18-tla-session-boundary-ayo.md`, `specs/research/2026-07-18-tla-session-boundary-flo.md`.

Frozen laws:

- at most one live owner; exact PID/process-token evidence is required for handoff, release, and signal authorization;
- effective runtime egress is a subset of durable reviewed authority within the managed boundary;
- normal success means durable, recorded-applied, actual runtime, and positively inspected generations agree;
- unknown durability/runtime state blocks normal authority mutation until verified recovery or proven teardown;
- corrupt, stale, and commit-uncertain persistence never becomes silent success;
- claim is an operation and known-new `Created -> Running` commit, never a new status;
- behavior characterization is authoritative during refactor; any discovered defect is a separate approved RED→GREEN change;
- TLA+ and the production reducer are independently authored and mechanically graph-equivalent over reviewed finite bounds.

State abstraction map (frozen before implementation):

| Concrete state/evidence | Abstract treatment |
|---|---|
| `Session.Status` | Persisted `Created|Running|Stopped`; claim is `operation=Claim` until known-new commit. |
| `PID` + `ProcessToken` | Exact owner pair; an absent legacy token is `LegacyUnverified`, outside the exact-token theorem. |
| `Detached` | Bare-PID versus process-group signal mode; it is not evidence of claim versus coupled execution. |
| `recordRevision` | Durable record version and commit-candidate identity; preserved by concrete adapter round-trips. |
| immutable `PersistentEgress` | `BaseAuthority`, retained by every modeled authority state. |
| `EgressGrants` + `GrantRevision` | Durable mutable authority and symbolic generation revision. |
| `appliedEgressRevision` + `appliedEgressHash` | Durable recorded-applied generation; hash maps only by equality to `Generation(authority, revision)`. |
| persisted `EgressTransition` | Direction, candidate authority/revision/generation, and interrupted recovery phase. A narrow intent marker is durable before apply; the final durable authority list shrinks only after positive runtime inspect/ACK. |
| `Environment` + `Network` + `Backend` | Eligibility/precondition for modeled container-deny authority; otherwise framed unchanged, not protocol state. |
| fixed `network_authority_uncertain` failure | Durable blocked/uncertain marker; other failure/metadata/credential fields are framed unchanged. |
| actual proxy generation/effective authority | External truth variable supplied through apply/inspect effects, never inferred from the record. |
| process liveness/token check | `ProcessAliveSession` plus full pair comparison maps to `ExactLive`, `Dead`, `TokenMismatch`, or `Unknown`; only `ExactLive` can authorize the exact operation, and the model does not prove the OS observation. |
| teardown/reap result | `Proven` only after every required owned-boundary teardown callback returns nil and absence is positively established by that effect contract; any error, timeout, skipped required effect, or ambiguous result maps to `Unknown` and cannot authorize a stopped claim. |
| socket, PTY, stage paths, timestamps, names, recipe/image metadata | Out-of-model or normalized away; existing characterization remains authoritative. |

Current mutation owners to close in Wave 1: lifecycle/owner writes in `markRunning`, `HandoffRunningDetached`, `ReleaseRunningClaim`, `Finish`, reconcile, and `Stop`; authority/runtime-state writes in `AppendGrant`, `RevokeGrant`, `SetEgressRuntimeState`, `withAppliedGeneration`, `withTransition`, `copySessionAuthority`, uncertainty stop/recovery candidate construction, and persistence codec/constructor paths. The generated inventory—not this prose—is the exact allowlist and must also detect any additional site.

Verified effect ordering to preserve rather than redesign: `Store.Stop` performs optional credential revoke → exact liveness/token recheck → signal → socket removal → reap callbacks in argument order → terminal commit; `Store.Remove` performs running refusal → optional revoke → reap callbacks → socket removal → atomic record removal. `Remove` stays characterized and outside the initial reducer relation. `Finish` remains a record transition whose callers retain their existing teardown-before-finish obligations.

Baseline (2026-07-18, `ab08404`, clean main): focused session/container/CLI tests, focused session/CLI race tests, `make check`, and `make build` all exited 0; strict Emacs ran 242 tests with 241 expected passes and one expected raw-Doom skip. No TLA+ files/checker or model target exists. Local TLC v1.7.4 was independently probed with `-dump dot,actionlabels` and emitted parseable named state edges.

## Wave 1 — preserve current semantics

- [x] Freeze stable-boundary characterization and mutation inventory
  FILE:     `specs/0116-tla-session-boundary.md`; focused additions only to `internal/engine/session/*_test.go`, `internal/engine/container/*_test.go`, `internal/cli/*_test.go`; new temporary/read-only inventory helper under `internal/engine/session` test code if needed
  CHANGE:   Record exact focused test names/count and a complete sorted inventory of non-test writes/calls affecting `Session.Status`, owner identity, detached ownership, egress grants/revision, applied generation, and persisted transition across all `internal/`. Give new characterization tests one stable `TestSessionProtocolCharacterization*` prefix and assert the enumerated count so later focused commands cannot silently omit them. Add characterization only where current old/new/unknown commit, stale/decode shape, widen/narrow fault, recovery, teardown, claim/handoff/release/signal, exact Stop/Remove callback order, or value-free/public behavior lacks a stable assertion. Do not edit production semantics. If expected behavior is ambiguous, stop and amend the decision rather than choosing one.
  VERIFY:   `git diff --check && go test ./internal/engine/session ./internal/engine/container ./internal/cli -run 'Session|Store|Atomic|Concurrent|Egress|Grant|Revoke|Transition|Generation|Uncertain|Detach|Supervise|Claim|Handoff' -count=1 && go test -race ./internal/engine/session ./internal/cli -run 'Session|Store|Concurrent|Egress|Transition|Claim|Handoff' -count=1 && make check && make build`
  EXPECTED: Baseline behavior and model-owned mutation sites are explicit and reproducible; all pre-existing gates remain green; production files are unchanged.

  TASK 1 EVIDENCE: The focused selection contains 208 existing tests; the exact new stable characterization set contains 2 tests: `TestSessionProtocolCharacterizationStoreStopEffectOrder` and `TestSessionProtocolCharacterizationRemoveEffectOrder`. Existing focused tests already pin stale/decode rejection, old/new/commit-uncertain persistence, widen/narrow recovery, uncertainty blocking, claim/handoff/release/signal identity, generation ACK, teardown, and value-free contracts. The two renamed-and-strengthened tests close the missing exact callback-order assertions without adding a duplicate. Exact pre-reducer mutation/call inventory, sorted by path then function (`state` includes the fixed uncertainty marker):

  ```text
  internal/cli/session.go|clearNarrowTransition|authority+applied-generation+transition|commit restored durable candidate
  internal/cli/session.go|cmdSessionListWithDeps|lifecycle+owner|call ListReconciled
  internal/cli/session.go|cmdSessionRunWithDeps|lifecycle+owner|call MarkRunning
  internal/cli/session.go|cmdSessionStatusWithDeps|lifecycle+owner|call GetReconciled
  internal/cli/session.go|cmdSessionStopWithDeps|lifecycle+owner|call GetReconciled then Stop
  internal/cli/session.go|commitEgressFailureState|state|commit bounded uncertainty marker
  internal/cli/session.go|commitRecoveredGeneration|applied-generation+transition|commit cleared transition
  internal/cli/session.go|copySessionAuthority|egress-grants+grant-revision|direct write
  internal/cli/session.go|egressSessionWithDeps|authority+applied-generation+transition|call running recovery under record lock
  internal/cli/session.go|failClosedEgressWithDeps|lifecycle+owner+authority+state|construct and commit teardown/uncertainty result
  internal/cli/session.go|finishSessionRun|lifecycle+owner|call Finish
  internal/cli/session.go|grantSessionEgressWithDeps|authority+applied-generation+transition|call AppendGrant and commit pending/final candidates
  internal/cli/session.go|recoverRunningSessionEgressWithDeps|authority+applied-generation+transition|direct candidate writes and recovery commits
  internal/cli/session.go|revokeSessionEgressWithDeps|authority+applied-generation+transition|call RevokeGrant and commit pending/final candidates
  internal/cli/session.go|runDetachWithDeps|lifecycle+owner|call MarkRunning/ReleaseRunningClaim/Finish
  internal/cli/session.go|stopForEgressUncertainty|lifecycle+owner+applied-generation+transition|direct write
  internal/cli/session.go|withAppliedGeneration|applied-generation+transition|direct runtime-state write
  internal/cli/session.go|withEgressUncertaintyFailure|state|write fixed blocked marker
  internal/cli/session.go|withTransition|applied-generation+transition|direct runtime-state write
  internal/cli/supervise.go|superviseWithDeps|lifecycle+owner|call HandoffRunningDetached/Finish
  internal/engine/session/egress_grant.go|AppendGrant|egress-grants+grant-revision|direct write
  internal/engine/session/egress_grant.go|RevokeGrant|egress-grants+grant-revision|direct write
  internal/engine/session/egress_grant.go|SetEgressRuntimeState|applied-generation+transition|direct write
  internal/engine/session/session.go|Finish|lifecycle+owner+applied-generation+transition|direct write and commit
  internal/engine/session/session.go|GetReconciled|lifecycle+owner+applied-generation+transition|call reconcile and commit
  internal/engine/session/session.go|HandoffRunningDetached|owner+detached|direct write and commit
  internal/engine/session/session.go|ListReconciled|lifecycle+owner+applied-generation+transition|call GetReconciled
  internal/engine/session/session.go|ReleaseRunningClaim|lifecycle+owner+detached|direct write and commit
  internal/engine/session/session.go|SetFailure|state|direct failure-marker write
  internal/engine/session/session.go|Stop|lifecycle+owner+applied-generation+transition|direct write and commit
  internal/engine/session/session.go|markRunning|lifecycle+owner+detached|direct write and commit
  internal/engine/session/session.go|reconcile|lifecycle+owner+applied-generation+transition|direct write
  internal/engine/session/store.go|Create|lifecycle|construct created record
  internal/engine/session/store.go|RecordTx.Commit|protocol state|generic locked persistence gateway
  internal/engine/session/store.go|Save|protocol state|generic stale-checked persistence gateway
  internal/engine/session/store.go|Update|protocol state|generic callback persistence gateway
  internal/engine/session/store.go|WithLocked|protocol state|generic locked callback gateway
  internal/engine/session/store.go|decodeRecord|protocol state|rehydrate public and private recovery state
  internal/engine/session/store.go|encodeRecord|protocol state|encode public and private recovery state
  internal/engine/session/store.go|writeLocked|protocol state|atomic old/new/uncertain commit boundary
  ```

## Wave 2 — independent bounded safety model

- [ ] Pin and verify the development-only TLC toolchain
  FILE:     new `formal/tla2tools.lock`, new `scripts/check-tla-session.sh`, `.gitignore`, `Makefile`
  CHANGE:   Pin official TLA+ Tools v1.7.4 URL and SHA-256 `936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88`. Fetch via temporary file into ignored `.build/tla` only when absent, verify before every Java invocation, accept only a byte-identical `TLA2TOOLS_JAR`, support `TLA_OFFLINE=1`, stable locale/timezone, one worker, isolated metadir, retained artifacts, and fixed setup errors. Add a `bootstrap-tla-session` acquisition/verification target; do not add TLA to `make check` until conformance is complete.
  VERIFY:   `rm -rf .build/tla && make bootstrap-tla-session && TLA_OFFLINE=1 make bootstrap-tla-session && test ! -e formal/tla2tools.jar && git diff --check`
  EXPECTED: First use acquires only verified bytes; the offline rerun succeeds without network; bad/foreign bytes fail before Java; the shipped binary inputs do not reference TLA/Java.

- [ ] Model the raw transactional session protocol and expected unsafe mutants
  FILE:     new `formal/session/SessionBoundary.tla`, `formal/session/SessionBoundary.cfg`, `formal/session/mutants/*.cfg`, `formal/session/README.md`, `scripts/check-tla-session.sh`, `Makefile`
  CHANGE:   Model one persisted `Created|Running|Stopped` session, claim as operation/commit, two same-PID/different-token owners, one record lock, two symbolic grants, revisions 0..2, durable/runtime truth, correlated possible worlds, inspected generation, health/mode/operation/effect/evidence/result, crash between atomic boundaries, widen durable-first, narrow durable-intent-marker → runtime apply+positive inspect → final authority shrink, recovery, and proven/unknown teardown. Two grants cover empty/one/multiple, duplicate-add, and remove-one shapes; revisions 0..2 cover two consecutive generation changes, without claiming an unbounded proof. Check the decision-note invariant set on raw states. Add terminal-aware deadlock safety and no fairness/liveness claim. Add one isolated mutant each for runtime-before-durable, second owner, stale token, unknown-as-old, ACK-without-inspect, and stop-without-proof; require the named invariant and action anchor rather than generic failure. Document each mutant's enabling initial predicate and first violating action. Set reviewed positive ceilings of 100,000 normalized states and 120 seconds; stop and revise the design before production edits if the model cannot fit honestly.
  VERIFY:   `make check-tla-session-model && TLA_OFFLINE=1 make check-tla-session-model && rg -n 'AtMostOneLiveOwner|RuntimeAuthoritySubsetOfDurableAuthority|CommitUnknownNeverAssumedOld|TeardownClaimRequiresProvenTeardown' formal/session/SessionBoundary.tla`
  EXPECTED: Positive exhaustive TLC exits 0 with all named invariants and records state/edge/runtime counts; every mutant exits nonzero only for its expected law/action; no simulation, hidden constraint, or invented Claimed state is used.

## Wave 3 — production semantic kernel and graph equality

- [ ] Define the pure production protocol reducer under characterization
  FILE:     new `internal/engine/session/protocol.go`, `internal/engine/session/protocol_test.go`; representation-only helpers in `internal/engine/session/{session.go,egress_grant.go,store.go}` and `internal/engine/egress/generation.go` only when tests force them
  CHANGE:   TDD closed enums/values for protocol state, event/action, bounded outcome, one tagged effect, and caller result; implement effect-free `Reduce`, finite-domain `Enabled`, `InitialStates`, normalization, concrete Session mapping/round-trip, candidate construction, and symbolic/concrete generation-equality fixtures. No I/O, clock, randomness, logging, callbacks, expected-state table, or duplicated guard inside `Enabled`. Characterize currently decodable stale lifecycle/owner shapes as non-normal rather than silently tightening decode behavior. At this checkpoint production call paths remain unchanged.
  VERIFY:   `go test ./internal/engine/session -run 'Protocol|Reducer|Enabled|Adapter|Generation' -count=1 -v && go test -race ./internal/engine/session -run 'Protocol|Reducer|Adapter' -count=1 && git diff --check`
  EXPECTED: Every transition/effect/outcome is typed and deterministic, concrete representation round-trips preserve bytes/semantics, stale shapes remain compatible, and no production caller uses a test-only mirror yet.

- [ ] Add strict TLC graph parsing and comparator negative controls
  FILE:     new `internal/engine/session/tla_graph_test.go`, new minimal `internal/engine/session/testdata/tlc-v1.7.4-actionlabels.dot`, `formal/session/README.md`
  CHANGE:   TDD a parser for only the verified v1.7.4 DOT/action-label and finite value grammar. Enumerate every accepted attribute and every dropped checker-only field; reject unknown syntax/attributes, malformed escapes, duplicate/missing nodes, unknown actions, and absent initial markers. Canonicalize records/sets/sentinels to sorted JSON. Compare initial keys, states, and labelled edges in both directions; tests must add/drop/rename an edge and alter an initial state and assert directional shortest-witness diagnostics. Do not embed a third transition relation.
  VERIFY:   `go test ./internal/engine/session -run 'TLCGraph|TLCParser|GraphComparator|InitialState' -count=1 -v && git diff --check`
  EXPECTED: The pinned fixture parses deterministically; every parser/comparator mutant fails for its intended reason; initial nodes are mechanically recognized rather than assumed.

- [ ] Prove bounded TLA+/Go graph equivalence and mutation ownership
  FILE:     new `internal/engine/session/tla_conformance_test.go`, `internal/engine/session/protocol_mutation_test.go`, `internal/engine/session/testdata/protocol-mutation-allowlist.txt`, `scripts/check-tla-session.sh`, `Makefile`
  CHANGE:   Parse bounds from the positive cfg; compare TLC initial nodes with adapter-round-tripped Go `InitialStates` before BFS; enumerate the reducer graph and require exact bidirectional initial/state/labelled-edge equality. Require every non-sentinel Go action/outcome/effect and every positive TLA action to be reachable/mapped. Add an end-to-end dropped-edge control. Add an all-`internal/` syntax-aware mutation scanner, exact sorted baseline at `internal/engine/session/testdata/protocol-mutation-allowlist.txt` with stable `path|function|field|operation` lines (no line numbers), byte-stability test, and synthetic forbidden-write control; no new model-owned mutation site may appear. Add `check-tla-session` but still keep it separate from full `make check` until production wiring finishes.
  VERIFY:   `make check-tla-session && TLA_OFFLINE=1 make check-tla-session && go test ./internal/engine/session -run 'TLAConformance|ProtocolMutation' -count=1 -v`
  EXPECTED: Initial, reachable-state, and labelled-edge sets are exactly equal both ways; coverage is non-vacuous; the mutation baseline matches Wave 1 and rejects growth.

## Wave 4 — route lifecycle ownership through the reducer

- [ ] Route claim, handoff, and release decisions through the production reducer
  FILE:     `internal/engine/session/{protocol.go,session.go,protocol_test.go,session_test.go}`, `internal/cli/{session.go,supervise.go,cli_detach_test.go,supervise_test.go}`, mutation allowlist
  CHANGE:   In three RED→GREEN substeps, preserve existing signatures/errors/timestamps and route atomic claim, exact detached parent→supervisor handoff, and exact failed-spawn release through `Reduce`; execute current Store/process effects and feed their classified outcomes back. A known-new claim goes directly `Created -> Running`; no persisted/transient lifecycle discriminator is invented. After each substep remove only its old mutation keys and rerun focused characterization plus graph equality before continuing.
  VERIFY:   `go test ./internal/engine/session ./internal/cli -run 'MarkRunning|LaunchClaim|ConcurrentRunDetach|CoupledSessionRun|Handoff|ReleaseRunning|Supervis' -count=1 -v && go test -race ./internal/engine/session ./internal/cli -run 'MarkRunning|Claim|Handoff|Release|ConcurrentRunDetach' -count=1 && make check-tla-session`
  EXPECTED: Concurrent coupled/detached launch still has one winner, stale owners cannot handoff/release, launch failure releases only its exact claim, public envelopes are unchanged, and TLA+/Go graphs remain equal.

- [ ] Route reconcile, stop/signal authorization, and terminal lifecycle decisions through the reducer
  FILE:     `internal/engine/session/{protocol.go,session.go,protocol_test.go,session_test.go}`, affected `internal/cli/*_test.go`, mutation allowlist
  CHANGE:   In serialized RED→GREEN substeps, move exact process observation/reconcile, immediate pre-signal authorization, and stopped finalization decisions behind `Reduce` while preserving current callbacks, group/bare-PID routing, tokenless legacy limitations, error ordering, and idempotency. The driver may execute only the reducer's single tagged effect at each step, and exact characterization pins the existing Stop sequence: optional revoke → immediate exact recheck → signal → socket removal → ordered reap callbacks → terminal commit. `Remove` remains outside the reducer and retains revoke → reap → socket → remove ordering. Do not claim the model proves OS liveness or eliminate the existing effect-boundary recheck. Remove migrated mutation keys only after focused behavior passes.
  VERIFY:   `go test ./internal/engine/session ./internal/cli -run 'Stop|Signal|ProcessToken|Reconcile|Finish|Detached|Supervisor|Legacy' -count=1 -v && go test -race ./internal/engine/session ./internal/cli -run 'Stop|Reconcile|Finish|Detached' -count=1 && make check-tla-session`
  EXPECTED: Stale identities are never signalled, coupled/detached cleanup semantics and terminal records are unchanged, tokenless legacy behavior stays explicitly outside the exact-token theorem, and graph equality remains exact.

## Wave 5 — route egress authority through the reducer

- [ ] Separate runtime apply and positive inspect effects without behavior change
  FILE:     `internal/engine/container/{egress_grant_apply.go,egress_grant_apply_test.go}`, `internal/cli/{dependencies.go,egress_grant_apply_test.go}`, `internal/engine/egress/generation.go`
  CHANGE:   Characterize then expose the already-existing apply/replace and generation/hash inspect boundaries as separately classifiable effects. Keep current wrapper APIs, exact proxy replacement, readiness/hash ACK, uncertainty errors, overlay bytes, and command ordering. This is effect-boundary extraction only; do not route authority decisions or change behavior yet.
  VERIFY:   `go test ./internal/engine/container ./internal/cli -run 'ApplyEgressGeneration|InspectEgressGeneration|Proxy|Generation|Ack|Uncertain' -count=1 -v && go test -race ./internal/engine/container ./internal/cli -run 'EgressGeneration|Proxy' -count=1 && make check-tla-session`
  EXPECTED: Existing callers and fake-engine argv remain compatible; apply success cannot stand in for inspect ACK; model graphs are unchanged.

- [ ] Route running-session widen through the reducer
  FILE:     `internal/engine/session/{protocol.go,protocol_test.go,egress_grant.go}`, new `internal/cli/session_protocol.go`, `internal/cli/{session.go,dependencies.go,egress_grant_apply_test.go,session_concurrency_test.go}`, transaction tests, mutation allowlist
  CHANGE:   TDD then delegate only running-session widen: lock, durable upper-bound transition, known-new/known-old/unknown commit classification, apply, positive inspect, final applied-generation commit, recovery/teardown request, and caller result. Preserve duplicate and non-running grant behavior, grant IDs/timestamps, fixed value-free errors, and lock duration. Unknown commit/apply/final commit must never follow a known-old path. Remove only widen-owned mutation keys.
  VERIFY:   `go test ./internal/engine/session ./internal/engine/container ./internal/cli -run 'Grant|Widen|Transition|Generation|CommitUncertain|Teardown|Concurrent' -count=1 -v && go test -race ./internal/engine/session ./internal/cli -run 'Grant|Widen|Transition|Concurrent' -count=1 && make check-tla-session`
  EXPECTED: Runtime widening is unreachable before durable authority, every injected old/new/unknown path preserves existing output and fail-closed behavior, and graph equality/mutants pass.

- [ ] Route running-session narrow through the reducer
  FILE:     same protocol/CLI egress files and focused tests as widen; mutation allowlist
  CHANGE:   TDD then delegate narrow intent, target apply, positive inspect, final durable shrink, known restoration, and every unknown outcome. Preserve duplicate/not-found behavior and current revision ordering. A narrower runtime is safe under old/new durability; never restore wider runtime after unknown final shrink durability. Remove only narrow-owned mutation keys.
  VERIFY:   `go test ./internal/engine/session ./internal/engine/container ./internal/cli -run 'Revoke|Narrow|Transition|Generation|CommitUncertain|Restore|Teardown' -count=1 -v && go test -race ./internal/engine/session ./internal/cli -run 'Revoke|Narrow|Transition' -count=1 && make check-tla-session`
  EXPECTED: Every injected narrow interleaving preserves runtime⊆durable, known failures retain current restoration semantics, uncertain failures tear down/block as before, and graph equality/mutants pass.

- [ ] Route persisted recovery and fail-closed teardown through the reducer
  FILE:     `internal/engine/session/{protocol.go,protocol_test.go}`, `internal/cli/{session_protocol.go,session.go,dependencies.go,egress_grant_apply_test.go,session_concurrency_test.go}`, transaction/recovery tests, mutation allowlist
  CHANGE:   First migrate persisted widen/narrow recovery, then uncertainty-marker blocking, then teardown/stopped commit in separate green substeps. Recovery rereads/validates/inspects and filters correlated old/new possibilities; it never guesses. Only proven teardown may claim stopped/empty runtime; failed/unknown teardown retains the durable upper bound and fixed uncertainty marker. Preserve bounded failure-record retries, legacy generation bootstrap, lock ownership, and value-free contracts. Shrink the allowlist to constructor/codec/protocol adapter owners.
  VERIFY:   `go test ./internal/engine/session ./internal/engine/container ./internal/cli -run 'Recover|Legacy|Bootstrap|Uncertain|Teardown|Transition|Generation|Commit' -count=1 -v && go test -race ./internal/engine/session ./internal/cli -run 'Recover|Uncertain|Teardown|Transition' -count=1 && make check-tla-session`
  EXPECTED: Interrupted transitions converge only through verified state or proven teardown, terminal uncertainty remains blocked, legacy behavior is unchanged, graph equality is exact, and no unapproved direct mutation site remains.

## Wave 6 — durable operation and final proof

- [ ] Integrate formal checking into native CI and document the claim boundary
  FILE:     `Makefile`, `README.md`, `CONTRIBUTING.md`, `formal/session/README.md`, `specs/0116-tla-session-boundary.md`, `.github/workflows/go.yml`, `.woodpecker/go.yml`
  CHANGE:   Add `check-tla-session` to `make check`; keep both CI surfaces single-source through `make check`/`make build`. Add GitHub `actions/setup-java@v4` with Temurin 21 before `make check`; add an explicit `java -version` preflight and documented Java 17+ host prerequisite to the local Woodpecker lane rather than installing a runtime from pipeline code. Document Java/checker bootstrap, offline use, pin update review, state/runtime budgets, source-of-truth direction, the complete concrete→abstract field map above, accepted/dropped DOT grammar fields, epistemic quotient limitation, counterexample classification/manual Go regression workflow, assumptions, opaque platform-specific process tokens, tokenless/external-runtime limitations, and separate behavior-fix rule. Update skills only if an operator surface actually changed (none is planned).
  VERIFY:   `git diff --check && rg -n 'check-tla-session|TLA_OFFLINE|graph|counterexample|tokenless|finite' README.md CONTRIBUTING.md formal/session/README.md Makefile && rg -n 'setup-java|java-version|make check|make build' .github/workflows/go.yml && rg -n 'java -version|make check|make build' .woodpecker/go.yml`
  EXPECTED: GitHub has a deterministic Java setup, Woodpecker fails early with its documented host prerequisite, both execute the same native gate, and no operator/runtime dependency or public behavior is added.

- [ ] Run independent model/spec/code review and authoritative verification
  FILE:     whole branch; acceptance note in `specs/0116-tla-session-boundary.md`
  CHANGE:   Have isolated reviewers inspect all model assumptions/actions/invariants, mutant sensitivity, raw-state versus quotient claims, reducer use in production, graph comparator/parser, mutation allowlist, every frozen behavior contract, and shipped dependency closure. Resolve blocking/high findings with separate RED→GREEN proof; rerun affected and full gates. Record exact TLC state/edge/runtime counts, mutation-site final count, supported host evidence, and limitations without claiming unbounded proof.
  VERIFY:   `git diff --check && make check-tla-session && TLA_OFFLINE=1 make check-tla-session && go test -shuffle=on ./... && go test -race ./internal/engine/session ./internal/engine/container ./internal/cli && make check && make build && ! go list -deps ./cmd/safeslop | rg -i 'tla|java' && git status --short --branch`
  EXPECTED: Positive raw-state invariants and terminal-aware deadlock safety pass exhaustively; each unsafe mutant fails for its exact reason; normalized initial/state/edge graphs match both ways; all action/outcome/effect coverage and mutation controls are non-vacuous; focused/full/race/strict/build gates pass; the binary dependency closure and public contracts are unchanged; no blocker/high remains.
