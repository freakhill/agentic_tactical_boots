package promotion

import (
	"reflect"
	"testing"
)

func validSource() SourceSnapshot {
	return SourceSnapshot{ID: "sess-1", AdHoc: true, Status: "created", ETag: "source-v1", GrantRevision: 2,
		Grants: []Grant{{ID: "g-a", Host: "api.example.com", Port: 443}, {ID: "g-b", Host: "registry.example.com", Port: 80}}}
}

func absentPolicy() PolicySnapshot { return PolicySnapshot{} }

func TestPreparePromotionEmptySelectionHasNoDurableRule(t *testing.T) {
	plan, err := Prepare(Request{TargetName: "saved"}, validSource(), absentPolicy(), "candidate")
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if len(plan.Rules) != 0 || len(plan.SelectedGrantIDs) != 0 {
		t.Fatalf("empty selection gained durable authority: %+v", plan)
	}
}

func TestPreparePromotionMapsOnlySelectedGrantIDsExactly(t *testing.T) {
	plan, err := Prepare(Request{TargetName: "saved", GrantIDs: []string{"g-b"}}, validSource(), absentPolicy(), "candidate")
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if got, want := plan.Rules, []Rule{{Host: "registry.example.com", Port: 80}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("rules = %#v, want %#v", got, want)
	}
	if got, want := plan.SelectedGrantIDs, []string{"g-b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("selected IDs = %#v, want %#v", got, want)
	}
}

func TestPreparePromotionRejectsUnsafeSourcesAndTargets(t *testing.T) {
	for _, tc := range []struct {
		name    string
		source  SourceSnapshot
		policy  PolicySnapshot
		request Request
	}{
		{"profile-backed", SourceSnapshot{ID: "sess-1", AdHoc: false, Status: "created", ETag: "v", GrantRevision: 0}, absentPolicy(), Request{TargetName: "saved"}},
		{"existing", validSource(), PolicySnapshot{Exists: true, Hash: "old", Profiles: []string{"saved"}}, Request{TargetName: "saved"}},
		{"builtin", validSource(), PolicySnapshot{Builtins: []string{"pi"}}, Request{TargetName: "pi"}},
		{"unknown grant", validSource(), absentPolicy(), Request{TargetName: "saved", GrantIDs: []string{"g-missing"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Prepare(tc.request, tc.source, tc.policy, "candidate"); err == nil {
				t.Fatal("Prepare accepted unsafe input")
			}
		})
	}
}

func TestCheckApplyPromotionRequiresExactSourceAndPolicyEvidence(t *testing.T) {
	plan, err := Prepare(Request{TargetName: "saved", GrantIDs: []string{"g-a"}}, validSource(), absentPolicy(), "candidate")
	if err != nil {
		t.Fatal(err)
	}
	changedSource := validSource()
	changedSource.GrantRevision++
	if _, err := CheckApply(plan, changedSource, absentPolicy(), "candidate", true); err == nil {
		t.Fatal("apply accepted changed source grants")
	}
	if _, err := CheckApply(plan, validSource(), PolicySnapshot{Exists: true, Hash: "new"}, "candidate", true); err == nil {
		t.Fatal("apply accepted changed target policy")
	}
}

func TestCheckApplyPromotionAllowsOnlyCreatedOrStoppedAndValidatesBeforeReplace(t *testing.T) {
	plan, err := Prepare(Request{TargetName: "saved", GrantIDs: []string{"g-a"}}, validSource(), absentPolicy(), "candidate")
	if err != nil {
		t.Fatal(err)
	}
	running := validSource()
	running.Status = "running"
	if _, err := CheckApply(plan, running, absentPolicy(), "candidate", true); err == nil {
		t.Fatal("apply accepted running source")
	}
	stopped := validSource()
	stopped.Status = "stopped"
	if _, err := CheckApply(plan, stopped, absentPolicy(), "candidate", false); err == nil {
		t.Fatal("apply replaced without validation")
	}
	if effect, err := CheckApply(plan, stopped, absentPolicy(), "candidate", true); err != nil || effect != EffectReplace {
		t.Fatalf("apply = %q, %v; want replacement", effect, err)
	}
}

func TestStepPromotionEmitsOnlyDataEffectsAndUnknownCommitIsNotSuccess(t *testing.T) {
	plan, err := Prepare(Request{TargetName: "saved"}, validSource(), absentPolicy(), "candidate")
	if err != nil {
		t.Fatal(err)
	}
	state, effects := Step(State{Phase: PhasePreview, Plan: plan}, Event{})
	if state.Phase != PhasePrepared || !reflect.DeepEqual(effects, []Effect{EffectReadSource, EffectReadPolicy, EffectValidate}) {
		t.Fatalf("preview step = %+v, %#v", state, effects)
	}
	state, effects = Step(state, Event{Source: validSource(), Policy: absentPolicy(), CandidateHash: "candidate", Valid: true, Commit: CommitUnknown})
	if state.Phase != PhaseCommitUncertain {
		t.Fatalf("unknown commit = %s, want uncertainty", state.Phase)
	}
	if !reflect.DeepEqual(effects, []Effect{EffectReplace, EffectReport}) {
		t.Fatalf("unknown effects = %#v", effects)
	}
	for _, effect := range append(effects, EffectReadSource, EffectReadPolicy, EffectValidate) {
		switch effect {
		case EffectReadSource, EffectReadPolicy, EffectValidate, EffectReplace, EffectReport:
		default:
			t.Fatalf("unsafe effect %q", effect)
		}
	}
}

func TestStepPromotionKnownFailedCommitDoesNotRequestReplacement(t *testing.T) {
	plan, err := Prepare(Request{TargetName: "saved"}, validSource(), absentPolicy(), "candidate")
	if err != nil {
		t.Fatal(err)
	}
	prepared, _ := Step(State{Phase: PhasePreview, Plan: plan}, Event{})
	state, effects := Step(prepared, Event{Source: validSource(), Policy: absentPolicy(), CandidateHash: "candidate", Valid: true, Commit: CommitKnownFailed})
	if state.Phase != PhaseFailed {
		t.Fatalf("known failed commit = %s, want failed", state.Phase)
	}
	if !reflect.DeepEqual(effects, []Effect{EffectReport}) {
		t.Fatalf("known failed effects = %#v, want report only", effects)
	}
}
