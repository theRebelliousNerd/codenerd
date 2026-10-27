package defaults

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

func parseScannerFixture(t *testing.T, filename, source string) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), filename, source, 0)
	if err != nil {
		t.Fatalf("parse scanner fixture: %v", err)
	}
	return file
}

func requireRoot(t *testing.T, roots map[string]struct{}, predicate string, want bool) {
	t.Helper()
	_, found := roots[predicate]
	if found != want {
		t.Fatalf("root %q present=%t, want %t; roots=%v", predicate, found, want, roots)
	}
}

func TestConclusionScannerTracksFunctionLocalQueryAssignments(t *testing.T) {
	file := parseScannerFixture(t, "fixture.go", `package sample
import "fmt"
func read(kernel Kernel, requestID, sessionID string) {
	query := fmt.Sprintf("mcp_remote_permitted(%s)", requestID)
	kernel.Query(query)
	var browserQuery string
	browserQuery = fmt.Sprintf("browser_clock_sample(%s)", sessionID)
	kernel.Query(browserQuery)
	stale := fmt.Sprintf("must_be_invalidated(%s)", requestID)
	stale = requestID
	kernel.Query(stale)
	unrelated := "genuinely_dormant(Thing)"
	_ = unrelated
}
`)
	roots := queryRootsInFile(file, nil)
	requireRoot(t, roots, "mcp_remote_permitted", true)
	requireRoot(t, roots, "browser_clock_sample", true)
	requireRoot(t, roots, "must_be_invalidated", false)
	requireRoot(t, roots, "genuinely_dormant", false)
}

func TestConclusionScannerFiniteWrapperRootsPreserveDormantHeads(t *testing.T) {
	file := parseScannerFixture(t, "browser_reasoning.go", `package research
type browserPredicateSpec struct{}
type Kernel interface { Query(string) ([]int, error) }
var browserPredicateSpecs = map[string]browserPredicateSpec{
	"actual_browser_root": {},
	"actual_second_root": {},
}
var unrelatedText = "unrelated_literal_is_not_a_root"
func queryScopedBrowserFacts(kernel Kernel, query string) ([]int, error) {
	return kernel.Query(query)
}
func readFacts(kernel Kernel) {
	_, _ = queryScopedBrowserFacts(kernel, "actual_browser_root")
}
`)
	wrapper := finiteQueryWrapper{function: "queryScopedBrowserFacts", mapName: "browserPredicateSpecs"}
	roots := finiteWrapperRoots([]*ast.File{file}, wrapper)
	requireRoot(t, roots, "actual_browser_root", true)
	requireRoot(t, roots, "actual_second_root", true)
	requireRoot(t, roots, "unrelated_literal_is_not_a_root", false)

	rules := []mgRule{
		{head: "actual_browser_root", body: []string{"browser_helper"}},
		{head: "genuinely_dormant", body: []string{"dormant_input"}},
	}
	live := liveConclusions(rules, roots)
	if _, ok := live["browser_helper"]; !ok {
		t.Fatal("finite wrapper root did not retain rule-body closure")
	}
	if _, ok := live["genuinely_dormant"]; ok {
		t.Fatal("dormant conclusion became live without a production query root")
	}
	if _, ok := live["unrelated_literal_is_not_a_root"]; ok {
		t.Fatal("unrelated literal became a query root")
	}
}

func TestConclusionScannerUsesCurrentBrowserAllowlistAndMCPLocalQuery(t *testing.T) {
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	root := repoRootFrom(t, cwd)
	roots := goQueryRoots(t, root)
	requireRoot(t, roots, "mcp_remote_permitted", true)
	requireRoot(t, roots, "root_cause_at", true)

	// Every exposed browser predicate is sourced from browserPredicateSpecs;
	// the scanner must not replace that finite source with hand-maintained roots.
	browserDir := filepath.Join(root, "internal", "tools", "research")
	entries, err := filepath.Glob(filepath.Join(browserDir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	files := make([]*ast.File, 0, len(entries))
	for _, path := range entries {
		if !isProductionGo(path) {
			continue
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			continue
		}
		files = append(files, file)
	}
	wrapperRoots := finiteWrapperRoots(files, finiteQueryWrappers["internal/tools/research/browser_reasoning.go"])
	if len(wrapperRoots) == 0 {
		t.Fatal("production browser query wrapper yielded no finite map-backed roots")
	}
	for predicate := range wrapperRoots {
		if _, found := roots[predicate]; !found {
			t.Errorf("production browser allowlist predicate %q missing from query roots", predicate)
		}
	}
}
