package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/freakhill/safeslop/internal/engine/container"
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

func runRunningSessionNarrowProtocol(d *dependencies, ctx context.Context, tx egressRecordTransaction, current, next engsession.Session) error {
	adapter, _ := engsession.NewProtocolAdapter(current)
	oldSymbol := engsession.ProtocolGeneration{Authority: engsession.ProtocolGrantSet(engsession.ProtocolGrantA), Revision: 0, Valid: true}
	newSymbol := engsession.ProtocolGeneration{Revision: 1, Valid: true}
	baseState, err := adapter.RebaseGeneration(oldSymbol)
	if err != nil {
		return err
	}
	if err := adapter.BindGeneration(newSymbol, next); err != nil {
		return err
	}
	state, ok := engsession.ReduceProtocol(baseState, engsession.ProtocolEvent{Action: engsession.ProtocolNarrowStart, Grant: engsession.ProtocolGrantA})
	if !ok || state.Effect != engsession.ProtocolApply {
		return fmt.Errorf("session protocol narrow start is invalid")
	}
	intent, err := adapter.Candidate(state)
	if err != nil {
		return err
	}
	commitErr := tx.Commit(intent)
	switch {
	case commitErr == nil:
	case errors.Is(commitErr, engsession.ErrCommitUncertain):
		// Old and intent worlds have the same durable authority. Retain the
		// intent frame if teardown is unproven rather than selecting either.
		return failClosedEgressWithUpperBound(d, tx, &current, &intent)
	default:
		return commitErr
	}

	application, err := d.beginEgressOverlayApply(ctx, next, sessionEgressViews(next))
	if err != nil {
		return restoreNarrowAfterApplyFailure(d, ctx, tx, adapter, baseState, current, err)
	}
	state, ok = engsession.ReduceProtocol(state, engsession.ProtocolEvent{Action: engsession.ProtocolApplyAction, Outcome: engsession.ProtocolSucceeded})
	if !ok || state.Effect != engsession.ProtocolInspect {
		return failClosedEgressWithDeps(d, tx, &current)
	}
	inspected, inspectErr := application.Inspect(ctx)
	if inspectErr != nil || !adapter.GenerationEqual(newSymbol, inspected) {
		if inspectErr == nil {
			inspectErr = container.ErrEgressGenerationUncertain
		}
		return restoreNarrowAfterApplyFailure(d, ctx, tx, adapter, baseState, current, inspectErr)
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
		state, ok = engsession.ReduceProtocol(state, engsession.ProtocolEvent{Action: engsession.ProtocolNarrowFinalCommit, Outcome: engsession.ProtocolKnownNew})
		if !ok || state.Effect != engsession.ProtocolNoEffect || state.Result != engsession.ProtocolSuccess {
			return failClosedEgressWithDeps(d, tx, &current)
		}
		return nil
	case errors.Is(commitErr, engsession.ErrCommitUncertain):
		if !protocolUnknownCommitBlocked(state, engsession.ProtocolNarrowFinalCommit) {
			return fmt.Errorf("session protocol narrow final uncertain commit is invalid")
		}
		// The narrower record may already be durable. Never restore the wider
		// runtime across this uncertainty.
		return failClosedEgressWithDeps(d, tx, nil)
	default:
		knownOld, reduced := engsession.ReduceProtocol(state, engsession.ProtocolEvent{Action: engsession.ProtocolNarrowFinalCommit, Outcome: engsession.ProtocolKnownOld})
		if !reduced || knownOld.Result != engsession.ProtocolFailure {
			return fmt.Errorf("session protocol narrow final known-old commit is invalid")
		}
		if restoreErr := d.applyEgressOverlay(ctx, current, sessionEgressViews(current)); restoreErr != nil {
			return failClosedEgressWithDeps(d, tx, &current)
		}
		restored, projectionErr := adapter.CandidateFrom(knownOld, tx.Session())
		if projectionErr != nil {
			return failClosedEgressWithDeps(d, tx, &current)
		}
		// This cleanup commit changes no durable/runtime authority. Its unknown
		// result may retain either safe old-intent or stable-old record.
		_ = tx.Commit(restored)
		return commitErr
	}
}

func restoreNarrowAfterApplyFailure(d *dependencies, ctx context.Context, tx egressRecordTransaction, adapter engsession.ProtocolAdapter, baseState engsession.ProtocolState, current engsession.Session, applyErr error) error {
	if restoreErr := d.applyEgressOverlay(ctx, current, sessionEgressViews(current)); restoreErr != nil {
		return failClosedEgressWithDeps(d, tx, &current)
	}
	restored, err := adapter.CandidateFrom(baseState, tx.Session())
	if err != nil {
		return failClosedEgressWithDeps(d, tx, &current)
	}
	if clearErr := tx.Commit(restored); errors.Is(clearErr, engsession.ErrCommitUncertain) {
		return failClosedEgressWithDeps(d, tx, &current)
	}
	return applyErr
}

func protocolUnknownCommitBlocked(state engsession.ProtocolState, action engsession.ProtocolAction) bool {
	oldWorld, newWorld, ok := engsession.ReduceProtocolUnknownCommit(state, action)
	return ok && oldWorld != newWorld &&
		oldWorld.Health == engsession.ProtocolUncertain && oldWorld.Mode == engsession.ProtocolBlocked &&
		newWorld.Health == engsession.ProtocolUncertain && newWorld.Mode == engsession.ProtocolBlocked
}
