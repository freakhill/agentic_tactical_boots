package session

import (
	"reflect"
	"testing"
	"time"
)

func TestProtocolInitialStateAndEnabledAreClosed(t *testing.T) {
	initial := InitialProtocolStates(ProtocolBounds{})
	if len(initial) != 1 || initial[0].Status != ProtocolCreated || initial[0].Operation != ProtocolIdle {
		t.Fatalf("initial states = %+v", initial)
	}
	events := EnabledProtocol(initial[0])
	if len(events) == 0 {
		t.Fatal("created initial state has no enabled event")
	}
	for _, event := range events {
		if _, ok := ReduceProtocol(initial[0], event); !ok {
			t.Fatalf("Enabled returned rejected event %+v", event)
		}
	}
}

func TestProtocolReducerWidenRequiresDurableBeforeRuntime(t *testing.T) {
	s := runningProtocolState()
	s, ok := ReduceProtocol(s, ProtocolEvent{Action: ProtocolWidenStart, Grant: ProtocolGrantA})
	if !ok || s.Effect != ProtocolCommit || s.DurableAuthority.Has(ProtocolGrantA) {
		t.Fatalf("widen start = %+v, ok=%v", s, ok)
	}
	s, ok = ReduceProtocol(s, ProtocolEvent{Action: ProtocolWidenCommit, Outcome: ProtocolKnownNew})
	if !ok || !s.DurableAuthority.Has(ProtocolGrantA) || s.Effect != ProtocolApply {
		t.Fatalf("widen commit = %+v, ok=%v", s, ok)
	}
	s, ok = ReduceProtocol(s, ProtocolEvent{Action: ProtocolApplyAction, Outcome: ProtocolSucceeded})
	if !ok || !s.RuntimeAuthority.Has(ProtocolGrantA) || s.Effect != ProtocolInspect {
		t.Fatalf("widen apply = %+v, ok=%v", s, ok)
	}
}

func TestProtocolReducerNarrowFinalUnknownBlocksWithoutWideningRuntime(t *testing.T) {
	s := runningProtocolState()
	s.DurableAuthority = ProtocolGrantSet(ProtocolGrantA)
	s.DurableRevision = 1
	s.RecordedGeneration = protocolGeneration(s.DurableAuthority, 1)
	s.RuntimeAuthority = s.DurableAuthority
	s.RuntimeGeneration = s.RecordedGeneration
	s.InspectedGeneration = s.RecordedGeneration

	steps := []ProtocolEvent{
		{Action: ProtocolNarrowStart, Grant: ProtocolGrantA},
		{Action: ProtocolApplyAction, Outcome: ProtocolSucceeded},
		{Action: ProtocolInspectAction, Outcome: ProtocolMatched},
		{Action: ProtocolNarrowFinalCommit, Outcome: ProtocolCommitUnknown, Truth: ProtocolOld},
	}
	var ok bool
	for _, step := range steps {
		s, ok = ReduceProtocol(s, step)
		if !ok {
			t.Fatalf("step %+v rejected in %+v", step, s)
		}
	}
	if s.Mode != ProtocolBlocked || s.Health != ProtocolUncertain || !s.RuntimeAuthority.Empty() {
		t.Fatalf("unknown narrow final = %+v", s)
	}
}

func TestProtocolReducerLifecycleRecoveryAndExactOwnership(t *testing.T) {
	initial := InitialProtocolStates(ProtocolBounds{})[0]
	claiming, ok := ReduceProtocol(initial, ProtocolEvent{Action: ProtocolClaimStart})
	if !ok {
		t.Fatal("claim start rejected")
	}
	unknown, ok := ReduceProtocol(claiming, ProtocolEvent{Action: ProtocolClaimCommit, Outcome: ProtocolCommitUnknown, Truth: ProtocolOld})
	if !ok || unknown.Health != ProtocolUncertain || unknown.Mode != ProtocolBlocked || unknown.Result != ProtocolUncertainResult {
		t.Fatalf("unknown claim = %+v, ok=%v", unknown, ok)
	}
	recovered, ok := ReduceProtocol(unknown, ProtocolEvent{Action: ProtocolRecoverClaim})
	if !ok || recovered.Status != ProtocolCreated || recovered.Mode != ProtocolNormal || recovered.Result != ProtocolFailure {
		t.Fatalf("recovered old claim = %+v, ok=%v", recovered, ok)
	}

	claiming, ok = ReduceProtocol(recovered, ProtocolEvent{Action: ProtocolClaimStart})
	if !ok {
		t.Fatal("second claim start rejected")
	}
	running, ok := ReduceProtocol(claiming, ProtocolEvent{Action: ProtocolClaimCommit, Outcome: ProtocolKnownNew})
	if !ok || running.Owners != ProtocolOwnerA {
		t.Fatalf("known-new claim = %+v, ok=%v", running, ok)
	}
	handedOff, ok := ReduceProtocol(running, ProtocolEvent{Action: ProtocolHandoffExact})
	if !ok || handedOff.Owners != ProtocolOwnerB || !handedOff.Detached {
		t.Fatalf("handoff = %+v, ok=%v", handedOff, ok)
	}
	if _, ok := ReduceProtocol(handedOff, ProtocolEvent{Action: ProtocolReleaseExact}); ok {
		t.Fatal("OwnerB release unexpectedly enabled")
	}
	released, ok := ReduceProtocol(running, ProtocolEvent{Action: ProtocolReleaseExact})
	if !ok || released.Status != ProtocolCreated || released.Owners != 0 || released.Detached || released.Result != ProtocolFailure {
		t.Fatalf("release = %+v, ok=%v", released, ok)
	}
}

func TestProtocolReducerWidenFinalCommitAndRecovery(t *testing.T) {
	start := runningProtocolState()
	steps := []ProtocolEvent{
		{Action: ProtocolWidenStart, Grant: ProtocolGrantA},
		{Action: ProtocolWidenCommit, Outcome: ProtocolKnownNew},
		{Action: ProtocolApplyAction, Outcome: ProtocolSucceeded},
		{Action: ProtocolInspectAction, Outcome: ProtocolMatched},
	}
	state := start
	var ok bool
	for _, step := range steps {
		state, ok = ReduceProtocol(state, step)
		if !ok {
			t.Fatalf("step %+v rejected in %+v", step, state)
		}
	}
	if _, ok := ReduceProtocol(state, ProtocolEvent{Action: ProtocolWidenCommit, Outcome: ProtocolKnownNew}); ok {
		t.Fatal("initial widen commit accepted at final commit boundary")
	}
	committed, ok := ReduceProtocol(state, ProtocolEvent{Action: ProtocolWidenFinalCommit, Outcome: ProtocolKnownNew})
	want := protocolGeneration(ProtocolGrantSet(ProtocolGrantA), 1)
	if !ok || committed.Operation != ProtocolIdle || committed.RecordedGeneration != want || committed.Result != ProtocolSuccess {
		t.Fatalf("widen final commit = %+v, ok=%v", committed, ok)
	}

	knownOld, ok := ReduceProtocol(state, ProtocolEvent{Action: ProtocolWidenFinalCommit, Outcome: ProtocolKnownOld})
	if !ok || knownOld.Mode != ProtocolBlocked || knownOld.Health != ProtocolUncertain || knownOld.Result != ProtocolFailure {
		t.Fatalf("known-old widen final = %+v, ok=%v", knownOld, ok)
	}
	recovered, ok := ReduceProtocol(knownOld, ProtocolEvent{Action: ProtocolRecoverEgress})
	if !ok || recovered.Mode != ProtocolNormal || recovered.RecordedGeneration != want || recovered.Result != ProtocolSuccess {
		t.Fatalf("recovered widen final = %+v, ok=%v", recovered, ok)
	}

	if _, ok := ReduceProtocol(state, ProtocolEvent{Action: ProtocolWidenFinalCommit, Outcome: ProtocolCommitUnknown}); ok {
		t.Fatal("unknown final commit accepted without an old/new truth branch")
	}
	unknown, ok := ReduceProtocol(state, ProtocolEvent{Action: ProtocolWidenFinalCommit, Outcome: ProtocolCommitUnknown, Truth: ProtocolOld})
	if !ok || unknown.Mode != ProtocolBlocked || unknown.Result != ProtocolUncertainResult {
		t.Fatalf("unknown widen final = %+v, ok=%v", unknown, ok)
	}
}

func TestProtocolReducerInvalidPersistenceStopAndTeardown(t *testing.T) {
	initial := InitialProtocolStates(ProtocolBounds{})[0]
	blocked, ok := ReduceProtocol(initial, ProtocolEvent{Action: ProtocolObserveInvalidPersistence, Outcome: ProtocolObservedCorrupt})
	if !ok || blocked.Health != ProtocolCorrupt || blocked.Mode != ProtocolBlocked || blocked.Result != ProtocolFailure {
		t.Fatalf("invalid persistence = %+v, ok=%v", blocked, ok)
	}
	tearingDown, ok := ReduceProtocol(blocked, ProtocolEvent{Action: ProtocolRequestTeardown})
	if !ok || tearingDown.Operation != ProtocolTeardown || tearingDown.Effect != ProtocolTeardownEffect {
		t.Fatalf("request teardown = %+v, ok=%v", tearingDown, ok)
	}
	unknown, ok := ReduceProtocol(tearingDown, ProtocolEvent{Action: ProtocolTeardownAction, Outcome: ProtocolTeardownUnknown})
	if !ok || unknown.Mode != ProtocolBlocked || unknown.Result != ProtocolUncertainResult {
		t.Fatalf("unknown teardown = %+v, ok=%v", unknown, ok)
	}
	terminal, ok := ReduceProtocol(unknown, ProtocolEvent{Action: ProtocolTeardownAction, Outcome: ProtocolTeardownProven})
	if !ok || terminal.Status != ProtocolStopped || terminal.Mode != ProtocolTerminal || terminal.Owners != 0 || !terminal.RuntimeAuthority.Empty() || terminal.RuntimeGeneration.Valid {
		t.Fatalf("proven teardown = %+v, ok=%v", terminal, ok)
	}
	stuttered, ok := ReduceProtocol(terminal, ProtocolEvent{Action: ProtocolTerminalStutter})
	if !ok || stuttered != terminal {
		t.Fatalf("terminal stutter = %+v, ok=%v", stuttered, ok)
	}

	running := runningProtocolState()
	stopping, ok := ReduceProtocol(running, ProtocolEvent{Action: ProtocolStopStart})
	if !ok || stopping.Operation != ProtocolStop || stopping.Effect != ProtocolTeardownEffect {
		t.Fatalf("stop start = %+v, ok=%v", stopping, ok)
	}
}

func TestProtocolNormalizeCanonicalizesNoGeneration(t *testing.T) {
	state := runningProtocolState()
	state.RuntimeGeneration = ProtocolGeneration{Revision: 99}
	state.InspectedGeneration = ProtocolGeneration{Authority: ProtocolGrantSet(ProtocolGrantA)}
	normalized := NormalizeProtocolState(state)
	want := ProtocolGeneration{Revision: -1}
	if normalized.RuntimeGeneration != want || normalized.InspectedGeneration != want {
		t.Fatalf("normalized generations = %+v / %+v", normalized.RuntimeGeneration, normalized.InspectedGeneration)
	}
}

func TestProtocolEveryModelActionIsReachable(t *testing.T) {
	want := map[ProtocolLabel]bool{
		ProtocolLabelClaimStart: true, ProtocolLabelClaimCommitNew: true, ProtocolLabelClaimCommitOld: true,
		ProtocolLabelClaimCommitUnknown: true, ProtocolLabelRecoverClaim: true, ProtocolLabelHandoffExact: true,
		ProtocolLabelReleaseExact: true, ProtocolLabelWidenStart: true, ProtocolLabelWidenCommitNew: true,
		ProtocolLabelWidenCommitOld: true, ProtocolLabelWidenCommitUnknown: true, ProtocolLabelWidenApply: true,
		ProtocolLabelPositiveInspect: true, ProtocolLabelWidenFinalCommitNew: true, ProtocolLabelWidenFinalCommitOld: true,
		ProtocolLabelWidenFinalCommitUnknown: true, ProtocolLabelNarrowStart: true, ProtocolLabelNarrowApply: true,
		ProtocolLabelNarrowFinalCommitNew: true, ProtocolLabelNarrowFinalCommitOld: true,
		ProtocolLabelNarrowFinalCommitUnknown: true, ProtocolLabelRecoverEgress: true,
		ProtocolLabelObserveInvalidPersistence: true, ProtocolLabelStopStart: true,
		ProtocolLabelRequestTeardown: true, ProtocolLabelTeardownProven: true,
		ProtocolLabelTeardownUnknown: true, ProtocolLabelTerminalStutter: true,
	}
	seenLabels := make(map[ProtocolLabel]bool)
	seenStates := make(map[ProtocolState]bool)
	queue := append([]ProtocolState(nil), InitialProtocolStates(ProtocolBounds{})...)
	for len(queue) > 0 {
		state := queue[0]
		queue = queue[1:]
		state = NormalizeProtocolState(state)
		if seenStates[state] {
			continue
		}
		seenStates[state] = true
		for _, event := range EnabledProtocol(state) {
			next, ok := ReduceProtocol(state, event)
			if !ok {
				t.Fatalf("enabled event rejected: %+v", event)
			}
			label, ok := ProtocolEventLabel(state, event)
			if !ok {
				t.Fatalf("enabled event has no label: %+v", event)
			}
			seenLabels[label] = true
			queue = append(queue, NormalizeProtocolState(next))
		}
	}
	if !reflect.DeepEqual(seenLabels, want) {
		t.Fatalf("reachable labels = %#v\nwant %#v", seenLabels, want)
	}
}

func TestProtocolAdapterRoundTripPreservesConcreteSession(t *testing.T) {
	now := time.Date(2026, 7, 18, 1, 0, 0, 0, time.UTC)
	original := Session{ID: "sess-a", Agent: "pi", Environment: "container", Network: "deny", Status: StatusRunning, PID: 41, ProcessToken: "token-a", Detached: true, CreatedAt: now, UpdatedAt: now, GrantRevision: 0}
	adapter, state := NewProtocolAdapter(original)
	got, err := adapter.Candidate(state)
	if err != nil {
		t.Fatalf("candidate: %v", err)
	}
	if !reflect.DeepEqual(got, original) {
		t.Fatalf("round trip changed session:\n got=%+v\nwant=%+v", got, original)
	}
}

func TestProtocolAdapterBuildsLifecycleAndGenerationCandidates(t *testing.T) {
	now := time.Date(2026, 7, 18, 2, 0, 0, 0, time.UTC)
	original := Session{
		ID: "sess-adapter", Agent: "pi", Status: StatusRunning, PID: 41, ProcessToken: "token-a",
		Detached: false, StartedAt: now, CreatedAt: now, UpdatedAt: now, GrantRevision: 7,
		recordRevision: 19, runtimeID: "sess-adapter", stageLayout: StageLayoutSessionID,
	}
	adapter, state := NewProtocolAdapter(original)
	adapter.BindOwner(ProtocolOwnerB, 52, "token-b")
	handedOff, ok := ReduceProtocol(state, ProtocolEvent{Action: ProtocolHandoffExact})
	if !ok {
		t.Fatal("handoff rejected")
	}
	candidate, err := adapter.Candidate(handedOff)
	if err != nil {
		t.Fatalf("handoff candidate: %v", err)
	}
	if candidate.PID != 52 || candidate.ProcessToken != "token-b" || !candidate.Detached || candidate.recordRevision != original.recordRevision || candidate.GrantRevision != original.GrantRevision {
		t.Fatalf("handoff candidate = %+v", candidate)
	}

	released, ok := ReduceProtocol(state, ProtocolEvent{Action: ProtocolReleaseExact})
	if !ok {
		t.Fatal("release rejected")
	}
	candidate, err = adapter.Candidate(released)
	if err != nil {
		t.Fatalf("release candidate: %v", err)
	}
	if candidate.Status != StatusCreated || candidate.PID != 0 || candidate.ProcessToken != "" || !candidate.StartedAt.IsZero() || candidate.recordRevision != original.recordRevision || candidate.GrantRevision != original.GrantRevision {
		t.Fatalf("release candidate = %+v", candidate)
	}

	grant := EgressGrant{ID: "g-000001", Host: "api.example.com", Port: 443, Source: "operator", CreatedAt: now}
	nextConcrete := original
	nextConcrete.EgressGrants = []EgressGrant{grant}
	nextConcrete.GrantRevision = 8
	symbolic := protocolGeneration(ProtocolGrantSet(ProtocolGrantA), 1)
	if err := adapter.BindGeneration(symbolic, nextConcrete); err != nil {
		t.Fatalf("bind generation: %v", err)
	}
	concreteGeneration, ok := canonicalEgressGeneration(nextConcrete, nextConcrete.EgressGrants, nextConcrete.GrantRevision)
	if !ok || !adapter.GenerationEqual(symbolic, concreteGeneration) {
		t.Fatalf("symbolic generation did not match concrete: %+v, ok=%v", concreteGeneration, ok)
	}
	concreteGeneration.Revision++
	if adapter.GenerationEqual(symbolic, concreteGeneration) {
		t.Fatal("symbolic generation matched a different concrete revision")
	}
}

func TestProtocolAdapterClaimStatePreservesStaleDetachedAndTokenlessCompatibility(t *testing.T) {
	original := Session{ID: "sess-claim-compat", Status: StatusCreated, PID: 99, ProcessToken: "stale", Detached: true}
	adapter, mapped := NewProtocolAdapter(original)
	if mapped.Mode != ProtocolBlocked {
		t.Fatalf("stale baseline unexpectedly normal: %+v", mapped)
	}
	state, err := adapter.ClaimState()
	if err != nil {
		t.Fatalf("claim state: %v", err)
	}
	if state.Mode != ProtocolNormal || state.Status != ProtocolCreated || state.Owners != 0 || state.Detached {
		t.Fatalf("claim state = %+v", state)
	}
	if err := adapter.BindClaimOwner(41, "", true); err != nil {
		t.Fatalf("bind tokenless detached claim owner: %v", err)
	}
	state, ok := ReduceProtocol(state, ProtocolEvent{Action: ProtocolClaimStart})
	if !ok {
		t.Fatal("claim start rejected")
	}
	candidate, err := adapter.EffectCandidate(state)
	if err != nil {
		t.Fatalf("claim candidate: %v", err)
	}
	if candidate.Status != StatusRunning || candidate.PID != 41 || candidate.ProcessToken != "" || !candidate.Detached {
		t.Fatalf("claim compatibility candidate = %+v", candidate)
	}
}

func TestProtocolAdapterLegacyRunningStatePreservesLifecycleCompatibility(t *testing.T) {
	original := Session{ID: "sess-owner-compat", Status: StatusRunning, PID: 41}
	adapter, mapped := NewProtocolAdapter(original)
	if mapped.Mode != ProtocolBlocked {
		t.Fatalf("legacy baseline unexpectedly normal: %+v", mapped)
	}
	state, err := adapter.LegacyRunningState()
	if err != nil {
		t.Fatalf("legacy running state: %v", err)
	}
	if state.Mode != ProtocolNormal || state.Owners != ProtocolOwnerA {
		t.Fatalf("legacy running state = %+v", state)
	}
	released, ok := ReduceProtocol(state, ProtocolEvent{Action: ProtocolReleaseExact})
	if !ok {
		t.Fatal("legacy compatibility release rejected")
	}
	candidate, err := adapter.Candidate(released)
	if err != nil {
		t.Fatalf("legacy release candidate: %v", err)
	}
	if candidate.Status != StatusCreated || candidate.PID != 0 || candidate.ProcessToken != "" {
		t.Fatalf("legacy release candidate = %+v", candidate)
	}

	adapter, _ = NewProtocolAdapter(original)
	state, err = adapter.LegacyRunningState()
	if err != nil {
		t.Fatalf("legacy handoff state: %v", err)
	}
	if err := adapter.BindHandoffOwner(52, ""); err != nil {
		t.Fatalf("bind tokenless handoff owner: %v", err)
	}
	handedOff, ok := ReduceProtocol(state, ProtocolEvent{Action: ProtocolHandoffExact})
	if !ok {
		t.Fatal("legacy compatibility handoff rejected")
	}
	candidate, err = adapter.Candidate(handedOff)
	if err != nil {
		t.Fatalf("legacy handoff candidate: %v", err)
	}
	if candidate.PID != 52 || candidate.ProcessToken != "" || !candidate.Detached {
		t.Fatalf("legacy handoff candidate = %+v", candidate)
	}
}

func TestProtocolExactHandoffAndReleaseRejectDetachedSource(t *testing.T) {
	state := runningProtocolState()
	state.Detached = true
	if _, ok := ReduceProtocol(state, ProtocolEvent{Action: ProtocolHandoffExact}); ok {
		t.Fatal("handoff accepted an already-detached source")
	}
	if _, ok := ReduceProtocol(state, ProtocolEvent{Action: ProtocolReleaseExact}); ok {
		t.Fatal("release accepted an already-detached source")
	}
}

func TestProtocolAdapterBlockedTeardownStateReachesOnlyProvenTerminal(t *testing.T) {
	for _, original := range []Session{
		{ID: "created-teardown", Status: StatusCreated},
		{ID: "stale-running-teardown", Status: StatusRunning},
	} {
		adapter, _ := NewProtocolAdapter(original)
		state, err := adapter.BlockedTeardownState()
		if err != nil {
			t.Fatalf("%s blocked teardown state: %v", original.ID, err)
		}
		if state.Mode != ProtocolBlocked || state.Effect == ProtocolTeardownEffect {
			t.Fatalf("%s blocked state = %+v", original.ID, state)
		}
		state, ok := ReduceProtocol(state, ProtocolEvent{Action: ProtocolRequestTeardown})
		if !ok || state.Effect != ProtocolTeardownEffect {
			t.Fatalf("%s request teardown = %+v, ok=%v", original.ID, state, ok)
		}
		terminal, ok := ReduceProtocol(state, ProtocolEvent{Action: ProtocolTeardownAction, Outcome: ProtocolTeardownProven})
		if !ok {
			t.Fatalf("%s proven teardown rejected", original.ID)
		}
		candidate, err := adapter.Candidate(terminal)
		if err != nil {
			t.Fatalf("%s terminal candidate: %v", original.ID, err)
		}
		if candidate.Status != StatusStopped || candidate.PID != 0 || candidate.EgressRuntimeState() != (EgressRuntimeState{}) {
			t.Fatalf("%s terminal candidate = %+v", original.ID, candidate)
		}
	}
}

func TestProtocolAdapterTerminalCandidateCanRestoreProvenAuthority(t *testing.T) {
	now := time.Date(2026, 7, 18, 6, 30, 0, 0, time.UTC)
	grant := EgressGrant{ID: "g-000001", Host: "api.example.com", Port: 443, Source: "operator", CreatedAt: now}
	original := Session{ID: "sess-terminal-restore", Environment: "container", Network: "deny", Status: StatusRunning, PID: 41, EgressGrants: []EgressGrant{grant}, GrantRevision: 1}
	generation, _ := canonicalEgressGeneration(original, original.EgressGrants, original.GrantRevision)
	original.appliedEgressRevision, original.appliedEgressHash = generation.Revision, generation.Hash
	adapter, _ := NewProtocolAdapter(original)
	state, err := adapter.BlockedTeardownState()
	if err != nil {
		t.Fatal(err)
	}
	requested, ok := ReduceProtocol(state, ProtocolEvent{Action: ProtocolRequestTeardown})
	if !ok {
		t.Fatal("request teardown rejected")
	}
	unknown, ok := ReduceProtocol(requested, ProtocolEvent{Action: ProtocolTeardownAction, Outcome: ProtocolTeardownUnknown})
	if !ok {
		t.Fatal("unknown teardown rejected")
	}
	blocked, err := adapter.TeardownUnknownCandidate(unknown, now)
	if err != nil {
		t.Fatal(err)
	}
	if blocked.Status != StatusRunning || blocked.LastFailure == nil || blocked.LastFailure.Code != "network_authority_uncertain" || blocked.EgressRuntimeState().AppliedHash != generation.Hash {
		t.Fatalf("blocked teardown candidate=%+v runtime=%+v", blocked, blocked.EgressRuntimeState())
	}
	state, ok = ReduceProtocol(requested, ProtocolEvent{Action: ProtocolTeardownAction, Outcome: ProtocolTeardownProven})
	if !ok {
		t.Fatal("proven teardown rejected")
	}
	restore := original
	restore.EgressGrants, restore.GrantRevision = nil, 0
	candidate, err := adapter.TeardownProvenCandidate(state, &restore, now)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Status != StatusStopped || len(candidate.EgressGrants) != 0 || candidate.GrantRevision != 0 || candidate.LastFailure == nil || candidate.LastFailure.Code != "network_authority_uncertain" || candidate.EgressRuntimeState() != (EgressRuntimeState{}) {
		t.Fatalf("terminal restore candidate=%+v runtime=%+v", candidate, candidate.EgressRuntimeState())
	}
}

func TestProtocolAdapterBuildsCommitEffectCandidates(t *testing.T) {
	created := Session{ID: "sess-claim", Environment: "container", Network: "deny", Status: StatusCreated}
	claimAdapter, claimState := NewProtocolAdapter(created)
	if err := claimAdapter.BindOwner(ProtocolOwnerA, 41, "token-a"); err != nil {
		t.Fatalf("bind claim owner: %v", err)
	}
	claimState, ok := ReduceProtocol(claimState, ProtocolEvent{Action: ProtocolClaimStart})
	if !ok {
		t.Fatal("claim start rejected")
	}
	claimCandidate, err := claimAdapter.EffectCandidate(claimState)
	if err != nil {
		t.Fatalf("claim effect candidate: %v", err)
	}
	if claimCandidate.Status != StatusRunning || claimCandidate.PID != 41 || claimCandidate.ProcessToken != "token-a" {
		t.Fatalf("claim effect candidate = %+v", claimCandidate)
	}

	now := time.Date(2026, 7, 18, 2, 30, 0, 0, time.UTC)
	original := Session{ID: "sess-widen", Environment: "container", Network: "deny", Status: StatusRunning, PID: 41, ProcessToken: "token-a"}
	oldGeneration, ok := canonicalEgressGeneration(original, nil, 0)
	if !ok {
		t.Fatal("build old generation")
	}
	original.appliedEgressRevision, original.appliedEgressHash = oldGeneration.Revision, oldGeneration.Hash
	adapter, state := NewProtocolAdapter(original)
	grant := EgressGrant{ID: "g-000001", Host: "api.example.com", Port: 443, Source: "operator", CreatedAt: now}
	nextConcrete := original
	nextConcrete.EgressGrants, nextConcrete.GrantRevision = []EgressGrant{grant}, 1
	newSymbol := protocolGeneration(ProtocolGrantSet(ProtocolGrantA), 1)
	if err := adapter.BindGeneration(newSymbol, nextConcrete); err != nil {
		t.Fatalf("bind widened generation: %v", err)
	}
	state, ok = ReduceProtocol(state, ProtocolEvent{Action: ProtocolWidenStart, Grant: ProtocolGrantA})
	if !ok {
		t.Fatal("widen start rejected")
	}
	if candidate, err := adapter.Candidate(state); err == nil {
		t.Fatalf("truth-state projection produced uncommitted widen candidate %+v", candidate)
	}
	pending, err := adapter.EffectCandidate(state)
	if err != nil {
		t.Fatalf("widen commit candidate: %v", err)
	}
	if len(pending.EgressGrants) != 1 || pending.GrantRevision != 1 || pending.egressTransition == nil || !validEgressRuntimeState(pending) {
		t.Fatalf("widen pending candidate = %+v, runtime=%+v", pending, pending.EgressRuntimeState())
	}

	for _, event := range []ProtocolEvent{
		{Action: ProtocolWidenCommit, Outcome: ProtocolKnownNew},
		{Action: ProtocolApplyAction, Outcome: ProtocolSucceeded},
		{Action: ProtocolInspectAction, Outcome: ProtocolMatched},
	} {
		state, ok = ReduceProtocol(state, event)
		if !ok {
			t.Fatalf("widen step %+v rejected", event)
		}
	}
	final, err := adapter.EffectCandidate(state)
	if err != nil {
		t.Fatalf("widen final candidate: %v", err)
	}
	newGeneration, _ := canonicalEgressGeneration(nextConcrete, nextConcrete.EgressGrants, nextConcrete.GrantRevision)
	finalRuntime := final.EgressRuntimeState()
	if finalRuntime.Transition != nil || finalRuntime.AppliedRevision != newGeneration.Revision || finalRuntime.AppliedHash != newGeneration.Hash || !validEgressRuntimeState(final) {
		t.Fatalf("widen final candidate = %+v, runtime=%+v", final, finalRuntime)
	}
}

func TestProtocolAdapterRebaseGenerationFramesLegacyRunningOwner(t *testing.T) {
	original := Session{ID: "sess-legacy-widen", Environment: "container", Network: "deny", Status: StatusRunning}
	oldConcrete, ok := canonicalEgressGeneration(original, nil, 0)
	if !ok {
		t.Fatal("build old generation")
	}
	original.appliedEgressRevision, original.appliedEgressHash = oldConcrete.Revision, oldConcrete.Hash
	adapter, _ := NewProtocolAdapter(original)
	oldSymbol := protocolGeneration(0, 0)
	state, err := adapter.RebaseGeneration(oldSymbol)
	if err != nil {
		t.Fatalf("rebase legacy running generation: %v", err)
	}
	next := original
	next.EgressGrants = []EgressGrant{{ID: "g-000001", Host: "api.example.com", Port: 443, Source: "operator", CreatedAt: time.Date(2026, 7, 18, 3, 0, 0, 0, time.UTC)}}
	next.GrantRevision = 1
	if err := adapter.BindGeneration(protocolGeneration(ProtocolGrantSet(ProtocolGrantA), 1), next); err != nil {
		t.Fatal(err)
	}
	state, ok = ReduceProtocol(state, ProtocolEvent{Action: ProtocolWidenStart, Grant: ProtocolGrantA})
	if !ok {
		t.Fatal("legacy widen start rejected")
	}
	candidate, err := adapter.EffectCandidate(state)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.PID != 0 || candidate.ProcessToken != "" || candidate.Status != StatusRunning {
		t.Fatalf("legacy owner framing changed: %+v", candidate)
	}
}

func TestReduceProtocolUnknownCommitPreservesBothWorlds(t *testing.T) {
	state := runningProtocolState()
	state, ok := ReduceProtocol(state, ProtocolEvent{Action: ProtocolWidenStart, Grant: ProtocolGrantA})
	if !ok {
		t.Fatal("widen start rejected")
	}
	oldWorld, newWorld, ok := ReduceProtocolUnknownCommit(state, ProtocolWidenCommit)
	if !ok {
		t.Fatal("unknown commit worlds rejected")
	}
	if oldWorld.DurableAuthority == newWorld.DurableAuthority || oldWorld.Health != ProtocolUncertain || newWorld.Health != ProtocolUncertain || oldWorld.Mode != ProtocolBlocked || newWorld.Mode != ProtocolBlocked {
		t.Fatalf("unknown worlds old=%+v new=%+v", oldWorld, newWorld)
	}
}

func TestProtocolAdapterBuildsNarrowIntentAndFinalCandidates(t *testing.T) {
	now := time.Date(2026, 7, 18, 2, 45, 0, 0, time.UTC)
	grant := EgressGrant{ID: "g-000001", Host: "api.example.com", Port: 443, Source: "operator", CreatedAt: now}
	original := Session{
		ID: "sess-narrow", Environment: "container", Network: "deny", Status: StatusRunning,
		PID: 41, ProcessToken: "token-a", EgressGrants: []EgressGrant{grant}, GrantRevision: 7,
	}
	oldConcrete, _ := canonicalEgressGeneration(original, original.EgressGrants, original.GrantRevision)
	original.appliedEgressRevision, original.appliedEgressHash = oldConcrete.Revision, oldConcrete.Hash
	adapter, _ := NewProtocolAdapter(original)
	oldSymbol := protocolGeneration(ProtocolGrantSet(ProtocolGrantA), 0)
	newSymbol := protocolGeneration(0, 1)
	state, err := adapter.RebaseGeneration(oldSymbol)
	if err != nil {
		t.Fatalf("rebase old generation: %v", err)
	}
	nextConcrete := original
	nextConcrete.EgressGrants, nextConcrete.GrantRevision = nil, 8
	if err := adapter.BindGeneration(newSymbol, nextConcrete); err != nil {
		t.Fatalf("bind narrow generation: %v", err)
	}
	state, ok := ReduceProtocol(state, ProtocolEvent{Action: ProtocolNarrowStart, Grant: ProtocolGrantA})
	if !ok {
		t.Fatal("narrow start rejected")
	}
	intent, err := adapter.Candidate(state)
	if err != nil {
		t.Fatalf("narrow intent candidate: %v", err)
	}
	if len(intent.EgressGrants) != 1 || intent.egressTransition == nil || intent.egressTransition.Direction != EgressDirectionNarrow || !validEgressRuntimeState(intent) {
		t.Fatalf("narrow intent candidate = %+v, runtime=%+v", intent, intent.EgressRuntimeState())
	}
	for _, event := range []ProtocolEvent{
		{Action: ProtocolApplyAction, Outcome: ProtocolSucceeded},
		{Action: ProtocolInspectAction, Outcome: ProtocolMatched},
	} {
		state, ok = ReduceProtocol(state, event)
		if !ok {
			t.Fatalf("narrow step %+v rejected", event)
		}
	}
	final, err := adapter.EffectCandidate(state)
	if err != nil {
		t.Fatalf("narrow final candidate: %v", err)
	}
	newConcrete, _ := canonicalEgressGeneration(nextConcrete, nil, nextConcrete.GrantRevision)
	finalRuntime := final.EgressRuntimeState()
	if len(final.EgressGrants) != 0 || final.GrantRevision != 8 || finalRuntime.Transition != nil || finalRuntime.AppliedRevision != newConcrete.Revision || finalRuntime.AppliedHash != newConcrete.Hash || !validEgressRuntimeState(final) {
		t.Fatalf("narrow final candidate = %+v, runtime=%+v", final, finalRuntime)
	}
}

func TestProtocolAdapterEffectCandidateRejectsBlockedCommitSources(t *testing.T) {
	adapter, claim := NewProtocolAdapter(Session{ID: "sess-forged", Status: StatusCreated})
	if err := adapter.BindOwner(ProtocolOwnerA, 41, "token-a"); err != nil {
		t.Fatalf("bind owner: %v", err)
	}
	claim, ok := ReduceProtocol(claim, ProtocolEvent{Action: ProtocolClaimStart})
	if !ok {
		t.Fatal("claim start rejected")
	}
	claim.Health, claim.Mode = ProtocolUncertain, ProtocolBlocked
	if candidate, err := adapter.EffectCandidate(claim); err == nil {
		t.Fatalf("blocked claim produced commit candidate %+v", candidate)
	}

	widen := runningProtocolState()
	for _, event := range []ProtocolEvent{
		{Action: ProtocolWidenStart, Grant: ProtocolGrantA},
		{Action: ProtocolWidenCommit, Outcome: ProtocolKnownNew},
		{Action: ProtocolApplyAction, Outcome: ProtocolSucceeded},
		{Action: ProtocolInspectAction, Outcome: ProtocolMatched},
	} {
		widen, ok = ReduceProtocol(widen, event)
		if !ok {
			t.Fatalf("widen step %+v rejected", event)
		}
	}
	widen.Health, widen.Mode = ProtocolUncertain, ProtocolBlocked
	if candidate, err := adapter.EffectCandidate(widen); err == nil {
		t.Fatalf("blocked widen final produced commit candidate %+v", candidate)
	}

	narrow := runningProtocolState()
	narrow.DurableAuthority, narrow.DurableRevision = ProtocolGrantSet(ProtocolGrantA), 1
	narrow.RecordedGeneration = protocolGeneration(narrow.DurableAuthority, narrow.DurableRevision)
	narrow.RuntimeAuthority, narrow.RuntimeGeneration, narrow.InspectedGeneration = narrow.DurableAuthority, narrow.RecordedGeneration, narrow.RecordedGeneration
	for _, event := range []ProtocolEvent{
		{Action: ProtocolNarrowStart, Grant: ProtocolGrantA},
		{Action: ProtocolApplyAction, Outcome: ProtocolSucceeded},
		{Action: ProtocolInspectAction, Outcome: ProtocolMatched},
	} {
		narrow, ok = ReduceProtocol(narrow, event)
		if !ok {
			t.Fatalf("narrow step %+v rejected", event)
		}
	}
	narrow.Health, narrow.Mode = ProtocolUncertain, ProtocolBlocked
	if candidate, err := adapter.EffectCandidate(narrow); err == nil {
		t.Fatalf("blocked narrow final produced commit candidate %+v", candidate)
	}
}

func TestProtocolAdapterRejectsUnpersistableUnknownState(t *testing.T) {
	original := Session{ID: "sess-unknown", Status: StatusCreated}
	adapter, state := NewProtocolAdapter(original)
	if err := adapter.BindOwner(ProtocolOwnerA, 41, "token-a"); err != nil {
		t.Fatalf("bind owner: %v", err)
	}
	claiming, ok := ReduceProtocol(state, ProtocolEvent{Action: ProtocolClaimStart})
	if !ok {
		t.Fatal("claim start rejected")
	}
	unknown, ok := ReduceProtocol(claiming, ProtocolEvent{Action: ProtocolClaimCommit, Outcome: ProtocolCommitUnknown, Truth: ProtocolNew})
	if !ok {
		t.Fatal("unknown claim rejected")
	}
	if candidate, err := adapter.Candidate(unknown); err == nil {
		t.Fatalf("unknown state produced normal-looking candidate %+v", candidate)
	}
}

func TestProtocolAdapterRejectsLegacyAndConflictingOwnerBindings(t *testing.T) {
	legacy := Session{ID: "sess-legacy", Status: StatusRunning, PID: 41}
	_, state := NewProtocolAdapter(legacy)
	if state.Mode == ProtocolNormal || state.Health != ProtocolStale {
		t.Fatalf("legacy tokenless owner entered exact theorem: %+v", state)
	}

	adapter, _ := NewProtocolAdapter(Session{ID: "sess-owner", Status: StatusCreated})
	if err := adapter.BindOwner(ProtocolOwnerA, 41, "token-a"); err != nil {
		t.Fatalf("first owner binding: %v", err)
	}
	if err := adapter.BindOwner(ProtocolOwnerA, 42, "token-b"); err == nil {
		t.Fatal("conflicting owner binding succeeded")
	}
	if err := adapter.BindOwner(ProtocolOwnerB, 41, "token-a"); err == nil {
		t.Fatal("second symbolic owner aliased the first exact pair")
	}
}

func TestProtocolAdapterRejectsConflictingGenerationBinding(t *testing.T) {
	now := time.Date(2026, 7, 18, 3, 0, 0, 0, time.UTC)
	original := Session{ID: "sess-generation", Environment: "container", Network: "deny", Status: StatusRunning, PID: 41, ProcessToken: "token-a"}
	adapter, _ := NewProtocolAdapter(original)
	if err := adapter.BindGeneration(protocolGeneration(ProtocolGrantSet(ProtocolGrantB), 1), original); err == nil {
		t.Fatal("two symbolic generations aliased one concrete generation")
	}
	symbol := protocolGeneration(ProtocolGrantSet(ProtocolGrantA), 1)
	first := original
	first.EgressGrants = []EgressGrant{{ID: "g-000001", Host: "one.example.com", Port: 443, Source: "operator", CreatedAt: now}}
	first.GrantRevision = 1
	if err := adapter.BindGeneration(symbol, first); err != nil {
		t.Fatalf("first generation binding: %v", err)
	}
	second := original
	second.EgressGrants = []EgressGrant{{ID: "g-000002", Host: "two.example.com", Port: 443, Source: "operator", CreatedAt: now}}
	second.GrantRevision = 1
	if err := adapter.BindGeneration(symbol, second); err == nil {
		t.Fatal("conflicting generation binding succeeded")
	}
}

func TestProtocolAdapterRejectsNonUnitConcreteWiden(t *testing.T) {
	now := time.Date(2026, 7, 18, 3, 30, 0, 0, time.UTC)
	original := Session{ID: "sess-delta", Environment: "container", Network: "deny", Status: StatusRunning, PID: 41, ProcessToken: "token-a"}
	oldGeneration, _ := canonicalEgressGeneration(original, nil, 0)
	original.appliedEgressRevision, original.appliedEgressHash = oldGeneration.Revision, oldGeneration.Hash
	adapter, state := NewProtocolAdapter(original)
	candidate := original
	candidate.EgressGrants = []EgressGrant{{ID: "g-000001", Host: "api.example.com", Port: 443, Source: "operator", CreatedAt: now}}
	candidate.GrantRevision = 2
	if err := adapter.BindGeneration(protocolGeneration(ProtocolGrantSet(ProtocolGrantA), 1), candidate); err != nil {
		t.Fatalf("bind candidate: %v", err)
	}
	state, ok := ReduceProtocol(state, ProtocolEvent{Action: ProtocolWidenStart, Grant: ProtocolGrantA})
	if !ok {
		t.Fatal("widen start rejected")
	}
	if candidate, err := adapter.EffectCandidate(state); err == nil {
		t.Fatalf("non-unit concrete widen produced candidate %+v", candidate)
	}
}

func TestProtocolAdapterRejectsMalformedStableRuntimeState(t *testing.T) {
	sess := Session{ID: "sess-runtime", Environment: "container", Network: "deny", Status: StatusRunning, PID: 41, ProcessToken: "token-a"}
	sess.appliedEgressHash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	_, state := NewProtocolAdapter(sess)
	if state.Mode == ProtocolNormal || state.Health != ProtocolCorrupt {
		t.Fatalf("malformed stable runtime entered normal protocol: %+v", state)
	}
}

func TestProtocolAdapterBuildsPersistedEgressRecoveryStates(t *testing.T) {
	now := time.Date(2026, 7, 18, 6, 0, 0, 0, time.UTC)
	old := Session{ID: "sess-recover", Environment: "container", Network: "deny", Status: StatusRunning, PID: 41}
	oldGeneration, _ := canonicalEgressGeneration(old, nil, 0)
	grant := EgressGrant{ID: "g-000001", Host: "api.example.com", Port: 443, Source: "operator", CreatedAt: now}
	widen := old
	widen.EgressGrants, widen.GrantRevision = []EgressGrant{grant}, 1
	newGeneration, _ := canonicalEgressGeneration(widen, widen.EgressGrants, widen.GrantRevision)
	widen.appliedEgressRevision, widen.appliedEgressHash = oldGeneration.Revision, oldGeneration.Hash
	widen.egressTransition = &EgressTransition{Direction: EgressDirectionWiden, CandidateRevision: 1, CandidateHash: newGeneration.Hash, CandidateGrants: []EgressGrant{grant}}

	adapter, _ := NewProtocolAdapter(widen)
	active, recovery, err := adapter.EgressTransitionStates()
	if err != nil {
		t.Fatal(err)
	}
	if active.Operation != ProtocolWiden || active.Effect != ProtocolApply || recovery.Operation != ProtocolRecover || recovery.Effect != ProtocolInspect || recovery.Mode != ProtocolBlocked {
		t.Fatalf("active=%+v recovery=%+v", active, recovery)
	}
	recovered, ok := ReduceProtocol(recovery, ProtocolEvent{Action: ProtocolRecoverEgress})
	if !ok {
		t.Fatal("persisted widen recovery rejected")
	}
	candidate, err := adapter.CandidateFrom(recovered, widen)
	if err != nil {
		t.Fatal(err)
	}
	if state := candidate.EgressRuntimeState(); state.Transition != nil || state.AppliedHash != newGeneration.Hash {
		t.Fatalf("recovered candidate=%+v runtime=%+v", candidate, state)
	}
}

func TestProtocolAdapterVerifiedGenerationBootstrapsCanonicalAppliedState(t *testing.T) {
	sess := Session{ID: "sess-bootstrap", Environment: "container", Network: "deny", Status: StatusRunning}
	adapter, _ := NewProtocolAdapter(sess)
	symbol := protocolGeneration(0, 0)
	state, err := adapter.RebaseVerifiedGeneration(symbol)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := adapter.CandidateFrom(state, sess)
	if err != nil {
		t.Fatal(err)
	}
	generation, _ := canonicalEgressGeneration(sess, nil, 0)
	if runtime := candidate.EgressRuntimeState(); runtime.AppliedRevision != generation.Revision || runtime.AppliedHash != generation.Hash || runtime.Transition != nil {
		t.Fatalf("verified bootstrap runtime=%+v want=%+v", runtime, generation)
	}
}

func TestProtocolAdapterRejectsMalformedConcreteTransition(t *testing.T) {
	now := time.Date(2026, 7, 18, 4, 0, 0, 0, time.UTC)
	old := Session{ID: "sess-transition", Environment: "container", Network: "deny", Status: StatusRunning, PID: 41, ProcessToken: "token-a"}
	oldGeneration, ok := canonicalEgressGeneration(old, nil, 0)
	if !ok {
		t.Fatal("build old generation")
	}
	grant := EgressGrant{ID: "g-000001", Host: "api.example.com", Port: 443, Source: "operator", CreatedAt: now}
	candidate := old
	candidate.EgressGrants, candidate.GrantRevision = []EgressGrant{grant}, 1
	candidateGeneration, ok := canonicalEgressGeneration(candidate, candidate.EgressGrants, candidate.GrantRevision)
	if !ok {
		t.Fatal("build candidate generation")
	}
	candidate.appliedEgressRevision, candidate.appliedEgressHash = oldGeneration.Revision, oldGeneration.Hash
	candidate.egressTransition = &EgressTransition{
		Direction: EgressDirectionWiden, CandidateRevision: 1, CandidateHash: candidateGeneration.Hash,
		CandidateGrants: []EgressGrant{grant},
	}
	candidate.egressTransition.CandidateHash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	_, state := NewProtocolAdapter(candidate)
	if state.Mode == ProtocolNormal || state.Health != ProtocolCorrupt {
		t.Fatalf("malformed transition entered normal protocol: %+v", state)
	}
}

func TestProtocolAdapterUncertaintyMarkerOverridesTransitionStage(t *testing.T) {
	now := time.Date(2026, 7, 18, 5, 0, 0, 0, time.UTC)
	sess := Session{ID: "sess-blocked", Environment: "container", Network: "deny", Status: StatusRunning, PID: 41, ProcessToken: "token-a"}
	oldGeneration, _ := canonicalEgressGeneration(sess, nil, 0)
	grant := EgressGrant{ID: "g-000001", Host: "api.example.com", Port: 443, Source: "operator", CreatedAt: now}
	sess.EgressGrants, sess.GrantRevision = []EgressGrant{grant}, 1
	newGeneration, _ := canonicalEgressGeneration(sess, sess.EgressGrants, sess.GrantRevision)
	sess.appliedEgressRevision, sess.appliedEgressHash = oldGeneration.Revision, oldGeneration.Hash
	sess.egressTransition = &EgressTransition{Direction: EgressDirectionWiden, CandidateRevision: 1, CandidateHash: newGeneration.Hash, CandidateGrants: []EgressGrant{grant}}
	sess.LastFailure = &Failure{Phase: "network", Code: "network_authority_uncertain"}
	_, state := NewProtocolAdapter(sess)
	if state.Mode != ProtocolBlocked || state.Health != ProtocolUncertain || state.Operation != ProtocolRecover || state.Effect != ProtocolNoEffect {
		t.Fatalf("uncertainty marker normalized as %+v", state)
	}
}

func TestProtocolAdapterMarksDecodableStaleOwnerShapesBlocked(t *testing.T) {
	cases := []Session{
		{ID: "created-owner", Status: StatusCreated, PID: 99, ProcessToken: "old"},
		{ID: "running-no-owner", Status: StatusRunning},
		{ID: "stopped-owner", Status: StatusStopped, PID: 99, ProcessToken: "old", Detached: true},
	}
	for _, stale := range cases {
		_, state := NewProtocolAdapter(stale)
		if state.Mode != ProtocolBlocked || state.Health != ProtocolStale {
			t.Fatalf("%s stale shape normalized as %+v", stale.ID, state)
		}
	}
}

func runningProtocolState() ProtocolState {
	s := InitialProtocolStates(ProtocolBounds{})[0]
	s.Status = ProtocolRunning
	s.Owners = ProtocolOwnerA
	s.Result = ProtocolSuccess
	return s
}
