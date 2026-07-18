package session

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type protocolGraphEdge struct {
	From  string
	Label ProtocolLabel
	To    string
}

type protocolGraph struct {
	Initial      map[string]struct{}
	States       map[string]ProtocolState
	Edges        map[protocolGraphEdge]struct{}
	RawNodeCount int
	RawEdgeCount int
}

func TestTLCGraphParsesPinnedActionLabelFixtureDeterministically(t *testing.T) {
	fixture := readTLCGraphFixture(t)
	first, err := parseTLCActionGraph(fixture)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	second, err := parseTLCActionGraph(fixture)
	if err != nil {
		t.Fatalf("parse fixture again: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("pinned fixture parse is nondeterministic")
	}
	if first.RawNodeCount != 3 || first.RawEdgeCount != 2 || len(first.States) != 3 || len(first.Edges) != 2 || len(first.Initial) != 1 {
		t.Fatalf("fixture graph counts = raw nodes %d raw edges %d states %d edges %d initial %d", first.RawNodeCount, first.RawEdgeCount, len(first.States), len(first.Edges), len(first.Initial))
	}
	initialKey, err := canonicalProtocolStateJSON(InitialProtocolStates(ProtocolBounds{})[0])
	if err != nil {
		t.Fatalf("initial key: %v", err)
	}
	if _, ok := first.Initial[initialKey]; !ok {
		t.Fatalf("style=filled initial was not recognized; initials=%v", sortedGraphKeys(first.Initial))
	}
}

func TestTLCParserRejectsUnpinnedSyntaxAndMalformedGraphs(t *testing.T) {
	fixture := string(readTLCGraphFixture(t))
	initialLine := firstLineContaining(t, fixture, `-2485336354511760763 [label=`)
	tests := []struct {
		name    string
		mutate  func(string) string
		wantErr string
	}{
		{
			name: "unknown attribute",
			mutate: func(input string) string {
				return strings.Replace(input, `,fontcolor="black"]`, `,fontcolor="black",penwidth=1]`, 1)
			},
			wantErr: "unknown DOT syntax or attribute",
		},
		{
			name: "malformed escape",
			mutate: func(input string) string {
				return strings.Replace(input, `\n  owners`, `\t  owners`, 1)
			},
			wantErr: "unsupported DOT escape",
		},
		{
			name: "duplicate node",
			mutate: func(input string) string {
				return strings.Replace(input, initialLine, initialLine+"\n"+initialLine, 1)
			},
			wantErr: "duplicate node",
		},
		{
			name: "missing node",
			mutate: func(input string) string {
				return strings.Replace(input, `-2485336354511760763 -> -4960434399711919478`, `-2485336354511760763 -> 999`, 1)
			},
			wantErr: "edge references missing node",
		},
		{
			name: "unknown action",
			mutate: func(input string) string {
				return strings.Replace(input, `label="ClaimStart"`, `label="InventedAction"`, 1)
			},
			wantErr: "unknown action",
		},
		{
			name: "unknown state field",
			mutate: func(input string) string {
				return strings.Replace(input, `status |->`, `mystery |->`, 1)
			},
			wantErr: "unexpected state fields",
		},
		{
			name: "node after rank metadata",
			mutate: func(input string) string {
				without := strings.Replace(input, initialLine+"\n", "", 1)
				return strings.Replace(without, "{rank = same; -2485336354511760763;}", "{rank = same; -2485336354511760763;}\n"+initialLine, 1)
			},
			wantErr: "node after rank metadata",
		},
		{
			name:    "trailing LF outside pinned grammar",
			mutate:  func(input string) string { return input + "\n" },
			wantErr: "unpinned trailing LF",
		},
		{
			name: "incomplete rank metadata",
			mutate: func(input string) string {
				return strings.Replace(input, "{rank = same; 7173379135665419145;}\n", "", 1)
			},
			wantErr: "rank metadata omits a node",
		},
		{
			name: "invalid dropped field value",
			mutate: func(input string) string {
				return strings.Replace(input, `evidence |-> \"None\"`, `evidence |-> \"Invented\"`, 1)
			},
			wantErr: "unknown evidence",
		},
		{
			name: "duplicate state field",
			mutate: func(input string) string {
				return strings.Replace(input, `status |-> \"Created\",`, `status |-> \"Created\", status |-> \"Created\",`, 1)
			},
			wantErr: "duplicate TLA record field",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseTLCActionGraph([]byte(tt.mutate(fixture)))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestInitialStateRequiresMechanicalFilledMarker(t *testing.T) {
	fixture := strings.Replace(string(readTLCGraphFixture(t)), `,style = filled]`, `];`, 1)
	_, err := parseTLCActionGraph([]byte(fixture))
	if err == nil || !strings.Contains(err.Error(), "absent initial marker") {
		t.Fatalf("error = %v, want absent initial marker", err)
	}
}

func TestTLCParserCanonicalizesSetsAndSentinels(t *testing.T) {
	state := InitialProtocolStates(ProtocolBounds{})[0]
	state.Owners = ProtocolOwnerA | ProtocolOwnerB
	state.DurableAuthority = ProtocolGrantSet(ProtocolGrantA | ProtocolGrantB)
	state.RuntimeGeneration = protocolNoGeneration()
	key, err := canonicalProtocolStateJSON(state)
	if err != nil {
		t.Fatalf("canonical state: %v", err)
	}
	for _, want := range []string{
		`"owners":["OwnerA","OwnerB"]`,
		`"durable_authority":["GrantA","GrantB"]`,
		`"runtime_generation":"NoGen"`,
	} {
		if !strings.Contains(key, want) {
			t.Fatalf("canonical key %s does not contain %s", key, want)
		}
	}
	if strings.Contains(key, "evidence") || strings.Contains(key, "txOwner") {
		t.Fatalf("checker-only field leaked into canonical key %s", key)
	}
}

func TestGraphComparatorDirectionalEdgeControls(t *testing.T) {
	baseline, err := parseTLCActionGraph(readTLCGraphFixture(t))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	edge := firstProtocolGraphEdge(t, baseline)

	t.Run("drop edge", func(t *testing.T) {
		mutant := cloneProtocolGraph(baseline)
		delete(mutant.Edges, edge)
		err := compareProtocolGraphs("TLA", baseline, "Go", mutant)
		assertGraphMismatch(t, err, "TLA - Go edge", string(edge.Label), "shortest witness")
	})

	t.Run("add edge", func(t *testing.T) {
		mutant := cloneProtocolGraph(baseline)
		extra := protocolGraphEdge{From: edge.From, Label: ProtocolLabelReleaseExact, To: edge.To}
		if extra == edge {
			extra.Label = ProtocolLabelHandoffExact
		}
		mutant.Edges[extra] = struct{}{}
		err := compareProtocolGraphs("TLA", baseline, "Go", mutant)
		assertGraphMismatch(t, err, "Go - TLA edge", string(extra.Label), "shortest witness")
	})

	t.Run("rename edge", func(t *testing.T) {
		mutant := cloneProtocolGraph(baseline)
		delete(mutant.Edges, edge)
		renamed := edge
		renamed.Label = ProtocolLabelHandoffExact
		if renamed.Label == edge.Label {
			renamed.Label = ProtocolLabelReleaseExact
		}
		mutant.Edges[renamed] = struct{}{}
		err := compareProtocolGraphs("TLA", baseline, "Go", mutant)
		assertGraphMismatch(t, err, "TLA - Go edge", string(edge.Label), "shortest witness")
	})
}

func TestGraphComparatorReportsAlteredInitialStateDirection(t *testing.T) {
	baseline, err := parseTLCActionGraph(readTLCGraphFixture(t))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	mutant := cloneProtocolGraph(baseline)
	oldKey := sortedGraphKeys(mutant.Initial)[0]
	changed := mutant.States[oldKey]
	changed.Result = ProtocolFailure
	newKey, err := canonicalProtocolStateJSON(changed)
	if err != nil {
		t.Fatalf("changed key: %v", err)
	}
	replaceProtocolGraphStateKey(&mutant, oldKey, newKey, changed)
	err = compareProtocolGraphs("TLA", baseline, "Go", mutant)
	assertGraphMismatch(t, err, "TLA - Go initial", "shortest witness")
}

func readTLCGraphFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/tlc-v1.7.4-actionlabels.dot")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

func firstLineContaining(t *testing.T, input, needle string) string {
	t.Helper()
	for _, line := range strings.Split(input, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	t.Fatalf("fixture has no line containing %q", needle)
	return ""
}

func firstProtocolGraphEdge(t *testing.T, graph protocolGraph) protocolGraphEdge {
	t.Helper()
	edges := sortedProtocolGraphEdges(graph.Edges)
	if len(edges) == 0 {
		t.Fatal("graph has no edge")
	}
	return edges[0]
}

func cloneProtocolGraph(graph protocolGraph) protocolGraph {
	clone := protocolGraph{
		Initial:      make(map[string]struct{}, len(graph.Initial)),
		States:       make(map[string]ProtocolState, len(graph.States)),
		Edges:        make(map[protocolGraphEdge]struct{}, len(graph.Edges)),
		RawNodeCount: graph.RawNodeCount,
		RawEdgeCount: graph.RawEdgeCount,
	}
	for key := range graph.Initial {
		clone.Initial[key] = struct{}{}
	}
	for key, state := range graph.States {
		clone.States[key] = state
	}
	for edge := range graph.Edges {
		clone.Edges[edge] = struct{}{}
	}
	return clone
}

func replaceProtocolGraphStateKey(graph *protocolGraph, oldKey, newKey string, state ProtocolState) {
	delete(graph.States, oldKey)
	graph.States[newKey] = state
	if _, ok := graph.Initial[oldKey]; ok {
		delete(graph.Initial, oldKey)
		graph.Initial[newKey] = struct{}{}
	}
	replaced := make(map[protocolGraphEdge]struct{}, len(graph.Edges))
	for edge := range graph.Edges {
		if edge.From == oldKey {
			edge.From = newKey
		}
		if edge.To == oldKey {
			edge.To = newKey
		}
		replaced[edge] = struct{}{}
	}
	graph.Edges = replaced
}

func assertGraphMismatch(t *testing.T, err error, substrings ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("comparator unexpectedly accepted mutant")
	}
	for _, substring := range substrings {
		if !strings.Contains(err.Error(), substring) {
			t.Fatalf("error %q does not contain %q", err, substring)
		}
	}
}

func sortedGraphKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedProtocolGraphEdges(values map[protocolGraphEdge]struct{}) []protocolGraphEdge {
	edges := make([]protocolGraphEdge, 0, len(values))
	for edge := range values {
		edges = append(edges, edge)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].Label != edges[j].Label {
			return edges[i].Label < edges[j].Label
		}
		return edges[i].To < edges[j].To
	})
	return edges
}

type tlcRawGraphNode struct {
	state   ProtocolState
	initial bool
}

type tlcRawGraphEdge struct {
	from  string
	label ProtocolLabel
	to    string
}

var (
	tlcNodeLinePattern = regexp.MustCompile(`^(-?[0-9]+) \[label="((?:\\.|[^"\\])*)"(,style = filled)?\](;?)$`)
	tlcEdgeLinePattern = regexp.MustCompile(`^(-?[0-9]+) -> (-?[0-9]+) \[label="([A-Za-z][A-Za-z0-9]*)",color="black",fontcolor="black"\];$`)
	tlcNodeIDPattern   = regexp.MustCompile(`^-?[0-9]+$`)
)

func parseTLCActionGraph(data []byte) (protocolGraph, error) {
	text := string(data)
	if strings.Contains(text, "\r") {
		return protocolGraph{}, fmt.Errorf("TLC DOT must use LF line endings")
	}
	if strings.HasSuffix(text, "\n") {
		return protocolGraph{}, fmt.Errorf("TLC DOT has an unpinned trailing LF")
	}
	lines := strings.Split(text, "\n")
	if len(lines) < 7 || lines[0] != "strict digraph DiskGraph {" || lines[1] != "nodesep=0.35;" || lines[2] != "subgraph cluster_graph {" || lines[3] != `color="white";` || lines[len(lines)-2] != "}" || lines[len(lines)-1] != "}" {
		return protocolGraph{}, fmt.Errorf("unknown DOT graph envelope")
	}

	nodes := make(map[string]tlcRawGraphNode)
	rawEdges := make(map[tlcRawGraphEdge]struct{})
	ranked := make(map[string]struct{})
	seenRank := false
	for lineNumber, line := range lines[4 : len(lines)-2] {
		lineNumber += 5
		if line == "" {
			return protocolGraph{}, fmt.Errorf("line %d: unknown DOT syntax or attribute", lineNumber)
		}
		if match := tlcNodeLinePattern.FindStringSubmatch(line); match != nil {
			if seenRank {
				return protocolGraph{}, fmt.Errorf("line %d: node after rank metadata", lineNumber)
			}
			id := match[1]
			if (match[3] != "" && match[4] != "") || (match[3] == "" && match[4] != ";") {
				return protocolGraph{}, fmt.Errorf("line %d: unknown DOT syntax or attribute", lineNumber)
			}
			if _, duplicate := nodes[id]; duplicate {
				return protocolGraph{}, fmt.Errorf("line %d: duplicate node %s", lineNumber, id)
			}
			label, err := unescapeTLCDOTLabel(match[2])
			if err != nil {
				return protocolGraph{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			state, err := parseTLCStateLabel(label)
			if err != nil {
				return protocolGraph{}, fmt.Errorf("line %d node %s: %w", lineNumber, id, err)
			}
			nodes[id] = tlcRawGraphNode{state: state, initial: match[3] != ""}
			continue
		}
		if match := tlcEdgeLinePattern.FindStringSubmatch(line); match != nil {
			if seenRank {
				return protocolGraph{}, fmt.Errorf("line %d: edge after rank metadata", lineNumber)
			}
			label := ProtocolLabel(match[3])
			if !knownTLCActionLabels()[label] {
				return protocolGraph{}, fmt.Errorf("line %d: unknown action %q", lineNumber, label)
			}
			edge := tlcRawGraphEdge{from: match[1], to: match[2], label: label}
			if _, duplicate := rawEdges[edge]; duplicate {
				return protocolGraph{}, fmt.Errorf("line %d: duplicate edge", lineNumber)
			}
			rawEdges[edge] = struct{}{}
			continue
		}
		if strings.HasPrefix(line, "{rank = same;") {
			seenRank = true
			ids, err := parseTLCRankLine(line)
			if err != nil {
				return protocolGraph{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			for _, id := range ids {
				if _, duplicate := ranked[id]; duplicate {
					return protocolGraph{}, fmt.Errorf("line %d: duplicate rank node %s", lineNumber, id)
				}
				ranked[id] = struct{}{}
			}
			continue
		}
		return protocolGraph{}, fmt.Errorf("line %d: unknown DOT syntax or attribute", lineNumber)
	}

	if len(nodes) == 0 {
		return protocolGraph{}, fmt.Errorf("TLC graph has no nodes")
	}
	for edge := range rawEdges {
		if _, ok := nodes[edge.from]; !ok {
			return protocolGraph{}, fmt.Errorf("edge references missing node %s", edge.from)
		}
		if _, ok := nodes[edge.to]; !ok {
			return protocolGraph{}, fmt.Errorf("edge references missing node %s", edge.to)
		}
	}
	for id := range ranked {
		if _, ok := nodes[id]; !ok {
			return protocolGraph{}, fmt.Errorf("rank references missing node %s", id)
		}
	}
	if len(ranked) != len(nodes) {
		return protocolGraph{}, fmt.Errorf("rank metadata omits a node")
	}

	graph := protocolGraph{
		Initial:      make(map[string]struct{}),
		States:       make(map[string]ProtocolState),
		Edges:        make(map[protocolGraphEdge]struct{}),
		RawNodeCount: len(nodes),
		RawEdgeCount: len(rawEdges),
	}
	nodeKeys := make(map[string]string, len(nodes))
	for id, node := range nodes {
		key, err := canonicalProtocolStateJSON(node.state)
		if err != nil {
			return protocolGraph{}, fmt.Errorf("node %s canonical state: %w", id, err)
		}
		nodeKeys[id] = key
		graph.States[key] = node.state
		if node.initial {
			graph.Initial[key] = struct{}{}
		}
	}
	if len(graph.Initial) == 0 {
		return protocolGraph{}, fmt.Errorf("TLC graph has absent initial marker")
	}
	for edge := range rawEdges {
		graph.Edges[protocolGraphEdge{From: nodeKeys[edge.from], Label: edge.label, To: nodeKeys[edge.to]}] = struct{}{}
	}
	return graph, nil
}

func unescapeTLCDOTLabel(escaped string) (string, error) {
	var out strings.Builder
	out.Grow(len(escaped))
	for index := 0; index < len(escaped); index++ {
		if escaped[index] != '\\' {
			out.WriteByte(escaped[index])
			continue
		}
		if index+1 >= len(escaped) {
			return "", fmt.Errorf("unterminated DOT escape")
		}
		index++
		switch escaped[index] {
		case 'n':
			out.WriteByte('\n')
		case '"':
			out.WriteByte('"')
		default:
			return "", fmt.Errorf("unsupported DOT escape \\%c", escaped[index])
		}
	}
	return out.String(), nil
}

func parseTLCRankLine(line string) ([]string, error) {
	const prefix = "{rank = same; "
	if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, "}") {
		return nil, fmt.Errorf("invalid rank metadata")
	}
	body := strings.TrimSuffix(strings.TrimPrefix(line, prefix), "}")
	if body == "" || !strings.HasSuffix(body, ";") {
		return nil, fmt.Errorf("invalid rank metadata")
	}
	parts := strings.Split(strings.TrimSuffix(body, ";"), ";")
	ids := make([]string, 0, len(parts))
	for _, id := range parts {
		if !tlcNodeIDPattern.MatchString(id) {
			return nil, fmt.Errorf("invalid rank node %q", id)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func knownTLCActionLabels() map[ProtocolLabel]bool {
	labels := []ProtocolLabel{
		ProtocolLabelClaimStart,
		ProtocolLabelClaimCommitNew,
		ProtocolLabelClaimCommitOld,
		ProtocolLabelClaimCommitUnknown,
		ProtocolLabelRecoverClaim,
		ProtocolLabelHandoffExact,
		ProtocolLabelReleaseExact,
		ProtocolLabelWidenStart,
		ProtocolLabelWidenCommitNew,
		ProtocolLabelWidenCommitOld,
		ProtocolLabelWidenCommitUnknown,
		ProtocolLabelWidenApply,
		ProtocolLabelPositiveInspect,
		ProtocolLabelWidenFinalCommitNew,
		ProtocolLabelWidenFinalCommitOld,
		ProtocolLabelWidenFinalCommitUnknown,
		ProtocolLabelNarrowStart,
		ProtocolLabelNarrowApply,
		ProtocolLabelNarrowFinalCommitNew,
		ProtocolLabelNarrowFinalCommitOld,
		ProtocolLabelNarrowFinalCommitUnknown,
		ProtocolLabelRecoverEgress,
		ProtocolLabelObserveInvalidPersistence,
		ProtocolLabelStopStart,
		ProtocolLabelRequestTeardown,
		ProtocolLabelTeardownProven,
		ProtocolLabelTeardownUnknown,
		ProtocolLabelTerminalStutter,
	}
	known := make(map[ProtocolLabel]bool, len(labels))
	for _, label := range labels {
		known[label] = true
	}
	return known
}

type tlcValue interface{}
type tlcRecord map[string]tlcValue
type tlcSet []tlcValue

type tlcValueParser struct {
	input string
	at    int
}

func parseTLCStateLabel(label string) (ProtocolState, error) {
	parser := tlcValueParser{input: label}
	name, err := parser.parseIdentifier()
	if err != nil || name != "s" {
		return ProtocolState{}, fmt.Errorf("state label must begin with s")
	}
	if err := parser.expect("="); err != nil {
		return ProtocolState{}, err
	}
	value, err := parser.parseValue()
	if err != nil {
		return ProtocolState{}, err
	}
	parser.skipSpace()
	if parser.at != len(parser.input) {
		return ProtocolState{}, fmt.Errorf("unexpected TLA value suffix at byte %d", parser.at)
	}
	record, ok := value.(tlcRecord)
	if !ok {
		return ProtocolState{}, fmt.Errorf("state value is not a record")
	}
	return protocolStateFromTLCRecord(record)
}

func (p *tlcValueParser) parseValue() (tlcValue, error) {
	p.skipSpace()
	if p.at >= len(p.input) {
		return nil, fmt.Errorf("missing TLA value at byte %d", p.at)
	}
	switch p.input[p.at] {
	case '"':
		return p.parseString()
	case '[':
		return p.parseRecord()
	case '{':
		return p.parseSet()
	case '-':
		return p.parseInteger()
	default:
		if p.input[p.at] >= '0' && p.input[p.at] <= '9' {
			return p.parseInteger()
		}
		identifier, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}
		switch identifier {
		case "TRUE":
			return true, nil
		case "FALSE":
			return false, nil
		default:
			return nil, fmt.Errorf("unsupported bare TLA value %q", identifier)
		}
	}
}

func (p *tlcValueParser) parseRecord() (tlcValue, error) {
	if err := p.expect("["); err != nil {
		return nil, err
	}
	record := make(tlcRecord)
	p.skipSpace()
	if p.consume("]") {
		return record, nil
	}
	for {
		field, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}
		if _, duplicate := record[field]; duplicate {
			return nil, fmt.Errorf("duplicate TLA record field %q", field)
		}
		if err := p.expect("|->"); err != nil {
			return nil, err
		}
		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		record[field] = value
		p.skipSpace()
		if p.consume("]") {
			return record, nil
		}
		if err := p.expect(","); err != nil {
			return nil, err
		}
	}
}

func (p *tlcValueParser) parseSet() (tlcValue, error) {
	if err := p.expect("{"); err != nil {
		return nil, err
	}
	set := make(tlcSet, 0)
	p.skipSpace()
	if p.consume("}") {
		return set, nil
	}
	for {
		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		set = append(set, value)
		p.skipSpace()
		if p.consume("}") {
			return set, nil
		}
		if err := p.expect(","); err != nil {
			return nil, err
		}
	}
}

func (p *tlcValueParser) parseString() (tlcValue, error) {
	if p.at >= len(p.input) || p.input[p.at] != '"' {
		return nil, fmt.Errorf("missing TLA string at byte %d", p.at)
	}
	p.at++
	start := p.at
	for p.at < len(p.input) && p.input[p.at] != '"' {
		if p.input[p.at] == '\\' || p.input[p.at] < 0x20 {
			return nil, fmt.Errorf("unsupported TLA string syntax at byte %d", p.at)
		}
		p.at++
	}
	if p.at >= len(p.input) {
		return nil, fmt.Errorf("unterminated TLA string")
	}
	value := p.input[start:p.at]
	p.at++
	return value, nil
}

func (p *tlcValueParser) parseInteger() (tlcValue, error) {
	p.skipSpace()
	start := p.at
	if p.at < len(p.input) && p.input[p.at] == '-' {
		p.at++
	}
	digitStart := p.at
	for p.at < len(p.input) && p.input[p.at] >= '0' && p.input[p.at] <= '9' {
		p.at++
	}
	if p.at == digitStart {
		return nil, fmt.Errorf("invalid TLA integer at byte %d", start)
	}
	value, err := strconv.Atoi(p.input[start:p.at])
	if err != nil {
		return nil, fmt.Errorf("invalid TLA integer: %w", err)
	}
	return value, nil
}

func (p *tlcValueParser) parseIdentifier() (string, error) {
	p.skipSpace()
	start := p.at
	for p.at < len(p.input) {
		char := p.input[p.at]
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' {
			p.at++
			continue
		}
		break
	}
	if p.at == start || (p.input[start] >= '0' && p.input[start] <= '9') {
		return "", fmt.Errorf("invalid TLA identifier at byte %d", start)
	}
	return p.input[start:p.at], nil
}

func (p *tlcValueParser) skipSpace() {
	for p.at < len(p.input) {
		switch p.input[p.at] {
		case ' ', '\n', '\t':
			p.at++
		default:
			return
		}
	}
}

func (p *tlcValueParser) consume(token string) bool {
	p.skipSpace()
	if strings.HasPrefix(p.input[p.at:], token) {
		p.at += len(token)
		return true
	}
	return false
}

func (p *tlcValueParser) expect(token string) error {
	if !p.consume(token) {
		return fmt.Errorf("expected %q at byte %d", token, p.at)
	}
	return nil
}

var tlcStateFields = []string{
	"status",
	"owners",
	"detached",
	"durableAuthority",
	"durableRevision",
	"recordedGen",
	"runtimeAuthority",
	"runtimeGen",
	"inspectedGen",
	"health",
	"mode",
	"operation",
	"effect",
	"evidence",
	"result",
	"pendingAuthority",
	"pendingRevision",
	"direction",
	"worlds",
	"teardownProven",
	"txOwner",
	"lastOwnerAction",
}

func protocolStateFromTLCRecord(record tlcRecord) (ProtocolState, error) {
	if err := requireExactTLCFields(record, tlcStateFields); err != nil {
		return ProtocolState{}, err
	}
	status, err := tlcStringField(record, "status")
	if err != nil {
		return ProtocolState{}, err
	}
	owners, err := tlcOwnerSet(record["owners"])
	if err != nil {
		return ProtocolState{}, fmt.Errorf("owners: %w", err)
	}
	detached, err := tlcBoolField(record, "detached")
	if err != nil {
		return ProtocolState{}, err
	}
	durableAuthority, err := tlcGrantSet(record["durableAuthority"])
	if err != nil {
		return ProtocolState{}, fmt.Errorf("durableAuthority: %w", err)
	}
	durableRevision, err := tlcIntField(record, "durableRevision")
	if err != nil {
		return ProtocolState{}, err
	}
	recordedGeneration, err := tlcGeneration(record["recordedGen"])
	if err != nil {
		return ProtocolState{}, fmt.Errorf("recordedGen: %w", err)
	}
	runtimeAuthority, err := tlcGrantSet(record["runtimeAuthority"])
	if err != nil {
		return ProtocolState{}, fmt.Errorf("runtimeAuthority: %w", err)
	}
	runtimeGeneration, err := tlcGeneration(record["runtimeGen"])
	if err != nil {
		return ProtocolState{}, fmt.Errorf("runtimeGen: %w", err)
	}
	inspectedGeneration, err := tlcGeneration(record["inspectedGen"])
	if err != nil {
		return ProtocolState{}, fmt.Errorf("inspectedGen: %w", err)
	}
	health, err := tlcStringField(record, "health")
	if err != nil {
		return ProtocolState{}, err
	}
	mode, err := tlcStringField(record, "mode")
	if err != nil {
		return ProtocolState{}, err
	}
	operation, err := tlcStringField(record, "operation")
	if err != nil {
		return ProtocolState{}, err
	}
	effect, err := tlcStringField(record, "effect")
	if err != nil {
		return ProtocolState{}, err
	}
	result, err := tlcStringField(record, "result")
	if err != nil {
		return ProtocolState{}, err
	}
	pendingAuthority, err := tlcGrantSet(record["pendingAuthority"])
	if err != nil {
		return ProtocolState{}, fmt.Errorf("pendingAuthority: %w", err)
	}
	pendingRevision, err := tlcIntField(record, "pendingRevision")
	if err != nil {
		return ProtocolState{}, err
	}
	direction, err := tlcStringField(record, "direction")
	if err != nil {
		return ProtocolState{}, err
	}

	if err := validateTLCDroppedFields(record); err != nil {
		return ProtocolState{}, err
	}
	state := NormalizeProtocolState(ProtocolState{
		Status:              ProtocolStatus(status),
		Owners:              owners,
		Detached:            detached,
		DurableAuthority:    durableAuthority,
		DurableRevision:     durableRevision,
		RecordedGeneration:  recordedGeneration,
		RuntimeAuthority:    runtimeAuthority,
		RuntimeGeneration:   runtimeGeneration,
		InspectedGeneration: inspectedGeneration,
		Health:              ProtocolHealth(health),
		Mode:                ProtocolMode(mode),
		Operation:           ProtocolOperation(operation),
		Effect:              ProtocolEffect(effect),
		Result:              ProtocolResult(result),
		PendingAuthority:    pendingAuthority,
		PendingRevision:     pendingRevision,
		Direction:           ProtocolDirection(direction),
	})
	if !protocolStateInDomain(ProtocolBounds{}, state) {
		return ProtocolState{}, fmt.Errorf("normalized state is outside the finite protocol domain")
	}
	return state, nil
}

func requireExactTLCFields(record tlcRecord, expected []string) error {
	expectedSet := make(map[string]bool, len(expected))
	for _, field := range expected {
		expectedSet[field] = true
	}
	var missing, unknown []string
	for _, field := range expected {
		if _, ok := record[field]; !ok {
			missing = append(missing, field)
		}
	}
	for field := range record {
		if !expectedSet[field] {
			unknown = append(unknown, field)
		}
	}
	if len(missing) != 0 || len(unknown) != 0 {
		sort.Strings(missing)
		sort.Strings(unknown)
		return fmt.Errorf("unexpected state fields: missing=%v unknown=%v", missing, unknown)
	}
	return nil
}

func tlcStringField(record tlcRecord, field string) (string, error) {
	value, ok := record[field].(string)
	if !ok {
		return "", fmt.Errorf("field %s is not a string", field)
	}
	return value, nil
}

func tlcIntField(record tlcRecord, field string) (int, error) {
	value, ok := record[field].(int)
	if !ok {
		return 0, fmt.Errorf("field %s is not an integer", field)
	}
	return value, nil
}

func tlcBoolField(record tlcRecord, field string) (bool, error) {
	value, ok := record[field].(bool)
	if !ok {
		return false, fmt.Errorf("field %s is not a boolean", field)
	}
	return value, nil
}

func tlcStringSet(value tlcValue) ([]string, error) {
	set, ok := value.(tlcSet)
	if !ok {
		return nil, fmt.Errorf("value is not a set")
	}
	values := make([]string, 0, len(set))
	seen := make(map[string]bool, len(set))
	for _, element := range set {
		text, ok := element.(string)
		if !ok {
			return nil, fmt.Errorf("set element is not a string")
		}
		if seen[text] {
			return nil, fmt.Errorf("set contains duplicate %q", text)
		}
		seen[text] = true
		values = append(values, text)
	}
	sort.Strings(values)
	return values, nil
}

func tlcOwnerSet(value tlcValue) (ProtocolOwner, error) {
	values, err := tlcStringSet(value)
	if err != nil {
		return 0, err
	}
	var owners ProtocolOwner
	for _, value := range values {
		switch value {
		case "OwnerA":
			owners |= ProtocolOwnerA
		case "OwnerB":
			owners |= ProtocolOwnerB
		default:
			return 0, fmt.Errorf("unknown owner %q", value)
		}
	}
	return owners, nil
}

func tlcGrantSet(value tlcValue) (ProtocolGrantSet, error) {
	values, err := tlcStringSet(value)
	if err != nil {
		return 0, err
	}
	var grants ProtocolGrantSet
	for _, value := range values {
		switch value {
		case "GrantA":
			grants |= ProtocolGrantSet(ProtocolGrantA)
		case "GrantB":
			grants |= ProtocolGrantSet(ProtocolGrantB)
		default:
			return 0, fmt.Errorf("unknown grant %q", value)
		}
	}
	return grants, nil
}

func tlcGeneration(value tlcValue) (ProtocolGeneration, error) {
	record, ok := value.(tlcRecord)
	if !ok {
		return ProtocolGeneration{}, fmt.Errorf("generation is not a record")
	}
	if err := requireExactTLCFields(record, []string{"authority", "revision"}); err != nil {
		return ProtocolGeneration{}, err
	}
	authority, err := tlcGrantSet(record["authority"])
	if err != nil {
		return ProtocolGeneration{}, err
	}
	revision, ok := record["revision"].(int)
	if !ok {
		return ProtocolGeneration{}, fmt.Errorf("generation revision is not an integer")
	}
	if revision == -1 {
		if !authority.Empty() {
			return ProtocolGeneration{}, fmt.Errorf("NoGen sentinel has authority")
		}
		return protocolNoGeneration(), nil
	}
	return protocolGeneration(authority, revision), nil
}

func validateTLCDroppedFields(record tlcRecord) error {
	evidence, err := tlcStringField(record, "evidence")
	if err != nil {
		return err
	}
	if !stringIn(evidence, "None", "CommitOld", "CommitNew", "CommitUnknown", "ApplySucceeded", "InspectMatch", "ExactLiveA", "ExactLiveB", "TokenMismatch", "TeardownProven", "TeardownUnknown") {
		return fmt.Errorf("unknown evidence %q", evidence)
	}
	worlds, err := tlcStringSet(record["worlds"])
	if err != nil {
		return fmt.Errorf("worlds: %w", err)
	}
	for _, world := range worlds {
		if world != "Old" && world != "New" {
			return fmt.Errorf("unknown world %q", world)
		}
	}
	if _, err := tlcBoolField(record, "teardownProven"); err != nil {
		return err
	}
	txOwner, err := tlcStringField(record, "txOwner")
	if err != nil {
		return err
	}
	if !stringIn(txOwner, "NoOwner", "OwnerA", "OwnerB") {
		return fmt.Errorf("unknown transaction owner %q", txOwner)
	}
	lastOwnerAction, err := tlcStringField(record, "lastOwnerAction")
	if err != nil {
		return err
	}
	if !stringIn(lastOwnerAction, "None", "Handoff", "Release", "Signal") {
		return fmt.Errorf("unknown last owner action %q", lastOwnerAction)
	}
	return nil
}

func stringIn(value string, values ...string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func canonicalProtocolStateJSON(state ProtocolState) (string, error) {
	state = NormalizeProtocolState(state)
	if !protocolStateInDomain(ProtocolBounds{}, state) {
		return "", fmt.Errorf("state is outside the finite protocol domain")
	}
	owners, err := canonicalOwnerSet(state.Owners)
	if err != nil {
		return "", err
	}
	durableAuthority, err := canonicalGrantSet(state.DurableAuthority)
	if err != nil {
		return "", err
	}
	runtimeAuthority, err := canonicalGrantSet(state.RuntimeAuthority)
	if err != nil {
		return "", err
	}
	pendingAuthority, err := canonicalGrantSet(state.PendingAuthority)
	if err != nil {
		return "", err
	}
	object := map[string]interface{}{
		"status":               string(state.Status),
		"owners":               owners,
		"detached":             state.Detached,
		"durable_authority":    durableAuthority,
		"durable_revision":     state.DurableRevision,
		"recorded_generation":  canonicalGeneration(state.RecordedGeneration),
		"runtime_authority":    runtimeAuthority,
		"runtime_generation":   canonicalGeneration(state.RuntimeGeneration),
		"inspected_generation": canonicalGeneration(state.InspectedGeneration),
		"health":               string(state.Health),
		"mode":                 string(state.Mode),
		"operation":            string(state.Operation),
		"effect":               string(state.Effect),
		"result":               string(state.Result),
		"pending_authority":    pendingAuthority,
		"pending_revision":     state.PendingRevision,
		"direction":            string(state.Direction),
	}
	encoded, err := json.Marshal(object)
	if err != nil {
		return "", fmt.Errorf("canonical state JSON: %w", err)
	}
	return string(encoded), nil
}

func canonicalOwnerSet(owners ProtocolOwner) ([]string, error) {
	if owners&^(ProtocolOwnerA|ProtocolOwnerB) != 0 {
		return nil, fmt.Errorf("unknown symbolic owner bits")
	}
	values := make([]string, 0, 2)
	if owners&ProtocolOwnerA != 0 {
		values = append(values, "OwnerA")
	}
	if owners&ProtocolOwnerB != 0 {
		values = append(values, "OwnerB")
	}
	return values, nil
}

func canonicalGrantSet(grants ProtocolGrantSet) ([]string, error) {
	if grants&^ProtocolGrantSet(ProtocolGrantA|ProtocolGrantB) != 0 {
		return nil, fmt.Errorf("unknown symbolic grant bits")
	}
	values := make([]string, 0, 2)
	if grants.Has(ProtocolGrantA) {
		values = append(values, "GrantA")
	}
	if grants.Has(ProtocolGrantB) {
		values = append(values, "GrantB")
	}
	return values, nil
}

func canonicalGeneration(generation ProtocolGeneration) interface{} {
	if !generation.Valid {
		return "NoGen"
	}
	authority, _ := canonicalGrantSet(generation.Authority)
	return map[string]interface{}{"authority": authority, "revision": generation.Revision}
}

func compareProtocolGraphs(leftName string, left protocolGraph, rightName string, right protocolGraph) error {
	if key, ok := firstGraphKeyDifference(left.Initial, right.Initial); ok {
		return fmt.Errorf("%s - %s initial %s\nshortest witness: %s", leftName, rightName, key, shortestProtocolGraphWitness(left, key))
	}
	if key, ok := firstGraphKeyDifference(right.Initial, left.Initial); ok {
		return fmt.Errorf("%s - %s initial %s\nshortest witness: %s", rightName, leftName, key, shortestProtocolGraphWitness(right, key))
	}
	leftStates := protocolGraphStateKeys(left)
	rightStates := protocolGraphStateKeys(right)
	if key, ok := firstGraphKeyDifference(leftStates, rightStates); ok {
		return fmt.Errorf("%s - %s state %s\nshortest witness: %s", leftName, rightName, key, shortestProtocolGraphWitness(left, key))
	}
	if key, ok := firstGraphKeyDifference(rightStates, leftStates); ok {
		return fmt.Errorf("%s - %s state %s\nshortest witness: %s", rightName, leftName, key, shortestProtocolGraphWitness(right, key))
	}
	if edge, ok := firstGraphEdgeDifference(left.Edges, right.Edges); ok {
		return fmt.Errorf("%s - %s edge %s --%s--> %s\nshortest witness: %s", leftName, rightName, edge.From, edge.Label, edge.To, shortestProtocolEdgeWitness(left, edge))
	}
	if edge, ok := firstGraphEdgeDifference(right.Edges, left.Edges); ok {
		return fmt.Errorf("%s - %s edge %s --%s--> %s\nshortest witness: %s", rightName, leftName, edge.From, edge.Label, edge.To, shortestProtocolEdgeWitness(right, edge))
	}
	return nil
}

func protocolGraphStateKeys(graph protocolGraph) map[string]struct{} {
	keys := make(map[string]struct{}, len(graph.States))
	for key := range graph.States {
		keys[key] = struct{}{}
	}
	return keys
}

func firstGraphKeyDifference(left, right map[string]struct{}) (string, bool) {
	var keys []string
	for key := range left {
		if _, ok := right[key]; !ok {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return "", false
	}
	sort.Strings(keys)
	return keys[0], true
}

func firstGraphEdgeDifference(left, right map[protocolGraphEdge]struct{}) (protocolGraphEdge, bool) {
	missing := make(map[protocolGraphEdge]struct{})
	for edge := range left {
		if _, ok := right[edge]; !ok {
			missing[edge] = struct{}{}
		}
	}
	if len(missing) == 0 {
		return protocolGraphEdge{}, false
	}
	return sortedProtocolGraphEdges(missing)[0], true
}

func shortestProtocolGraphWitness(graph protocolGraph, target string) string {
	initials := sortedGraphKeys(graph.Initial)
	if len(initials) == 0 {
		return "<no initial state>"
	}
	paths := make(map[string]string, len(graph.States))
	queue := make([]string, 0, len(graph.States))
	for _, initial := range initials {
		paths[initial] = "initial " + initial
		queue = append(queue, initial)
	}
	adjacency := make(map[string][]protocolGraphEdge)
	for edge := range graph.Edges {
		adjacency[edge.From] = append(adjacency[edge.From], edge)
	}
	for source := range adjacency {
		sort.Slice(adjacency[source], func(i, j int) bool {
			if adjacency[source][i].Label != adjacency[source][j].Label {
				return adjacency[source][i].Label < adjacency[source][j].Label
			}
			return adjacency[source][i].To < adjacency[source][j].To
		})
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current == target {
			return paths[current]
		}
		for _, edge := range adjacency[current] {
			if _, seen := paths[edge.To]; seen {
				continue
			}
			paths[edge.To] = fmt.Sprintf("%s --%s--> %s", paths[current], edge.Label, edge.To)
			queue = append(queue, edge.To)
		}
	}
	return "<unreachable " + target + ">"
}

func shortestProtocolEdgeWitness(graph protocolGraph, edge protocolGraphEdge) string {
	return fmt.Sprintf("%s --%s--> %s", shortestProtocolGraphWitness(graph, edge.From), edge.Label, edge.To)
}
