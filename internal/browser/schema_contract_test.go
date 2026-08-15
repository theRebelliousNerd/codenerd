package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"codenerd/internal/mangle"
)

// Schema contract suite.
//
// Two silent-failure modes motivate these tests, both observed in this package:
//
//  1. A predicate asserted from Go with no Decl in schemas_browser.mg. The
//     engine rejects the fact with an error most call sites discard, so the
//     evidence simply never arrives.
//  2. A predicate asserted with the wrong argument type for its Decl. Nothing
//     errors: a "%.0f"-formatted coordinate stored an ast.String where the Decl
//     said /number, so `position(Elem, X, _, _, _), X < -1000` could never
//     unify and honeypot_offscreen/honeypot_zero_size were dead on live pages.
//
// Both are compile-clean and test-clean unless something checks the contract.

// declaredPredicate is one Decl from a .mg schema file.
type declaredPredicate struct {
	Name   string
	Bounds []string
}

var declPattern = regexp.MustCompile(`(?m)^\s*Decl\s+([a-z_0-9]+)\s*\(([^)]*)\)\s*bound\s*\[([^\]]*)\]`)

func repoRoot(t *testing.T) string {
	t.Helper()
	return getProjectRoot(t)
}

// loadBrowserDecls parses every Decl in schemas_browser.mg.
func loadBrowserDecls(t *testing.T) map[string]declaredPredicate {
	t.Helper()
	path := filepath.Join(repoRoot(t), "internal/core/defaults/schemas_browser.mg")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	decls := make(map[string]declaredPredicate)
	for _, match := range declPattern.FindAllStringSubmatch(string(data), -1) {
		bounds := strings.Split(match[3], ",")
		for i := range bounds {
			bounds[i] = strings.TrimSpace(bounds[i])
		}
		decls[match[1]] = declaredPredicate{Name: match[1], Bounds: bounds}
	}
	if len(decls) < 20 {
		t.Fatalf("parsed only %d Decls from schemas_browser.mg; the parser is broken", len(decls))
	}
	return decls
}

// browserSourceFiles lists the package's non-test Go sources.
func browserSourceFiles(t *testing.T) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(repoRoot(t), "internal/browser", "*.go"))
	if err != nil {
		t.Fatalf("glob sources: %v", err)
	}
	files := make([]string, 0, len(matches))
	for _, file := range matches {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		files = append(files, file)
	}
	if len(files) == 0 {
		t.Fatal("no browser sources found")
	}
	return files
}

var (
	assertPattern = regexp.MustCompile(`Predicate:\s*"([a-z_0-9]+)"`)
	pushPattern   = regexp.MustCompile(`PushFact\("([a-z_0-9]+)"`)
	queryPattern  = regexp.MustCompile(`(?:QueryFacts|EvaluateRule|GetFacts)\("([a-z_0-9]+)"`)
)

// TestBrowserPredicates_WhenUsedInGo_ShouldBeDeclaredInSchema is the ⊆ contract:
// every predicate this package asserts or queries must have a Decl.
func TestBrowserPredicates_WhenUsedInGo_ShouldBeDeclaredInSchema(t *testing.T) {
	decls := loadBrowserDecls(t)

	// Predicates this package touches that are declared elsewhere. Keep this
	// list empty unless a cross-schema dependency is genuinely intended.
	external := map[string]bool{}

	used := make(map[string][]string)
	for _, file := range browserSourceFiles(t) {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		text := string(data)
		for _, pattern := range []*regexp.Regexp{assertPattern, pushPattern, queryPattern} {
			for _, match := range pattern.FindAllStringSubmatch(text, -1) {
				used[match[1]] = append(used[match[1]], filepath.Base(file))
			}
		}
	}
	if len(used) == 0 {
		t.Fatal("scanned no predicate usages; the source scan is broken")
	}

	var missing []string
	for name, files := range used {
		if external[name] || decls[name].Name != "" {
			continue
		}
		missing = append(missing, fmt.Sprintf("%s (used in %s)", name, strings.Join(unique(files), ", ")))
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("predicates used in internal/browser with no Decl in schemas_browser.mg:\n  %s",
			strings.Join(missing, "\n  "))
	}
}

func unique(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

// checkFactAgainstDecl reports why a fact would not land as its Decl intends.
//
// The rules mirror mangle.convertValueToTypedTerm: only /string and /name
// coerce Go strings, everything else falls through to Go's dynamic type, so a
// string in a /number slot becomes an ast.String and a float64 in a /number
// slot becomes an ast.Float64. Neither errors and neither ever unifies.
func checkFactAgainstDecl(fact mangle.Fact, decl declaredPredicate) []string {
	var problems []string
	if len(fact.Args) != len(decl.Bounds) {
		return []string{fmt.Sprintf("%s: got %d args, Decl has %d", fact.Predicate, len(fact.Args), len(decl.Bounds))}
	}
	for i, arg := range fact.Args {
		bound := decl.Bounds[i]
		switch bound {
		case "/string":
			if _, ok := arg.(string); !ok {
				problems = append(problems, fmt.Sprintf("%s arg %d: %T is not a /string", fact.Predicate, i, arg))
			}
		case "/name":
			text, ok := arg.(string)
			if !ok {
				problems = append(problems, fmt.Sprintf("%s arg %d: %T is not a /name", fact.Predicate, i, arg))
				continue
			}
			if !strings.HasPrefix(text, "/") {
				problems = append(problems, fmt.Sprintf(
					"%s arg %d: /name slot given %q without a leading slash; it is stored as /%s and a string-shaped query never matches it",
					fact.Predicate, i, text, text))
			}
		case "/number":
			switch arg.(type) {
			case int, int32, int64:
			default:
				problems = append(problems, fmt.Sprintf(
					"%s arg %d: /number slot given %T; only integer types become ast.Number", fact.Predicate, i, arg))
			}
		case "/float64":
			switch arg.(type) {
			case float32, float64:
			default:
				problems = append(problems, fmt.Sprintf("%s arg %d: /float64 slot given %T", fact.Predicate, i, arg))
			}
		}
	}
	return problems
}

func sampleDOMNodes() []domSnapshotNode {
	nodes := make([]domSnapshotNode, 0, 4)
	for _, spec := range []struct {
		id      string
		tag     string
		typ     string
		visible bool
	}{
		{"login", "FORM", "", true},
		{"user", "INPUT", "text", true},
		{"agree", "INPUT", "checkbox", true},
		{"submit", "BUTTON", "", false},
	} {
		node := domSnapshotNode{
			ID:     spec.id,
			Tag:    spec.tag,
			Text:   "sample text",
			Parent: "root",
			Attrs:  map[string]string{"type": spec.typ, "name": spec.id, "value": "x"},
			Styles: map[string]string{"display": "block", "visibility": "visible", "opacity": "1", "pointerEvents": "auto"},
		}
		node.Layout.X = 10.5
		node.Layout.Y = 20.5
		node.Layout.Width = 100.25
		node.Layout.Height = 30.75
		node.Layout.Visible = spec.visible
		nodes = append(nodes, node)
	}
	// An anchor and a textarea exercise the remaining interactable branches.
	link := domSnapshotNode{ID: "home", Tag: "A", Parent: "root", Attrs: map[string]string{"href": "/"}, Styles: map[string]string{"display": "inline"}}
	area := domSnapshotNode{ID: "notes", Tag: "TEXTAREA", Parent: "root", Attrs: map[string]string{}, Styles: map[string]string{"display": "block"}}
	return append(nodes, link, area)
}

// TestSnapshotDOMFacts_WhenBuilt_ShouldMatchDeclaredBoundTypes checks every
// fact SnapshotDOM produces against schemas_browser.mg.
func TestSnapshotDOMFacts_WhenBuilt_ShouldMatchDeclaredBoundTypes(t *testing.T) {
	decls := loadBrowserDecls(t)
	manager := newSessionManager(DefaultConfig(), nil)

	facts := manager.buildDOMFacts("sess1", sampleDOMNodes(), time.Now())
	if len(facts) == 0 {
		t.Fatal("buildDOMFacts produced no facts")
	}

	seen := make(map[string]bool)
	var problems []string
	for _, fact := range facts {
		seen[fact.Predicate] = true
		decl, ok := decls[fact.Predicate]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s has no Decl in schemas_browser.mg", fact.Predicate))
			continue
		}
		problems = append(problems, checkFactAgainstDecl(fact, decl)...)
	}
	if len(problems) > 0 {
		t.Errorf("SnapshotDOM fact/Decl mismatches:\n  %s", strings.Join(unique(problems), "\n  "))
	}

	// Guard against the builder quietly losing a whole predicate family.
	for _, required := range []string{
		"dom_node", "dom_text", "dom_attr", "attribute", "dom_layout",
		"element", "position", "geometry", "interactable", "computed_style",
		"css_property", "dom_updated",
	} {
		if !seen[required] {
			t.Errorf("SnapshotDOM no longer asserts %s", required)
		}
	}
}

// TestSnapshotDOMFacts_WhenAssertedIntoEngine_ShouldBeAccepted proves the
// contract end to end: a real engine loaded with the real schema takes every
// fact without rejecting one.
func TestSnapshotDOMFacts_WhenAssertedIntoEngine_ShouldBeAccepted(t *testing.T) {
	engine := newBrowserTestEngine(t)
	manager := NewSessionManager(DefaultConfig(), engine)

	facts := manager.buildDOMFacts("sess1", sampleDOMNodes(), time.Now())
	if err := engine.AddFacts(facts); err != nil {
		t.Fatalf("engine rejected a SnapshotDOM fact: %v", err)
	}

	// position/5 must land as numbers, not strings: the honeypot geometry rules
	// depend on it and comparison against an ast.String silently never fires.
	stored, err := engine.GetFacts("position")
	if err != nil {
		t.Fatalf("GetFacts(position): %v", err)
	}
	if len(stored) == 0 {
		t.Fatal("no position facts stored")
	}
	for _, fact := range stored {
		for i := 1; i < len(fact.Args); i++ {
			if _, ok := fact.Args[i].(int64); !ok {
				t.Errorf("position arg %d stored as %T (%v), want int64", i, fact.Args[i], fact.Args[i])
			}
		}
	}
}

// TestManagerControlFacts_WhenBuilt_ShouldMatchDeclaredBoundTypes covers the
// facts the manager authors itself - epoch watermarks, stream saturation, and
// interaction refusals - which no DOM capture would exercise.
func TestManagerControlFacts_WhenBuilt_ShouldMatchDeclaredBoundTypes(t *testing.T) {
	decls := loadBrowserDecls(t)
	now := time.Now()

	facts := blockedInteractionFacts("sess1", "honeypot click: Hidden via display:none", now)
	facts = append(facts,
		mangle.Fact{Predicate: "browser_epoch", Args: []any{"sess1", int64(3), now.UnixMilli()}},
		mangle.Fact{Predicate: "browser_stream_saturated", Args: []any{"sess1", int64(3), int64(20000), now.UnixMilli()}},
		mangle.Fact{Predicate: "css_clip_rect", Args: []any{"e1", int64(0), int64(0), int64(0), int64(0)}},
		mangle.Fact{Predicate: "link_url_pattern", Args: []any{"e1", "/bait_path"}},
	)

	engine := newBrowserTestEngine(t)
	var problems []string
	for _, fact := range facts {
		decl, ok := decls[fact.Predicate]
		if !ok {
			problems = append(problems, fact.Predicate+" has no Decl")
			continue
		}
		problems = append(problems, checkFactAgainstDecl(fact, decl)...)
	}
	if len(problems) > 0 {
		t.Errorf("manager control fact mismatches:\n  %s", strings.Join(problems, "\n  "))
	}
	if err := engine.AddFacts(facts); err != nil {
		t.Errorf("engine rejected a manager control fact: %v", err)
	}
}

// TestCheckFactAgainstDecl_WhenTypesDrift_ShouldReportProblem pins the checker
// itself. Without this, a checker that silently accepted everything would make
// the two contract tests above look green forever.
func TestCheckFactAgainstDecl_WhenTypesDrift_ShouldReportProblem(t *testing.T) {
	decls := loadBrowserDecls(t)

	// The exact shape of the position/5 regression: coordinates formatted as
	// strings into /number slots.
	stringCoords := mangle.Fact{Predicate: "position", Args: []any{"e", "-9999", "0", "1", "1"}}
	if problems := checkFactAgainstDecl(stringCoords, decls["position"]); len(problems) != 4 {
		t.Errorf("string coordinates in /number slots reported %d problems, want 4: %v", len(problems), problems)
	}

	// float64 also loses: it becomes ast.Float64, not ast.Number.
	floatCoords := mangle.Fact{Predicate: "position", Args: []any{"e", 1.5, 2.5, 3.5, 4.5}}
	if problems := checkFactAgainstDecl(floatCoords, decls["position"]); len(problems) != 4 {
		t.Errorf("float coordinates in /number slots reported %d problems, want 4: %v", len(problems), problems)
	}

	// A bare string in a /name slot is stored as a Name and never matches a
	// string-shaped query.
	bareName := mangle.Fact{Predicate: "interactable", Args: []any{"e", "click"}}
	if problems := checkFactAgainstDecl(bareName, decls["interactable"]); len(problems) != 1 {
		t.Errorf("bare /name value reported %d problems, want 1: %v", len(problems), problems)
	}

	wrongArity := mangle.Fact{Predicate: "position", Args: []any{"e"}}
	if problems := checkFactAgainstDecl(wrongArity, decls["position"]); len(problems) != 1 {
		t.Errorf("arity mismatch reported %d problems, want 1: %v", len(problems), problems)
	}

	good := mangle.Fact{Predicate: "position", Args: []any{"e", int64(1), int64(2), int64(3), int64(4)}}
	if problems := checkFactAgainstDecl(good, decls["position"]); len(problems) != 0 {
		t.Errorf("well-typed fact reported problems: %v", problems)
	}
}

// newBrowserTestEngine returns an engine with the browser schema and honeypot
// policy loaded from the real default files.
func newBrowserTestEngine(t *testing.T) *mangle.Engine {
	t.Helper()
	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	root := repoRoot(t)
	for _, path := range []string{
		filepath.Join(root, "internal/core/defaults/schemas_browser.mg"),
		filepath.Join(root, "internal/core/defaults/policy/browser_honeypot.mg"),
	} {
		if err := engine.LoadSchema(path); err != nil {
			t.Fatalf("load %s: %v", path, err)
		}
	}
	return engine
}
