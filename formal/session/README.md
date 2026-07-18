# Session-boundary TLA+ model

`SessionBoundary.tla` is a development/CI-only bounded safety model. It does not
ship in the safeslop binary, generate production code, monitor production, or
make a liveness or unbounded proof claim.

## Running the pinned checker

A Java 17+ runtime is required for development checks; GitHub CI uses Temurin 21.
Java and TLA+ are not dependencies of the signed Go binary.

```sh
make bootstrap-tla-session
make check-tla-session-model
make check-tla-session
TLA_OFFLINE=1 make check-tla-session
```

`bootstrap-tla-session` reads `formal/tla2tools.lock`, accepts only the official
release URL shape, and verifies TLA+ Tools v1.7.4 by the locked SHA-256 before
every Java invocation. It downloads through a temporary file only when the
ignored `.build/tla` cache is absent. `TLA2TOOLS_JAR` is accepted only when its
bytes match the same lock. `TLA_OFFLINE=1` forbids acquisition and therefore
requires that verified cache or override.

Checks use one TLC worker, deterministic locale/timezone, isolated metadirs, a
120-second process ceiling, and a reviewed 100,000-distinct-state ceiling. Stop
and revise the design if either ceiling is exceeded; do not hide growth with a
state constraint. Complete TLC logs, metadirs, and action-labelled DOT graphs
remain under ignored `.build/tla/session/model/`.

A checker pin update is a security/toolchain review, not a routine download:

1. review the official TLA+ Tools release and update `VERSION`, `URL`, and
   `SHA256` together in `formal/tla2tools.lock`;
2. independently hash the downloaded release artifact and review the lock diff;
3. remove `.build/tla`, run a clean online bootstrap/check, then run
   `TLA_OFFLINE=1 make check-tla-session`;
4. review parser fixtures, positive state/edge/runtime counts, every expected
   mutant failure, and bidirectional graph equality before accepting the pin.

## Boundary and finite bounds

The model owns one durable record, one record lock/transaction owner, and two
exact owners (`OwnerA`, `OwnerB`) that represent the same PID with different
opaque process-start tokens. It retains only the three persisted lifecycle
values `Created`, `Running`, and `Stopped`; claim is an operation whose known-new
commit moves directly from Created to Running.

Two symbolic grants cover empty, singleton, multiple, duplicate-add, and
remove-one shapes. Revisions 0..2 cover two consecutive generation changes; each
cfg supplies `MaxRevision = 2`, and Go parses that positive-cfg bound before graph
enumeration. These representative finite bounds are not a proof for arbitrary
grants, revisions, owners, or sessions. Generations are symbolic records of exact
authority plus revision; the model proves equality relationships, not SHA-256 or
collision resistance.

The raw state keeps durable authority, actual runtime authority, durable
recorded-applied generation, positive inspected generation, persisted transition
intent, old/new possible worlds after an unknown commit, health/mode, operation,
one pending effect, exact-owner evidence, teardown proof, and caller result
distinct. Unknown commit actions nondeterministically retain the actual old or
new truth while knowledge remains `{Old, New}` until recovery inspects it.

Widening commits the durable upper bound before runtime apply. Narrowing first
commits only the recovery intent represented by `direction/pending*`, then
applies and positively inspects the smaller runtime, and only then commits the
final durable authority. A known-old narrow final commit restores the proven old
generation. An unknown final commit never restores wider runtime without
recovery. Only proven teardown may claim Stopped with empty effective runtime
authority.

The positive cfg exhaustively checks the named invariants on raw truth states and
checks `TerminalAwareDeadlock`; terminal states have an explicit stuttering step.
There is no fairness constraint, hidden state constraint, simulation mode, or
`Claimed` lifecycle.

## Complete concrete-to-abstract map

| Concrete state or evidence | Abstract treatment |
|---|---|
| `Session.Status` | Persisted `Created|Running|Stopped`; claim remains `operation=Claim` until a known-new commit. |
| `PID` + `ProcessToken` | One exact owner pair. Tokens are opaque, platform-specific process-start identities compared only for equality. An absent legacy token is `LegacyUnverified`, outside the exact-token theorem. |
| `Detached` | Bare-PID versus process-group signal mode; never evidence of claim versus coupled execution. |
| `recordRevision` | Durable version/candidate identity at the concrete commit boundary. It is preserved by adapter framing, not added as a fourth lifecycle or normalized graph field. |
| immutable `PersistentEgress` | Constant `BaseAuthority`, retained and framed by every modeled authority state. |
| `EgressGrants` + `GrantRevision` | Durable mutable authority plus symbolic generation revision. |
| `appliedEgressRevision` + `appliedEgressHash` | Durable recorded-applied generation. A concrete hash maps only by exact equality to a bound `Generation(authority, revision)`; hash construction is outside the proof. |
| persisted `EgressTransition` | Direction, candidate authority/revision/generation, and interrupted recovery phase. Narrow intent is durable before apply; final durable authority shrinks only after positive inspect/ACK. |
| `Environment` + `Network` + `Backend` | Eligibility precondition for modeled container-deny authority; otherwise framed unchanged, not protocol state. |
| fixed `network_authority_uncertain` failure | Durable blocked/uncertain marker. Other failure, metadata, and credential fields are framed unchanged. |
| actual proxy generation/effective authority | External truth supplied through apply/inspect effect contracts, never inferred from the record. |
| process liveness/token check | `ProcessAliveSession` plus full pair comparison maps to `ExactLive`, `Dead`, `TokenMismatch`, or `Unknown`; only `ExactLive` authorizes exact owner actions. |
| teardown/reap result | `Proven` only when every required owned-boundary callback returns nil and its contract positively establishes absence; error, timeout, skipped work, or ambiguity is `Unknown`. |
| socket, PTY, stage paths, timestamps, names, recipe/image metadata | Out of model or normalized away and carried by concrete framing; frozen behavior tests remain authoritative. |

## Source-of-truth direction and graph conformance

The reviewed safety laws and frozen behavior characterization are the design
authority. The TLA+ relation and `internal/engine/session` Go reducer are
independently authored peer implementations; neither is generated from, parsed
into, or silently updated from the other. Production depends only on the Go
reducer. TLC exports the TLA+ reachable graph, Go independently enumerates its
reachable graph, and the conformance test compares normalized initial states,
states, and labelled directed edges in both directions.

This equality is over an explicit epistemic quotient, not every raw checker
field. Each TLC node must first contain all 22 raw fields with valid finite
domains. Normalization retains the 17 production-relation fields: lifecycle,
owners/detached, durable/runtime/pending authority and revisions,
recorded/runtime/inspected generations, health, mode, operation, effect, result,
and direction. It then drops only the five checker-only fields `evidence`,
`worlds`, `teardownProven`, `txOwner`, and `lastOwnerAction`. Raw-state TLC
invariants are checked before this quotient. Graph equality therefore does not
claim that Go stores those five epistemic witnesses, and it says nothing about
states outside the reviewed finite bounds.

## Pinned TLC DOT grammar

The parser accepts only the verified TLC v1.7.4 `-dump dot,actionlabels` shape:

- envelope `strict digraph DiskGraph`, `nodesep=0.35`, and one white
  `cluster_graph`, with no trailing LF;
- signed-decimal node IDs with a `label`; mechanically marked initial nodes may
  additionally carry `style = filled` in TLC's observed no-semicolon form;
- edges with exactly `label`, `color="black"`, and `fontcolor="black"`;
- `rank = same` rows with complete rank metadata;
- node-label escapes `\\n` and `\\"`, and only the finite TLA value grammar used
  here: records, sets, strings, integers, and booleans.

After validation, DOT node IDs, rank/layout metadata, colors, fonts, and the
consumed initial style are dropped. Records become lexicographically keyed JSON,
owner/grant sets become sorted arrays, and
`[authority |-> {}, revision |-> -1]` becomes one `NoGen` sentinel. Unknown
attributes, syntax, escapes, raw fields, action labels, duplicate definitions,
missing references, incomplete ranks, or absent initial markers fail closed.
Shortest graph witnesses are reported for one-sided states or edges.

## Expected-failure controls

Each mutant adds exactly one unsafe action to the positive relation. The harness
requires TLC to name both the expected invariant and the violating action anchor.

| cfg / constant | Enabling predicate | First violating action | Expected invariant |
|---|---|---|---|
| `RuntimeBeforeDurable` | reachable Running/Normal/Idle state with a grant available | `MutantRuntimeBeforeDurable` expands runtime without durable authority | `RuntimeAuthoritySubsetOfDurableAuthority` |
| `SecondOwner` | reachable Running state owned by OwnerA | `MutantSecondOwner` adds same-PID/different-token OwnerB | `AtMostOneLiveOwner` |
| `StaleToken` | reachable Running state owned by OwnerA | `MutantStaleToken` hands off on token-mismatch evidence | `ExactOwnerRequiredForHandoffReleaseSignal` |
| `UnknownAsOld` | reachable Widen commit boundary | `MutantUnknownAsOld` resumes normal from an unknown commit as old | `CommitUnknownNeverAssumedOld` |
| `AckWithoutInspect` | reachable Running/Normal/Idle state with a grant available | `MutantAckWithoutInspect` records a new generation without positive inspect | `NormalStateHasGenerationAgreement` |
| `StopWithoutProof` | reachable Running/Normal/Idle state | `MutantStopWithoutProof` claims Stopped without teardown evidence | `TeardownClaimRequiresProvenTeardown` |

## Counterexample and behavior-fix workflow

A positive-model failure or conformance witness is evidence to classify, not a
license to make one side match the other:

1. retain and inspect `.build/tla/session/model/**/tlc.log` and `states.dot`;
2. classify the result as checker/pin/parser issue, model defect, Go
   reducer/adapter defect, or disagreement with frozen production behavior;
3. follow the shortest labelled witness and translate its abstract fields through
   the concrete map above;
4. manually add a focused RED Go regression for the corresponding concrete
   boundary and reproduce it—production trace logging or generated tests are not
   used;
5. fix the responsible artifact, then rerun the focused test, all mutants,
   bidirectional graph conformance, and online/offline checker gates.

If characterization exposes a pre-existing behavior defect, stop. Amend the
approved design and land that behavior change as a separate RED→GREEN fix. Never
silently change public/legacy behavior, the model, or characterization merely to
make graph equality green.

## Assumptions and limitations

The model assumes the record lock serializes one transaction, commit outcomes are
classified as known-old/known-new/unknown, and apply/inspect/teardown callbacks
honor their documented contracts. Filesystem atomicity/durability, Docker and
Squid implementation, kernel process observations, process liveness, external
runtime tampering, and cryptographic hashes are not proved. A successful managed
container/network reap is the egress teardown proof boundary; signal delivery
alone is not proof.

Process tokens remain opaque and platform-specific. Exact-token laws cover only
records with a verified nonempty token. Tokenless legacy and unsupported-platform
compatibility paths remain explicitly outside handoff/release/signal theorems and
retain their characterized fail-closed limitations. The model cannot prevent an
external actor from mutating the runtime behind safeslop; recovery trusts only a
fresh exact generation/hash inspection or proven boundary teardown.

The checker proves safety only for the finite state space above. It makes no
fairness, progress, eventual-recovery, OS-liveness, CUE-policy, credential,
workspace/mount, public-JSON, or unbounded authority claim.
