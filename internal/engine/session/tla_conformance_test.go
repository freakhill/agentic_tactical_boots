package session

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

const (
	tlaSessionCFGPath          = "../../../formal/session/SessionBoundary.cfg"
	tlaSessionDefaultGraphPath = "../../../.build/tla/session/model/positive/states.dot"
)

type protocolGraphCoverage struct {
	actions  map[ProtocolAction]bool
	outcomes map[ProtocolOutcome]bool
	effects  map[ProtocolEffect]bool
	labels   map[ProtocolLabel]bool
}

func TestTLAConformanceBoundsComeFromPositiveCFG(t *testing.T) {
	data, err := os.ReadFile(tlaSessionCFGPath)
	if err != nil {
		t.Fatalf("read positive cfg: %v", err)
	}
	bounds, err := parseProtocolBoundsCFG(data)
	if err != nil {
		t.Fatalf("parse positive cfg bounds: %v", err)
	}
	if bounds.MaxRevision != 2 || !reflect.DeepEqual(bounds.Grants, []ProtocolGrant{ProtocolGrantA, ProtocolGrantB}) {
		t.Fatalf("bounds = %+v", bounds)
	}
	for name, input := range map[string]string{
		"missing":   `CONSTANT Mutant = "None"`,
		"duplicate": "CONSTANT Mutant = \"None\"\nCONSTANT MaxRevision = 2\nCONSTANT MaxRevision = 3\n",
		"invalid":   "CONSTANT Mutant = \"None\"\nCONSTANT MaxRevision = nope\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseProtocolBoundsCFG([]byte(input)); err == nil {
				t.Fatal("invalid cfg bounds unexpectedly parsed")
			}
		})
	}
}

func TestTLAConformanceGraphsEqualBidirectionally(t *testing.T) {
	bounds := readProtocolBoundsForConformance(t)
	tlaGraph := readTLCConformanceGraph(t)
	initialStates := adapterRoundTrippedProtocolInitialStates(t, bounds)
	initialKeys := make(map[string]struct{}, len(initialStates))
	for _, state := range initialStates {
		key, err := canonicalProtocolStateJSON(state)
		if err != nil {
			t.Fatalf("canonical Go initial: %v", err)
		}
		initialKeys[key] = struct{}{}
	}
	if key, ok := firstGraphKeyDifference(tlaGraph.Initial, initialKeys); ok {
		t.Fatalf("TLA - Go initial before BFS: %s", key)
	}
	if key, ok := firstGraphKeyDifference(initialKeys, tlaGraph.Initial); ok {
		t.Fatalf("Go - TLA initial before BFS: %s", key)
	}

	goGraph, coverage := enumerateGoProtocolGraph(t, bounds, initialStates)
	if err := compareProtocolGraphs("TLA", tlaGraph, "Go", goGraph); err != nil {
		t.Fatal(err)
	}
	assertProtocolGraphCoverage(t, coverage)
	t.Logf("raw TLC nodes=%d edges=%d; normalized states=%d edges=%d", tlaGraph.RawNodeCount, tlaGraph.RawEdgeCount, len(tlaGraph.States), len(tlaGraph.Edges))
}

func TestTLAConformanceDroppedEdgeControl(t *testing.T) {
	bounds := readProtocolBoundsForConformance(t)
	tlaGraph := readTLCConformanceGraph(t)
	goGraph, _ := enumerateGoProtocolGraph(t, bounds, adapterRoundTrippedProtocolInitialStates(t, bounds))
	if err := compareProtocolGraphs("TLA", tlaGraph, "Go", goGraph); err != nil {
		t.Fatalf("control precondition: %v", err)
	}
	mutant := cloneProtocolGraph(goGraph)
	edge := firstProtocolGraphEdge(t, mutant)
	delete(mutant.Edges, edge)
	err := compareProtocolGraphs("TLA", tlaGraph, "Go", mutant)
	assertGraphMismatch(t, err, "TLA - Go edge", string(edge.Label), "shortest witness")
}

func parseProtocolBoundsCFG(data []byte) (ProtocolBounds, error) {
	var maxRevision *int
	mutantNone := false
	for lineNumber, line := range strings.Split(string(data), "\n") {
		lineNumber++
		if line == `CONSTANT Mutant = "None"` {
			mutantNone = true
			continue
		}
		const prefix = "CONSTANT MaxRevision = "
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		if maxRevision != nil {
			return ProtocolBounds{}, fmt.Errorf("line %d: duplicate MaxRevision", lineNumber)
		}
		value, err := strconv.Atoi(strings.TrimPrefix(line, prefix))
		if err != nil || value <= 0 {
			return ProtocolBounds{}, fmt.Errorf("line %d: invalid MaxRevision", lineNumber)
		}
		maxRevision = &value
	}
	if !mutantNone {
		return ProtocolBounds{}, fmt.Errorf("cfg is not the positive Mutant=None configuration")
	}
	if maxRevision == nil {
		return ProtocolBounds{}, fmt.Errorf("cfg has no MaxRevision bound")
	}
	return ProtocolBounds{MaxRevision: *maxRevision, Grants: []ProtocolGrant{ProtocolGrantA, ProtocolGrantB}}, nil
}

func readProtocolBoundsForConformance(t *testing.T) ProtocolBounds {
	t.Helper()
	data, err := os.ReadFile(tlaSessionCFGPath)
	if err != nil {
		t.Fatalf("read positive cfg: %v", err)
	}
	bounds, err := parseProtocolBoundsCFG(data)
	if err != nil {
		t.Fatalf("parse positive cfg bounds: %v", err)
	}
	return bounds
}

func readTLCConformanceGraph(t *testing.T) protocolGraph {
	t.Helper()
	path := os.Getenv("TLA_SESSION_GRAPH")
	if path == "" {
		path = tlaSessionDefaultGraphPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && os.Getenv("TLA_SESSION_REQUIRE_GRAPH") != "1" {
			t.Skip("full TLC graph absent; run make check-tla-session")
		}
		t.Fatalf("read TLC graph: %v", err)
	}
	graph, err := parseTLCActionGraph(data)
	if err != nil {
		t.Fatalf("parse TLC graph: %v", err)
	}
	return graph
}

func adapterRoundTrippedProtocolInitialStates(t *testing.T, bounds ProtocolBounds) []ProtocolState {
	t.Helper()
	initial := InitialProtocolStates(bounds)
	roundTripped := make([]ProtocolState, 0, len(initial))
	for index, want := range initial {
		concrete := Session{ID: fmt.Sprintf("tla-conformance-%d", index), Status: StatusCreated}
		adapter, mapped := NewProtocolAdapter(concrete)
		if mapped != want {
			t.Fatalf("initial %d concrete map = %+v, want %+v", index, mapped, want)
		}
		candidate, err := adapter.Candidate(mapped)
		if err != nil {
			t.Fatalf("initial %d candidate: %v", index, err)
		}
		if !reflect.DeepEqual(candidate, concrete) {
			t.Fatalf("initial %d concrete round trip changed representation", index)
		}
		_, remapped := NewProtocolAdapter(candidate)
		if remapped != want {
			t.Fatalf("initial %d remap = %+v, want %+v", index, remapped, want)
		}
		roundTripped = append(roundTripped, remapped)
	}
	return roundTripped
}

func enumerateGoProtocolGraph(t *testing.T, bounds ProtocolBounds, initial []ProtocolState) (protocolGraph, protocolGraphCoverage) {
	t.Helper()
	graph := protocolGraph{
		Initial: make(map[string]struct{}),
		States:  make(map[string]ProtocolState),
		Edges:   make(map[protocolGraphEdge]struct{}),
	}
	coverage := protocolGraphCoverage{
		actions:  make(map[ProtocolAction]bool),
		outcomes: make(map[ProtocolOutcome]bool),
		effects:  make(map[ProtocolEffect]bool),
		labels:   make(map[ProtocolLabel]bool),
	}
	queue := make([]ProtocolState, 0, len(initial))
	seen := make(map[ProtocolState]bool)
	for _, state := range initial {
		state = NormalizeProtocolState(state)
		key, err := canonicalProtocolStateJSON(state)
		if err != nil {
			t.Fatalf("canonical initial: %v", err)
		}
		graph.Initial[key] = struct{}{}
		queue = append(queue, state)
	}
	for len(queue) > 0 {
		state := NormalizeProtocolState(queue[0])
		queue = queue[1:]
		if seen[state] {
			continue
		}
		seen[state] = true
		from, err := canonicalProtocolStateJSON(state)
		if err != nil {
			t.Fatalf("canonical Go state: %v", err)
		}
		graph.States[from] = state
		coverage.effects[state.Effect] = true
		for _, event := range EnabledProtocolWithin(bounds, state) {
			next, ok := ReduceProtocolWithin(bounds, state, event)
			if !ok {
				t.Fatalf("Enabled returned rejected event %+v", event)
			}
			label, ok := ProtocolEventLabelWithin(bounds, state, event)
			if !ok {
				t.Fatalf("enabled event has no TLA label: %+v", event)
			}
			next = NormalizeProtocolState(next)
			to, err := canonicalProtocolStateJSON(next)
			if err != nil {
				t.Fatalf("canonical Go target: %v", err)
			}
			graph.States[to] = next
			graph.Edges[protocolGraphEdge{From: from, Label: label, To: to}] = struct{}{}
			coverage.actions[event.Action] = true
			coverage.outcomes[event.Outcome] = true
			coverage.effects[next.Effect] = true
			coverage.labels[label] = true
			if !seen[next] {
				queue = append(queue, next)
			}
		}
	}
	graph.RawNodeCount, graph.RawEdgeCount = len(graph.States), len(graph.Edges)
	return graph, coverage
}

func assertProtocolGraphCoverage(t *testing.T, coverage protocolGraphCoverage) {
	t.Helper()
	for _, action := range protocolActions {
		if !coverage.actions[action] {
			t.Errorf("Go action %q is unreachable", action)
		}
	}
	for _, outcome := range protocolOutcomes {
		if outcome != "" && !coverage.outcomes[outcome] {
			t.Errorf("Go outcome %q is unreachable", outcome)
		}
	}
	for _, effect := range []ProtocolEffect{ProtocolCommit, ProtocolApply, ProtocolInspect, ProtocolTeardownEffect} {
		if !coverage.effects[effect] {
			t.Errorf("Go effect %q is unreachable", effect)
		}
	}
	for label := range knownTLCActionLabels() {
		if !coverage.labels[label] {
			t.Errorf("TLA action %q has no reachable Go mapping", label)
		}
	}
}
