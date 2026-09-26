package defaults

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// =============================================================================
// EXECUTIVE CONCLUSIONS ARE CONSUMED
// =============================================================================
// The dual of the starved-predicate gate. That one guards predicates a rule
// READS and nothing produces; this one guards conclusions the kernel PRODUCES
// and the executive never acts on -- while Go decides the same thing itself.
// The 2026-09-23 sweep (radical/01-executive-drift) found that drift in every
// subsystem it read: campaign_task_shard derived and never queried while three
// Go tables picked the shard; replan_needed derived, and the live replan
// decided in Go; next_action atoms routed to handlers that reported success
// without work.
//
// Two measurements, one checked-in inventory that may only shrink:
//
//  1. Liveness. A predicate is live when production Go asks the kernel for it
//     -- the predicate named, as a literal, a const or a Sprintf template, in
//     the first argument of a query-shaped call -- or when it feeds, through a
//     rule body, a predicate that is live. A rule head that is not live is a
//     conclusion nothing reads. A bare string literal elsewhere does not count:
//     a gate that a literal satisfies teaches writing literals (see the
//     starved gate's turn_age_category note).
//  2. Routing. Every constant action a rule derives as next_action(/X) must
//     reach a case of the VirtualStore's executeAction switch; otherwise the
//     kernel can decide an action that nothing executes.
//
// Regenerate: CODENERD_UPDATE_CONCLUSIONS=1 go test ./internal/core/defaults/ -run TestExecutiveConclusionsAreConsumed

const conclusionsBaselinePath = "testdata/unconsumed_conclusions.txt"

// queryCalls are the calls through which production Go asks the kernel for a
// predicate: the kernel's own query methods and the helpers that wrap them.
// A new helper that wraps Query must be added here, or everything it reads
// shows up as unconsumed.
var queryCalls = map[string]bool{
	"Query":                true,
	"QueryFacts":           true,
	"GetFacts":             true,
	"QueryRouting":         true,
	"QueryAllLearned":      true,
	"SubscribeToFacts":     true,
	"turnRows":             true,
	"derivedFor":           true,
	"oneDerivedFor":        true,
	"first":                true,
	"acceptanceDerived":    true,
	"askWithCampaignState": true,
	"derivedColumn":        true,
	"queryKernelStrings":   true,
	"queryOrBlock":         true,
}

var predicateNameRe = regexp.MustCompile(`^\s*([a-z_][a-zA-Z0-9_]*)`)

// mgRule is one rule of the corpus: its head predicate and the predicates its
// body reads.
type mgRule struct {
	head     string
	headText string
	body     []string
	file     string
}

// mangleRules returns every rule (a statement with a body) in the corpus.
func mangleRules(t *testing.T, corpusDir string) []mgRule {
	t.Helper()
	var rules []mgRule
	err := filepath.WalkDir(corpusDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".mg") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		var lines []string
		for _, l := range strings.Split(string(data), "\n") {
			lines = append(lines, strings.SplitN(l, "#", 2)[0])
		}
		for _, stmt := range statementSplit.Split(strings.Join(lines, "\n"), -1) {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" || strings.HasPrefix(stmt, "Decl") {
				continue
			}
			head, body, isRule := strings.Cut(stmt, ":-")
			if !isRule {
				continue
			}
			m := starvedHeadRe.FindStringSubmatch(strings.TrimSpace(head))
			if m == nil {
				continue
			}
			r := mgRule{head: m[1], headText: strings.TrimSpace(head), file: filepath.Base(path)}
			for _, bm := range starvedAtomRe.FindAllStringSubmatch(body, -1) {
				r.body = append(r.body, bm[1])
			}
			rules = append(rules, r)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk corpus: %v", err)
	}
	return rules
}

// predicateFromExpr names the predicate an argument asks for: a string literal
// ("p" or "p(X)"), a const holding one, a Sprintf template starting with one,
// a concatenation starting with one, or (SubscribeToFacts) a literal list.
func predicateFromExpr(expr ast.Expr, consts map[string]string) []string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return nil
		}
		s, err := strconv.Unquote(e.Value)
		if err != nil {
			return nil
		}
		if m := predicateNameRe.FindStringSubmatch(s); m != nil {
			return []string{m[1]}
		}
	case *ast.Ident:
		if s, ok := consts[e.Name]; ok {
			if m := predicateNameRe.FindStringSubmatch(s); m != nil {
				return []string{m[1]}
			}
		}
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Sprintf" && len(e.Args) > 0 {
			return predicateFromExpr(e.Args[0], consts)
		}
	case *ast.BinaryExpr:
		return predicateFromExpr(e.X, consts)
	case *ast.CompositeLit:
		var out []string
		for _, el := range e.Elts {
			out = append(out, predicateFromExpr(el, consts)...)
		}
		return out
	}
	return nil
}

// goQueryRoots returns every predicate production Go asks the kernel for.
func goQueryRoots(t *testing.T, root string) map[string]struct{} {
	t.Helper()
	roots := make(map[string]struct{})
	// Files grouped by directory, so a const declared in one file of a package
	// resolves in another.
	byDir := make(map[string][]*ast.File)
	fset := token.NewFileSet()
	for _, sub := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, sub), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !isProductionGo(path) {
				return nil
			}
			f, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return nil
			}
			byDir[filepath.Dir(path)] = append(byDir[filepath.Dir(path)], f)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", sub, err)
		}
	}
	for _, files := range byDir {
		consts := make(map[string]string)
		for _, f := range files {
			ast.Inspect(f, func(n ast.Node) bool {
				gd, ok := n.(*ast.GenDecl)
				if !ok || gd.Tok != token.CONST {
					return true
				}
				for _, spec := range gd.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, name := range vs.Names {
						if i < len(vs.Values) {
							if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
								if s, err := strconv.Unquote(lit.Value); err == nil {
									consts[name.Name] = s
								}
							}
						}
					}
				}
				return true
			})
		}
		for _, f := range files {
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				var name string
				switch fn := call.Fun.(type) {
				case *ast.Ident:
					name = fn.Name
				case *ast.SelectorExpr:
					name = fn.Sel.Name
				}
				if !queryCalls[name] {
					return true
				}
				// The predicate is the first argument that names one; a
				// context, a kernel handle or an action ID comes before it in
				// some helpers (queryOrBlock takes both of the last two).
				for i, arg := range call.Args {
					if i > 2 {
						break
					}
					if preds := predicateFromExpr(arg, consts); len(preds) > 0 {
						for _, p := range preds {
							roots[p] = struct{}{}
						}
						break
					}
				}
				return true
			})
		}
	}
	return roots
}

// liveConclusions closes the roots over the rule graph: a rule whose head is
// live makes every predicate its body reads live.
func liveConclusions(rules []mgRule, roots map[string]struct{}) map[string]struct{} {
	live := make(map[string]struct{}, len(roots))
	for p := range roots {
		live[p] = struct{}{}
	}
	for changed := true; changed; {
		changed = false
		for _, r := range rules {
			if _, ok := live[r.head]; !ok {
				continue
			}
			for _, b := range r.body {
				if _, ok := live[b]; !ok {
					live[b] = struct{}{}
					changed = true
				}
			}
		}
	}
	return live
}

var nextActionConstRe = regexp.MustCompile(`^next_action\(\s*/([a-z_][a-z0-9_]*)\s*\)$`)

// routedActions returns the ActionType values executeAction dispatches: the
// consts named by the cases of its switch, resolved to their string values.
func routedActions(t *testing.T, root string) map[string]struct{} {
	t.Helper()
	coreDir := filepath.Join(root, "internal", "core")
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, coreDir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", coreDir, err)
	}
	values := make(map[string]string)
	var execute *ast.FuncDecl
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			for _, decl := range f.Decls {
				switch d := decl.(type) {
				case *ast.GenDecl:
					if d.Tok != token.CONST {
						continue
					}
					for _, spec := range d.Specs {
						vs, ok := spec.(*ast.ValueSpec)
						if !ok {
							continue
						}
						for i, name := range vs.Names {
							if i < len(vs.Values) {
								if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
									if s, err := strconv.Unquote(lit.Value); err == nil {
										values[name.Name] = s
									}
								}
							}
						}
					}
				case *ast.FuncDecl:
					if d.Name.Name == "executeAction" && d.Recv != nil {
						execute = d
					}
				}
			}
		}
	}
	if execute == nil {
		t.Fatal("VirtualStore.executeAction not found; the routing measurement has nothing to read")
	}
	routed := make(map[string]struct{})
	ast.Inspect(execute.Body, func(n ast.Node) bool {
		cc, ok := n.(*ast.CaseClause)
		if !ok {
			return true
		}
		for _, e := range cc.List {
			if id, ok := e.(*ast.Ident); ok {
				if v, ok := values[id.Name]; ok {
					routed[strings.TrimPrefix(v, "/")] = struct{}{}
				}
			}
		}
		return true
	})
	if len(routed) == 0 {
		t.Fatal("executeAction routes no ActionType; the routing measurement has nothing to read")
	}
	return routed
}

// currentUnconsumedConclusions computes the inventory: "unconsumed <pred>" for
// each rule head nothing reads, "unrouted /<action>" for each constant
// next_action nothing executes.
func currentUnconsumedConclusions(t *testing.T) []string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := repoRootFrom(t, cwd)
	rules := mangleRules(t, filepath.Join(root, "internal", "core", "defaults"))
	if len(rules) == 0 {
		t.Fatal("no rule found in the corpus")
	}
	roots := goQueryRoots(t, root)
	if len(roots) == 0 {
		t.Fatal("no query root found in production Go; the liveness measurement has nothing to start from")
	}
	live := liveConclusions(rules, roots)
	routed := routedActions(t, root)

	seen := make(map[string]struct{})
	for _, r := range rules {
		if _, ok := live[r.head]; !ok {
			seen["unconsumed "+r.head] = struct{}{}
		}
		if r.head == "next_action" {
			if m := nextActionConstRe.FindStringSubmatch(r.headText); m != nil {
				if _, ok := routed[m[1]]; !ok {
					seen["unrouted /"+m[1]] = struct{}{}
				}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for line := range seen {
		out = append(out, line)
	}
	sort.Strings(out)
	return out
}

// TestExecutiveConclusionsAreConsumed fails when a rule starts deriving a
// conclusion nothing reads, or a next_action nothing routes, and when an entry
// on the list is wired without being removed from it.
func TestExecutiveConclusionsAreConsumed(t *testing.T) {
	current := currentUnconsumedConclusions(t)

	if os.Getenv("CODENERD_UPDATE_CONCLUSIONS") == "1" {
		writeConclusionsBaseline(t, current)
		t.Logf("rewrote %s with %d entr(ies)", conclusionsBaselinePath, len(current))
		return
	}

	data, err := os.ReadFile(conclusionsBaselinePath)
	if err != nil {
		t.Fatalf("read %s: %v\nRegenerate with: CODENERD_UPDATE_CONCLUSIONS=1 go test ./internal/core/defaults/ -run TestExecutiveConclusionsAreConsumed",
			conclusionsBaselinePath, err)
	}
	baseline := make(map[string]struct{})
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		baseline[line] = struct{}{}
	}
	currentSet := make(map[string]struct{}, len(current))
	var added []string
	for _, line := range current {
		currentSet[line] = struct{}{}
		if _, known := baseline[line]; !known {
			added = append(added, line)
		}
	}
	var wired []string
	for line := range baseline {
		if _, still := currentSet[line]; !still {
			wired = append(wired, line)
		}
	}
	sort.Strings(wired)

	if len(added) > 0 {
		t.Errorf(`%d new unconsumed conclusion(s): %v

"unconsumed p": a rule derives p and nothing reads it -- no production Go
query names it, and no rule whose head is read has it in its body. Either the
executive should act on it (query it where the decision is made, and delete
the Go that decides the same thing), or the rule should go.
"unrouted /x": a rule derives next_action(/x) and VirtualStore.executeAction
has no case for it.

A new query helper that wraps Query belongs in queryCalls.
Baseline: %s`, len(added), added, conclusionsBaselinePath)
	}
	if len(wired) > 0 {
		t.Errorf(`%d entr(ies) on the list are consumed now: %v

Good. Refresh the baseline so the count stays a measurement:
CODENERD_UPDATE_CONCLUSIONS=1 go test ./internal/core/defaults/ -run TestExecutiveConclusionsAreConsumed

Baseline: %s`, len(wired), wired, conclusionsBaselinePath)
	}
}

func writeConclusionsBaseline(t *testing.T, lines []string) {
	t.Helper()
	const header = `# Unconsumed executive conclusions -- derived by the kernel, acted on by nothing.
#
# Regenerate: CODENERD_UPDATE_CONCLUSIONS=1 go test ./internal/core/defaults/ -run TestExecutiveConclusionsAreConsumed
# See executive_conclusions_test.go. "unconsumed p": no production query reads p,
# directly or through a rule whose head is read. "unrouted /x": next_action(/x)
# is derived and VirtualStore.executeAction has no case for it.
#
# The list may only shrink. Each line is either a decision Go makes itself
# while the kernel's answer goes unread, or a rule nobody needs.

`
	body := header + strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(conclusionsBaselinePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", conclusionsBaselinePath, err)
	}
}
