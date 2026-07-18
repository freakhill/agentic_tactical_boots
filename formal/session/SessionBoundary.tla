---- MODULE SessionBoundary ----
EXTENDS Integers, FiniteSets, TLC

CONSTANTS Mutant, MaxRevision

Owners == {"OwnerA", "OwnerB"}
Grants == {"GrantA", "GrantB"}
NoOwner == "NoOwner"
NoGen == [authority |-> {}, revision |-> -1]
NoDirection == "NoDirection"
WorldOld == "Old"
WorldNew == "New"

Generation == [authority : SUBSET Grants, revision : -1..MaxRevision]
Gen(authority, revision) == [authority |-> authority, revision |-> revision]

VARIABLE s
vars == <<s>>

BaseState == [
  status |-> "Created",
  owners |-> {},
  detached |-> FALSE,
  durableAuthority |-> {},
  durableRevision |-> 0,
  recordedGen |-> Gen({}, 0),
  runtimeAuthority |-> {},
  runtimeGen |-> Gen({}, 0),
  inspectedGen |-> Gen({}, 0),
  health |-> "Healthy",
  mode |-> "Normal",
  operation |-> "Idle",
  effect |-> "None",
  evidence |-> "None",
  result |-> "Pending",
  pendingAuthority |-> {},
  pendingRevision |-> 0,
  direction |-> NoDirection,
  worlds |-> {},
  teardownProven |-> FALSE,
  txOwner |-> NoOwner,
  lastOwnerAction |-> "None"
]

Init == s = BaseState

ResetPending(rec) == [rec EXCEPT
  !.operation = "Idle",
  !.effect = "None",
  !.pendingAuthority = {},
  !.pendingRevision = 0,
  !.direction = NoDirection,
  !.worlds = {},
  !.txOwner = NoOwner
]

ClaimStart ==
  /\ s.status = "Created"
  /\ s.mode = "Normal"
  /\ s.operation = "Idle"
  /\ s' = [s EXCEPT
      !.operation = "Claim",
      !.effect = "Commit",
      !.result = "Pending",
      !.txOwner = "OwnerA",
      !.evidence = "None"]

ClaimCommitNew ==
  /\ s.operation = "Claim"
  /\ s.effect = "Commit"
  /\ s' = ResetPending([s EXCEPT
      !.status = "Running",
      !.owners = {"OwnerA"},
      !.detached = FALSE,
      !.result = "Success",
      !.evidence = "CommitNew"])

ClaimCommitOld ==
  /\ s.operation = "Claim"
  /\ s.effect = "Commit"
  /\ s' = ResetPending([s EXCEPT
      !.result = "Failure",
      !.evidence = "CommitOld"])

ClaimCommitUnknown ==
  /\ s.operation = "Claim"
  /\ s.effect = "Commit"
  /\ s' \in {
      [s EXCEPT
        !.health = "Uncertain", !.mode = "Blocked", !.operation = "Recover",
        !.effect = "Inspect", !.evidence = "CommitUnknown", !.result = "Uncertain",
        !.worlds = {WorldOld, WorldNew}],
      [s EXCEPT
        !.status = "Running", !.owners = {"OwnerA"}, !.health = "Uncertain",
        !.mode = "Blocked", !.operation = "Recover", !.effect = "Inspect",
        !.evidence = "CommitUnknown", !.result = "Uncertain",
        !.worlds = {WorldOld, WorldNew}]
    }

RecoverClaim ==
  /\ s.operation = "Recover"
  /\ s.direction = NoDirection
  /\ s.effect = "Inspect"
  /\ s' = ResetPending([s EXCEPT
      !.health = "Healthy",
      !.mode = "Normal",
      !.evidence = IF s.status = "Running" THEN "CommitNew" ELSE "CommitOld",
      !.result = IF s.status = "Running" THEN "Success" ELSE "Failure"])

HandoffExact ==
  /\ s.status = "Running"
  /\ s.mode = "Normal"
  /\ s.operation = "Idle"
  /\ s.owners = {"OwnerA"}
  /\ s' = [s EXCEPT
      !.owners = {"OwnerB"},
      !.detached = TRUE,
      !.evidence = "ExactLiveA",
      !.lastOwnerAction = "Handoff"]

ReleaseExact ==
  /\ s.status = "Running"
  /\ s.mode = "Normal"
  /\ s.operation = "Idle"
  /\ s.owners = {"OwnerA"}
  /\ s' = [s EXCEPT
      !.status = "Created",
      !.owners = {},
      !.detached = FALSE,
      !.evidence = "ExactLiveA",
      !.result = "Failure",
      !.lastOwnerAction = "Release"]

WidenStart ==
  /\ s.status = "Running"
  /\ s.mode = "Normal"
  /\ s.operation = "Idle"
  /\ s.durableRevision < MaxRevision
  /\ Grants \ s.durableAuthority # {}
  /\ \E grant \in Grants \ s.durableAuthority:
      s' = [s EXCEPT
        !.operation = "Widen",
        !.effect = "Commit",
        !.pendingAuthority = s.durableAuthority \cup {grant},
        !.pendingRevision = s.durableRevision + 1,
        !.direction = "Widen",
        !.result = "Pending",
        !.txOwner = CHOOSE owner \in s.owners : TRUE,
        !.evidence = "None"]

WidenCommitNew ==
  /\ s.operation = "Widen"
  /\ s.effect = "Commit"
  /\ s.evidence = "None"
  /\ s' = [s EXCEPT
      !.durableAuthority = s.pendingAuthority,
      !.durableRevision = s.pendingRevision,
      !.effect = "Apply",
      !.evidence = "CommitNew",
      !.worlds = {WorldNew}]

WidenCommitOld ==
  /\ s.operation = "Widen"
  /\ s.effect = "Commit"
  /\ s.evidence = "None"
  /\ s' = ResetPending([s EXCEPT
      !.result = "Failure",
      !.evidence = "CommitOld"])

WidenCommitUnknown ==
  /\ s.operation = "Widen"
  /\ s.effect = "Commit"
  /\ s.evidence = "None"
  /\ s' \in {
      [s EXCEPT
        !.health = "Uncertain", !.mode = "Blocked", !.operation = "Recover",
        !.effect = "Inspect", !.evidence = "CommitUnknown", !.result = "Uncertain",
        !.worlds = {WorldOld, WorldNew}],
      [s EXCEPT
        !.durableAuthority = s.pendingAuthority, !.durableRevision = s.pendingRevision,
        !.health = "Uncertain", !.mode = "Blocked", !.operation = "Recover",
        !.effect = "Inspect", !.evidence = "CommitUnknown", !.result = "Uncertain",
        !.worlds = {WorldOld, WorldNew}]
    }

WidenApply ==
  /\ s.direction = "Widen"
  /\ s.operation \in {"Widen", "Recover"}
  /\ s.effect = "Apply"
  /\ s.durableAuthority = s.pendingAuthority
  /\ s.durableRevision = s.pendingRevision
  /\ s' = [s EXCEPT
      !.runtimeAuthority = s.pendingAuthority,
      !.runtimeGen = Gen(s.pendingAuthority, s.pendingRevision),
      !.effect = "Inspect",
      !.evidence = "ApplySucceeded"]

PositiveInspect ==
  /\ s.operation \in {"Widen", "Narrow"}
  /\ s.effect = "Inspect"
  /\ s.runtimeGen = Gen(s.pendingAuthority, s.pendingRevision)
  /\ s' = [s EXCEPT
      !.inspectedGen = s.runtimeGen,
      !.effect = "Commit",
      !.evidence = "InspectMatch"]

WidenFinalCommitNew ==
  /\ s.operation = "Widen"
  /\ s.effect = "Commit"
  /\ s.evidence = "InspectMatch"
  /\ s' = ResetPending([s EXCEPT
      !.recordedGen = Gen(s.pendingAuthority, s.pendingRevision),
      !.health = "Healthy",
      !.mode = "Normal",
      !.result = "Success",
      !.evidence = "CommitNew"])

WidenFinalCommitOld ==
  /\ s.operation = "Widen"
  /\ s.effect = "Commit"
  /\ s.evidence = "InspectMatch"
  /\ s' = [s EXCEPT
      !.health = "Uncertain", !.mode = "Blocked", !.operation = "Recover",
      !.effect = "Inspect", !.evidence = "CommitOld", !.result = "Failure",
      !.worlds = {WorldOld}]

WidenFinalCommitUnknown ==
  /\ s.operation = "Widen"
  /\ s.effect = "Commit"
  /\ s.evidence = "InspectMatch"
  /\ s' \in {
      [s EXCEPT
        !.health = "Uncertain", !.mode = "Blocked", !.operation = "Recover",
        !.effect = "Inspect", !.evidence = "CommitUnknown", !.result = "Uncertain",
        !.worlds = {WorldOld, WorldNew}],
      [s EXCEPT
        !.recordedGen = Gen(s.pendingAuthority, s.pendingRevision),
        !.health = "Uncertain", !.mode = "Blocked", !.operation = "Recover",
        !.effect = "Inspect", !.evidence = "CommitUnknown", !.result = "Uncertain",
        !.worlds = {WorldOld, WorldNew}]
    }

NarrowStart ==
  /\ s.status = "Running"
  /\ s.mode = "Normal"
  /\ s.operation = "Idle"
  /\ s.durableRevision < MaxRevision
  /\ s.durableAuthority # {}
  /\ \E grant \in s.durableAuthority:
      s' = [s EXCEPT
        !.operation = "Narrow",
        !.effect = "Apply",
        !.pendingAuthority = @ \ {grant},
        !.pendingRevision = s.durableRevision + 1,
        !.direction = "Narrow",
        !.result = "Pending",
        !.txOwner = CHOOSE owner \in s.owners : TRUE,
        !.evidence = "CommitNew"]

NarrowApply ==
  /\ s.operation = "Narrow"
  /\ s.effect = "Apply"
  /\ s' = [s EXCEPT
      !.runtimeAuthority = s.pendingAuthority,
      !.runtimeGen = Gen(s.pendingAuthority, s.pendingRevision),
      !.effect = "Inspect",
      !.evidence = "ApplySucceeded"]

NarrowFinalCommitNew ==
  /\ s.operation = "Narrow"
  /\ s.effect = "Commit"
  /\ s.evidence = "InspectMatch"
  /\ s' = ResetPending([s EXCEPT
      !.durableAuthority = s.pendingAuthority,
      !.durableRevision = s.pendingRevision,
      !.recordedGen = Gen(s.pendingAuthority, s.pendingRevision),
      !.health = "Healthy",
      !.mode = "Normal",
      !.result = "Success",
      !.evidence = "CommitNew"])

NarrowFinalCommitOld ==
  /\ s.operation = "Narrow"
  /\ s.effect = "Commit"
  /\ s.evidence = "InspectMatch"
  /\ s' = ResetPending([s EXCEPT
      !.runtimeAuthority = s.durableAuthority,
      !.runtimeGen = Gen(s.durableAuthority, s.durableRevision),
      !.inspectedGen = Gen(s.durableAuthority, s.durableRevision),
      !.result = "Failure",
      !.evidence = "CommitOld"])

NarrowFinalCommitUnknown ==
  /\ s.operation = "Narrow"
  /\ s.effect = "Commit"
  /\ s.evidence = "InspectMatch"
  /\ s' \in {
      [s EXCEPT
        !.health = "Uncertain", !.mode = "Blocked", !.operation = "Recover",
        !.effect = "Inspect", !.evidence = "CommitUnknown", !.result = "Uncertain",
        !.worlds = {WorldOld, WorldNew}],
      [s EXCEPT
        !.durableAuthority = s.pendingAuthority, !.durableRevision = s.pendingRevision,
        !.recordedGen = Gen(s.pendingAuthority, s.pendingRevision),
        !.health = "Uncertain", !.mode = "Blocked", !.operation = "Recover",
        !.effect = "Inspect", !.evidence = "CommitUnknown", !.result = "Uncertain",
        !.worlds = {WorldOld, WorldNew}]
    }

RecoverEgress ==
  /\ s.operation = "Recover"
  /\ s.direction \in {"Widen", "Narrow"}
  /\ s.effect = "Inspect"
  /\ IF s.durableAuthority = s.pendingAuthority /\ s.durableRevision = s.pendingRevision
      THEN s' = ResetPending([s EXCEPT
        !.runtimeAuthority = s.durableAuthority,
        !.runtimeGen = Gen(s.durableAuthority, s.durableRevision),
        !.inspectedGen = Gen(s.durableAuthority, s.durableRevision),
        !.recordedGen = Gen(s.durableAuthority, s.durableRevision),
        !.health = "Healthy", !.mode = "Normal", !.result = "Success",
        !.evidence = "InspectMatch"])
      ELSE s' = ResetPending([s EXCEPT
        !.runtimeAuthority = s.durableAuthority,
        !.runtimeGen = Gen(s.durableAuthority, s.durableRevision),
        !.inspectedGen = Gen(s.durableAuthority, s.durableRevision),
        !.recordedGen = Gen(s.durableAuthority, s.durableRevision),
        !.health = "Healthy", !.mode = "Normal", !.result = "Failure",
        !.evidence = "InspectMatch"])

ObserveInvalidPersistence ==
  /\ s.status = "Created"
  /\ s.mode = "Normal"
  /\ s.operation = "Idle"
  /\ s' \in {
      [s EXCEPT !.health = "Corrupt", !.mode = "Blocked", !.operation = "Recover", !.result = "Failure"],
      [s EXCEPT !.health = "Stale", !.mode = "Blocked", !.operation = "Recover", !.result = "Failure"]
    }

StopStart ==
  /\ s.status = "Running"
  /\ s.mode = "Normal"
  /\ s.operation = "Idle"
  /\ s' = [s EXCEPT
      !.operation = "Stop",
      !.effect = "Teardown",
      !.txOwner = CHOOSE owner \in s.owners : TRUE,
      !.evidence = IF "OwnerA" \in s.owners THEN "ExactLiveA" ELSE "ExactLiveB",
      !.lastOwnerAction = "Signal",
      !.result = "Pending"]

RequestTeardown ==
  /\ s.mode = "Blocked"
  /\ s.effect # "Teardown"
  /\ s' = [s EXCEPT
      !.operation = "Teardown",
      !.effect = "Teardown",
      !.result = "Pending"]

TeardownProven ==
  /\ s.effect = "Teardown"
  /\ s.operation \in {"Stop", "Teardown"}
  /\ s' = [s EXCEPT
      !.status = "Stopped",
      !.owners = {},
      !.detached = FALSE,
      !.runtimeAuthority = {},
      !.runtimeGen = NoGen,
      !.inspectedGen = NoGen,
      !.health = "Healthy",
      !.mode = "Terminal",
      !.operation = "Idle",
      !.effect = "None",
      !.evidence = "TeardownProven",
      !.result = "Failure",
      !.pendingAuthority = {},
      !.pendingRevision = 0,
      !.direction = NoDirection,
      !.worlds = {},
      !.teardownProven = TRUE,
      !.txOwner = NoOwner]

TeardownUnknown ==
  /\ s.effect = "Teardown"
  /\ s.operation \in {"Stop", "Teardown"}
  /\ s' = [s EXCEPT
      !.health = "Uncertain",
      !.mode = "Blocked",
      !.operation = "Teardown",
      !.effect = "Teardown",
      !.evidence = "TeardownUnknown",
      !.result = "Uncertain"]

TerminalStutter ==
  /\ s.status = "Stopped"
  /\ UNCHANGED s

MutantRuntimeBeforeDurable ==
  /\ Mutant = "RuntimeBeforeDurable"
  /\ s.status = "Running" /\ s.mode = "Normal" /\ s.operation = "Idle"
  /\ Grants \ s.durableAuthority # {}
  /\ \E grant \in Grants \ s.durableAuthority:
      s' = [s EXCEPT
        !.runtimeAuthority = @ \cup {grant},
        !.runtimeGen = Gen(s.runtimeAuthority \cup {grant}, s.durableRevision),
        !.operation = "Widen", !.effect = "Apply"]

MutantSecondOwner ==
  /\ Mutant = "SecondOwner"
  /\ s.status = "Running" /\ s.owners = {"OwnerA"}
  /\ s' = [s EXCEPT !.owners = Owners]

MutantStaleToken ==
  /\ Mutant = "StaleToken"
  /\ s.status = "Running" /\ s.owners = {"OwnerA"}
  /\ s' = [s EXCEPT
      !.owners = {"OwnerB"}, !.detached = TRUE,
      !.evidence = "TokenMismatch", !.lastOwnerAction = "Handoff"]

MutantUnknownAsOld ==
  /\ Mutant = "UnknownAsOld"
  /\ s.operation = "Widen" /\ s.effect = "Commit"
  /\ s' = ResetPending([s EXCEPT
      !.health = "Healthy", !.mode = "Normal", !.evidence = "CommitUnknown",
      !.worlds = {WorldOld}, !.result = "Failure"])

MutantAckWithoutInspect ==
  /\ Mutant = "AckWithoutInspect"
  /\ s.status = "Running" /\ s.mode = "Normal" /\ s.operation = "Idle"
  /\ s.durableRevision < MaxRevision
  /\ Grants \ s.durableAuthority # {}
  /\ \E grant \in Grants \ s.durableAuthority:
      LET authority == s.durableAuthority \cup {grant}
          revision == s.durableRevision + 1
      IN s' = [s EXCEPT
          !.durableAuthority = authority, !.durableRevision = revision,
          !.runtimeAuthority = authority, !.runtimeGen = Gen(authority, revision),
          !.recordedGen = Gen(authority, revision), !.result = "Success"]

MutantStopWithoutProof ==
  /\ Mutant = "StopWithoutProof"
  /\ s.status = "Running" /\ s.mode = "Normal" /\ s.operation = "Idle"
  /\ s' = [s EXCEPT
      !.status = "Stopped", !.owners = {}, !.detached = FALSE,
      !.runtimeAuthority = {}, !.runtimeGen = NoGen, !.inspectedGen = NoGen,
      !.mode = "Terminal", !.result = "Failure", !.teardownProven = FALSE]

SafeNext ==
  \/ ClaimStart \/ ClaimCommitNew \/ ClaimCommitOld \/ ClaimCommitUnknown \/ RecoverClaim
  \/ HandoffExact \/ ReleaseExact
  \/ WidenStart \/ WidenCommitNew \/ WidenCommitOld \/ WidenCommitUnknown
  \/ WidenApply \/ PositiveInspect \/ WidenFinalCommitNew \/ WidenFinalCommitOld \/ WidenFinalCommitUnknown
  \/ NarrowStart \/ NarrowApply \/ NarrowFinalCommitNew \/ NarrowFinalCommitOld \/ NarrowFinalCommitUnknown
  \/ RecoverEgress \/ ObserveInvalidPersistence \/ StopStart \/ RequestTeardown
  \/ TeardownProven \/ TeardownUnknown \/ TerminalStutter

Next == SafeNext
  \/ MutantRuntimeBeforeDurable \/ MutantSecondOwner \/ MutantStaleToken
  \/ MutantUnknownAsOld \/ MutantAckWithoutInspect \/ MutantStopWithoutProof

TypeOK ==
  /\ s.status \in {"Created", "Running", "Stopped"}
  /\ s.owners \in SUBSET Owners
  /\ s.detached \in BOOLEAN
  /\ s.durableAuthority \in SUBSET Grants
  /\ s.durableRevision \in 0..MaxRevision
  /\ s.recordedGen \in Generation
  /\ s.runtimeAuthority \in SUBSET Grants
  /\ s.runtimeGen \in Generation
  /\ s.inspectedGen \in Generation
  /\ s.health \in {"Healthy", "Uncertain", "Corrupt", "Stale"}
  /\ s.mode \in {"Normal", "Blocked", "Terminal"}
  /\ s.operation \in {"Idle", "Claim", "Widen", "Narrow", "Recover", "Stop", "Teardown"}
  /\ s.effect \in {"None", "Commit", "Apply", "Inspect", "Teardown"}
  /\ s.evidence \in {"None", "CommitOld", "CommitNew", "CommitUnknown", "ApplySucceeded",
                      "InspectMatch", "ExactLiveA", "ExactLiveB", "TokenMismatch",
                      "TeardownProven", "TeardownUnknown"}
  /\ s.result \in {"Pending", "Success", "Failure", "Uncertain"}
  /\ s.pendingAuthority \in SUBSET Grants
  /\ s.pendingRevision \in 0..MaxRevision
  /\ s.direction \in {NoDirection, "Widen", "Narrow"}
  /\ s.worlds \in SUBSET {WorldOld, WorldNew}
  /\ s.teardownProven \in BOOLEAN
  /\ s.txOwner \in Owners \cup {NoOwner}
  /\ s.lastOwnerAction \in {"None", "Handoff", "Release", "Signal"}

ValidRecordShape ==
  /\ (s.status = "Created" => s.owners = {})
  /\ (s.status = "Running" => s.owners # {})
  /\ (s.status = "Stopped" => s.owners = {})

AtMostOneLiveOwner == Cardinality(s.owners) <= 1

ExactOwnerRequiredForHandoffReleaseSignal ==
  /\ (s.lastOwnerAction = "Handoff" /\ s.evidence \in {"ExactLiveA", "ExactLiveB", "TokenMismatch"} => s.evidence = "ExactLiveA")
  /\ (s.lastOwnerAction = "Release" /\ s.evidence \in {"ExactLiveA", "ExactLiveB", "TokenMismatch"} => s.evidence = "ExactLiveA")
  /\ (s.lastOwnerAction = "Signal" /\ s.evidence \in {"ExactLiveA", "ExactLiveB", "TokenMismatch"} => s.evidence \in {"ExactLiveA", "ExactLiveB"})

RuntimeAuthoritySubsetOfDurableAuthority == s.runtimeAuthority \subseteq s.durableAuthority

NormalStateHasGenerationAgreement ==
  (s.status = "Running" /\ s.mode = "Normal" /\ s.operation = "Idle") =>
    /\ s.recordedGen = Gen(s.durableAuthority, s.durableRevision)
    /\ s.runtimeAuthority = s.durableAuthority
    /\ s.runtimeGen = s.recordedGen
    /\ s.inspectedGen = s.recordedGen

UncertaintyBlocksNormalAuthorityMutation ==
  s.health = "Uncertain" =>
    /\ s.mode = "Blocked"
    /\ s.operation \in {"Recover", "Teardown"}

TeardownClaimRequiresProvenTeardown == s.status = "Stopped" => s.teardownProven

StoppedHasNoOwnerAndNoEffectiveRuntimeAuthority ==
  s.status = "Stopped" => s.owners = {} /\ s.runtimeAuthority = {}

CommitUnknownNeverAssumedOld ==
  s.evidence = "CommitUnknown" =>
    /\ s.worlds = {WorldOld, WorldNew}
    /\ s.mode = "Blocked"

InvalidPersistenceCannotSucceed ==
  s.health \in {"Corrupt", "Stale"} => s.result # "Success"

TerminalAwareDeadlock == s.status = "Stopped" \/ ENABLED Next

Spec == Init /\ [][Next]_vars

====
