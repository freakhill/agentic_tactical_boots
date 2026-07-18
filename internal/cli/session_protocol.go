package cli

import (
	"context"
	"errors"
	"fmt"

	engsession "github.com/freakhill/safeslop/internal/engine/session"
)

func runRunningSessionWidenProtocol(d *dependencies, ctx context.Context, tx egressRecordTransaction, current, next engsession.Session) error {
	adapter, _ := engsession.NewProtocolAdapter(current)
	oldSymbol := engsession.ProtocolGeneration{Revision: 0, Valid: true}
	newSymbol := engsession.ProtocolGeneration{Authority: engsession.ProtocolGrantSet(engsession.ProtocolGrantA), Revision: 1, Valid: true}
	state, err := adapter.RebaseGeneration(oldSymbol)
	if err != nil {
		return err
	}
	if err := adapter.BindGeneration(newSymbol, next); err != nil {
		return err
	}
	state, ok := engsession.ReduceProtocol(state, engsession.ProtocolEvent{Action: engsession.ProtocolWidenStart, Grant: engsession.ProtocolGrantA})
	if !ok || state.Effect != engsession.ProtocolCommit {
		return fmt.Errorf("session protocol widen start is invalid")
	}

	pending, err := adapter.EffectCandidate(state)
	if err != nil {
		return err
	}
	pending.UpdatedAt = next.UpdatedAt
	commitErr := tx.Commit(pending)
	switch {
	case commitErr == nil:
		state, ok = engsession.ReduceProtocol(state, engsession.ProtocolEvent{Action: engsession.ProtocolWidenCommit, Outcome: engsession.ProtocolKnownNew})
		if !ok || state.Effect != engsession.ProtocolApply {
			return fmt.Errorf("session protocol widen durable commit is invalid")
		}
	case errors.Is(commitErr, engsession.ErrCommitUncertain):
		if !protocolUnknownCommitBlocked(state, engsession.ProtocolWidenCommit) {
			return fmt.Errorf("session protocol widen uncertain commit is invalid")
		}
		return failClosedEgressWithUpperBound(d, tx, &current, &pending)
	default:
		if _, ok := engsession.ReduceProtocol(state, engsession.ProtocolEvent{Action: engsession.ProtocolWidenCommit, Outcome: engsession.ProtocolKnownOld}); !ok {
			return fmt.Errorf("session protocol widen known-old commit is invalid")
		}
		return commitErr
	}

	application, err := d.beginEgressOverlayApply(ctx, next, sessionEgressViews(next))
	if err != nil {
		return failClosedEgressWithDeps(d, tx, &current)
	}
	state, ok = engsession.ReduceProtocol(state, engsession.ProtocolEvent{Action: engsession.ProtocolApplyAction, Outcome: engsession.ProtocolSucceeded})
	if !ok || state.Effect != engsession.ProtocolInspect {
		return failClosedEgressWithDeps(d, tx, &current)
	}
	inspected, inspectErr := application.Inspect(ctx)
	if inspectErr != nil || !adapter.GenerationEqual(newSymbol, inspected) {
		return failClosedEgressWithDeps(d, tx, &current)
	}
	state, ok = engsession.ReduceProtocol(state, engsession.ProtocolEvent{Action: engsession.ProtocolInspectAction, Outcome: engsession.ProtocolMatched})
	if !ok || state.Effect != engsession.ProtocolCommit {
		return failClosedEgressWithDeps(d, tx, &current)
	}

	final, err := adapter.EffectCandidateFrom(state, tx.Session())
	if err != nil {
		return failClosedEgressWithDeps(d, tx, &current)
	}
	commitErr = tx.Commit(final)
	switch {
	case commitErr == nil:
		state, ok = engsession.ReduceProtocol(state, engsession.ProtocolEvent{Action: engsession.ProtocolWidenFinalCommit, Outcome: engsession.ProtocolKnownNew})
		if !ok || state.Effect != engsession.ProtocolNoEffect || state.Result != engsession.ProtocolSuccess {
			return failClosedEgressWithDeps(d, tx, &current)
		}
		return nil
	case errors.Is(commitErr, engsession.ErrCommitUncertain):
		if !protocolUnknownCommitBlocked(state, engsession.ProtocolWidenFinalCommit) {
			return fmt.Errorf("session protocol widen final uncertain commit is invalid")
		}
	default:
		knownOld, reduced := engsession.ReduceProtocol(state, engsession.ProtocolEvent{Action: engsession.ProtocolWidenFinalCommit, Outcome: engsession.ProtocolKnownOld})
		if !reduced || knownOld.Health != engsession.ProtocolUncertain || knownOld.Mode != engsession.ProtocolBlocked {
			return fmt.Errorf("session protocol widen final known-old commit is invalid")
		}
	}
	return failClosedEgressWithDeps(d, tx, &current)
}

func protocolUnknownCommitBlocked(state engsession.ProtocolState, action engsession.ProtocolAction) bool {
	oldWorld, newWorld, ok := engsession.ReduceProtocolUnknownCommit(state, action)
	return ok && oldWorld != newWorld &&
		oldWorld.Health == engsession.ProtocolUncertain && oldWorld.Mode == engsession.ProtocolBlocked &&
		newWorld.Health == engsession.ProtocolUncertain && newWorld.Mode == engsession.ProtocolBlocked
}
