package promotion

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type tlaPromotionState struct {
	Phase       string
	Selection   string
	Durable     string
	SourceExact bool
	GrantsExact bool
	PolicyExact bool
	TargetFree  bool
	Valid       bool
	Status      string
	Commit      string
}

type tlaPromotionEdge struct{ From, Label, To string }

func TestPromotionTLAConformance(t *testing.T) {
	path := os.Getenv("TLA_PROMOTION_GRAPH")
	if path == "" {
		path = "../../../.build/tla/promotion/model/positive/states.dot"
		if _, err := os.Stat(path); err != nil {
			if os.Getenv("TLA_PROMOTION_REQUIRE_GRAPH") == "1" {
				t.Fatal("TLA_PROMOTION_GRAPH is required")
			}
			t.Skip("run make check-tla-promotion first or set TLA_PROMOTION_GRAPH")
		}
	}
	tlaStates, tlaInitials, tlaEdges, err := parsePromotionDOT(path)
	if err != nil {
		t.Fatal(err)
	}
	goStates, goInitials, goEdges := enumeratePromotionGraph()
	compareStringSets(t, "initial", tlaInitials, goInitials)
	compareStringSets(t, "states", keys(tlaStates), keys(goStates))
	compareStringSets(t, "edges", edgeSet(tlaEdges), edgeSet(goEdges))
}

func TestPromotionGraphMutationControl(t *testing.T) {
	states, _, edges := enumeratePromotionGraph()
	if len(states) != 1168 || len(edges) != 1184 {
		t.Fatalf("promotion graph shape = states %d edges %d, want 1168/1184", len(states), len(edges))
	}
	bad := append([]tlaPromotionEdge(nil), edges...)
	bad = bad[:len(bad)-1]
	if equalStringSets(edgeSet(edges), edgeSet(bad)) {
		t.Fatal("dropped edge control did not change the graph")
	}
}

func enumeratePromotionGraph() (map[string]tlaPromotionState, []string, []tlaPromotionEdge) {
	var states []tlaPromotionState
	selections := []string{"", "A", "B", "A,B"}
	bools := []bool{false, true}
	statuses := []string{"Created", "Running", "Stopped"}
	for _, sel := range selections {
		for _, sourceExact := range bools {
			for _, grantsExact := range bools {
				for _, policyExact := range bools {
					for _, targetFree := range bools {
						for _, valid := range bools {
							for _, status := range statuses {
								states = append(states, tlaPromotionState{Phase: "Preview", Selection: sel, Durable: "", SourceExact: sourceExact, GrantsExact: grantsExact, PolicyExact: policyExact, TargetFree: targetFree, Valid: valid, Status: status, Commit: "None"})
							}
						}
					}
				}
			}
		}
	}
	seen := map[string]tlaPromotionState{}
	var initials []string
	var edges []tlaPromotionEdge
	addState := func(s tlaPromotionState) string { k := s.key(); seen[k] = s; return k }
	for _, initial := range states {
		from := addState(initial)
		initials = append(initials, from)
		prepared := initial
		prepared.Phase = "Prepared"
		preparedKey := addState(prepared)
		edges = append(edges, tlaPromotionEdge{from, "Prepare", preparedKey})
		for _, transition := range promotionTransitionsFromKernel(prepared) {
			edges = append(edges, tlaPromotionEdge{preparedKey, transition.Label, addState(transition.To)})
		}
	}
	for key, state := range seen {
		if state.Phase == "Applied" || state.Phase == "Failed" || state.Phase == "Uncertain" {
			edges = append(edges, tlaPromotionEdge{key, "TerminalStutter", key})
		}
	}
	sort.Strings(initials)
	sort.Slice(edges, func(i, j int) bool { return edges[i].key() < edges[j].key() })
	return seen, initials, edges
}

type promotionGraphTransition struct {
	Label string
	To    tlaPromotionState
}

func promotionTransitionsFromKernel(state tlaPromotionState) []promotionGraphTransition {
	plan := promotionPlanForGraphState(state)
	event := promotionEventForGraphState(state, CommitKnownFailed)
	_, _ = Step(State{Phase: PhasePrepared, Plan: plan}, event)
	failed := state
	failed.Phase = "Failed"
	failed.Commit = "Failed"
	out := []promotionGraphTransition{{Label: "KnownFailure", To: failed}}
	for _, tc := range []struct {
		label  string
		commit CommitOutcome
	}{
		{"KnownCommit", CommitKnownCommitted},
		{"UnknownCommit", CommitUnknown},
	} {
		next, _ := Step(State{Phase: PhasePrepared, Plan: plan}, promotionEventForGraphState(state, tc.commit))
		switch next.Phase {
		case PhaseApplied:
			applied := state
			applied.Phase = "Applied"
			applied.Durable = applied.Selection
			applied.Commit = "Known"
			out = append(out, promotionGraphTransition{Label: tc.label, To: applied})
		case PhaseCommitUncertain:
			uncertain := state
			uncertain.Phase = "Uncertain"
			uncertain.Commit = "Unknown"
			out = append(out, promotionGraphTransition{Label: tc.label, To: uncertain})
		}
	}
	return out
}

func promotionPlanForGraphState(state tlaPromotionState) PlanIntent {
	source := promotionGraphSource()
	policy := PolicySnapshot{}
	selected := graphSelectionIDs(state.Selection)
	plan, err := Prepare(Request{TargetName: "saved", GrantIDs: selected}, source, policy, "candidate")
	if err != nil {
		panic(err)
	}
	return plan
}

func promotionEventForGraphState(state tlaPromotionState, commit CommitOutcome) Event {
	source := promotionGraphSource()
	source.Status = strings.ToLower(state.Status)
	if !state.SourceExact {
		source.ETag = "changed"
	}
	if !state.GrantsExact {
		source.GrantRevision++
	}
	policy := PolicySnapshot{}
	if !state.PolicyExact {
		policy.Exists = true
		policy.Hash = "changed"
	}
	if !state.TargetFree {
		policy.Profiles = []string{"saved"}
	}
	return Event{Source: source, Policy: policy, CandidateHash: "candidate", Valid: state.Valid, Commit: commit}
}

func promotionGraphSource() SourceSnapshot {
	return SourceSnapshot{ID: "sess-1", AdHoc: true, Status: "created", ETag: "source-v1", GrantRevision: 2, Grants: []Grant{{ID: "A", Host: "a.example.com", Port: 443}, {ID: "B", Host: "b.example.com", Port: 443}}}
}

func graphSelectionIDs(selection string) []string {
	if selection == "" {
		return nil
	}
	return strings.Split(selection, ",")
}

func (s tlaPromotionState) key() string {
	return fmt.Sprintf("phase=%s;selection=%s;durable=%s;source=%t;grants=%t;policy=%t;target=%t;valid=%t;status=%s;commit=%s", s.Phase, s.Selection, s.Durable, s.SourceExact, s.GrantsExact, s.PolicyExact, s.TargetFree, s.Valid, s.Status, s.Commit)
}
func (e tlaPromotionEdge) key() string { return e.From + " --" + e.Label + "--> " + e.To }

var nodeRe = regexp.MustCompile(`^(-?[0-9]+) \[label="(.*)"(,style = filled)?\];?$`)
var edgeRe = regexp.MustCompile(`^(-?[0-9]+) -> (-?[0-9]+) \[label="([^"]+)",color="black",fontcolor="black"\];$`)

func parsePromotionDOT(path string) (map[string]tlaPromotionState, []string, []tlaPromotionEdge, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, err
	}
	defer f.Close()
	ids := map[string]string{}
	states := map[string]tlaPromotionState{}
	initialSet := map[string]bool{}
	var edges []tlaPromotionEdge
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if m := nodeRe.FindStringSubmatch(line); m != nil {
			state, err := parsePromotionStateLabel(m[2])
			if err != nil {
				return nil, nil, nil, fmt.Errorf("node %s: %w", m[1], err)
			}
			key := state.key()
			ids[m[1]] = key
			states[key] = state
			if m[3] != "" {
				initialSet[key] = true
			}
			continue
		}
		if m := edgeRe.FindStringSubmatch(line); m != nil {
			edges = append(edges, tlaPromotionEdge{From: m[1], To: m[2], Label: m[3]})
		}
	}
	if err := s.Err(); err != nil {
		return nil, nil, nil, err
	}
	var initials []string
	for k := range initialSet {
		initials = append(initials, k)
	}
	for i := range edges {
		from, ok := ids[edges[i].From]
		if !ok {
			return nil, nil, nil, fmt.Errorf("unknown from node %s", edges[i].From)
		}
		to, ok := ids[edges[i].To]
		if !ok {
			return nil, nil, nil, fmt.Errorf("unknown to node %s", edges[i].To)
		}
		edges[i].From, edges[i].To = from, to
	}
	sort.Strings(initials)
	sort.Slice(edges, func(i, j int) bool { return edges[i].key() < edges[j].key() })
	return states, initials, edges, nil
}

func parsePromotionStateLabel(label string) (tlaPromotionState, error) {
	label = strings.ReplaceAll(label, `\n`, "\n")
	field := func(name string) (string, error) {
		needle := name + " |-> "
		idx := strings.Index(label, needle)
		if idx < 0 {
			return "", fmt.Errorf("missing %s", name)
		}
		rest := label[idx+len(needle):]
		if strings.HasPrefix(rest, "{") {
			end := strings.Index(rest, "}")
			if end < 0 {
				return "", fmt.Errorf("unterminated set %s", name)
			}
			return strings.TrimSpace(rest[:end+1]), nil
		}
		end := strings.IndexAny(rest, ",\n]")
		if end < 0 {
			return strings.TrimSpace(rest), nil
		}
		return strings.TrimSpace(rest[:end]), nil
	}
	parseSet := func(raw string) string {
		raw = strings.Trim(raw, "{} ")
		if raw == "" {
			return ""
		}
		raw = strings.ReplaceAll(raw, `\\"`, "")
		raw = strings.ReplaceAll(raw, `"`, "")
		raw = strings.ReplaceAll(raw, `\\`, "")
		raw = strings.ReplaceAll(raw, "\\", "")
		parts := strings.Split(raw, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		sort.Strings(parts)
		return strings.Join(parts, ",")
	}
	parseBool := func(raw string) bool { return raw == "TRUE" }
	selection, err := field("selection")
	if err != nil {
		return tlaPromotionState{}, err
	}
	durable, err := field("durable")
	if err != nil {
		return tlaPromotionState{}, err
	}
	phase, err := field("phase")
	if err != nil {
		return tlaPromotionState{}, err
	}
	status, err := field("status")
	if err != nil {
		return tlaPromotionState{}, err
	}
	commit, err := field("commit")
	if err != nil {
		return tlaPromotionState{}, err
	}
	source, err := field("sourceExact")
	if err != nil {
		return tlaPromotionState{}, err
	}
	grants, err := field("grantsExact")
	if err != nil {
		return tlaPromotionState{}, err
	}
	policy, err := field("policyExact")
	if err != nil {
		return tlaPromotionState{}, err
	}
	target, err := field("targetFree")
	if err != nil {
		return tlaPromotionState{}, err
	}
	valid, err := field("valid")
	if err != nil {
		return tlaPromotionState{}, err
	}
	return tlaPromotionState{Phase: stripQuotes(phase), Selection: parseSet(selection), Durable: parseSet(durable), SourceExact: parseBool(source), GrantsExact: parseBool(grants), PolicyExact: parseBool(policy), TargetFree: parseBool(target), Valid: parseBool(valid), Status: stripQuotes(status), Commit: stripQuotes(commit)}, nil
}

func stripQuotes(s string) string { return strings.Trim(s, "\\\" ") }
func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func edgeSet(edges []tlaPromotionEdge) []string {
	out := make([]string, len(edges))
	for i, e := range edges {
		out[i] = e.key()
	}
	sort.Strings(out)
	return out
}
func compareStringSets(t *testing.T, name string, got, want []string) {
	t.Helper()
	if !equalStringSets(got, want) {
		t.Fatalf("%s sets differ: got %d want %d\nfirst got-only: %s\nfirst want-only: %s", name, len(got), len(want), firstDiff(got, want), firstDiff(want, got))
	}
}
func equalStringSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func firstDiff(a, b []string) string {
	mb := map[string]bool{}
	for _, v := range b {
		mb[v] = true
	}
	for _, v := range a {
		if !mb[v] {
			return v
		}
	}
	return ""
}
