package world

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// pathArgIndexes names, per predicate, which argument slots carry a file or
// directory identity. Keeping it explicit (rather than sniffing for a "/") is
// what lets the property test distinguish a path from an import path or a
// signature that also happens to contain a slash.
var pathArgIndexes = map[string][]int{
	"file_topology":  {0},
	"directory":      {0},
	"file_dir":       {0, 1},
	"test_file_for":  {0, 1},
	"symbol_graph":   {3},
	"entry_point":    {0},
	"code_defines":   {0},
	"function_scope": {0},
}

// factPathArgs returns the path identities carried by a fact.
func factPathArgs(f Fact) []string {
	var out []string
	if f.Predicate == "dependency_link" {
		// Slot 0 is always the importing file. Slot 1 is a file only once
		// resolution has turned the raw "pkg:"/"mod:"/"crate:" token into one.
		if len(f.Args) > 0 {
			if s, ok := f.Args[0].(string); ok {
				out = append(out, s)
			}
		}
		if len(f.Args) > 1 {
			if s, ok := f.Args[1].(string); ok && !strings.Contains(s, ":") {
				out = append(out, s)
			}
		}
		return out
	}
	for _, i := range pathArgIndexes[f.Predicate] {
		if i < len(f.Args) {
			if s, ok := f.Args[i].(string); ok && s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func assertCanonicalIdentity(t *testing.T, who, p string) {
	t.Helper()
	if strings.Contains(p, `\`) {
		t.Errorf("%s: path identity %q contains a backslash; identities are slash-normalized on every platform", who, p)
	}
	if strings.HasPrefix(p, "/") || (len(p) > 2 && p[1] == ':') {
		t.Errorf("%s: path identity %q is absolute; identities must be workspace-relative or the store stops being portable", who, p)
	}
	if strings.HasPrefix(p, "./") || strings.Contains(p, "/./") || strings.Contains(p, "..") {
		t.Errorf("%s: path identity %q is not cleaned", who, p)
	}
}

// randomWorkspace materializes a randomized, multi-language file tree. Random
// shapes (nested dirs, spaces, unicode, mixed extensions) are the point: the
// canonicalization bugs this guards against only appeared for specific path
// shapes.
func randomWorkspace(t *testing.T, seed int64) string {
	t.Helper()
	rng := rand.New(rand.NewSource(seed))
	root := t.TempDir()

	dirNames := []string{"internal", "pkg core", "src", "λ-module", "a", "deep/nested/tree"}
	fileTemplates := []struct {
		ext     string
		content string
	}{
		{".go", "package p\n\nimport \"strings\"\n\nfunc Do%d() string { return strings.TrimSpace(\"x\") }\n"},
		{".py", "import os\n\ndef do_%d():\n    return os.getcwd()\n"},
		{".ts", "import { thing } from \"./other\";\nexport function do%d() { return thing(); }\n"},
		{".js", "export function do%d() { return 1; }\n"},
		{".rs", "pub fn do_%d() -> u32 { 1 }\n"},
		{".md", "# doc %d\n"},
	}

	nDirs := 2 + rng.Intn(4)
	for i := range nDirs {
		dir := dirNames[rng.Intn(len(dirNames))]
		full := filepath.Join(root, filepath.FromSlash(dir), fmt.Sprintf("sub%d", i))
		if err := os.MkdirAll(full, 0o755); err != nil {
			t.Fatal(err)
		}
		for j := range 2 + rng.Intn(3) {
			tpl := fileTemplates[rng.Intn(len(fileTemplates))]
			name := fmt.Sprintf("file %d%s", j, tpl.ext)
			if rng.Intn(2) == 0 {
				name = fmt.Sprintf("mod_%d%s", j, tpl.ext)
			}
			if err := os.WriteFile(filepath.Join(full, name), fmt.Appendf(nil, tpl.content, j), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/rand\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func pathIdentitySet(facts []Fact) map[string]struct{} {
	set := make(map[string]struct{})
	for _, f := range facts {
		for _, p := range factPathArgs(f) {
			set[p] = struct{}{}
		}
	}
	return set
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestScanners_WhenSameWorkspace_ShouldProduceIdenticalPathIdentities is the
// property that ties the three producers together: whatever a full scan calls a
// file, an incremental delta scan must call it too. They disagreed before —
// the delta path handed the absolute walk path to the AST parsers — so
// symbol_graph facts asserted after an edit keyed a file no file_topology row
// mentioned, and every symbol/file join silently emptied.
func TestScanners_WhenSameWorkspace_ShouldProduceIdenticalPathIdentities(t *testing.T) {
	for _, seed := range []int64{1, 7, 42, 1337, 90210} {
		t.Run(fmt.Sprintf("seed%d", seed), func(t *testing.T) {
			root := randomWorkspace(t, seed)
			scanner := NewScanner()
			ctx := context.Background()

			fullFacts, err := scanner.ScanWorkspaceCtx(ctx, root)
			if err != nil {
				t.Fatalf("full scan: %v", err)
			}
			fullSet := pathIdentitySet(fullFacts)
			if len(fullSet) == 0 {
				t.Fatal("full scan produced no path identities")
			}
			for p := range fullSet {
				assertCanonicalIdentity(t, "full scan", p)
			}

			// Prime the incremental cache, then touch every file so the delta
			// path (not the full fallback) re-derives all of them.
			if _, err := scanner.ScanWorkspaceIncremental(ctx, root, nil, IncrementalOptions{}); err != nil {
				t.Fatalf("priming incremental scan: %v", err)
			}
			future := time.Now().Add(2 * time.Second)
			if err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() || strings.Contains(p, ".nerd") {
					return nil
				}
				return os.Chtimes(p, future, future)
			}); err != nil {
				t.Fatal(err)
			}

			delta, err := scanner.ScanWorkspaceIncremental(ctx, root, nil, IncrementalOptions{})
			if err != nil {
				t.Fatalf("delta scan: %v", err)
			}
			if delta.Full {
				t.Fatal("expected a delta scan, got a full fallback")
			}
			deltaSet := pathIdentitySet(delta.NewFacts)
			for p := range deltaSet {
				assertCanonicalIdentity(t, "incremental scan", p)
			}

			// The delta re-derived every file, so both scanners must name the
			// same identities. Directory facts are emitted by both.
			gotFull, gotDelta := sortedKeys(fullSet), sortedKeys(deltaSet)
			if strings.Join(gotFull, "\n") != strings.Join(gotDelta, "\n") {
				onlyFull, onlyDelta := diffSets(fullSet, deltaSet)
				t.Errorf("path identities differ between full and incremental scans\nonly in full: %v\nonly in incremental: %v", onlyFull, onlyDelta)
			}
		})
	}
}

func diffSets(a, b map[string]struct{}) (onlyA, onlyB []string) {
	for k := range a {
		if _, ok := b[k]; !ok {
			onlyA = append(onlyA, k)
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			onlyB = append(onlyB, k)
		}
	}
	sort.Strings(onlyA)
	sort.Strings(onlyB)
	return onlyA, onlyB
}

// TestDeepScan_WhenGivenAbsoluteOrCanonicalPaths_ShouldEmitOneIdentity covers
// the third producer: deep facts must land on the same identity as the fast
// scan no matter which spelling the caller had on hand (chat passes canonical
// paths read out of file_topology, the session scope passes absolute ones).
func TestDeepScan_WhenGivenAbsoluteOrCanonicalPaths_ShouldEmitOneIdentity(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "svc"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := "package svc\n\nfunc Helper() int { return 1 }\n\nfunc Run() int { return Helper() }\n"
	abs := filepath.Join(root, "internal", "svc", "run.go")
	if err := os.WriteFile(abs, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	fromAbs, err := EnsureDeepFactsInRoot(context.Background(), root, []string{abs}, nil, 1)
	if err != nil {
		t.Fatalf("deep scan (absolute): %v", err)
	}
	fromCanonical, err := EnsureDeepFactsInRoot(context.Background(), root, []string{"internal/svc/run.go"}, nil, 1)
	if err != nil {
		t.Fatalf("deep scan (canonical): %v", err)
	}

	absSet := pathIdentitySet(fromAbs.NewFacts)
	canSet := pathIdentitySet(fromCanonical.NewFacts)
	if len(absSet) == 0 {
		t.Fatal("deep scan produced no code_defines facts")
	}
	for p := range absSet {
		assertCanonicalIdentity(t, "deep scan", p)
	}
	if strings.Join(sortedKeys(absSet), ",") != strings.Join(sortedKeys(canSet), ",") {
		t.Errorf("deep scan identity depends on the caller's path spelling: absolute=%v canonical=%v",
			sortedKeys(absSet), sortedKeys(canSet))
	}

	// And that identity must be the one the fast scanner emits.
	scanner := NewScanner()
	fastFacts, err := scanner.ScanWorkspaceCtx(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	fast := pathIdentitySet(fastFacts)
	for p := range absSet {
		if _, ok := fast[p]; !ok {
			t.Errorf("deep identity %q is not a fast-scan identity (%v); deep and fast facts cannot join", p, sortedKeys(fast))
		}
	}
}

// TestCanonicalPath_WhenReapplied_ShouldBeIdempotent — canonicalization is
// applied defensively at several layers, so applying it to an already-canonical
// value must not change it.
func TestCanonicalPath_WhenReapplied_ShouldBeIdempotent(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	cases := []string{
		filepath.Join(root, "a", "b.go"),
		"a/b.go",
		`a\b.go`,
		"./a/b.go",
		`C:\ws\a\b.go`,
		"a//b.go",
	}
	for _, in := range cases {
		once := CanonicalPath(root, in)
		twice := CanonicalPath(root, once)
		if once != twice {
			t.Errorf("CanonicalPath not idempotent for %q: %q then %q", in, once, twice)
		}
		if strings.Contains(once, `\`) {
			t.Errorf("CanonicalPath(%q) = %q still contains a backslash", in, once)
		}
	}
}

// TestResolveWorkspacePath_WhenCanonical_ShouldOpenTheRealFile — canonical
// identities are not openable on their own; the deep-scan cache silently
// no-op'd for exactly this reason whenever the process was not chdir'd into the
// workspace.
func TestResolveWorkspacePath_WhenCanonical_ShouldOpenTheRealFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "x", "y.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ResolveWorkspacePath(root, "x/y.go")); err != nil {
		t.Fatalf("canonical path did not resolve to a readable file: %v", err)
	}
	abs := filepath.Join(root, "x", "y.go")
	if got := ResolveWorkspacePath(root, abs); got != abs {
		t.Errorf("ResolveWorkspacePath rewrote an absolute path: %q", got)
	}
}
