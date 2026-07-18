# TLA+ session-boundary architecture decision

Date: 2026-07-18  
Project state: `main@ab08404`  
Decision: approved for implementation planning

## Decision

Use one bounded TLA+ session-boundary model as the normative abstract safety relation and a pure production Go reducer in `internal/engine/session/protocol.go`. TLC and Go independently enumerate the same finite protocol. CI compares normalized initial-state sets, reachable-state sets, and labelled directed-edge sets in both directions; neither implementation is generated and the comparator contains no third transition table.

TLA+/Java remains development/CI-only. Existing characterization tests remain authoritative for observable behavior while the reducer is introduced. A disagreement stops the refactor; it does not authorize behavior change.

## Locked laws

- At most one live owner; exact PID/process-token identity is required for handoff, release, and signal authorization.
- Effective runtime egress is always a subset of durable reviewed authority within the safeslop-managed boundary.
- Normal success requires durable, recorded, actual runtime, and positively inspected generations to agree.
- Ambiguous durability/runtime state cannot resume normal authority mutation without verified recovery or proven teardown.
- Corrupt, stale, or commit-uncertain persistence cannot become silent success.
- Public v1 JSON/JSONL/CUE, errors, ordering, side effects, defaults, and legacy behavior remain unchanged.

## Semantic boundary

Persisted lifecycle remains exactly `Created | Running | Stopped`. Claim is an operation context: the durable record remains Created until a known-new commit moves it directly to Running. The model does not invent a `Claimed` status or pretend a non-detached Running record distinguishes a coupled owner from a detached parent claim.

State separates durable record truth, effective authority, actual runtime generation, possible correlated worlds after unknown outcomes, inspected generation, record health, mode, transaction owner, operation, one pending effect, owner observation, teardown proof, and caller result. External effects are nondeterministic classified outcomes. Hashes become symbolic `Generation(authority, revision)` values.

The positive model checks `TypeOK`, `ValidRecordShape`, `AtMostOneLiveOwner`, `ExactOwnerRequiredForHandoffReleaseSignal`, `RuntimeAuthoritySubsetOfDurableAuthority`, `NormalStateHasGenerationAgreement`, `UncertaintyBlocksNormalAuthorityMutation`, `TeardownClaimRequiresProvenTeardown`, `StoppedHasNoOwnerAndNoEffectiveRuntimeAuthority`, `CommitUnknownNeverAssumedOld`, `InvalidPersistenceCannotSucceed`, and terminal-aware deadlock freedom.

## Correspondence gate

TLC v1.7.4 emits the complete finite graph using `-dump dot,actionlabels`. A strict pinned-format parser normalizes the model's knowledge view. A deterministic BFS invokes production `Enabled` and `Reduce` over bounds parsed from the positive cfg. The gate compares:

1. TLC initial nodes marked by the pinned initial-node attribute versus Go `InitialStates(bounds)` after concrete adapter round-trip;
2. all normalized reachable states;
3. all `(source, action, target)` edges.

It reports both `TLA − Go` and `Go − TLA` with a shortest witness. Raw TLA truth states still receive every invariant check before the explicitly documented epistemic quotient.

A repository-native mutation-site gate scans all non-test Go under `internal/`, records an exact baseline, forbids new model-owned lifecycle/authority mutation sites, and shrinks as branches move behind the protocol adapter.

## Tooling and negative controls

Pin official TLA+ Tools v1.7.4 at SHA-256 `936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88`. Verify every invocation; cache under ignored `.build/tla`; support offline reruns and only byte-identical overrides. Positive checking is exhaustive, single-worker, bounded by reviewed state/time ceilings.

Expected-failure mutants cover runtime-before-durable widening, a second owner, stale same-PID/different-token authorization, unknown commit treated as old, ACK without positive inspect, and stopped without proven teardown. Comparator tests add/drop/rename edges and alter initial states.

## Adversarial evaluation

Locked weights: safety fidelity 30, mechanical correspondence 30, incremental refactor safety 20, reproducible operation 10, scope/plan quality 10.

The first isolated evaluation scored 90/100. Host review found one semantic flaw: the draft included a transient `Claimed` lifecycle despite no concrete persisted discriminator. The artifact was corrected to model claim only as an operation/commit, expand the mutation scan to all internal packages, mechanize initial-state equality, characterize currently decodable stale shapes, and record baseline metrics. Clean cross-family re-evaluation scored **89.5/100** with no blocker or deterministic-law violation.

Remaining limitations are explicit: finite bounds are not an unbounded proof; normalized graph equality is weaker than raw-state bisimulation; the model assumes atomic old/new commits and sound positive apply/inspect/teardown outcomes; external manual runtime tampering and tokenless legacy identity are outside the exact theorem; no liveness claim is made.

## Rejected alternatives

1. Sampled Go trace inclusion: lower churn but one-directional and coverage-dependent.
2. TLA+ plus prose/current tests: useful exploration but cannot resist semantic drift.
3. Generated production code or a new DSL: too much coupling and irreversible machinery.
