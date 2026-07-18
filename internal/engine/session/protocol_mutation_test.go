package session

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const protocolMutationAllowlistPath = "testdata/protocol-mutation-allowlist.txt"

type protocolMutationEntry struct {
	path      string
	function  string
	field     string
	operation string
}

func (entry protocolMutationEntry) String() string {
	return strings.Join([]string{entry.path, entry.function, entry.field, entry.operation}, "|")
}

func TestProtocolMutationAllowlistIsExactAndByteStable(t *testing.T) {
	actual, err := scanInternalProtocolMutations("../../..")
	if err != nil {
		t.Fatalf("scan protocol mutations: %v", err)
	}
	data, err := os.ReadFile(protocolMutationAllowlistPath)
	if err != nil {
		t.Fatalf("read mutation allowlist: %v\nactual:\n%s", err, strings.Join(actual, "\n"))
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatal("protocol mutation allowlist must end with one LF")
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if !sort.StringsAreSorted(lines) {
		t.Fatal("protocol mutation allowlist is not sorted")
	}
	for index, line := range lines {
		if line == "" {
			t.Fatal("protocol mutation allowlist contains an empty line")
		}
		if index > 0 && lines[index-1] == line {
			t.Fatalf("protocol mutation allowlist contains duplicate %q", line)
		}
	}
	if err := compareProtocolMutationLines(lines, actual); err != nil {
		t.Fatal(err)
	}
	wantBytes := strings.Join(actual, "\n") + "\n"
	if string(data) != wantBytes {
		t.Fatal("protocol mutation allowlist bytes are not canonical")
	}
}

func TestProtocolMutationScannerRejectsSyntheticForbiddenWrite(t *testing.T) {
	source := `package rogue
import engsession "github.com/freakhill/safeslop/internal/engine/session"
type Alias = engsession.Session
func mutate(sess engsession.Session, renamed *engsession.RecordTx) {
	// sess.Status = "comment-only" must not count.
	_ = "sess.Status = stopped"
	sess.Status = "stopped"
	_ = Alias{PID: 41, ProcessToken: "token"}
	_ = engsession.Session{"positional"}
	_ = renamed.Commit(sess)
}`
	entries, err := scanProtocolMutationSource("internal/rogue/rogue.go", []byte(source))
	if err != nil {
		t.Fatalf("scan synthetic source: %v", err)
	}
	for _, want := range []string{
		"internal/rogue/rogue.go|mutate|Status|assign",
		"internal/rogue/rogue.go|mutate|PID|construct",
		"internal/rogue/rogue.go|mutate|ProcessToken|construct",
		"internal/rogue/rogue.go|mutate|Session|positional-construct",
		"internal/rogue/rogue.go|mutate|protocol-state|call:Commit",
	} {
		if !reflectStringSetContains(entries, want) {
			t.Fatalf("synthetic entries = %v, want %q", entries, want)
		}
	}
	if err := compareProtocolMutationLines(nil, entries); err == nil || !strings.Contains(err.Error(), "unauthorized protocol mutation") {
		t.Fatalf("allowlist error = %v", err)
	}
}

func TestProtocolMutationTypedScannerRejectsAliasReceiverAndPositionalBypasses(t *testing.T) {
	sessionSource := `package session
	type Session struct { Status string; PID int; ProcessToken string }
	type RecordTx struct{}
	func (*RecordTx) Commit(Session) error { return nil }
	`
	sessionFiles := token.NewFileSet()
	sessionSyntax, err := parser.ParseFile(sessionFiles, "session.go", sessionSource, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	sessionPackage, err := (&types.Config{}).Check(protocolSessionPackagePath, sessionFiles, []*ast.File{sessionSyntax}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rogueSource := `package rogue
	import engsession "github.com/freakhill/safeslop/internal/engine/session"
	type Alias = engsession.Session
	func mutate(renamed *engsession.RecordTx) {
		sess := Alias{PID: 41, ProcessToken: "token"}
		_ = Alias{"stopped", 41, "token"}
		_ = renamed.Commit(sess)
	}
	`
	files := token.NewFileSet()
	rogueSyntax, err := parser.ParseFile(files, "internal/rogue/rogue.go", rogueSource, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	configuration := types.Config{Importer: protocolStaticImporter{path: protocolSessionPackagePath, pkg: sessionPackage}}
	if _, err := configuration.Check("github.com/freakhill/safeslop/internal/rogue", files, []*ast.File{rogueSyntax}, info); err != nil {
		t.Fatal(err)
	}
	entries := make(map[string]struct{})
	for _, declaration := range rogueSyntax.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok {
			scanTypedProtocolMutationNode("internal/rogue/rogue.go", function.Name.Name, function.Body, info, entries)
		}
	}
	for _, want := range []string{
		"internal/rogue/rogue.go|mutate|PID|construct",
		"internal/rogue/rogue.go|mutate|ProcessToken|construct",
		"internal/rogue/rogue.go|mutate|Session|positional-construct",
		"internal/rogue/rogue.go|mutate|protocol-state|call:Commit",
	} {
		if _, ok := entries[want]; !ok {
			t.Fatalf("typed entries = %v, want %q", entries, want)
		}
	}
}

type protocolStaticImporter struct {
	path string
	pkg  *types.Package
}

func (importer protocolStaticImporter) Import(path string) (*types.Package, error) {
	if path == importer.path {
		return importer.pkg, nil
	}
	return nil, fmt.Errorf("unexpected synthetic import %s", path)
}

func scanInternalProtocolMutations(root string) ([]string, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	entries := make(map[string]struct{})
	for _, goos := range []string{"darwin", "linux"} {
		lines, err := scanTypedInternalProtocolMutations(absoluteRoot, goos)
		if err != nil {
			return nil, err
		}
		for _, line := range lines {
			entries[line] = struct{}{}
		}
	}
	lines := make([]string, 0, len(entries))
	for line := range entries {
		lines = append(lines, line)
	}
	sort.Strings(lines)
	return lines, nil
}

type protocolGoListPackage struct {
	ImportPath string
	Dir        string
	Export     string
	GoFiles    []string
	CgoFiles   []string
	Standard   bool
}

func scanTypedInternalProtocolMutations(root, goos string) ([]string, error) {
	command := exec.Command("go", "list", "-deps", "-export", "-json", "./internal/...")
	command.Dir = root
	command.Env = append(os.Environ(), "GOOS="+goos, "GOARCH=amd64", "CGO_ENABLED=0")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("go list internal packages for %s: %w: %s", goos, err, strings.TrimSpace(stderr.String()))
	}
	decoder := json.NewDecoder(&stdout)
	var packages []protocolGoListPackage
	exports := make(map[string]string)
	for {
		var listed protocolGoListPackage
		if err := decoder.Decode(&listed); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("decode go list output for %s: %w", goos, err)
		}
		if listed.Export != "" {
			exports[listed.ImportPath] = listed.Export
		}
		if strings.HasPrefix(listed.ImportPath, "github.com/freakhill/safeslop/internal/") {
			packages = append(packages, listed)
		}
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].ImportPath < packages[j].ImportPath })

	entries := make(map[string]struct{})
	for _, listed := range packages {
		files := token.NewFileSet()
		var syntax []*ast.File
		paths := append(append([]string(nil), listed.GoFiles...), listed.CgoFiles...)
		sort.Strings(paths)
		for _, name := range paths {
			path := filepath.Join(listed.Dir, name)
			parsed, err := parser.ParseFile(files, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return nil, fmt.Errorf("parse %s for %s: %w", path, goos, err)
			}
			syntax = append(syntax, parsed)
		}
		info := &types.Info{
			Types:      make(map[ast.Expr]types.TypeAndValue),
			Defs:       make(map[*ast.Ident]types.Object),
			Uses:       make(map[*ast.Ident]types.Object),
			Selections: make(map[*ast.SelectorExpr]*types.Selection),
		}
		lookup := func(path string) (io.ReadCloser, error) {
			export, ok := exports[path]
			if !ok {
				return nil, fmt.Errorf("no export data for %s", path)
			}
			return os.Open(export)
		}
		configuration := types.Config{Importer: importer.ForCompiler(files, "gc", lookup)}
		if _, err := configuration.Check(listed.ImportPath, files, syntax, info); err != nil {
			return nil, fmt.Errorf("type-check %s for %s: %w", listed.ImportPath, goos, err)
		}
		for _, file := range syntax {
			position := files.Position(file.Pos())
			relative, err := filepath.Rel(root, position.Filename)
			if err != nil {
				return nil, err
			}
			path := filepath.ToSlash(relative)
			for _, declaration := range file.Decls {
				switch declaration := declaration.(type) {
				case *ast.FuncDecl:
					if declaration.Body == nil {
						continue
					}
					function := declaration.Name.Name
					if declaration.Recv != nil && len(declaration.Recv.List) == 1 {
						function = protocolReceiverName(declaration.Recv.List[0].Type) + "." + function
					}
					scanTypedProtocolMutationNode(path, function, declaration.Body, info, entries)
				case *ast.GenDecl:
					scanTypedProtocolMutationNode(path, "<package>", declaration, info, entries)
				}
			}
		}
	}
	lines := make([]string, 0, len(entries))
	for line := range entries {
		lines = append(lines, line)
	}
	sort.Strings(lines)
	return lines, nil
}

func scanProtocolMutationSource(path string, source []byte) ([]string, error) {
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, path, source, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if !protocolMutationRelevantFile(path, file) {
		return nil, nil
	}
	entries := make(map[string]struct{})
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			if declaration.Body == nil {
				continue
			}
			function := declaration.Name.Name
			if declaration.Recv != nil && len(declaration.Recv.List) == 1 {
				function = protocolReceiverName(declaration.Recv.List[0].Type) + "." + function
			}
			scanProtocolMutationNode(path, function, declaration.Body, entries)
		case *ast.GenDecl:
			scanProtocolMutationNode(path, "<package>", declaration, entries)
		}
	}
	lines := make([]string, 0, len(entries))
	for line := range entries {
		lines = append(lines, line)
	}
	sort.Strings(lines)
	return lines, nil
}

func protocolMutationRelevantFile(path string, file *ast.File) bool {
	if strings.HasPrefix(path, "internal/engine/session/") && file.Name.Name == "session" {
		return true
	}
	for _, imported := range file.Imports {
		if strings.Trim(imported.Path.Value, `"`) == "github.com/freakhill/safeslop/internal/engine/session" {
			return true
		}
	}
	return false
}

func protocolReceiverName(expression ast.Expr) string {
	switch expression := expression.(type) {
	case *ast.Ident:
		return expression.Name
	case *ast.StarExpr:
		return protocolReceiverName(expression.X)
	case *ast.IndexExpr:
		return protocolReceiverName(expression.X)
	case *ast.IndexListExpr:
		return protocolReceiverName(expression.X)
	default:
		return "<receiver>"
	}
}

const (
	protocolSessionPackagePath = "github.com/freakhill/safeslop/internal/engine/session"
	protocolCLIPackagePath     = "github.com/freakhill/safeslop/internal/cli"
)

func scanTypedProtocolMutationNode(path, function string, node ast.Node, info *types.Info, entries map[string]struct{}) {
	specific := make(map[string]struct{})
	generic := make(map[string]struct{})
	addSpecific := func(field, operation string) {
		addProtocolMutationEntry(specific, path, function, field, operation)
	}
	ast.Inspect(node, func(current ast.Node) bool {
		switch current := current.(type) {
		case *ast.AssignStmt:
			operation := "assign"
			if current.Tok != token.ASSIGN && current.Tok != token.DEFINE {
				operation = "compound-assign"
			}
			for _, target := range current.Lhs {
				for _, field := range typedProtocolMutationTargetFields(target, info) {
					addSpecific(field, operation)
				}
			}
		case *ast.IncDecStmt:
			operation := "increment"
			if current.Tok == token.DEC {
				operation = "decrement"
			}
			for _, field := range typedProtocolMutationTargetFields(current.X, info) {
				addSpecific(field, operation)
			}
		case *ast.CompositeLit:
			if !typedProtocolConcreteType(info.TypeOf(current)) {
				break
			}
			if len(current.Elts) != 0 {
				if _, keyed := current.Elts[0].(*ast.KeyValueExpr); !keyed {
					addSpecific("Session", "positional-construct")
					break
				}
			}
			for _, element := range current.Elts {
				keyed, ok := element.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				identifier, ok := keyed.Key.(*ast.Ident)
				if ok && protocolMutationFields[identifier.Name] {
					addSpecific(identifier.Name, "construct")
				}
			}
		case *ast.CallExpr:
			called := typedProtocolMutationCall(current.Fun, info)
			if called == nil || called.Pkg() == nil {
				break
			}
			name := called.Name()
			field, tracked := protocolMutationCalls[name]
			if !tracked || !typedProtocolMutationCallPackage(called.Pkg().Path(), name) {
				break
			}
			if protocolGenericMutationCalls[name] {
				addProtocolMutationEntry(generic, path, function, field, "call:"+name)
			} else {
				addSpecific(field, "call:"+name)
			}
		}
		return true
	})
	if protocolMutationGatewayFunctions[function] {
		addSpecific("protocol-state", "gateway")
	}
	for line := range specific {
		entries[line] = struct{}{}
	}
	if len(specific) != 0 {
		for line := range generic {
			entries[line] = struct{}{}
		}
	}
}

func typedProtocolMutationTargetFields(expression ast.Expr, info *types.Info) []string {
	var fields []string
	ast.Inspect(expression, func(current ast.Node) bool {
		selector, ok := current.(*ast.SelectorExpr)
		if !ok || !protocolMutationFields[selector.Sel.Name] {
			return true
		}
		selection := info.Selections[selector]
		if selection != nil && typedProtocolConcreteType(selection.Recv()) {
			fields = append(fields, selector.Sel.Name)
		}
		return true
	})
	return fields
}

func typedProtocolConcreteType(value types.Type) bool {
	if value == nil {
		return false
	}
	value = types.Unalias(value)
	if pointer, ok := value.(*types.Pointer); ok {
		value = types.Unalias(pointer.Elem())
	}
	named, ok := value.(*types.Named)
	if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != protocolSessionPackagePath {
		return false
	}
	return named.Obj().Name() == "Session" || named.Obj().Name() == "diskRecord"
}

func typedProtocolMutationCall(expression ast.Expr, info *types.Info) *types.Func {
	var object types.Object
	switch expression := expression.(type) {
	case *ast.Ident:
		object = info.Uses[expression]
	case *ast.SelectorExpr:
		object = info.Uses[expression.Sel]
	}
	function, _ := object.(*types.Func)
	return function
}

func typedProtocolMutationCallPackage(path, name string) bool {
	if path == protocolSessionPackagePath {
		return true
	}
	if path != protocolCLIPackagePath {
		return false
	}
	switch name {
	case "copySessionAuthority", "withAppliedGeneration", "withTransition", "withEgressUncertaintyFailure", "stopForEgressUncertainty", "recoverRunningSessionEgressWithDeps":
		return true
	default:
		return false
	}
}

var protocolMutationFields = map[string]bool{
	"Status":                true,
	"PID":                   true,
	"ProcessToken":          true,
	"Detached":              true,
	"PersistentEgress":      true,
	"EgressGrants":          true,
	"GrantRevision":         true,
	"RecordRevision":        true,
	"AppliedEgressRevision": true,
	"AppliedEgressHash":     true,
	"EgressTransition":      true,
	"LastFailure":           true,
	"recordRevision":        true,
	"appliedEgressRevision": true,
	"appliedEgressHash":     true,
	"egressTransition":      true,
}

var protocolMutationCalls = map[string]string{
	"SetEgressRuntimeState":               "egress-runtime",
	"SetFailure":                          "failure",
	"AppendGrant":                         "egress-authority",
	"RevokeGrant":                         "egress-authority",
	"MarkRunning":                         "lifecycle+owner",
	"MarkRunningDetached":                 "lifecycle+owner",
	"HandoffRunningDetached":              "owner",
	"ReleaseRunningClaim":                 "lifecycle+owner",
	"Finish":                              "lifecycle+owner+egress-runtime",
	"GetReconciled":                       "lifecycle+owner+egress-runtime",
	"ListReconciled":                      "lifecycle+owner+egress-runtime",
	"Stop":                                "lifecycle+owner+egress-runtime",
	"copySessionAuthority":                "egress-authority",
	"withAppliedGeneration":               "egress-runtime",
	"withTransition":                      "egress-runtime",
	"withEgressUncertaintyFailure":        "failure",
	"stopForEgressUncertainty":            "lifecycle+owner+egress-runtime",
	"recoverRunningSessionEgressWithDeps": "egress-authority+egress-runtime",
	"Save":                                "protocol-state",
	"Update":                              "protocol-state",
	"WithLocked":                          "protocol-state",
	"Commit":                              "protocol-state",
}

var protocolGenericMutationCalls = map[string]bool{
	"Save": true, "Update": true, "WithLocked": true, "Commit": true,
}

var protocolMutationGatewayFunctions = map[string]bool{
	"Store.Save":               true,
	"Store.Update":             true,
	"Store.WithLocked":         true,
	"RecordTx.Commit":          true,
	"Store.writeLocked":        true,
	"commitEgressFailureState": true,
}

func scanProtocolMutationNode(path, function string, node ast.Node, entries map[string]struct{}) {
	specific := make(map[string]struct{})
	abstractOnly := function == "ReduceProtocolWithin" || function == "ProtocolAdapter.EffectCandidate" || function == "NewProtocolAdapter"
	generic := make(map[string]struct{})
	addSpecific := func(field, operation string) {
		addProtocolMutationEntry(specific, path, function, field, operation)
	}
	ast.Inspect(node, func(current ast.Node) bool {
		switch current := current.(type) {
		case *ast.AssignStmt:
			if abstractOnly {
				break
			}
			operation := "assign"
			if current.Tok != token.ASSIGN && current.Tok != token.DEFINE {
				operation = "compound-assign"
			}
			for _, target := range current.Lhs {
				for _, field := range protocolMutationTargetFields(target) {
					addSpecific(field, operation)
				}
			}
		case *ast.IncDecStmt:
			if abstractOnly {
				break
			}
			operation := "increment"
			if current.Tok == token.DEC {
				operation = "decrement"
			}
			for _, field := range protocolMutationTargetFields(current.X) {
				addSpecific(field, operation)
			}
		case *ast.CompositeLit:
			if abstractOnly {
				break
			}
			if len(current.Elts) != 0 {
				if _, keyed := current.Elts[0].(*ast.KeyValueExpr); !keyed && protocolMutationCompositeType(current.Type) {
					addSpecific("Session", "positional-construct")
					break
				}
			}
			for _, element := range current.Elts {
				keyed, ok := element.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				identifier, ok := keyed.Key.(*ast.Ident)
				if ok && protocolMutationFields[identifier.Name] {
					addSpecific(identifier.Name, "construct")
				}
			}
		case *ast.CallExpr:
			name, receiver := protocolMutationCallName(current.Fun)
			field, tracked := protocolMutationCalls[name]
			if !tracked || !protocolMutationCallReceiverAllowed(name, receiver) {
				break
			}
			if protocolGenericMutationCalls[name] {
				addProtocolMutationEntry(generic, path, function, field, "call:"+name)
			} else {
				addSpecific(field, "call:"+name)
			}
		}
		return true
	})
	if protocolMutationGatewayFunctions[function] {
		addSpecific("protocol-state", "gateway")
	}
	for line := range specific {
		entries[line] = struct{}{}
	}
	if len(specific) != 0 {
		for line := range generic {
			entries[line] = struct{}{}
		}
	}
}

func protocolMutationCompositeType(expression ast.Expr) bool {
	var name string
	switch expression := expression.(type) {
	case *ast.Ident:
		name = expression.Name
	case *ast.SelectorExpr:
		name = expression.Sel.Name
	case *ast.StarExpr:
		return protocolMutationCompositeType(expression.X)
	default:
		return false
	}
	return name == "Session" || name == "diskRecord"
}

func protocolMutationTargetFields(expression ast.Expr) []string {
	var fields []string
	ast.Inspect(expression, func(current ast.Node) bool {
		selector, ok := current.(*ast.SelectorExpr)
		if ok && protocolMutationFields[selector.Sel.Name] {
			fields = append(fields, selector.Sel.Name)
		}
		return true
	})
	return fields
}

func protocolMutationCallName(expression ast.Expr) (string, string) {
	switch expression := expression.(type) {
	case *ast.Ident:
		return expression.Name, ""
	case *ast.SelectorExpr:
		return expression.Sel.Name, protocolMutationReceiverTail(expression.X)
	default:
		return "", ""
	}
}

func protocolMutationReceiverTail(expression ast.Expr) string {
	switch expression := expression.(type) {
	case *ast.Ident:
		return expression.Name
	case *ast.SelectorExpr:
		return expression.Sel.Name
	case *ast.ParenExpr:
		return protocolMutationReceiverTail(expression.X)
	default:
		return ""
	}
}

func protocolMutationCallReceiverAllowed(string, string) bool { return true }

func addProtocolMutationEntry(entries map[string]struct{}, path, function, field, operation string) {
	entry := protocolMutationEntry{path: path, function: function, field: field, operation: operation}
	entries[entry.String()] = struct{}{}
}

func compareProtocolMutationLines(allowed, actual []string) error {
	allowedSet := make(map[string]bool, len(allowed))
	for _, line := range allowed {
		allowedSet[line] = true
	}
	actualSet := make(map[string]bool, len(actual))
	for _, line := range actual {
		actualSet[line] = true
		if !allowedSet[line] {
			return fmt.Errorf("unauthorized protocol mutation: %s", line)
		}
	}
	for _, line := range allowed {
		if !actualSet[line] {
			return fmt.Errorf("stale protocol mutation allowlist entry: %s", line)
		}
	}
	return nil
}

func reflectStringSetContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
