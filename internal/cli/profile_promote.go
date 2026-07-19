package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/freakhill/safeslop/internal/engine/policy"
	"github.com/freakhill/safeslop/internal/engine/promotion"
	engsession "github.com/freakhill/safeslop/internal/engine/session"
	"github.com/freakhill/safeslop/internal/engine/trust"
	"github.com/freakhill/safeslop/internal/jsoncontract"
)

func cmdProfilePromoteWithDeps(d *dependencies) *cobra.Command {
	c := &cobra.Command{Use: "promote", Short: "Promote an ad-hoc session to a new profile"}
	c.AddCommand(cmdProfilePromotePreviewWithDeps(d), cmdProfilePromoteApplyWithDeps(d))
	return c
}

func cmdProfilePromotePreviewWithDeps(d *dependencies) *cobra.Command {
	var sessionID, planPath, output string
	var grantIDs []string
	c := &cobra.Command{Use: "preview NAME [safeslop.cue] --session-id ID [--grant-id G]... --plan PLAN --output json", Short: "Preview ad-hoc session promotion", Args: cobra.RangeArgs(1, 2), RunE: func(_ *cobra.Command, args []string) error {
		if output != "json" {
			return fmt.Errorf("profile promote preview requires --output json")
		}
		if sessionID == "" || planPath == "" {
			return emitContractError(jsoncontract.CodeInvalidArgument, "--session-id and --plan are required", nil)
		}
		sess, err := d.store.Get(sessionID)
		if err != nil {
			return promoteSessionError(sessionID, err)
		}
		path, cfg, bytes, exists, err := loadOrNewPromotionPolicy(argAt(args, 1))
		if err != nil {
			return emitContractError(jsoncontract.CodeIOError, "load safeslop.cue", nil)
		}
		plan, prof, candidate, err := buildPromotionPlan(args[0], sess, cfg, path, bytes, exists, grantIDs)
		if err != nil {
			return promoteError(err)
		}
		if _, err := policy.LoadBytes(candidate); err != nil {
			return emitContractError(jsoncontract.CodeSchemaViolation, "rendered safeslop.cue did not validate; not writing", nil)
		}
		if err := promotion.WritePlan(planPath, plan); err != nil {
			return emitContractError(jsoncontract.CodeIOError, "write promotion plan", nil)
		}
		emitContract(jsoncontract.OK(promotePreviewData(plan, prof, path, exists)))
		return nil
	}}
	c.Flags().StringVar(&sessionID, "session-id", "", "source ad-hoc session id")
	c.Flags().StringArrayVar(&grantIDs, "grant-id", nil, "session grant id to persist (repeatable)")
	c.Flags().StringVar(&planPath, "plan", "", "promotion plan path")
	c.Flags().StringVar(&output, "output", "", "output format: json")
	return c
}

func cmdProfilePromoteApplyWithDeps(d *dependencies) *cobra.Command {
	var planPath, output string
	c := &cobra.Command{Use: "apply --plan PLAN --output json", Short: "Apply an exact ad-hoc promotion plan", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		if output != "json" {
			return fmt.Errorf("profile promote apply requires --output json")
		}
		if planPath == "" {
			return emitContractError(jsoncontract.CodeInvalidArgument, "--plan is required", nil)
		}
		plan, err := promotion.ReadPlan(planPath)
		if err != nil {
			return emitContractError(jsoncontract.CodeInvalidArgument, "read promotion plan", nil)
		}
		sess, err := d.store.Get(plan.Source.ID)
		if err != nil {
			return promoteSessionError(plan.Source.ID, err)
		}
		path, cfg, bytes, exists, err := loadOrNewPromotionPolicy(plan.TargetPolicyPath)
		if err != nil {
			return emitContractError(jsoncontract.CodeIOError, "load safeslop.cue", nil)
		}
		currentPolicy := promotionPolicySnapshot(bytes, exists, cfg)
		currentSource, err := promotionSourceSnapshot(sess)
		if err != nil {
			return promoteError(err)
		}
		prof, candidate, err := renderPromotionCandidate(plan.TargetName, sess, cfg, plan.SelectedGrantIDs)
		if err != nil {
			return promoteError(err)
		}
		if _, err := policy.LoadBytes(candidate); err != nil {
			return emitContractError(jsoncontract.CodeSchemaViolation, "rendered safeslop.cue did not validate; not writing", nil)
		}
		if _, err := promotion.CheckApply(plan, currentSource, currentPolicy, trust.Hash(candidate), true); err != nil {
			return promoteError(err)
		}
		outcome, err := policy.ReplacePolicyAtomic(path, candidate, policy.TransactionOptions{ExpectedHash: expectedPromotionHash(currentPolicy), CreateOnly: !exists, Validate: func(b []byte) error { _, err := policy.LoadBytes(b); return err }})
		if err != nil {
			if errors.Is(err, policy.ErrPolicyCommitUncertain) {
				return emitContractError(jsoncontract.CodeIOError, "promotion policy commit uncertain", map[string]any{"outcome": string(outcome)})
			}
			return emitContractError(jsoncontract.CodeInvalidArgument, "apply promotion policy", nil)
		}
		emitContract(jsoncontract.OK(map[string]any{"applied": true, "profile": plan.TargetName, "path": path, "candidate_policy_hash": trust.Hash(candidate), "trusted": false, "session_unchanged": true, "profile_summary": prof}))
		return nil
	}}
	c.Flags().StringVar(&planPath, "plan", "", "promotion plan path")
	c.Flags().StringVar(&output, "output", "", "output format: json")
	return c
}

func loadOrNewPromotionPolicy(pathArg string) (string, *policy.Config, []byte, bool, error) {
	if pathArg != "" {
		if b, err := os.ReadFile(pathArg); err == nil {
			cfg, err := policy.LoadBytes(b)
			if err != nil {
				return "", nil, nil, false, err
			}
			if cfg.Profiles == nil {
				cfg.Profiles = map[string]policy.Profile{}
			}
			return pathArg, cfg, b, true, nil
		} else if errors.Is(err, os.ErrNotExist) {
			return pathArg, &policy.Config{Version: 1, Profiles: map[string]policy.Profile{}}, nil, false, nil
		} else {
			return "", nil, nil, false, err
		}
	}
	path, err := findConfig(pathArg)
	if err != nil {
		wd, werr := os.Getwd()
		if werr != nil {
			return "", nil, nil, false, werr
		}
		return filepath.Join(wd, "safeslop.cue"), &policy.Config{Version: 1, Profiles: map[string]policy.Profile{}}, nil, false, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", nil, nil, false, err
	}
	cfg, err := policy.LoadBytes(b)
	if err != nil {
		return "", nil, nil, false, err
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]policy.Profile{}
	}
	return path, cfg, b, true, nil
}

func buildPromotionPlan(name string, sess engsession.Session, cfg *policy.Config, path string, policyBytes []byte, exists bool, grantIDs []string) (promotion.PlanIntent, policy.Profile, []byte, error) {
	source, err := promotionSourceSnapshot(sess)
	if err != nil {
		return promotion.PlanIntent{}, policy.Profile{}, nil, err
	}
	policySnap := promotionPolicySnapshot(policyBytes, exists, cfg)
	prof, candidate, err := renderPromotionCandidate(name, sess, cfg, grantIDs)
	if err != nil {
		return promotion.PlanIntent{}, policy.Profile{}, nil, err
	}
	plan, err := promotion.Prepare(promotion.Request{TargetName: name, GrantIDs: grantIDs}, source, policySnap, trust.Hash(candidate))
	if err != nil {
		return promotion.PlanIntent{}, policy.Profile{}, nil, err
	}
	plan.Policy.Hash = trust.Hash(policyBytes)
	plan.TargetPolicyPath = path
	return plan, prof, candidate, nil
}

func promotionSourceSnapshot(sess engsession.Session) (promotion.SourceSnapshot, error) {
	if sess.Profile != "" || sess.PolicyPath != "" {
		return promotion.SourceSnapshot{}, promotion.ErrIneligibleSource
	}
	grants := make([]promotion.Grant, 0, len(sess.EgressGrants))
	for _, grant := range sess.EgressGrants {
		grants = append(grants, promotion.Grant{ID: grant.ID, Host: grant.Host, Port: grant.Port})
	}
	return promotion.SourceSnapshot{ID: sess.ID, AdHoc: true, Status: sess.Status, ETag: promotionSessionETag(sess), GrantRevision: sess.GrantRevision, Grants: grants}, nil
}

func promotionSessionETag(sess engsession.Session) string {
	return trust.Hash([]byte(fmt.Sprintf("%s|%s|%s|%s|%s|%d|%v|%v", sess.ID, sess.Agent, sess.Workspace, sess.Environment, sess.Network, sess.GrantRevision, sess.EgressGrants, sess.Resolved)))
}

func promotionPolicySnapshot(b []byte, exists bool, cfg *policy.Config) promotion.PolicySnapshot {
	profiles := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		profiles = append(profiles, name)
	}
	builtins := []string{}
	for _, b := range policy.BuiltinProfiles() {
		builtins = append(builtins, b.Name)
	}
	return promotion.PolicySnapshot{Exists: exists, Hash: trust.Hash(b), Profiles: profiles, Builtins: builtins}
}

func renderPromotionCandidate(name string, sess engsession.Session, cfg *policy.Config, grantIDs []string) (policy.Profile, []byte, error) {
	candidate := *cfg
	candidate.Profiles = make(map[string]policy.Profile, len(cfg.Profiles)+1)
	for n, p := range cfg.Profiles {
		candidate.Profiles[n] = p
	}
	if _, exists := candidate.Profiles[name]; exists {
		return policy.Profile{}, nil, promotion.ErrTargetExists
	}
	if _, builtin := policy.BuiltinProfileByName(name); builtin {
		return policy.Profile{}, nil, promotion.ErrTargetExists
	}
	selected := map[string]bool{}
	for _, id := range grantIDs {
		selected[id] = true
	}
	prof := policy.Profile{Agent: sess.Agent, Environment: sess.Environment, Network: sess.Network, Workspace: sess.Workspace}
	for _, grant := range sess.EgressGrants {
		if selected[grant.ID] {
			prof.PersistentEgress = append(prof.PersistentEgress, policy.PersistentEgressRule{FQDN: grant.Host, Port: grant.Port})
		}
	}
	if sess.Resolved != nil {
		if len(sess.Resolved.IdentitySet) == 0 {
			return policy.Profile{}, nil, promotion.ErrInvalidRequest
		}
		prof.Packages = append([]string(nil), sess.Resolved.IdentitySet...)
		prof.BareAgent = true
	}
	candidate.Profiles[name] = prof
	rendered, err := renderConfigCUE(&candidate)
	if err != nil {
		return policy.Profile{}, nil, err
	}
	return prof, rendered, nil
}

func expectedPromotionHash(p promotion.PolicySnapshot) string {
	if !p.Exists {
		return ""
	}
	return p.Hash
}

func promotePreviewData(plan promotion.PlanIntent, prof policy.Profile, path string, exists bool) map[string]any {
	return map[string]any{"profile": plan.TargetName, "path": path, "source_session_id": plan.Source.ID, "source_status": plan.Source.Status, "target_policy_state": map[string]any{"exists": exists, "hash": plan.Policy.Hash}, "selected_grant_ids": plan.SelectedGrantIDs, "selected_destinations": plan.Rules, "candidate_policy_hash": plan.CandidateHash, "profile_summary": prof, "session_unchanged": true, "trusted": false}
}

func promoteSessionError(id string, err error) error {
	if errors.Is(err, engsession.ErrNotFound) {
		return emitContractError(jsoncontract.CodeSessionNotFound, "session not found", map[string]any{"session_id": id})
	}
	return emitContractError(jsoncontract.CodeIOError, "load session", nil)
}
func promoteError(err error) error {
	if errors.Is(err, promotion.ErrIneligibleSource) || errors.Is(err, promotion.ErrTargetExists) || errors.Is(err, promotion.ErrInvalidRequest) || errors.Is(err, promotion.ErrStaleSource) || errors.Is(err, promotion.ErrStalePolicy) || errors.Is(err, promotion.ErrApplyRunning) || errors.Is(err, promotion.ErrInvalidCandidate) {
		return emitContractError(jsoncontract.CodeInvalidArgument, err.Error(), nil)
	}
	return emitContractError(jsoncontract.CodeIOError, "promotion failed", nil)
}
