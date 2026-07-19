---- MODULE Promotion ----
EXTENDS FiniteSets, TLC

CONSTANT Mutant

Grants == {"A", "B"}
Phases == {"Preview", "Prepared", "Applied", "Failed", "Uncertain"}
Statuses == {"Created", "Running", "Stopped"}
Commits == {"None", "Known", "Failed", "Unknown"}

VARIABLE p
vars == <<p>>

StateRecord(selection, sourceExact, grantsExact, policyExact, targetFree, valid, status) == [
  phase |-> "Preview",
  selection |-> selection,
  durable |-> {},
  sourceExact |-> sourceExact,
  grantsExact |-> grantsExact,
  policyExact |-> policyExact,
  targetFree |-> targetFree,
  valid |-> valid,
  status |-> status,
  commit |-> "None",
  trusted |-> FALSE,
  launched |-> FALSE
]

Init == p \in {StateRecord(sel, se, ge, pe, tf, va, st) :
  sel \in SUBSET Grants,
  se \in BOOLEAN,
  ge \in BOOLEAN,
  pe \in BOOLEAN,
  tf \in BOOLEAN,
  va \in BOOLEAN,
  st \in Statuses}

ReadyToReplace ==
  /\ p.phase = "Prepared"
  /\ p.sourceExact
  /\ p.grantsExact
  /\ p.policyExact
  /\ p.targetFree
  /\ p.valid
  /\ p.status \in {"Created", "Stopped"}

Prepare ==
  /\ p.phase = "Preview"
  /\ p' = [p EXCEPT !.phase = "Prepared"]

KnownCommit ==
  /\ ReadyToReplace
  /\ p' = [p EXCEPT !.phase = "Applied", !.durable = p.selection, !.commit = "Known"]

UnknownCommit ==
  /\ ReadyToReplace
  /\ p' = [p EXCEPT !.phase = "Uncertain", !.commit = "Unknown"]

KnownFailure ==
  /\ p.phase = "Prepared"
  /\ p' = [p EXCEPT !.phase = "Failed", !.commit = "Failed"]

MutantEmptySelection ==
  /\ Mutant = "EmptySelection"
  /\ p.phase = "Prepared"
  /\ p.selection = {}
  /\ p.sourceExact /\ p.grantsExact /\ p.policyExact /\ p.targetFree /\ p.valid
  /\ p.status \in {"Created", "Stopped"}
  /\ p' = [p EXCEPT !.phase = "Applied", !.durable = {"A"}, !.commit = "Known"]

MutantSourceSnapshot ==
  /\ Mutant = "SourceSnapshot"
  /\ p.phase = "Prepared"
  /\ ~p.sourceExact /\ p.grantsExact /\ p.policyExact /\ p.targetFree /\ p.valid
  /\ p.status \in {"Created", "Stopped"}
  /\ p' = [p EXCEPT !.phase = "Applied", !.durable = p.selection, !.commit = "Known"]

MutantGrantRevision ==
  /\ Mutant = "GrantRevision"
  /\ p.phase = "Prepared"
  /\ p.sourceExact /\ ~p.grantsExact /\ p.policyExact /\ p.targetFree /\ p.valid
  /\ p.status \in {"Created", "Stopped"}
  /\ p' = [p EXCEPT !.phase = "Applied", !.durable = p.selection, !.commit = "Known"]

MutantPolicyHash ==
  /\ Mutant = "PolicyHash"
  /\ p.phase = "Prepared"
  /\ p.sourceExact /\ p.grantsExact /\ ~p.policyExact /\ p.targetFree /\ p.valid
  /\ p.status \in {"Created", "Stopped"}
  /\ p' = [p EXCEPT !.phase = "Applied", !.durable = p.selection, !.commit = "Known"]

MutantCreateOnly ==
  /\ Mutant = "CreateOnly"
  /\ p.phase = "Prepared"
  /\ p.sourceExact /\ p.grantsExact /\ p.policyExact /\ ~p.targetFree /\ p.valid
  /\ p.status \in {"Created", "Stopped"}
  /\ p' = [p EXCEPT !.phase = "Applied", !.durable = p.selection, !.commit = "Known"]

MutantValidation ==
  /\ Mutant = "Validation"
  /\ p.phase = "Prepared"
  /\ p.sourceExact /\ p.grantsExact /\ p.policyExact /\ p.targetFree /\ ~p.valid
  /\ p.status \in {"Created", "Stopped"}
  /\ p' = [p EXCEPT !.phase = "Applied", !.durable = p.selection, !.commit = "Known"]

MutantReleaseSource ==
  /\ Mutant = "ReleaseSource"
  /\ p.phase = "Prepared"
  /\ ReadyToReplace
  /\ p' = [p EXCEPT !.sourceExact = FALSE, !.phase = "Applied", !.durable = p.selection, !.commit = "Known"]

MutantUnknownSuccess ==
  /\ Mutant = "UnknownSuccess"
  /\ p.phase = "Prepared"
  /\ ReadyToReplace
  /\ p' = [p EXCEPT !.phase = "Applied", !.durable = p.selection, !.commit = "Unknown"]

TerminalStutter ==
  /\ p.phase \in {"Applied", "Failed", "Uncertain"}
  /\ UNCHANGED p

Next == Prepare \/ KnownCommit \/ UnknownCommit \/ KnownFailure \/ TerminalStutter \/
        MutantEmptySelection \/ MutantSourceSnapshot \/ MutantGrantRevision \/ MutantPolicyHash \/
        MutantCreateOnly \/ MutantValidation \/ MutantReleaseSource \/ MutantUnknownSuccess

TypeOK ==
  /\ p.phase \in Phases
  /\ p.selection \subseteq Grants
  /\ p.durable \subseteq Grants
  /\ p.sourceExact \in BOOLEAN
  /\ p.grantsExact \in BOOLEAN
  /\ p.policyExact \in BOOLEAN
  /\ p.targetFree \in BOOLEAN
  /\ p.valid \in BOOLEAN
  /\ p.status \in Statuses
  /\ p.commit \in Commits
  /\ p.trusted = FALSE
  /\ p.launched = FALSE

EmptySelectionNoDurable == p.selection # {} \/ p.durable = {}
DurableExactlySelected == p.phase # "Applied" \/ p.durable = p.selection
ReplacementRequiresMatchingSnapshots == p.phase # "Applied" \/ (p.sourceExact /\ p.grantsExact /\ p.policyExact)
CreateOnly == p.phase # "Applied" \/ p.targetFree
ValidationBeforeReplacement == p.phase # "Applied" \/ p.valid
AppliedSuccessRequiresKnownCommit == p.phase # "Applied" \/ p.commit = "Known"
NoTrustOrLaunch == /\ ~p.trusted /\ ~p.launched
====
