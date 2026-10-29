package world

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/store"
	"codenerd/internal/tools"
)

// TestIncrementalScan_WhenGoDeclarationChanges_RefreshesSiblingCallRows is the
// package-symbol hole in the delta scan.
//
// goSymbolFacts spells b.go's code_calls from the package's symbols, and those
// symbols are read from every sibling file (cartographer.go symbolsFor). T(x)
// in b.go is a call when a.go declares func T and a conversion when a.go
// declares type T, so a.go's declaration change rewrites b.go's rows even
// though b.go's bytes did not change. The delta scan used to re-parse only the
// file whose size or mtime moved, and the deep cache then reused b.go's old
// rows forever: its fingerprint still matched.
//
// The cold subtest drops the in-process symbol cache first. A restarted
// process has no cache; the previous declaration set has to come from the
// code_defines rows the deep scan stored.
func TestIncrementalScan_WhenGoDeclarationChanges_RefreshesSiblingCallRows(t *testing.T) {
	for _, cold := range []bool{false, true} {
		name := "warm-cache"
		if cold {
			name = "cold-cache"
		}
		t.Run(name, func(t *testing.T) {
			root, db, scanner := scanSiblingPackage(t, cold)

			aSrc := "package p\n\ntype T int\n"
			if err := os.WriteFile(filepath.Join(root, "a.go"), []byte(aSrc), 0o644); err != nil {
				t.Fatal(err)
			}
			delta := rescan(t, scanner, root, db)

			if !hasFact(delta.RetractFacts, "code_calls", "fn:p.Use", "fn:p.T") {
				t.Fatal("b.go kept code_calls(fn:p.Use, fn:p.T) after a.go declared type T; the sibling's call rows are stale")
			}
			if !hasFact(delta.NewFacts, "code_calls", "p.Use", "p.T") {
				t.Fatal("refreshed b.go dropped the bare code_calls(p.Use, p.T) row")
			}
			if hasFact(delta.NewFacts, "code_calls", "fn:p.Use", "fn:p.T") {
				t.Fatal("refreshed b.go still emits code_calls(fn:p.Use, fn:p.T) for a conversion")
			}
			if hasFact(delta.RetractFacts, "code_calls", "fn:sub.Local", "fn:sub.helper") {
				t.Fatal("a declaration change in package p remapped sub/c.go, a different directory")
			}
			if hasFact(delta.RetractFacts, "code_calls", "fn:q.Q", "fn:q.qhelper") {
				t.Fatal("a declaration change in package p remapped q.go, a different package in the same directory")
			}

			gotTopo := topologyFiles(delta.NewFacts)
			wantTopo := []string{"a.go", "b.go"}
			if strings.Join(gotTopo, ",") != strings.Join(wantTopo, ",") {
				t.Fatalf("remapped files = %v, want %v", gotTopo, wantTopo)
			}

			want := mapGoCalls(t, filepath.Join(root, "b.go"))
			got := loadDeepCalls(t, db, root, "b.go")
			if !callSetsEqual(got, want) {
				t.Fatalf("stored b.go calls = %v, fresh map = %v", callList(got), callList(want))
			}
			if got[[2]string{"fn:p.Use", "fn:p.T"}] {
				t.Fatal("deep cache for b.go still holds the fn: call after T became a type")
			}
		})
	}
}

// TestIncrementalScan_WhenGoBodyChanges_RemapsOnlyThatFile pins the other
// half of the same rule. A body edit does not change the package declaration
// set, so sibling files keep the rows they already have: re-mapping the
// directory would retract and reassert every untouched file.
func TestIncrementalScan_WhenGoBodyChanges_RemapsOnlyThatFile(t *testing.T) {
	for _, cold := range []bool{false, true} {
		name := "warm-cache"
		if cold {
			name = "cold-cache"
		}
		t.Run(name, func(t *testing.T) {
			root, db, scanner := scanSiblingPackage(t, cold)

			body := "package p\n\nfunc T(x int) int { return x + 1 }\n"
			if err := os.WriteFile(filepath.Join(root, "a.go"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			delta := rescan(t, scanner, root, db)

			if len(delta.ChangedFiles) != 1 || filepath.Base(delta.ChangedFiles[0]) != "a.go" {
				t.Fatalf("changed files = %v, want only a.go", delta.ChangedFiles)
			}
			gotTopo := topologyFiles(delta.NewFacts)
			if strings.Join(gotTopo, ",") != "a.go" {
				t.Fatalf("remapped files = %v, want only a.go", gotTopo)
			}
			if hasFact(delta.RetractFacts, "code_calls", "fn:p.Use", "fn:p.T") {
				t.Fatal("a body edit retracted b.go's call rows")
			}
			if hasFact(delta.NewFacts, "code_calls", "fn:p.Use", "fn:p.T") {
				t.Fatal("a body edit re-emitted b.go's call rows")
			}
			got := loadDeepCalls(t, db, root, "b.go")
			if !got[[2]string{"fn:p.Use", "fn:p.T"}] {
				t.Fatalf("body edit dropped b.go's fn: call; stored = %v", callList(got))
			}
		})
	}
}

func scanSiblingPackage(t *testing.T, cold bool) (string, *store.LocalStore, *Scanner) {
	t.Helper()
	root := t.TempDir()
	db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	writeWorkspaceFile(t, root, "a.go", "package p\n\nfunc T(x int) int { return x }\n")
	writeWorkspaceFile(t, root, "b.go", "package p\n\nfunc Use(x int) int { return T(x) }\n")
	writeWorkspaceFile(t, root, "q.go", "package q\n\nfunc Q() int { return qhelper() }\n\nfunc qhelper() int { return 1 }\n")
	cPath := writeWorkspaceFile(t, root, "sub/c.go", "package sub\n\nfunc Local() int { return helper() }\n\nfunc helper() int { return 1 }\n")

	scanner := NewScanner()
	ctx := context.Background()
	if _, err := scanner.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{}); err != nil {
		t.Fatal(err)
	}
	paths := []string{
		filepath.Join(root, "a.go"),
		filepath.Join(root, "b.go"),
		filepath.Join(root, "q.go"),
		cPath,
	}
	if _, err := EnsureDeepFactsInRoot(ctx, root, paths, db, 1); err != nil {
		t.Fatal(err)
	}
	if !loadDeepCalls(t, db, root, "b.go")[[2]string{"fn:p.Use", "fn:p.T"}] {
		t.Fatal("fixture: b.go did not record code_calls(fn:p.Use, fn:p.T) while a.go declared func T")
	}
	canon := root
	if resolved, err := tools.CanonicalWorkspaceRoot(root); err == nil {
		canon = resolved
	}
	if cold {
		dropSymbolCacheUnder(root)
		if _, had := peekPkgSymbols(root, "p", false); had {
			t.Fatal("cold subtest: symbol cache for the temp root still holds package p")
		}
		if _, had := peekPkgSymbols(canon, "p", false); had {
			t.Fatalf("cold subtest: symbol cache for canonical root %s still holds package p", canon)
		}
	} else if sym, had := peekPkgSymbols(canon, "p", false); !had || !sym.funcs["T"] {
		if sym, had = peekPkgSymbols(root, "p", false); !had || !sym.funcs["T"] {
			t.Fatal("warm subtest: symbol cache does not hold func T, so the in-process declaration compare has no before-set")
		}
	}
	return root, db, scanner
}

func rescan(t *testing.T, scanner *Scanner, root string, db *store.LocalStore) *IncrementalResult {
	t.Helper()
	delta, err := scanner.ScanWorkspaceIncremental(context.Background(), root, db, IncrementalOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if delta.Full {
		t.Fatal("expected a delta scan")
	}
	return delta
}

func dropSymbolCacheUnder(root string) {
	dirs := []string{root}
	if canon, err := tools.CanonicalWorkspaceRoot(root); err == nil && canon != root {
		dirs = append(dirs, canon)
	}
	symbolCacheMu.Lock()
	defer symbolCacheMu.Unlock()
	for key := range symbolCache {
		for _, dir := range dirs {
			if strings.HasPrefix(key, dir+"\x00") || strings.HasPrefix(key, filepath.Clean(dir)+"\x00") {
				delete(symbolCache, key)
				break
			}
		}
	}
}

func topologyFiles(facts []core.Fact) []string {
	seen := map[string]struct{}{}
	for _, f := range facts {
		if f.Predicate != "file_topology" || len(f.Args) == 0 {
			continue
		}
		p, ok := f.Args[0].(string)
		if !ok || p == "" {
			continue
		}
		seen[p] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func mapGoCalls(t *testing.T, abs string) map[[2]string]bool {
	t.Helper()
	c := NewCartographer()
	t.Cleanup(func() { c.Close() })
	facts, err := c.MapFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	return callSet(facts)
}

func loadDeepCalls(t *testing.T, db *store.LocalStore, root, rel string) map[[2]string]bool {
	t.Helper()
	inputs, _, err := db.LoadWorldFactsForFile(rel, "deep")
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) == 0 {
		// The scan keys rows by the canonical root. rel is already canonical
		// for these fixtures ("b.go"); try the walk path only as a diagnostic.
		abs := filepath.Join(root, filepath.FromSlash(rel))
		canon := canonicalScanPath(root, abs)
		if canon != rel {
			inputs, _, err = db.LoadWorldFactsForFile(canon, "deep")
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	facts := make([]core.Fact, len(inputs))
	for i, in := range inputs {
		facts[i] = core.Fact{Predicate: in.Predicate, Args: in.Args}
	}
	return callSet(facts)
}

func callSet(facts []core.Fact) map[[2]string]bool {
	out := map[[2]string]bool{}
	for _, f := range facts {
		if f.Predicate != "code_calls" || len(f.Args) < 2 {
			continue
		}
		caller, ok1 := f.Args[0].(string)
		callee, ok2 := f.Args[1].(string)
		if ok1 && ok2 {
			out[[2]string{caller, callee}] = true
		}
	}
	return out
}

func callSetsEqual(a, b map[[2]string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func callList(m map[[2]string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k[0]+" -> "+k[1])
	}
	sort.Strings(out)
	return out
}
