// Package promotion defines the pure, bounded decision kernel for promoting an
// ad-hoc session into a new profile. It has no filesystem, session-store, CUE,
// clock, logging, or runtime dependency; drivers own those effects.
package promotion

import (
	"errors"
	"fmt"
)

var (
	ErrIneligibleSource = errors.New("promotion source is not an ad-hoc session")
	ErrTargetExists     = errors.New("promotion target already exists")
	ErrInvalidRequest   = errors.New("promotion request is invalid")
	ErrStaleSource      = errors.New("promotion source snapshot is stale")
	ErrStalePolicy      = errors.New("promotion policy snapshot is stale")
	ErrApplyRunning     = errors.New("promotion apply requires a created or stopped source")
	ErrInvalidCandidate = errors.New("promotion candidate is not validated")
)

const (
	PlanVersion     = 1
	RendererVersion = 1
	SchemaVersion   = 1
)

// Grant is a normalized, value-free session-scoped destination identified by a
// stable grant ID. The adapter, not this kernel, validates FQDN/port syntax.
type Grant struct {
	ID   string
	Host string
	Port int
}

// SourceSnapshot is the promotion-relevant subset of an authoritative session.
// Status is deliberately outside its ETag: a running source may become stopped
// between preview and apply without invalidating otherwise identical evidence.
type SourceSnapshot struct {
	ID            string
	AdHoc         bool
	Status        string
	ETag          string
	GrantRevision int
	Grants        []Grant
}

// PolicySnapshot represents the exact target bytes (or their absence) and the
// names that cannot be replaced. It contains no policy content.
type PolicySnapshot struct {
	Exists   bool
	Hash     string
	Profiles []string
	Builtins []string
}

type Request struct {
	TargetName string
	GrantIDs   []string
}

type Rule struct {
	Host string
	Port int
}

// PlanIntent is data-only evidence that a driver persists in its private plan.
type PlanIntent struct {
	Version          int
	Source           SourceSnapshot
	Policy           PolicySnapshot
	TargetName       string
	SelectedGrantIDs []string
	Rules            []Rule
	CandidateHash    string
	RendererVersion  int
	SchemaVersion    int
}

type Effect string

const (
	EffectReadSource Effect = "read-source"
	EffectReadPolicy Effect = "read-policy"
	EffectValidate   Effect = "validate-candidate"
	EffectReplace    Effect = "replace-exact"
	EffectReport     Effect = "report"
)

type Phase string

const (
	PhasePreview         Phase = "preview"
	PhasePrepared        Phase = "prepared"
	PhaseApplying        Phase = "applying"
	PhaseApplied         Phase = "applied"
	PhaseFailed          Phase = "failed"
	PhaseCommitUncertain Phase = "commit-uncertain"
)

type State struct {
	Phase Phase
	Plan  PlanIntent
}

type Event struct {
	Source        SourceSnapshot
	Policy        PolicySnapshot
	CandidateHash string
	Valid         bool
	Commit        CommitOutcome
}

type CommitOutcome string

const (
	CommitKnownFailed    CommitOutcome = "known-failed"
	CommitKnownCommitted CommitOutcome = "known-committed"
	CommitUnknown        CommitOutcome = "unknown"
)

// Prepare makes a create-only intent from a read-only source snapshot. Empty
// selection is valid and intentionally produces no durable rule.
func Prepare(request Request, source SourceSnapshot, policy PolicySnapshot, candidateHash string) (PlanIntent, error) {
	if !source.AdHoc {
		return PlanIntent{}, ErrIneligibleSource
	}
	if source.ID == "" || source.ETag == "" || source.GrantRevision < 0 || candidateHash == "" || request.TargetName == "" {
		return PlanIntent{}, ErrInvalidRequest
	}
	if !validPreviewStatus(source.Status) || !targetFree(request.TargetName, policy) {
		if !targetFree(request.TargetName, policy) {
			return PlanIntent{}, ErrTargetExists
		}
		return PlanIntent{}, ErrInvalidRequest
	}
	byID := make(map[string]Grant, len(source.Grants))
	for _, grant := range source.Grants {
		if grant.ID == "" || grant.Host == "" || grant.Port == 0 {
			return PlanIntent{}, ErrInvalidRequest
		}
		if _, duplicate := byID[grant.ID]; duplicate {
			return PlanIntent{}, ErrInvalidRequest
		}
		byID[grant.ID] = grant
	}
	selected := append([]string(nil), request.GrantIDs...)
	rules := make([]Rule, 0, len(selected))
	selectedIDs := make(map[string]struct{}, len(selected))
	destinations := make(map[Rule]struct{}, len(selected))
	for _, id := range selected {
		if _, duplicate := selectedIDs[id]; duplicate {
			return PlanIntent{}, ErrInvalidRequest
		}
		grant, ok := byID[id]
		if !ok {
			return PlanIntent{}, fmt.Errorf("%w: selected grant %q is absent", ErrInvalidRequest, id)
		}
		rule := Rule{Host: grant.Host, Port: grant.Port}
		if _, duplicate := destinations[rule]; duplicate {
			return PlanIntent{}, ErrInvalidRequest
		}
		selectedIDs[id], destinations[rule] = struct{}{}, struct{}{}
		rules = append(rules, rule)
	}
	return PlanIntent{
		Version: PlanVersion, Source: cloneSource(source), Policy: clonePolicy(policy), TargetName: request.TargetName,
		SelectedGrantIDs: selected, Rules: rules, CandidateHash: candidateHash,
		RendererVersion: RendererVersion, SchemaVersion: SchemaVersion,
	}, nil
}

// CheckApply rechecks all authority evidence. It never recomputes a candidate:
// callers must discard a stale plan and issue a fresh preview instead.
func CheckApply(plan PlanIntent, source SourceSnapshot, policy PolicySnapshot, candidateHash string, valid bool) (Effect, error) {
	if !valid || candidateHash == "" || candidateHash != plan.CandidateHash {
		return "", ErrInvalidCandidate
	}
	if plan.Version != PlanVersion || plan.RendererVersion != RendererVersion || plan.SchemaVersion != SchemaVersion || !plan.Source.AdHoc || !targetFree(plan.TargetName, policy) {
		return "", ErrInvalidRequest
	}
	if !samePromotionEvidence(plan.Source, source) {
		return "", ErrStaleSource
	}
	if source.Status != "created" && source.Status != "stopped" {
		return "", ErrApplyRunning
	}
	if !samePolicyEvidence(plan.Policy, policy) {
		return "", ErrStalePolicy
	}
	if !matchesSelectedRules(plan) {
		return "", ErrInvalidRequest
	}
	return EffectReplace, nil
}

// Step is the reducer relation used by drivers and the finite model adapter.
// It never executes its listed effects; they are a data-only work request.
func Step(state State, event Event) (State, []Effect) {
	switch state.Phase {
	case PhasePreview:
		return State{Phase: PhasePrepared, Plan: state.Plan}, []Effect{EffectReadSource, EffectReadPolicy, EffectValidate}
	case PhasePrepared:
		if _, err := CheckApply(state.Plan, event.Source, event.Policy, event.CandidateHash, event.Valid); err != nil {
			return State{Phase: PhaseFailed, Plan: state.Plan}, []Effect{EffectReport}
		}
		switch event.Commit {
		case CommitKnownCommitted:
			return State{Phase: PhaseApplied, Plan: state.Plan}, []Effect{EffectReplace, EffectReport}
		case CommitUnknown:
			return State{Phase: PhaseCommitUncertain, Plan: state.Plan}, []Effect{EffectReplace, EffectReport}
		default:
			return State{Phase: PhaseFailed, Plan: state.Plan}, []Effect{EffectReport}
		}
	default:
		return state, []Effect{EffectReport}
	}
}

func validPreviewStatus(status string) bool {
	return status == "created" || status == "running" || status == "stopped"
}

func targetFree(name string, policy PolicySnapshot) bool {
	for _, existing := range policy.Profiles {
		if existing == name {
			return false
		}
	}
	for _, builtin := range policy.Builtins {
		if builtin == name {
			return false
		}
	}
	return true
}

func samePromotionEvidence(want, got SourceSnapshot) bool {
	if want.ID != got.ID || want.AdHoc != got.AdHoc || want.ETag != got.ETag || want.GrantRevision != got.GrantRevision || len(want.Grants) != len(got.Grants) {
		return false
	}
	for i := range want.Grants {
		if want.Grants[i] != got.Grants[i] {
			return false
		}
	}
	return true
}

func samePolicyEvidence(want, got PolicySnapshot) bool {
	return want.Exists == got.Exists && want.Hash == got.Hash
}

func matchesSelectedRules(plan PlanIntent) bool {
	byID := make(map[string]Grant, len(plan.Source.Grants))
	for _, grant := range plan.Source.Grants {
		byID[grant.ID] = grant
	}
	if len(plan.SelectedGrantIDs) != len(plan.Rules) {
		return false
	}
	seen := map[string]struct{}{}
	for i, id := range plan.SelectedGrantIDs {
		grant, ok := byID[id]
		if !ok {
			return false
		}
		if _, duplicate := seen[id]; duplicate {
			return false
		}
		seen[id] = struct{}{}
		if plan.Rules[i] != (Rule{Host: grant.Host, Port: grant.Port}) {
			return false
		}
	}
	return true
}

func cloneSource(source SourceSnapshot) SourceSnapshot {
	source.Grants = append([]Grant(nil), source.Grants...)
	return source
}
func clonePolicy(policy PolicySnapshot) PolicySnapshot {
	policy.Profiles = append([]string(nil), policy.Profiles...)
	policy.Builtins = append([]string(nil), policy.Builtins...)
	return policy
}
