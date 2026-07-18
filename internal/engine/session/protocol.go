package session

import (
	"fmt"
	"time"

	"github.com/freakhill/safeslop/internal/engine/egress"
)

// Protocol values are a closed, effect-free representation of the bounded
// session safety relation. Drivers classify external work into ProtocolOutcome.
type ProtocolStatus string
type ProtocolMode string
type ProtocolHealth string
type ProtocolOperation string
type ProtocolEffect string
type ProtocolResult string
type ProtocolAction string
type ProtocolOutcome string
type ProtocolTruth string
type ProtocolDirection string
type ProtocolLabel string
type ProtocolOwner uint8
type ProtocolGrant uint8
type ProtocolGrantSet uint8

// ProtocolBounds selects the finite relation explored by EnabledProtocolWithin.
// Zero values select the reviewed model bounds.
type ProtocolBounds struct {
	MaxRevision int
	Grants      []ProtocolGrant
}

const (
	ProtocolCreated ProtocolStatus = "Created"
	ProtocolRunning ProtocolStatus = "Running"
	ProtocolStopped ProtocolStatus = "Stopped"

	ProtocolNormal   ProtocolMode = "Normal"
	ProtocolBlocked  ProtocolMode = "Blocked"
	ProtocolTerminal ProtocolMode = "Terminal"

	ProtocolHealthy   ProtocolHealth = "Healthy"
	ProtocolUncertain ProtocolHealth = "Uncertain"
	ProtocolCorrupt   ProtocolHealth = "Corrupt"
	ProtocolStale     ProtocolHealth = "Stale"

	ProtocolIdle     ProtocolOperation = "Idle"
	ProtocolClaim    ProtocolOperation = "Claim"
	ProtocolWiden    ProtocolOperation = "Widen"
	ProtocolNarrow   ProtocolOperation = "Narrow"
	ProtocolRecover  ProtocolOperation = "Recover"
	ProtocolStop     ProtocolOperation = "Stop"
	ProtocolTeardown ProtocolOperation = "Teardown"

	ProtocolNoEffect       ProtocolEffect = "None"
	ProtocolCommit         ProtocolEffect = "Commit"
	ProtocolApply          ProtocolEffect = "Apply"
	ProtocolInspect        ProtocolEffect = "Inspect"
	ProtocolTeardownEffect ProtocolEffect = "Teardown"

	ProtocolPending         ProtocolResult = "Pending"
	ProtocolSuccess         ProtocolResult = "Success"
	ProtocolFailure         ProtocolResult = "Failure"
	ProtocolUncertainResult ProtocolResult = "Uncertain"

	ProtocolKnownOld        ProtocolOutcome = "KnownOld"
	ProtocolKnownNew        ProtocolOutcome = "KnownNew"
	ProtocolCommitUnknown   ProtocolOutcome = "CommitUnknown"
	ProtocolSucceeded       ProtocolOutcome = "Succeeded"
	ProtocolMatched         ProtocolOutcome = "Matched"
	ProtocolObservedCorrupt ProtocolOutcome = "Corrupt"
	ProtocolObservedStale   ProtocolOutcome = "Stale"
	ProtocolTeardownProven  ProtocolOutcome = "Proven"
	ProtocolTeardownUnknown ProtocolOutcome = "Unknown"

	ProtocolOld ProtocolTruth = "Old"
	ProtocolNew ProtocolTruth = "New"

	ProtocolNoDirection     ProtocolDirection = "NoDirection"
	ProtocolWidenDirection  ProtocolDirection = "Widen"
	ProtocolNarrowDirection ProtocolDirection = "Narrow"

	ProtocolOwnerA ProtocolOwner = 1
	ProtocolOwnerB ProtocolOwner = 2
	ProtocolGrantA ProtocolGrant = 1
	ProtocolGrantB ProtocolGrant = 2
)

const (
	ProtocolClaimStart                ProtocolAction = "ClaimStart"
	ProtocolClaimCommit               ProtocolAction = "ClaimCommit"
	ProtocolRecoverClaim              ProtocolAction = "RecoverClaim"
	ProtocolHandoffExact              ProtocolAction = "HandoffExact"
	ProtocolReleaseExact              ProtocolAction = "ReleaseExact"
	ProtocolWidenStart                ProtocolAction = "WidenStart"
	ProtocolWidenCommit               ProtocolAction = "WidenCommit"
	ProtocolApplyAction               ProtocolAction = "Apply"
	ProtocolInspectAction             ProtocolAction = "Inspect"
	ProtocolWidenFinalCommit          ProtocolAction = "WidenFinalCommit"
	ProtocolNarrowStart               ProtocolAction = "NarrowStart"
	ProtocolNarrowFinalCommit         ProtocolAction = "NarrowFinalCommit"
	ProtocolRecoverEgress             ProtocolAction = "RecoverEgress"
	ProtocolObserveInvalidPersistence ProtocolAction = "ObserveInvalidPersistence"
	ProtocolStopStart                 ProtocolAction = "StopStart"
	ProtocolRequestTeardown           ProtocolAction = "RequestTeardown"
	ProtocolTeardownAction            ProtocolAction = "Teardown"
	ProtocolTerminalStutter           ProtocolAction = "TerminalStutter"
)

const (
	ProtocolLabelClaimStart                ProtocolLabel = "ClaimStart"
	ProtocolLabelClaimCommitNew            ProtocolLabel = "ClaimCommitNew"
	ProtocolLabelClaimCommitOld            ProtocolLabel = "ClaimCommitOld"
	ProtocolLabelClaimCommitUnknown        ProtocolLabel = "ClaimCommitUnknown"
	ProtocolLabelRecoverClaim              ProtocolLabel = "RecoverClaim"
	ProtocolLabelHandoffExact              ProtocolLabel = "HandoffExact"
	ProtocolLabelReleaseExact              ProtocolLabel = "ReleaseExact"
	ProtocolLabelWidenStart                ProtocolLabel = "WidenStart"
	ProtocolLabelWidenCommitNew            ProtocolLabel = "WidenCommitNew"
	ProtocolLabelWidenCommitOld            ProtocolLabel = "WidenCommitOld"
	ProtocolLabelWidenCommitUnknown        ProtocolLabel = "WidenCommitUnknown"
	ProtocolLabelWidenApply                ProtocolLabel = "WidenApply"
	ProtocolLabelPositiveInspect           ProtocolLabel = "PositiveInspect"
	ProtocolLabelWidenFinalCommitNew       ProtocolLabel = "WidenFinalCommitNew"
	ProtocolLabelWidenFinalCommitOld       ProtocolLabel = "WidenFinalCommitOld"
	ProtocolLabelWidenFinalCommitUnknown   ProtocolLabel = "WidenFinalCommitUnknown"
	ProtocolLabelNarrowStart               ProtocolLabel = "NarrowStart"
	ProtocolLabelNarrowApply               ProtocolLabel = "NarrowApply"
	ProtocolLabelNarrowFinalCommitNew      ProtocolLabel = "NarrowFinalCommitNew"
	ProtocolLabelNarrowFinalCommitOld      ProtocolLabel = "NarrowFinalCommitOld"
	ProtocolLabelNarrowFinalCommitUnknown  ProtocolLabel = "NarrowFinalCommitUnknown"
	ProtocolLabelRecoverEgress             ProtocolLabel = "RecoverEgress"
	ProtocolLabelObserveInvalidPersistence ProtocolLabel = "ObserveInvalidPersistence"
	ProtocolLabelStopStart                 ProtocolLabel = "StopStart"
	ProtocolLabelRequestTeardown           ProtocolLabel = "RequestTeardown"
	ProtocolLabelTeardownProven            ProtocolLabel = "TeardownProven"
	ProtocolLabelTeardownUnknown           ProtocolLabel = "TeardownUnknown"
	ProtocolLabelTerminalStutter           ProtocolLabel = "TerminalStutter"
)

var protocolActions = []ProtocolAction{
	ProtocolClaimStart,
	ProtocolClaimCommit,
	ProtocolRecoverClaim,
	ProtocolHandoffExact,
	ProtocolReleaseExact,
	ProtocolWidenStart,
	ProtocolWidenCommit,
	ProtocolApplyAction,
	ProtocolInspectAction,
	ProtocolWidenFinalCommit,
	ProtocolNarrowStart,
	ProtocolNarrowFinalCommit,
	ProtocolRecoverEgress,
	ProtocolObserveInvalidPersistence,
	ProtocolStopStart,
	ProtocolRequestTeardown,
	ProtocolTeardownAction,
	ProtocolTerminalStutter,
}

var protocolOutcomes = []ProtocolOutcome{
	"",
	ProtocolKnownOld,
	ProtocolKnownNew,
	ProtocolCommitUnknown,
	ProtocolSucceeded,
	ProtocolMatched,
	ProtocolObservedCorrupt,
	ProtocolObservedStale,
	ProtocolTeardownProven,
	ProtocolTeardownUnknown,
}

var protocolTruths = []ProtocolTruth{"", ProtocolOld, ProtocolNew}

// ProtocolGeneration is symbolic: equality means the exact authority and
// revision agree. It does not claim anything about a concrete SHA-256 digest.
type ProtocolGeneration struct {
	Authority ProtocolGrantSet `json:"authority"`
	Revision  int              `json:"revision"`
	Valid     bool             `json:"valid"`
}

type ProtocolState struct {
	Status              ProtocolStatus     `json:"status"`
	Owners              ProtocolOwner      `json:"owners"`
	Detached            bool               `json:"detached"`
	DurableAuthority    ProtocolGrantSet   `json:"durable_authority"`
	DurableRevision     int                `json:"durable_revision"`
	RecordedGeneration  ProtocolGeneration `json:"recorded_generation"`
	RuntimeAuthority    ProtocolGrantSet   `json:"runtime_authority"`
	RuntimeGeneration   ProtocolGeneration `json:"runtime_generation"`
	InspectedGeneration ProtocolGeneration `json:"inspected_generation"`
	Health              ProtocolHealth     `json:"health"`
	Mode                ProtocolMode       `json:"mode"`
	Operation           ProtocolOperation  `json:"operation"`
	Effect              ProtocolEffect     `json:"effect"`
	Result              ProtocolResult     `json:"result"`
	PendingAuthority    ProtocolGrantSet   `json:"pending_authority"`
	PendingRevision     int                `json:"pending_revision"`
	Direction           ProtocolDirection  `json:"direction"`
}

type ProtocolEvent struct {
	Action  ProtocolAction  `json:"action"`
	Outcome ProtocolOutcome `json:"outcome,omitempty"`
	Grant   ProtocolGrant   `json:"grant,omitempty"`
	Truth   ProtocolTruth   `json:"truth,omitempty"`
}

func protocolGeneration(authority ProtocolGrantSet, revision int) ProtocolGeneration {
	return ProtocolGeneration{Authority: authority, Revision: revision, Valid: true}
}

func protocolNoGeneration() ProtocolGeneration {
	return ProtocolGeneration{Revision: -1}
}

func (s ProtocolGrantSet) Has(grant ProtocolGrant) bool {
	return uint8(s)&uint8(grant) != 0
}

func (s ProtocolGrantSet) Empty() bool { return s == 0 }

func normalizeProtocolBounds(bounds ProtocolBounds) ProtocolBounds {
	if bounds.MaxRevision <= 0 {
		bounds.MaxRevision = 2
	}
	if len(bounds.Grants) == 0 {
		bounds.Grants = []ProtocolGrant{ProtocolGrantA, ProtocolGrantB}
	} else {
		bounds.Grants = append([]ProtocolGrant(nil), bounds.Grants...)
	}
	return bounds
}

func protocolGrantMask(bounds ProtocolBounds) ProtocolGrantSet {
	var mask ProtocolGrantSet
	for _, grant := range normalizeProtocolBounds(bounds).Grants {
		mask |= ProtocolGrantSet(grant)
	}
	return mask
}

func InitialProtocolStates(bounds ProtocolBounds) []ProtocolState {
	_ = normalizeProtocolBounds(bounds)
	generation := protocolGeneration(0, 0)
	return []ProtocolState{{
		Status:              ProtocolCreated,
		RecordedGeneration:  generation,
		RuntimeGeneration:   generation,
		InspectedGeneration: generation,
		Health:              ProtocolHealthy,
		Mode:                ProtocolNormal,
		Operation:           ProtocolIdle,
		Effect:              ProtocolNoEffect,
		Result:              ProtocolPending,
		Direction:           ProtocolNoDirection,
	}}
}

// NormalizeProtocolState applies the explicit graph quotient. Checker-only
// evidence/history fields are absent from ProtocolState; invalid generations
// use the single NoGen sentinel and an empty direction uses NoDirection.
func NormalizeProtocolState(state ProtocolState) ProtocolState {
	for _, generation := range []*ProtocolGeneration{
		&state.RecordedGeneration,
		&state.RuntimeGeneration,
		&state.InspectedGeneration,
	} {
		if !generation.Valid {
			*generation = protocolNoGeneration()
		}
	}
	if state.Direction == "" {
		state.Direction = ProtocolNoDirection
	}
	return state
}

// EnabledProtocol derives enabled events solely by asking ReduceProtocol. It
// deliberately has no second guard table that could drift from the reducer.
func EnabledProtocol(state ProtocolState) []ProtocolEvent {
	return EnabledProtocolWithin(ProtocolBounds{}, state)
}

func EnabledProtocolWithin(bounds ProtocolBounds, state ProtocolState) []ProtocolEvent {
	bounds = normalizeProtocolBounds(bounds)
	grants := append([]ProtocolGrant{0}, bounds.Grants...)
	var enabled []ProtocolEvent
	for _, action := range protocolActions {
		for _, outcome := range protocolOutcomes {
			for _, grant := range grants {
				for _, truth := range protocolTruths {
					event := ProtocolEvent{Action: action, Outcome: outcome, Grant: grant, Truth: truth}
					if _, ok := ReduceProtocolWithin(bounds, state, event); ok {
						enabled = append(enabled, event)
					}
				}
			}
		}
	}
	return enabled
}

func ReduceProtocol(state ProtocolState, event ProtocolEvent) (ProtocolState, bool) {
	return ReduceProtocolWithin(ProtocolBounds{}, state, event)
}

// ReduceProtocolUnknownCommit returns both correlated raw worlds for a commit
// whose durability cannot be classified. Production drivers must handle the
// pair as one blocked epistemic outcome and may not select either as known.
func ReduceProtocolUnknownCommit(state ProtocolState, action ProtocolAction) (ProtocolState, ProtocolState, bool) {
	oldWorld, oldOK := ReduceProtocol(state, ProtocolEvent{Action: action, Outcome: ProtocolCommitUnknown, Truth: ProtocolOld})
	newWorld, newOK := ReduceProtocol(state, ProtocolEvent{Action: action, Outcome: ProtocolCommitUnknown, Truth: ProtocolNew})
	return oldWorld, newWorld, oldOK && newOK
}

// ReduceProtocolWithin is deterministic and effect-free. CommitUnknown's Truth
// selects one raw old/new world for exhaustive bounded exploration; callers may
// not treat either branch as known because both remain Blocked until recovery.
func ReduceProtocolWithin(bounds ProtocolBounds, state ProtocolState, event ProtocolEvent) (ProtocolState, bool) {
	bounds = normalizeProtocolBounds(bounds)
	state = NormalizeProtocolState(state)
	if !protocolStateInDomain(bounds, state) {
		return state, false
	}
	next := state

	switch event.Action {
	case ProtocolClaimStart:
		if !protocolEmptyEvent(event) || state.Status != ProtocolCreated || state.Mode != ProtocolNormal || state.Operation != ProtocolIdle {
			return state, false
		}
		next.Operation, next.Effect, next.Result = ProtocolClaim, ProtocolCommit, ProtocolPending

	case ProtocolClaimCommit:
		if event.Grant != 0 || state.Operation != ProtocolClaim || state.Effect != ProtocolCommit || !protocolCommitEvent(event) {
			return state, false
		}
		switch event.Outcome {
		case ProtocolKnownNew:
			next.Status, next.Owners, next.Detached = ProtocolRunning, ProtocolOwnerA, false
			return resetProtocol(next, ProtocolSuccess), true
		case ProtocolKnownOld:
			return resetProtocol(next, ProtocolFailure), true
		case ProtocolCommitUnknown:
			if event.Truth == ProtocolNew {
				next.Status, next.Owners, next.Detached = ProtocolRunning, ProtocolOwnerA, false
			}
			blockProtocolRecovery(&next, ProtocolUncertainResult)
		default:
			return state, false
		}

	case ProtocolRecoverClaim:
		if !protocolEmptyEvent(event) || state.Operation != ProtocolRecover || state.Direction != ProtocolNoDirection || state.Effect != ProtocolInspect {
			return state, false
		}
		next.Health, next.Mode = ProtocolHealthy, ProtocolNormal
		if state.Status == ProtocolRunning {
			return resetProtocol(next, ProtocolSuccess), true
		}
		return resetProtocol(next, ProtocolFailure), true

	case ProtocolHandoffExact:
		if !protocolEmptyEvent(event) || state.Status != ProtocolRunning || state.Mode != ProtocolNormal || state.Operation != ProtocolIdle || state.Owners != ProtocolOwnerA || state.Detached {
			return state, false
		}
		next.Owners, next.Detached = ProtocolOwnerB, true

	case ProtocolReleaseExact:
		if !protocolEmptyEvent(event) || state.Status != ProtocolRunning || state.Mode != ProtocolNormal || state.Operation != ProtocolIdle || state.Owners != ProtocolOwnerA || state.Detached {
			return state, false
		}
		next.Status, next.Owners, next.Detached, next.Result = ProtocolCreated, 0, false, ProtocolFailure

	case ProtocolWidenStart:
		if event.Outcome != "" || event.Truth != "" || state.Status != ProtocolRunning || state.Mode != ProtocolNormal || state.Operation != ProtocolIdle || !protocolSingleGrant(bounds, event.Grant) || state.DurableAuthority.Has(event.Grant) || state.DurableRevision >= bounds.MaxRevision {
			return state, false
		}
		next.Operation, next.Effect, next.Result = ProtocolWiden, ProtocolCommit, ProtocolPending
		next.PendingAuthority = state.DurableAuthority | ProtocolGrantSet(event.Grant)
		next.PendingRevision = state.DurableRevision + 1
		next.Direction = ProtocolWidenDirection

	case ProtocolWidenCommit:
		if event.Grant != 0 || state.Operation != ProtocolWiden || state.Direction != ProtocolWidenDirection || state.Effect != ProtocolCommit || protocolPendingIsDurable(state) || !protocolCommitEvent(event) {
			return state, false
		}
		switch event.Outcome {
		case ProtocolKnownNew:
			next.DurableAuthority, next.DurableRevision, next.Effect = state.PendingAuthority, state.PendingRevision, ProtocolApply
		case ProtocolKnownOld:
			return resetProtocol(next, ProtocolFailure), true
		case ProtocolCommitUnknown:
			if event.Truth == ProtocolNew {
				next.DurableAuthority, next.DurableRevision = state.PendingAuthority, state.PendingRevision
			}
			blockProtocolRecovery(&next, ProtocolUncertainResult)
		default:
			return state, false
		}

	case ProtocolApplyAction:
		if event.Outcome != ProtocolSucceeded || event.Grant != 0 || event.Truth != "" || state.Effect != ProtocolApply {
			return state, false
		}
		switch {
		case state.Direction == ProtocolWidenDirection && (state.Operation == ProtocolWiden || state.Operation == ProtocolRecover) && protocolPendingIsDurable(state):
		case state.Direction == ProtocolNarrowDirection && state.Operation == ProtocolNarrow:
		default:
			return state, false
		}
		next.RuntimeAuthority = state.PendingAuthority
		next.RuntimeGeneration = protocolGeneration(state.PendingAuthority, state.PendingRevision)
		next.Effect = ProtocolInspect

	case ProtocolInspectAction:
		if event.Outcome != ProtocolMatched || event.Grant != 0 || event.Truth != "" || state.Effect != ProtocolInspect || (state.Operation != ProtocolWiden && state.Operation != ProtocolNarrow) || state.RuntimeGeneration != protocolGeneration(state.PendingAuthority, state.PendingRevision) {
			return state, false
		}
		next.InspectedGeneration, next.Effect = state.RuntimeGeneration, ProtocolCommit

	case ProtocolWidenFinalCommit:
		if event.Grant != 0 || state.Operation != ProtocolWiden || state.Direction != ProtocolWidenDirection || state.Effect != ProtocolCommit || !protocolPendingIsDurable(state) || state.InspectedGeneration != protocolGeneration(state.PendingAuthority, state.PendingRevision) || !protocolCommitEvent(event) {
			return state, false
		}
		switch event.Outcome {
		case ProtocolKnownNew:
			next.RecordedGeneration = protocolGeneration(state.PendingAuthority, state.PendingRevision)
			next.Health, next.Mode = ProtocolHealthy, ProtocolNormal
			return resetProtocol(next, ProtocolSuccess), true
		case ProtocolKnownOld:
			next.Health, next.Mode, next.Operation, next.Effect, next.Result = ProtocolUncertain, ProtocolBlocked, ProtocolRecover, ProtocolInspect, ProtocolFailure
		case ProtocolCommitUnknown:
			if event.Truth == ProtocolNew {
				next.RecordedGeneration = protocolGeneration(state.PendingAuthority, state.PendingRevision)
			}
			blockProtocolRecovery(&next, ProtocolUncertainResult)
		default:
			return state, false
		}

	case ProtocolNarrowStart:
		if event.Outcome != "" || event.Truth != "" || state.Status != ProtocolRunning || state.Mode != ProtocolNormal || state.Operation != ProtocolIdle || !protocolSingleGrant(bounds, event.Grant) || !state.DurableAuthority.Has(event.Grant) || state.DurableRevision >= bounds.MaxRevision {
			return state, false
		}
		next.Operation, next.Effect, next.Result = ProtocolNarrow, ProtocolApply, ProtocolPending
		next.PendingAuthority = state.DurableAuthority &^ ProtocolGrantSet(event.Grant)
		next.PendingRevision = state.DurableRevision + 1
		next.Direction = ProtocolNarrowDirection

	case ProtocolNarrowFinalCommit:
		if event.Grant != 0 || state.Operation != ProtocolNarrow || state.Direction != ProtocolNarrowDirection || state.Effect != ProtocolCommit || state.InspectedGeneration != protocolGeneration(state.PendingAuthority, state.PendingRevision) || !protocolCommitEvent(event) {
			return state, false
		}
		switch event.Outcome {
		case ProtocolKnownNew:
			next.DurableAuthority, next.DurableRevision = state.PendingAuthority, state.PendingRevision
			next.RecordedGeneration = protocolGeneration(state.PendingAuthority, state.PendingRevision)
			next.Health, next.Mode = ProtocolHealthy, ProtocolNormal
			return resetProtocol(next, ProtocolSuccess), true
		case ProtocolKnownOld:
			next.RuntimeAuthority = state.DurableAuthority
			next.RuntimeGeneration = protocolGeneration(state.DurableAuthority, state.DurableRevision)
			next.InspectedGeneration = next.RuntimeGeneration
			return resetProtocol(next, ProtocolFailure), true
		case ProtocolCommitUnknown:
			if event.Truth == ProtocolNew {
				next.DurableAuthority, next.DurableRevision = state.PendingAuthority, state.PendingRevision
				next.RecordedGeneration = protocolGeneration(state.PendingAuthority, state.PendingRevision)
			}
			blockProtocolRecovery(&next, ProtocolUncertainResult)
		default:
			return state, false
		}

	case ProtocolRecoverEgress:
		if !protocolEmptyEvent(event) || state.Operation != ProtocolRecover || (state.Direction != ProtocolWidenDirection && state.Direction != ProtocolNarrowDirection) || state.Effect != ProtocolInspect {
			return state, false
		}
		next.RuntimeAuthority = state.DurableAuthority
		next.RuntimeGeneration = protocolGeneration(state.DurableAuthority, state.DurableRevision)
		next.InspectedGeneration = next.RuntimeGeneration
		next.RecordedGeneration = next.RuntimeGeneration
		next.Health, next.Mode = ProtocolHealthy, ProtocolNormal
		if protocolPendingIsDurable(state) {
			return resetProtocol(next, ProtocolSuccess), true
		}
		return resetProtocol(next, ProtocolFailure), true

	case ProtocolObserveInvalidPersistence:
		if event.Grant != 0 || event.Truth != "" || state.Status != ProtocolCreated || state.Mode != ProtocolNormal || state.Operation != ProtocolIdle {
			return state, false
		}
		switch event.Outcome {
		case ProtocolObservedCorrupt:
			next.Health = ProtocolCorrupt
		case ProtocolObservedStale:
			next.Health = ProtocolStale
		default:
			return state, false
		}
		next.Mode, next.Operation, next.Result = ProtocolBlocked, ProtocolRecover, ProtocolFailure

	case ProtocolStopStart:
		if !protocolEmptyEvent(event) || state.Status != ProtocolRunning || state.Mode != ProtocolNormal || state.Operation != ProtocolIdle {
			return state, false
		}
		next.Operation, next.Effect, next.Result = ProtocolStop, ProtocolTeardownEffect, ProtocolPending

	case ProtocolRequestTeardown:
		if !protocolEmptyEvent(event) || state.Mode != ProtocolBlocked || state.Effect == ProtocolTeardownEffect {
			return state, false
		}
		next.Operation, next.Effect, next.Result = ProtocolTeardown, ProtocolTeardownEffect, ProtocolPending

	case ProtocolTeardownAction:
		if event.Grant != 0 || event.Truth != "" || state.Effect != ProtocolTeardownEffect || (state.Operation != ProtocolStop && state.Operation != ProtocolTeardown) {
			return state, false
		}
		switch event.Outcome {
		case ProtocolTeardownProven:
			next.Status, next.Owners, next.Detached = ProtocolStopped, 0, false
			next.RuntimeAuthority = 0
			next.RuntimeGeneration, next.InspectedGeneration = protocolNoGeneration(), protocolNoGeneration()
			next.Health, next.Mode = ProtocolHealthy, ProtocolTerminal
			next.Operation, next.Effect, next.Result = ProtocolIdle, ProtocolNoEffect, ProtocolFailure
			next.PendingAuthority, next.PendingRevision, next.Direction = 0, 0, ProtocolNoDirection
		case ProtocolTeardownUnknown:
			next.Health, next.Mode = ProtocolUncertain, ProtocolBlocked
			next.Operation, next.Effect, next.Result = ProtocolTeardown, ProtocolTeardownEffect, ProtocolUncertainResult
		default:
			return state, false
		}

	case ProtocolTerminalStutter:
		if !protocolEmptyEvent(event) || state.Status != ProtocolStopped {
			return state, false
		}
		return state, true

	default:
		return state, false
	}
	return NormalizeProtocolState(next), true
}

func protocolEmptyEvent(event ProtocolEvent) bool {
	return event.Outcome == "" && event.Grant == 0 && event.Truth == ""
}

func protocolCommitEvent(event ProtocolEvent) bool {
	switch event.Outcome {
	case ProtocolKnownOld, ProtocolKnownNew:
		return event.Truth == ""
	case ProtocolCommitUnknown:
		return event.Truth == ProtocolOld || event.Truth == ProtocolNew
	default:
		return false
	}
}

func protocolSingleGrant(bounds ProtocolBounds, grant ProtocolGrant) bool {
	if grant == 0 || grant&(grant-1) != 0 {
		return false
	}
	return protocolGrantMask(bounds).Has(grant)
}

func protocolPendingIsDurable(state ProtocolState) bool {
	return state.DurableAuthority == state.PendingAuthority && state.DurableRevision == state.PendingRevision
}

func blockProtocolRecovery(state *ProtocolState, result ProtocolResult) {
	state.Health, state.Mode = ProtocolUncertain, ProtocolBlocked
	state.Operation, state.Effect, state.Result = ProtocolRecover, ProtocolInspect, result
}

func resetProtocol(state ProtocolState, result ProtocolResult) ProtocolState {
	state.Operation, state.Effect, state.Result = ProtocolIdle, ProtocolNoEffect, result
	state.PendingAuthority, state.PendingRevision, state.Direction = 0, 0, ProtocolNoDirection
	return NormalizeProtocolState(state)
}

func protocolStateInDomain(bounds ProtocolBounds, state ProtocolState) bool {
	bounds = normalizeProtocolBounds(bounds)
	mask := protocolGrantMask(bounds)
	if ProtocolGrantSet(state.Owners)&^ProtocolGrantSet(ProtocolOwnerA|ProtocolOwnerB) != 0 || state.DurableAuthority&^mask != 0 || state.RuntimeAuthority&^mask != 0 || state.PendingAuthority&^mask != 0 {
		return false
	}
	if state.DurableRevision < 0 || state.DurableRevision > bounds.MaxRevision || state.PendingRevision < 0 || state.PendingRevision > bounds.MaxRevision {
		return false
	}
	for _, generation := range []ProtocolGeneration{state.RecordedGeneration, state.RuntimeGeneration, state.InspectedGeneration} {
		if generation.Valid {
			if generation.Revision < 0 || generation.Revision > bounds.MaxRevision || generation.Authority&^mask != 0 {
				return false
			}
		} else if generation != protocolNoGeneration() {
			return false
		}
	}
	return protocolStatusInDomain(state.Status) && protocolModeInDomain(state.Mode) && protocolHealthInDomain(state.Health) && protocolOperationInDomain(state.Operation) && protocolEffectInDomain(state.Effect) && protocolResultInDomain(state.Result) && protocolDirectionInDomain(state.Direction)
}

func protocolStatusInDomain(value ProtocolStatus) bool {
	return value == ProtocolCreated || value == ProtocolRunning || value == ProtocolStopped
}

func protocolModeInDomain(value ProtocolMode) bool {
	return value == ProtocolNormal || value == ProtocolBlocked || value == ProtocolTerminal
}

func protocolHealthInDomain(value ProtocolHealth) bool {
	return value == ProtocolHealthy || value == ProtocolUncertain || value == ProtocolCorrupt || value == ProtocolStale
}

func protocolOperationInDomain(value ProtocolOperation) bool {
	return value == ProtocolIdle || value == ProtocolClaim || value == ProtocolWiden || value == ProtocolNarrow || value == ProtocolRecover || value == ProtocolStop || value == ProtocolTeardown
}

func protocolEffectInDomain(value ProtocolEffect) bool {
	return value == ProtocolNoEffect || value == ProtocolCommit || value == ProtocolApply || value == ProtocolInspect || value == ProtocolTeardownEffect
}

func protocolResultInDomain(value ProtocolResult) bool {
	return value == ProtocolPending || value == ProtocolSuccess || value == ProtocolFailure || value == ProtocolUncertainResult
}

func protocolDirectionInDomain(value ProtocolDirection) bool {
	return value == ProtocolNoDirection || value == ProtocolWidenDirection || value == ProtocolNarrowDirection
}

// ProtocolEventLabel maps one accepted reducer event to the corresponding
// normative TLA+ action label without introducing another transition relation.
func ProtocolEventLabel(state ProtocolState, event ProtocolEvent) (ProtocolLabel, bool) {
	return ProtocolEventLabelWithin(ProtocolBounds{}, state, event)
}

func ProtocolEventLabelWithin(bounds ProtocolBounds, state ProtocolState, event ProtocolEvent) (ProtocolLabel, bool) {
	if _, ok := ReduceProtocolWithin(bounds, state, event); !ok {
		return "", false
	}
	switch event.Action {
	case ProtocolClaimStart:
		return ProtocolLabelClaimStart, true
	case ProtocolClaimCommit:
		return protocolCommitLabel(event.Outcome, ProtocolLabelClaimCommitNew, ProtocolLabelClaimCommitOld, ProtocolLabelClaimCommitUnknown)
	case ProtocolRecoverClaim:
		return ProtocolLabelRecoverClaim, true
	case ProtocolHandoffExact:
		return ProtocolLabelHandoffExact, true
	case ProtocolReleaseExact:
		return ProtocolLabelReleaseExact, true
	case ProtocolWidenStart:
		return ProtocolLabelWidenStart, true
	case ProtocolWidenCommit:
		return protocolCommitLabel(event.Outcome, ProtocolLabelWidenCommitNew, ProtocolLabelWidenCommitOld, ProtocolLabelWidenCommitUnknown)
	case ProtocolApplyAction:
		if state.Direction == ProtocolWidenDirection {
			return ProtocolLabelWidenApply, true
		}
		return ProtocolLabelNarrowApply, true
	case ProtocolInspectAction:
		return ProtocolLabelPositiveInspect, true
	case ProtocolWidenFinalCommit:
		return protocolCommitLabel(event.Outcome, ProtocolLabelWidenFinalCommitNew, ProtocolLabelWidenFinalCommitOld, ProtocolLabelWidenFinalCommitUnknown)
	case ProtocolNarrowStart:
		return ProtocolLabelNarrowStart, true
	case ProtocolNarrowFinalCommit:
		return protocolCommitLabel(event.Outcome, ProtocolLabelNarrowFinalCommitNew, ProtocolLabelNarrowFinalCommitOld, ProtocolLabelNarrowFinalCommitUnknown)
	case ProtocolRecoverEgress:
		return ProtocolLabelRecoverEgress, true
	case ProtocolObserveInvalidPersistence:
		return ProtocolLabelObserveInvalidPersistence, true
	case ProtocolStopStart:
		return ProtocolLabelStopStart, true
	case ProtocolRequestTeardown:
		return ProtocolLabelRequestTeardown, true
	case ProtocolTeardownAction:
		if event.Outcome == ProtocolTeardownProven {
			return ProtocolLabelTeardownProven, true
		}
		return ProtocolLabelTeardownUnknown, true
	case ProtocolTerminalStutter:
		return ProtocolLabelTerminalStutter, true
	default:
		return "", false
	}
}

func protocolCommitLabel(outcome ProtocolOutcome, knownNew, knownOld, unknown ProtocolLabel) (ProtocolLabel, bool) {
	switch outcome {
	case ProtocolKnownNew:
		return knownNew, true
	case ProtocolKnownOld:
		return knownOld, true
	case ProtocolCommitUnknown:
		return unknown, true
	default:
		return "", false
	}
}

type protocolConcreteOwner struct {
	pid   int
	token string
}

type protocolConcreteGeneration struct {
	grants          []EgressGrant
	grantRevision   int
	generation      egress.Generation
	appliedRevision int
	appliedHash     string
}

// ProtocolAdapter frames concrete identities, grant rows, hashes, revisions,
// timestamps, and metadata around the symbolic reducer state. Bindings let one
// operation rebase arbitrary concrete revisions/authority into the finite model.
type ProtocolAdapter struct {
	original      Session
	baseline      ProtocolState
	owners        map[ProtocolOwner]protocolConcreteOwner
	generations   map[ProtocolGeneration]protocolConcreteGeneration
	claimDetached *bool
}

func NewProtocolAdapter(sess Session) (ProtocolAdapter, ProtocolState) {
	base := InitialProtocolStates(ProtocolBounds{})[0]
	adapter := ProtocolAdapter{
		original:    sess,
		owners:      make(map[ProtocolOwner]protocolConcreteOwner),
		generations: make(map[ProtocolGeneration]protocolConcreteGeneration),
	}
	baseGeneration := protocolGeneration(0, 0)
	if err := adapter.bindGeneration(baseGeneration, sess, true); err != nil {
		base.Health, base.Mode, base.Operation, base.Result = ProtocolCorrupt, ProtocolBlocked, ProtocolRecover, ProtocolFailure
	}

	switch sess.Status {
	case StatusCreated:
	case StatusRunning:
		base.Status, base.Owners, base.Detached, base.Result = ProtocolRunning, ProtocolOwnerA, sess.Detached, ProtocolSuccess
		if err := adapter.BindOwner(ProtocolOwnerA, sess.PID, sess.ProcessToken); err != nil {
			markProtocolStale(&base)
		}
	case StatusStopped:
		base.Status, base.Mode, base.Result = ProtocolStopped, ProtocolTerminal, ProtocolFailure
		base.RuntimeAuthority = 0
		base.RuntimeGeneration, base.InspectedGeneration = protocolNoGeneration(), protocolNoGeneration()
	default:
		base.Health, base.Mode, base.Operation, base.Result = ProtocolCorrupt, ProtocolBlocked, ProtocolRecover, ProtocolFailure
	}

	if protocolConcreteOwnerShapeStale(sess) {
		markProtocolStale(&base)
	}
	if !validEgressRuntimeState(sess) {
		markProtocolCorrupt(&base)
	}
	if sess.egressTransition != nil && base.Status == ProtocolRunning && base.Mode == ProtocolNormal {
		adapter.mapConcreteTransition(&base, sess)
	}
	if sess.LastFailure != nil && sess.LastFailure.Phase == "network" && sess.LastFailure.Code == "network_authority_uncertain" {
		base.Health, base.Mode, base.Operation, base.Effect, base.Result = ProtocolUncertain, ProtocolBlocked, ProtocolRecover, ProtocolNoEffect, ProtocolUncertainResult
	}
	base = NormalizeProtocolState(base)
	adapter.baseline = base
	return adapter, base
}

func markProtocolStale(state *ProtocolState) {
	state.Health, state.Mode, state.Operation, state.Effect, state.Result = ProtocolStale, ProtocolBlocked, ProtocolRecover, ProtocolNoEffect, ProtocolFailure
}

func markProtocolCorrupt(state *ProtocolState) {
	state.Health, state.Mode, state.Operation, state.Effect, state.Result = ProtocolCorrupt, ProtocolBlocked, ProtocolRecover, ProtocolNoEffect, ProtocolFailure
}

func protocolConcreteOwnerShapeStale(sess Session) bool {
	hasOwner := sess.PID != 0 || sess.ProcessToken != "" || sess.Detached
	switch sess.Status {
	case StatusCreated, StatusStopped:
		return hasOwner
	case StatusRunning:
		return sess.PID <= 0
	default:
		return false
	}
}

func (a *ProtocolAdapter) mapConcreteTransition(state *ProtocolState, sess Session) {
	if !validEgressRuntimeState(sess) {
		markProtocolCorrupt(state)
		return
	}
	transition := sess.egressTransition
	a.generations = make(map[ProtocolGeneration]protocolConcreteGeneration)
	switch transition.Direction {
	case EgressDirectionWiden:
		if len(sess.EgressGrants) == 0 {
			markProtocolStale(state)
			return
		}
		old := sess
		old.EgressGrants = append([]EgressGrant(nil), sess.EgressGrants[:len(sess.EgressGrants)-1]...)
		old.GrantRevision = sess.appliedEgressRevision
		oldSymbol := protocolGeneration(0, 0)
		newSymbol := protocolGeneration(ProtocolGrantSet(ProtocolGrantA), 1)
		if err := a.bindGeneration(oldSymbol, old, true); err != nil || a.bindGeneration(newSymbol, sess, false) != nil {
			markProtocolStale(state)
			return
		}
		state.DurableAuthority, state.DurableRevision = newSymbol.Authority, newSymbol.Revision
		state.RecordedGeneration, state.RuntimeGeneration, state.InspectedGeneration = oldSymbol, oldSymbol, oldSymbol
		state.PendingAuthority, state.PendingRevision = newSymbol.Authority, newSymbol.Revision
		state.Direction, state.Operation, state.Effect, state.Result = ProtocolWidenDirection, ProtocolWiden, ProtocolApply, ProtocolPending
	case EgressDirectionNarrow:
		oldSymbol := protocolGeneration(ProtocolGrantSet(ProtocolGrantA), 0)
		newSymbol := protocolGeneration(0, 1)
		candidate := sess
		candidate.EgressGrants = append([]EgressGrant(nil), transition.CandidateGrants...)
		candidate.GrantRevision = transition.CandidateRevision
		if err := a.bindGeneration(oldSymbol, sess, true); err != nil || a.bindGeneration(newSymbol, candidate, false) != nil {
			markProtocolStale(state)
			return
		}
		state.DurableAuthority, state.DurableRevision = oldSymbol.Authority, oldSymbol.Revision
		state.RecordedGeneration, state.RuntimeGeneration, state.InspectedGeneration = oldSymbol, oldSymbol, oldSymbol
		state.PendingAuthority, state.PendingRevision = newSymbol.Authority, newSymbol.Revision
		state.Direction, state.Operation, state.Effect, state.Result = ProtocolNarrowDirection, ProtocolNarrow, ProtocolApply, ProtocolPending
	default:
		markProtocolStale(state)
	}
}

// ClaimState rebases a decodable Created record onto the claim operation while
// framing stale owner bytes that the existing claim semantics overwrite.
func (a *ProtocolAdapter) ClaimState() (ProtocolState, error) {
	if a.original.Status != StatusCreated {
		return ProtocolState{}, fmt.Errorf("protocol claim requires a created session")
	}
	state := NormalizeProtocolState(a.baseline)
	state.Status, state.Owners, state.Detached = ProtocolCreated, 0, false
	state.Health, state.Mode = ProtocolHealthy, ProtocolNormal
	state.Operation, state.Effect = ProtocolIdle, ProtocolNoEffect
	state.PendingAuthority, state.PendingRevision, state.Direction = 0, 0, ProtocolNoDirection
	return state, nil
}

// BindClaimOwner preserves unsupported-platform tokenless and historical
// detached claim projection without admitting either into exact-owner actions.
func (a *ProtocolAdapter) BindClaimOwner(pid int, token string, detached bool) error {
	if err := a.bindConcreteOwner(ProtocolOwnerA, pid, token, false); err != nil {
		return err
	}
	a.claimDetached = new(bool)
	*a.claimDetached = detached
	return nil
}

// BindHandoffOwner records the supervisor identity after exact parent
// authorization. A missing target token preserves unsupported-platform output,
// but the resulting record remaps outside the exact-token theorem.
func (a *ProtocolAdapter) BindHandoffOwner(pid int, token string) error {
	binding := protocolConcreteOwner{pid: pid, token: token}
	if current, ok := a.owners[ProtocolOwnerA]; ok && current == binding {
		// Hermetic callers historically hand off to the same observed process
		// while changing only detached signal mode. Move, rather than alias, the
		// symbolic role so at most one binding still names the exact pair.
		delete(a.owners, ProtocolOwnerA)
	}
	return a.bindConcreteOwner(ProtocolOwnerB, pid, token, false)
}

// OwnerActionState frames unrelated persisted egress recovery bytes while an
// exact handoff/release decision runs. This preserves the historical fact that
// owner-only lifecycle operations do not repair or reject egress state.
func (a *ProtocolAdapter) OwnerActionState() (ProtocolState, error) {
	if a.original.Status != StatusRunning || a.original.PID <= 0 || !validEgressRuntimeState(a.original) {
		return ProtocolState{}, fmt.Errorf("protocol owner action state is unavailable")
	}
	state := NormalizeProtocolState(a.baseline)
	if a.original.egressTransition != nil && state.Direction == ProtocolNoDirection {
		a.mapConcreteTransition(&state, a.original)
		if state.Health == ProtocolCorrupt {
			return ProtocolState{}, fmt.Errorf("protocol owner recovery state is invalid")
		}
	}
	if a.original.ProcessToken == "" {
		if err := a.bindConcreteOwner(ProtocolOwnerA, a.original.PID, "", false); err != nil {
			return ProtocolState{}, err
		}
	}
	state.Status, state.Owners, state.Detached = ProtocolRunning, ProtocolOwnerA, a.original.Detached
	state.Health, state.Mode = ProtocolHealthy, ProtocolNormal
	state.Operation, state.Effect, state.Result = ProtocolIdle, ProtocolNoEffect, ProtocolSuccess
	return NormalizeProtocolState(state), nil
}

// SignalAuthorizationState preserves the pre-existing Stop behavior for a
// decodable Created record carrying stale owner bytes. That compatibility state
// is deliberately outside ValidRecordShape and the exact-owner theorem.
func (a *ProtocolAdapter) SignalAuthorizationState() (ProtocolState, error) {
	if a.original.Status == StatusRunning {
		return a.OwnerActionState()
	}
	if a.original.Status != StatusCreated || a.original.PID <= 0 || !validEgressRuntimeState(a.original) {
		return ProtocolState{}, fmt.Errorf("protocol signal authorization state is unavailable")
	}
	state, err := a.ClaimState()
	if err != nil {
		return ProtocolState{}, err
	}
	if a.original.egressTransition != nil {
		a.mapConcreteTransition(&state, a.original)
		if state.Health == ProtocolCorrupt {
			return ProtocolState{}, fmt.Errorf("protocol signal recovery state is invalid")
		}
	}
	if err := a.bindConcreteOwner(ProtocolOwnerA, a.original.PID, a.original.ProcessToken, false); err != nil {
		return ProtocolState{}, err
	}
	state.Status, state.Owners, state.Detached = ProtocolRunning, ProtocolOwnerA, a.original.Detached
	state.Health, state.Mode = ProtocolHealthy, ProtocolNormal
	state.Operation, state.Effect, state.Result = ProtocolIdle, ProtocolNoEffect, ProtocolSuccess
	return NormalizeProtocolState(state), nil
}

// BlockedTeardownState frames a Created, dead, or unverifiable owner before
// RequestTeardown. It grants no signal authority and preserves durable egress
// bindings until a later proven teardown clears only runtime state.
func (a *ProtocolAdapter) BlockedTeardownState() (ProtocolState, error) {
	if (a.original.Status != StatusCreated && a.original.Status != StatusRunning) || !validEgressRuntimeState(a.original) {
		return ProtocolState{}, fmt.Errorf("protocol blocked teardown state is unavailable")
	}
	state := NormalizeProtocolState(a.baseline)
	if a.original.Status == StatusCreated {
		var err error
		state, err = a.ClaimState()
		if err != nil {
			return ProtocolState{}, err
		}
	} else if a.original.PID > 0 {
		var err error
		state, err = a.OwnerActionState()
		if err != nil {
			return ProtocolState{}, err
		}
	}
	if a.original.egressTransition != nil && state.Direction == ProtocolNoDirection {
		a.mapConcreteTransition(&state, a.original)
		if state.Health == ProtocolCorrupt {
			return ProtocolState{}, fmt.Errorf("protocol blocked teardown recovery state is invalid")
		}
	}
	state.Health, state.Mode = ProtocolStale, ProtocolBlocked
	state.Operation, state.Effect, state.Result = ProtocolRecover, ProtocolNoEffect, ProtocolFailure
	return NormalizeProtocolState(state), nil
}

// LegacyRunningState is an explicit compatibility projection for records with
// no process token. The caller must still perform the historical PID liveness
// effect; this state is outside the exact-token theorem.
func (a *ProtocolAdapter) LegacyRunningState() (ProtocolState, error) {
	if a.original.ProcessToken != "" {
		return ProtocolState{}, fmt.Errorf("protocol legacy owner state is unavailable")
	}
	return a.OwnerActionState()
}

// BindOwner associates one symbolic exact owner with an opaque concrete
// PID/process-token pair. Process-token verification remains an external effect.
func (a *ProtocolAdapter) BindOwner(owner ProtocolOwner, pid int, token string) error {
	return a.bindConcreteOwner(owner, pid, token, true)
}

func (a *ProtocolAdapter) bindConcreteOwner(owner ProtocolOwner, pid int, token string, requireToken bool) error {
	if (owner != ProtocolOwnerA && owner != ProtocolOwnerB) || pid <= 0 || (requireToken && token == "") {
		return fmt.Errorf("protocol exact owner binding is invalid")
	}
	if a.owners == nil {
		a.owners = make(map[ProtocolOwner]protocolConcreteOwner)
	}
	binding := protocolConcreteOwner{pid: pid, token: token}
	if existing, ok := a.owners[owner]; ok && existing != binding {
		return fmt.Errorf("protocol exact owner binding conflicts")
	}
	for existingOwner, existing := range a.owners {
		if existingOwner != owner && existing == binding {
			return fmt.Errorf("protocol exact owner bindings must be distinct")
		}
	}
	a.owners[owner] = binding
	return nil
}

// BindGeneration associates one symbolic generation with exact concrete grant
// rows and their canonical concrete generation.
func (a *ProtocolAdapter) BindGeneration(symbol ProtocolGeneration, candidate Session) error {
	return a.bindGeneration(symbol, candidate, false)
}

// RebaseGeneration frames the adapter's current stable concrete authority as a
// chosen symbolic generation. This is how arbitrary production revisions enter
// the reviewed finite relation without permitting two symbols to alias it.
func (a *ProtocolAdapter) RebaseGeneration(symbol ProtocolGeneration) (ProtocolState, error) {
	state := NormalizeProtocolState(a.baseline)
	stable := state.Health == ProtocolHealthy && state.Mode == ProtocolNormal && state.Operation == ProtocolIdle && state.Effect == ProtocolNoEffect
	ownerOnlyStale := state.Health == ProtocolStale && state.Mode == ProtocolBlocked && state.Operation == ProtocolRecover && state.Effect == ProtocolNoEffect
	if a.original.Status != StatusRunning || a.original.egressTransition != nil || !validEgressRuntimeState(a.original) || (!stable && !ownerOnlyStale) {
		return ProtocolState{}, fmt.Errorf("protocol generation rebase requires a stable running session")
	}
	// Generation changes do not authorize owner actions. Frame historical
	// tokenless or malformed owner bytes under one symbolic owner solely so
	// candidate projection preserves the already-decoded Running record.
	if _, ok := a.owners[ProtocolOwnerA]; !ok {
		a.owners[ProtocolOwnerA] = protocolConcreteOwner{pid: a.original.PID, token: a.original.ProcessToken}
	}
	state.Status, state.Owners, state.Detached = ProtocolRunning, ProtocolOwnerA, a.original.Detached
	state.Health, state.Mode = ProtocolHealthy, ProtocolNormal
	state.Operation, state.Effect, state.Result = ProtocolIdle, ProtocolNoEffect, ProtocolSuccess
	state.PendingAuthority, state.PendingRevision, state.Direction = 0, 0, ProtocolNoDirection
	a.generations = make(map[ProtocolGeneration]protocolConcreteGeneration)
	if err := a.bindGeneration(symbol, a.original, true); err != nil {
		return ProtocolState{}, err
	}
	state.DurableAuthority, state.DurableRevision = symbol.Authority, symbol.Revision
	state.RecordedGeneration, state.RuntimeGeneration, state.InspectedGeneration = symbol, symbol, symbol
	state.RuntimeAuthority = symbol.Authority
	a.baseline = NormalizeProtocolState(state)
	return a.baseline, nil
}

func (a *ProtocolAdapter) bindGeneration(symbol ProtocolGeneration, candidate Session, preserveApplied bool) error {
	symbol = NormalizeProtocolState(ProtocolState{RecordedGeneration: symbol}).RecordedGeneration
	if !symbol.Valid || symbol.Revision < 0 || symbol.Revision > normalizeProtocolBounds(ProtocolBounds{}).MaxRevision || symbol.Authority&^protocolGrantMask(ProtocolBounds{}) != 0 {
		return fmt.Errorf("protocol generation binding is invalid")
	}
	if !samePersistentAuthority(a.original, candidate) {
		return fmt.Errorf("protocol generation changes immutable base authority")
	}
	generation, ok := canonicalEgressGeneration(candidate, candidate.EgressGrants, candidate.GrantRevision)
	if !ok {
		return fmt.Errorf("protocol concrete generation is invalid")
	}
	binding := protocolConcreteGeneration{
		grants:          append([]EgressGrant(nil), candidate.EgressGrants...),
		grantRevision:   candidate.GrantRevision,
		generation:      generation,
		appliedRevision: generation.Revision,
		appliedHash:     generation.Hash,
	}
	if preserveApplied {
		binding.appliedRevision = candidate.appliedEgressRevision
		binding.appliedHash = candidate.appliedEgressHash
	}
	if a.generations == nil {
		a.generations = make(map[ProtocolGeneration]protocolConcreteGeneration)
	}
	if existing, ok := a.generations[symbol]; ok && !sameProtocolConcreteGeneration(existing, binding) {
		return fmt.Errorf("protocol generation binding conflicts")
	}
	for existingSymbol, existing := range a.generations {
		if existingSymbol != symbol && existing.generation == binding.generation {
			return fmt.Errorf("protocol generation bindings must be injective")
		}
	}
	a.generations[symbol] = binding
	return nil
}

func sameProtocolConcreteGeneration(a, b protocolConcreteGeneration) bool {
	return a.grantRevision == b.grantRevision &&
		a.generation == b.generation &&
		a.appliedRevision == b.appliedRevision &&
		a.appliedHash == b.appliedHash &&
		sameGrantList(a.grants, b.grants)
}

func samePersistentAuthority(a, b Session) bool {
	if len(a.PersistentEgress) != len(b.PersistentEgress) {
		return false
	}
	for i := range a.PersistentEgress {
		if a.PersistentEgress[i] != b.PersistentEgress[i] {
			return false
		}
	}
	return true
}

// GenerationEqual classifies a positive concrete inspect result only by exact
// revision/hash equality with a previously bound symbolic generation.
func (a ProtocolAdapter) GenerationEqual(symbol ProtocolGeneration, concrete egress.Generation) bool {
	symbol = NormalizeProtocolState(ProtocolState{RecordedGeneration: symbol}).RecordedGeneration
	binding, ok := a.generations[symbol]
	return ok && binding.generation == concrete
}

// EffectCandidate constructs the bytes proposed at a tagged Commit boundary.
// The returned bytes are only a candidate: ReduceProtocol must still classify
// the commit as known-old, known-new, or unknown before the driver proceeds.
func (a ProtocolAdapter) EffectCandidate(state ProtocolState) (Session, error) {
	state = NormalizeProtocolState(state)
	if !protocolStateInDomain(ProtocolBounds{}, state) || state.Effect != ProtocolCommit || state.Health != ProtocolHealthy || state.Mode != ProtocolNormal || state.Result != ProtocolPending {
		return Session{}, fmt.Errorf("protocol state has no commit candidate")
	}
	target := state
	switch state.Operation {
	case ProtocolClaim:
		if state.Status != ProtocolCreated || state.Owners != 0 || state.Direction != ProtocolNoDirection {
			return Session{}, fmt.Errorf("protocol claim commit source is invalid")
		}
		target.Status, target.Owners, target.Detached = ProtocolRunning, ProtocolOwnerA, false
		target.Health, target.Mode = ProtocolHealthy, ProtocolNormal
		target = resetProtocol(target, ProtocolSuccess)
	case ProtocolWiden:
		if state.Status != ProtocolRunning || (state.Owners != ProtocolOwnerA && state.Owners != ProtocolOwnerB) || state.Direction != ProtocolWidenDirection {
			return Session{}, fmt.Errorf("protocol widen commit candidate lacks direction")
		}
		if !protocolPendingIsDurable(state) {
			target.DurableAuthority, target.DurableRevision = state.PendingAuthority, state.PendingRevision
			target.Effect = ProtocolApply
			break
		}
		if state.InspectedGeneration != protocolGeneration(state.PendingAuthority, state.PendingRevision) {
			return Session{}, fmt.Errorf("protocol widen final candidate lacks positive inspect")
		}
		target.RecordedGeneration = protocolGeneration(state.PendingAuthority, state.PendingRevision)
		target.Health, target.Mode = ProtocolHealthy, ProtocolNormal
		target = resetProtocol(target, ProtocolSuccess)
	case ProtocolNarrow:
		if state.Status != ProtocolRunning || (state.Owners != ProtocolOwnerA && state.Owners != ProtocolOwnerB) || state.Direction != ProtocolNarrowDirection || state.InspectedGeneration != protocolGeneration(state.PendingAuthority, state.PendingRevision) {
			return Session{}, fmt.Errorf("protocol narrow final candidate lacks positive inspect")
		}
		target.DurableAuthority, target.DurableRevision = state.PendingAuthority, state.PendingRevision
		target.RecordedGeneration = protocolGeneration(state.PendingAuthority, state.PendingRevision)
		target.Health, target.Mode = ProtocolHealthy, ProtocolNormal
		target = resetProtocol(target, ProtocolSuccess)
	default:
		return Session{}, fmt.Errorf("protocol commit candidate operation is unsupported")
	}
	return a.Candidate(target)
}

// CandidateFrom projects a stable state from the most recently committed
// record frame, preserving transaction revision and out-of-model metadata.
func (a ProtocolAdapter) CandidateFrom(state ProtocolState, frame Session) (Session, error) {
	framed, err := a.withRecordFrame(frame)
	if err != nil {
		return Session{}, err
	}
	return framed.projectCandidate(state, false)
}

// EffectCandidateFrom projects a later effect commit from the most recently
// committed record frame while retaining the original concrete bindings.
func (a ProtocolAdapter) EffectCandidateFrom(state ProtocolState, frame Session) (Session, error) {
	framed, err := a.withRecordFrame(frame)
	if err != nil {
		return Session{}, err
	}
	return framed.EffectCandidate(state)
}

func (a ProtocolAdapter) withRecordFrame(frame Session) (ProtocolAdapter, error) {
	if frame.ID != a.original.ID || frame.Status != a.original.Status || frame.PID != a.original.PID || frame.ProcessToken != a.original.ProcessToken || frame.Detached != a.original.Detached || !samePersistentAuthority(a.original, frame) || !validEgressRuntimeState(frame) {
		return ProtocolAdapter{}, fmt.Errorf("protocol candidate frame is incompatible")
	}
	a.original = frame
	return a, nil
}

// Candidate projects modeled durable fields back onto the original Session and
// leaves every out-of-model field byte-for-byte framed for the effect driver.
func (a ProtocolAdapter) Candidate(state ProtocolState) (Session, error) {
	return a.projectCandidate(state, true)
}

// TerminalCandidate re-applies terminal projection after TerminalStutter. It is
// used by Finish to preserve idempotent lifecycle state while clearing the same
// internal runtime fields that Finish historically cleared.
func (a ProtocolAdapter) TerminalCandidate(state ProtocolState) (Session, error) {
	state = NormalizeProtocolState(state)
	if state.Status != ProtocolStopped {
		return Session{}, fmt.Errorf("protocol terminal candidate requires Stopped")
	}
	state.Owners, state.Detached, state.RuntimeAuthority = 0, false, 0
	state.RuntimeGeneration, state.InspectedGeneration = protocolNoGeneration(), protocolNoGeneration()
	state.Health, state.Mode = ProtocolHealthy, ProtocolTerminal
	state.Operation, state.Effect, state.Result = ProtocolIdle, ProtocolNoEffect, ProtocolFailure
	state.PendingAuthority, state.PendingRevision, state.Direction = 0, 0, ProtocolNoDirection
	return a.projectCandidate(state, false)
}

func (a ProtocolAdapter) projectCandidate(state ProtocolState, preserveBaseline bool) (Session, error) {
	state = NormalizeProtocolState(state)
	if preserveBaseline && state == a.baseline {
		return a.original, nil
	}
	if !protocolStateInDomain(ProtocolBounds{}, state) {
		return Session{}, fmt.Errorf("protocol candidate state is invalid")
	}
	if state.Health != ProtocolHealthy || state.Mode == ProtocolBlocked || state.Operation == ProtocolClaim || state.Operation == ProtocolRecover || state.Operation == ProtocolStop || state.Operation == ProtocolTeardown || (state.Effect == ProtocolCommit && (state.Operation == ProtocolWiden || state.Operation == ProtocolNarrow)) {
		return Session{}, fmt.Errorf("protocol state requires an effect driver before persistence")
	}

	candidate := a.original
	switch state.Status {
	case ProtocolCreated:
		if state.Owners != 0 || state.Detached {
			return Session{}, fmt.Errorf("protocol created candidate has an owner")
		}
		candidate.Status, candidate.PID, candidate.ProcessToken, candidate.Detached = StatusCreated, 0, "", false
		if a.baseline.Status == ProtocolRunning {
			candidate.StartedAt = time.Time{}
		}
	case ProtocolRunning:
		if state.Owners != ProtocolOwnerA && state.Owners != ProtocolOwnerB {
			return Session{}, fmt.Errorf("protocol running candidate lacks one owner")
		}
		owner, ok := a.owners[state.Owners]
		if !ok {
			return Session{}, fmt.Errorf("protocol concrete owner is unbound")
		}
		detached := state.Detached
		if a.claimDetached != nil && a.original.Status == StatusCreated && state.Owners == ProtocolOwnerA {
			detached = *a.claimDetached
		}
		candidate.Status, candidate.PID, candidate.ProcessToken, candidate.Detached = StatusRunning, owner.pid, owner.token, detached
	case ProtocolStopped:
		if state.Owners != 0 || state.Detached {
			return Session{}, fmt.Errorf("protocol stopped candidate has an owner")
		}
		// Detached is a historical concrete signal-routing byte. Terminal
		// protocol state normalizes it away, while existing persisted behavior
		// retains the byte after PID/token authority is cleared.
		candidate.Status, candidate.PID, candidate.ProcessToken, candidate.Detached = StatusStopped, 0, "", a.original.Detached
	default:
		return Session{}, fmt.Errorf("protocol candidate status is invalid")
	}

	durable, ok := a.generations[protocolGeneration(state.DurableAuthority, state.DurableRevision)]
	if !ok {
		return Session{}, fmt.Errorf("protocol durable generation is unbound")
	}
	candidate.EgressGrants = append([]EgressGrant(nil), durable.grants...)
	candidate.GrantRevision = durable.grantRevision

	runtimeState := EgressRuntimeState{}
	if state.RecordedGeneration.Valid {
		recorded, ok := a.generations[state.RecordedGeneration]
		if !ok {
			return Session{}, fmt.Errorf("protocol recorded generation is unbound")
		}
		runtimeState.AppliedRevision, runtimeState.AppliedHash = recorded.appliedRevision, recorded.appliedHash
	}
	if state.Direction != ProtocolNoDirection {
		pending, ok := a.generations[protocolGeneration(state.PendingAuthority, state.PendingRevision)]
		if !ok {
			return Session{}, fmt.Errorf("protocol pending generation is unbound")
		}
		direction := EgressDirectionWiden
		if state.Direction == ProtocolNarrowDirection {
			direction = EgressDirectionNarrow
		}
		runtimeState.Transition = &EgressTransition{
			Direction:         direction,
			CandidateRevision: pending.grantRevision,
			CandidateHash:     pending.generation.Hash,
			CandidateGrants:   append([]EgressGrant(nil), pending.grants...),
		}
	}
	if state.Status == ProtocolStopped {
		runtimeState = EgressRuntimeState{}
	}
	candidate.SetEgressRuntimeState(runtimeState)
	if !validEgressRuntimeState(candidate) {
		return Session{}, fmt.Errorf("protocol candidate egress state is invalid")
	}
	return candidate, nil
}
