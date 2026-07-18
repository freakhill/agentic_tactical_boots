# Session-boundary TLA+ model

`SessionBoundary.tla` is a development/CI-only bounded safety model. It does not ship in the safeslop binary and makes no liveness or unbounded proof claim.

Run:

```sh
make bootstrap-tla-session
make check-tla-session-model
TLA_OFFLINE=1 make check-tla-session-model
```

The bootstrap verifies official TLA+ Tools v1.7.4 by the SHA-256 in `formal/tla2tools.lock` before every Java invocation. Artifacts and complete TLC logs/graphs remain under ignored `.build/tla/session/model/`. Checks use one TLC worker, deterministic locale/timezone, isolated metadirs, a 120-second process ceiling, and a reviewed 100,000-distinct-state ceiling.

## Boundary and bounds

The model owns one durable record, one record lock/transaction owner, and two exact owners (`OwnerA`, `OwnerB`) that represent the same PID with different opaque process-start tokens. It retains only the three persisted lifecycle values `Created`, `Running`, and `Stopped`; claim is an operation whose known-new commit moves directly from Created to Running.

Two symbolic grants cover empty, singleton, multiple, duplicate-add, and remove-one shapes. Revisions 0..2 cover two consecutive generation changes; each cfg supplies `MaxRevision = 2`, and Go parses that positive-cfg bound before graph enumeration. These are representative finite bounds, not a proof for arbitrary grants/revisions. Generations are symbolic records of exact authority plus revision; the model proves equality relationships, not SHA-256.

The raw state keeps durable authority, actual runtime authority, durable recorded-applied generation, positive inspected generation, persisted transition intent, old/new possible worlds after an unknown commit, health/mode, operation, one pending effect, exact-owner evidence, teardown proof, and caller result distinct. Unknown commit actions nondeterministically retain the actual old or new truth while knowledge remains `{Old, New}` until recovery inspects it. External commit/apply/inspect/teardown and process observations are classified nondeterministic outcomes; Docker, Squid, the kernel, and filesystem durability are assumptions outside the model.

Widening commits the durable upper bound before runtime apply. Narrowing first commits only the recovery intent represented by `direction/pending*`, then applies and positively inspects the smaller runtime, and only then commits the final durable authority. A known-old narrow final commit restores the proven old generation. An unknown final commit never restores wider runtime without recovery. Only proven teardown may claim Stopped with empty effective runtime authority.

The positive cfg exhaustively checks the named invariants on raw truth states and checks `TerminalAwareDeadlock`; terminal states have an explicit stuttering step. There is no fairness constraint, hidden state constraint, simulation mode, or `Claimed` lifecycle.

## Pinned TLC graph grammar and normalization

The conformance parser accepts only the verified TLC v1.7.4 `dot,actionlabels` shape. The graph envelope is exactly `strict digraph DiskGraph`, `nodesep=0.35`, and one white `cluster_graph`, with no trailing LF. Signed-decimal node IDs carry a `label`; only mechanically marked initial nodes also carry `style = filled` and use TLC's observed no-semicolon form. Edges accept exactly `label`, `color="black"`, and `fontcolor="black"`. Rank rows accept only `rank = same`. Node labels accept only the observed `\\n` and `\\"` escapes and the finite TLA value grammar used here: records, sets, strings, integers, and booleans. Unknown attributes, syntax, escapes, fields, actions, duplicate definitions, missing references, incomplete rank metadata, and absent initial markers fail closed.

Every raw state must contain all 22 model fields. The normalized key retains lifecycle, owners/detached mode, durable/runtime/pending authority and revisions, recorded/runtime/inspected generations, health, mode, operation, effect, result, and direction. It deliberately drops the five checker-only fields `evidence`, `worlds`, `teardownProven`, `txOwner`, and `lastOwnerAction` only after validating their finite domains. DOT node IDs, rank/layout metadata, colors, fonts, and the consumed initial style are also omitted. Records become lexicographically keyed JSON, owner/grant sets become sorted arrays, and `[authority |-> {}, revision |-> -1]` becomes the single `NoGen` sentinel. Initial keys, normalized state keys, and labelled directed edges are then compared in both directions with shortest graph witnesses.

## Expected-failure controls

Each mutant adds exactly one unsafe action to the positive relation. The harness requires TLC to name both the expected invariant and the violating action anchor.

| cfg / constant | Enabling predicate | First violating action | Expected invariant |
|---|---|---|---|
| `RuntimeBeforeDurable` | reachable Running/Normal/Idle state with a grant available | `MutantRuntimeBeforeDurable` expands runtime without durable authority | `RuntimeAuthoritySubsetOfDurableAuthority` |
| `SecondOwner` | reachable Running state owned by OwnerA | `MutantSecondOwner` adds same-PID/different-token OwnerB | `AtMostOneLiveOwner` |
| `StaleToken` | reachable Running state owned by OwnerA | `MutantStaleToken` hands off on token-mismatch evidence | `ExactOwnerRequiredForHandoffReleaseSignal` |
| `UnknownAsOld` | reachable Widen commit boundary | `MutantUnknownAsOld` resumes normal from an unknown commit as old | `CommitUnknownNeverAssumedOld` |
| `AckWithoutInspect` | reachable Running/Normal/Idle state with a grant available | `MutantAckWithoutInspect` records a new generation without positive inspect | `NormalStateHasGenerationAgreement` |
| `StopWithoutProof` | reachable Running/Normal/Idle state | `MutantStopWithoutProof` claims Stopped without teardown evidence | `TeardownClaimRequiresProvenTeardown` |

Later conformance work will document the pinned TLC DOT/action-label grammar and the explicit raw-state-to-knowledge quotient used for TLA+/Go graph comparison.
