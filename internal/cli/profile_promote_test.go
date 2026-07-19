package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	engsession "github.com/freakhill/safeslop/internal/engine/session"
	"github.com/freakhill/safeslop/internal/engine/trust"
)

func TestProfilePromotePreviewApplyAdHocSessionValueFree(t *testing.T) {
	ws := t.TempDir()
	state := t.TempDir()
	t.Setenv("SAFESLOP_STATE_DIR", state)
	out, err := runRootForTest(t, ws, "session", "create", "--agent", "pi", "--environment", "container", "--network", "deny", "--workspace", ws, "--output", "json")
	if err != nil {
		t.Fatalf("create: %v\n%s", err, out)
	}
	sessionID := parseEnvelopeForTest(t, out).Data["session_id"].(string)
	out, err = runRootForTest(t, ws, "session", "egress", "grant", "--session-id", sessionID, "--host", "API.Example.com", "--port", "443", "--output", "json")
	if err != nil {
		t.Fatalf("grant: %v\n%s", err, out)
	}
	grants := parseEnvelopeForTest(t, out).Data["egress_grants"].([]any)
	grantID := grants[0].(map[string]any)["id"].(string)
	plan := filepath.Join(ws, "promotion.plan.json")
	out, err = runRootForTest(t, ws, "profile", "promote", "preview", "saved", "--session-id", sessionID, "--grant-id", grantID, "--plan", plan, "--output", "json")
	if err != nil {
		t.Fatalf("preview: %v\n%s", err, out)
	}
	preview := parseEnvelopeForTest(t, out)
	if !preview.OK || preview.Data["profile"] != "saved" || preview.Data["trusted"] != false || preview.Data["session_unchanged"] != true {
		t.Fatalf("preview = %+v", preview)
	}
	if strings.Contains(out, "op://") || strings.Contains(out, "TOKEN") {
		t.Fatalf("preview leaked value-like content: %s", out)
	}
	if info, err := os.Stat(plan); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("plan mode = %v %v", info, err)
	}
	out, err = runRootForTest(t, ws, "profile", "promote", "apply", "--plan", plan, "--output", "json")
	if err != nil {
		t.Fatalf("apply: %v\n%s", err, out)
	}
	applied := parseEnvelopeForTest(t, out)
	if !applied.OK || applied.Data["applied"] != true || applied.Data["trusted"] != false {
		t.Fatalf("apply = %+v", applied)
	}
	cue, err := os.ReadFile(filepath.Join(ws, "safeslop.cue"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cue), "persistentEgress") || !strings.Contains(string(cue), "api.example.com") {
		t.Fatalf("promoted CUE missing egress: %s", cue)
	}
	stored, err := sessionStore().Get(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Profile != "" || len(stored.EgressGrants) != 1 {
		t.Fatalf("promotion mutated session: %+v", stored)
	}
	if _, _, status, err := checkTrust(filepath.Join(ws, "safeslop.cue")); err != nil || status == trust.Trusted {
		t.Fatalf("promoted policy should be untrusted/changed, status=%v err=%v", status, err)
	}
}

func TestProfilePromoteRejectsProfileBackedAndRunningApply(t *testing.T) {
	ws := t.TempDir()
	t.Setenv("SAFESLOP_STATE_DIR", t.TempDir())
	if err := os.WriteFile(filepath.Join(ws, "safeslop.cue"), []byte("package safeslop\n\nsafeslop: {version: 1, profiles: base: {agent: \"pi\", environment: \"container\", network: \"deny\"}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	trustFixtureForTest(t, ws)
	out, err := runRootForTest(t, ws, "session", "create", "--profile", "base", "--output", "json")
	if err != nil {
		t.Fatalf("profile create: %v\n%s", err, out)
	}
	profileSessionID := parseEnvelopeForTest(t, out).Data["session_id"].(string)
	plan := filepath.Join(ws, "bad.plan")
	out, err = runRootForTest(t, ws, "profile", "promote", "preview", "saved", "--session-id", profileSessionID, "--plan", plan, "--output", "json")
	if !errors.Is(err, errOutputEmitted) {
		t.Fatalf("profile-backed preview err=%v out=%s", err, out)
	}

	out, err = runRootForTest(t, ws, "session", "create", "--agent", "pi", "--environment", "container", "--network", "deny", "--workspace", ws, "--output", "json")
	if err != nil {
		t.Fatalf("ad hoc create: %v\n%s", err, out)
	}
	id := parseEnvelopeForTest(t, out).Data["session_id"].(string)
	plan = filepath.Join(ws, "run.plan")
	out, err = runRootForTest(t, ws, "profile", "promote", "preview", "saved2", "--session-id", id, "--plan", plan, "--output", "json")
	if err != nil {
		t.Fatalf("preview: %v\n%s", err, out)
	}
	_, err = sessionStore().Update(id, func(sess engsession.Session) (engsession.Session, error) {
		sess.Status = engsession.StatusRunning
		return sess, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err = runRootForTest(t, ws, "profile", "promote", "apply", "--plan", plan, "--output", "json")
	if !errors.Is(err, errOutputEmitted) || !strings.Contains(out, "created or stopped") {
		t.Fatalf("running apply err=%v out=%s", err, out)
	}
}
