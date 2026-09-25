package campaign

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"codenerd/internal/build"
	"codenerd/internal/gates"
	"codenerd/internal/tools"
)

// The recurse DAG is derived from the workspace it sweeps, not written down.
//
// Until 2026-09-25 the sweep order was a Go table of codeNERD's own packages
// (internal/mangle, internal/core, ... cmd). Pointed at any other repository,
// recurse planned work on paths that did not exist; pointed at codeNERD, it
// saw fourteen hand-grouped subsystems where the module has over a hundred
// packages. The order is now what the code's own imports say: a node is a
// package directory, an edge is an import between two of them, and a node
// sweeps after everything it imports -- leaves first, entry points last, then
// the cross-cutting close (wiring, review, benchmarks).
//
// Languages read today:
//   - Go: `go list -json ./...` (exact packages and imports)
//   - Python: `import` / `from ... import` statements, absolute and relative,
//     resolved against the workspace's own package directories (and src/)
//   - JavaScript/TypeScript: relative import, require and dynamic import
//     specifiers resolved to files or index files
//   - Rust: one node per Cargo crate, edges from `path = "..."` dependencies
// Anything else contributes no nodes. A workspace with no readable structure
// is swept as one node, ".".
//
// Import cycles (legal outside Go) collapse into one node whose ID joins its
// members with "+", so the order stays a DAG without dropping an edge.

// Cross-cutting node IDs close every pass.
const (
	RecurseWiringNodeID = "wiring"
	RecurseReviewNodeID = "review"
	RecurseBenchNodeID  = "bench"
)

// deriveRecurseDAG is the DAG source recurse planning uses. A variable so
// planning tests can hand in a fixed graph instead of scanning a workspace.
var deriveRecurseDAG = DeriveWorkspaceDAG

// DeriveWorkspaceDAG returns the sweep DAG for the workspace at root: one
// node per package directory the workspace's own imports connect, plus the
// cross-cutting close. The result is unordered; run TopoOrder before planning.
func DeriveWorkspaceDAG(ctx context.Context, root string) ([]SubsystemNode, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("recurse DAG: empty workspace root")
	}
	// One spelling of the root: go list reports package directories through
	// the resolved path, so a symlinked or 8.3-aliased root compared as given
	// matched none of them and the sweep lost every Go package.
	abs, err := tools.CanonicalWorkspaceRoot(root)
	if err != nil {
		return nil, fmt.Errorf("recurse DAG: %w", err)
	}
	g := newWorkspaceGraph()
	files, err := collectSourceFiles(abs)
	if err != nil {
		return nil, fmt.Errorf("recurse DAG: walk %s: %w", abs, err)
	}
	if fileExists(filepath.Join(abs, "go.mod")) {
		if err := scanGoPackages(ctx, abs, g); err != nil {
			return nil, err
		}
	}
	scanRustCrates(abs, files["Cargo.toml"], g)
	scanPythonImports(abs, files[".py"], g)
	scanJSImports(abs, files[".js"], g)
	if len(g.nodes) == 0 {
		g.addNode(".", "")
	}
	return appendCrossCutting(g.collapse()), nil
}

// workspaceGraph is the package-level import graph, keyed by workspace-relative
// slash directory ("." for the root).
type workspaceGraph struct {
	nodes map[string]*graphNode
}

type graphNode struct {
	dir   string
	langs map[string]bool
	deps  map[string]bool
}

func newWorkspaceGraph() *workspaceGraph {
	return &workspaceGraph{nodes: map[string]*graphNode{}}
}

func (g *workspaceGraph) addNode(dir, lang string) *graphNode {
	n, ok := g.nodes[dir]
	if !ok {
		n = &graphNode{dir: dir, langs: map[string]bool{}, deps: map[string]bool{}}
		g.nodes[dir] = n
	}
	if lang != "" {
		n.langs[lang] = true
	}
	return n
}

// addEdge records that from imports to. Self-edges and edges to directories
// that are not nodes are dropped: an import of a third-party package is not
// part of the workspace's order.
func (g *workspaceGraph) addEdge(from, to string) {
	if from == to {
		return
	}
	src, ok := g.nodes[from]
	if !ok {
		return
	}
	if _, ok := g.nodes[to]; !ok {
		return
	}
	src.deps[to] = true
}

// collapse folds every strongly connected component into one SubsystemNode
// (Tarjan's algorithm) and returns the component DAG.
func (g *workspaceGraph) collapse() []SubsystemNode {
	dirs := make([]string, 0, len(g.nodes))
	for d := range g.nodes {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)

	index := map[string]int{}
	low := map[string]int{}
	onStack := map[string]bool{}
	var stack []string
	var comps [][]string
	next := 0
	var strong func(v string)
	strong = func(v string) {
		index[v], low[v] = next, next
		next++
		stack = append(stack, v)
		onStack[v] = true
		deps := sortedKeys(g.nodes[v].deps)
		for _, w := range deps {
			if _, seen := index[w]; !seen {
				strong(w)
				low[v] = min(low[v], low[w])
			} else if onStack[w] {
				low[v] = min(low[v], index[w])
			}
		}
		if low[v] == index[v] {
			var comp []string
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				comp = append(comp, w)
				if w == v {
					break
				}
			}
			sort.Strings(comp)
			comps = append(comps, comp)
		}
	}
	for _, d := range dirs {
		if _, seen := index[d]; !seen {
			strong(d)
		}
	}

	compOf := map[string]string{}
	for _, comp := range comps {
		id := strings.Join(comp, "+")
		for _, d := range comp {
			compOf[d] = id
		}
	}
	out := make([]SubsystemNode, 0, len(comps))
	for _, comp := range comps {
		id := strings.Join(comp, "+")
		langs := map[string]bool{}
		deps := map[string]bool{}
		for _, d := range comp {
			for l := range g.nodes[d].langs {
				langs[l] = true
			}
			for dep := range g.nodes[d].deps {
				if c := compOf[dep]; c != id {
					deps[c] = true
				}
			}
		}
		title := id
		if ls := sortedKeys(langs); len(ls) > 0 {
			title = fmt.Sprintf("%s (%s)", id, strings.Join(ls, ", "))
		}
		out = append(out, SubsystemNode{
			ID:        id,
			Title:     title,
			Paths:     append([]string(nil), comp...),
			DependsOn: sortedKeys(deps),
			Languages: sortedKeys(langs),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// appendCrossCutting closes the DAG: wiring after every node nothing else
// depends on (the entry points), then review, then benchmarks.
func appendCrossCutting(nodes []SubsystemNode) []SubsystemNode {
	depended := map[string]bool{}
	for _, n := range nodes {
		for _, d := range n.DependsOn {
			depended[d] = true
		}
	}
	var tops []string
	for _, n := range nodes {
		if !depended[n.ID] {
			tops = append(tops, n.ID)
		}
	}
	sort.Strings(tops)
	return append(nodes,
		SubsystemNode{ID: RecurseWiringNodeID, Title: "Cross-subsystem wiring", DependsOn: tops, CrossCutting: true},
		SubsystemNode{ID: RecurseReviewNodeID, Title: "Architectural review", DependsOn: []string{RecurseWiringNodeID}, CrossCutting: true},
		SubsystemNode{ID: RecurseBenchNodeID, Title: "Benchmarks and test creation", DependsOn: []string{RecurseReviewNodeID}, CrossCutting: true},
	)
}

// ---------------------------------------------------------------------------
// Walking
// ---------------------------------------------------------------------------

// collectSourceFiles walks the workspace once and groups the files the
// scanners read: ".py", ".js" (every JS/TS flavour) and "Cargo.toml".
// Hidden directories are skipped with the rest.
func collectSourceFiles(root string) (map[string][]string, error) {
	out := map[string][]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == root {
				return err
			}
			// An unreadable subtree is not the workspace's order; skip it.
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if p != root && gates.SkipDir(name) {
				return filepath.SkipDir
			}
			return nil
		}
		switch {
		case name == "Cargo.toml":
			out["Cargo.toml"] = append(out["Cargo.toml"], p)
		case strings.HasSuffix(name, ".py"):
			out[".py"] = append(out[".py"], p)
		case isJSSource(name):
			out[".js"] = append(out[".js"], p)
		}
		return nil
	})
	return out, err
}

var jsExts = []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts"}

func isJSSource(name string) bool {
	if strings.HasSuffix(name, ".d.ts") {
		return false
	}
	for _, e := range jsExts {
		if strings.HasSuffix(name, e) {
			return true
		}
	}
	return false
}

func relDir(root, file string) string {
	rel, err := filepath.Rel(root, filepath.Dir(file))
	if err != nil {
		return "."
	}
	return filepath.ToSlash(rel)
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// ---------------------------------------------------------------------------
// Go
// ---------------------------------------------------------------------------

// goListPackage is the slice of `go list -json` recurse reads.
type goListPackage struct {
	Dir        string   `json:"Dir"`
	ImportPath string   `json:"ImportPath"`
	Imports    []string `json:"Imports"`
}

// runGoList is `go list -e -json ./...` in root under the build environment.
// A variable so tests can substitute a canned listing.
var runGoList = func(ctx context.Context, root string) ([]byte, error) {
	env, argv := build.GoInvocation(root, root, []string{"list", "-e", "-json", "./..."})
	cmd := exec.CommandContext(ctx, "go", argv...)
	cmd.Dir = root
	cmd.Env = env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// scanGoPackages adds one node per package in the module and an edge per
// import between them.
func scanGoPackages(ctx context.Context, root string, g *workspaceGraph) error {
	out, err := runGoList(ctx, root)
	if err != nil {
		return fmt.Errorf("recurse DAG: %w", err)
	}
	var pkgs []goListPackage
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var p goListPackage
		if err := dec.Decode(&p); err != nil {
			return fmt.Errorf("recurse DAG: decode go list: %w", err)
		}
		if p.Dir == "" || p.ImportPath == "" {
			continue
		}
		pkgs = append(pkgs, p)
	}
	dirOf := map[string]string{}
	for _, p := range pkgs {
		dir := p.Dir
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			dir = resolved
		}
		rel, err := filepath.Rel(root, dir)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		dirOf[p.ImportPath] = filepath.ToSlash(rel)
		g.addNode(filepath.ToSlash(rel), "go")
	}
	for _, p := range pkgs {
		from, ok := dirOf[p.ImportPath]
		if !ok {
			continue
		}
		for _, imp := range p.Imports {
			if to, ok := dirOf[imp]; ok {
				g.addEdge(from, to)
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Python
// ---------------------------------------------------------------------------

var (
	pyFromImport = regexp.MustCompile(`^\s*from\s+(\.*)([A-Za-z_][\w.]*)?\s+import\b`)
	pyImport     = regexp.MustCompile(`^\s*import\s+([A-Za-z_][\w.]*(?:\s*,\s*[A-Za-z_][\w.]*)*)`)
)

// scanPythonImports adds a node per directory holding Python files and an
// edge per import that resolves to another of those directories.
func scanPythonImports(root string, files []string, g *workspaceGraph) {
	if len(files) == 0 {
		return
	}
	pyDirs := map[string]bool{}
	for _, f := range files {
		dir := relDir(root, f)
		pyDirs[dir] = true
		g.addNode(dir, "python")
	}
	for _, f := range files {
		from := relDir(root, f)
		for _, target := range pythonImportTargets(f) {
			if to, ok := resolvePythonModule(from, target, pyDirs); ok {
				g.addEdge(from, to)
			}
		}
	}
}

// pyImportTarget is one imported module: Up is the number of leading dots (0
// for an absolute import), Module the dotted remainder.
type pyImportTarget struct {
	Up     int
	Module string
}

func pythonImportTargets(file string) []pyImportTarget {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var out []pyImportTarget
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if m := pyFromImport.FindStringSubmatch(line); m != nil {
			out = append(out, pyImportTarget{Up: len(m[1]), Module: m[2]})
			continue
		}
		if m := pyImport.FindStringSubmatch(line); m != nil {
			for _, mod := range strings.Split(m[1], ",") {
				out = append(out, pyImportTarget{Module: strings.TrimSpace(mod)})
			}
		}
	}
	return out
}

// resolvePythonModule maps an import to the deepest workspace directory it
// names: "pkg.core.models" resolves to pkg/core/models if that directory holds
// Python, else pkg/core (models is a module or a symbol in it). Absolute
// imports are tried from the root and from src/.
func resolvePythonModule(fromDir string, t pyImportTarget, pyDirs map[string]bool) (string, bool) {
	var parts []string
	if t.Module != "" {
		parts = strings.Split(t.Module, ".")
	}
	var bases []string
	if t.Up > 0 {
		base := fromDir
		for i := 1; i < t.Up; i++ {
			base = path.Dir(base)
		}
		bases = []string{base}
	} else {
		bases = []string{".", "src"}
	}
	for _, base := range bases {
		for k := len(parts); k >= 0; k-- {
			if k == 0 && t.Up == 0 {
				break // an absolute import naming nothing below the base is not ours
			}
			cand := path.Clean(path.Join(append([]string{base}, parts[:k]...)...))
			if pyDirs[cand] {
				return cand, true
			}
		}
	}
	return "", false
}

// ---------------------------------------------------------------------------
// JavaScript / TypeScript
// ---------------------------------------------------------------------------

var jsSpecifier = regexp.MustCompile(`(?:\bfrom\s*|\brequire\s*\(\s*|\bimport\s*\(\s*|^\s*import\s+)["'](\.{1,2}/[^"']*)["']`)

// scanJSImports adds a node per directory holding JS/TS sources and an edge
// per relative import that resolves to a file in another directory.
// Bare specifiers ("react") are dependencies, not the workspace's order.
func scanJSImports(root string, files []string, g *workspaceGraph) {
	for _, f := range files {
		g.addNode(relDir(root, f), "js/ts")
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		from := relDir(root, f)
		for _, line := range strings.Split(string(data), "\n") {
			for _, m := range jsSpecifier.FindAllStringSubmatch(line, -1) {
				if target, ok := resolveJSSpecifier(filepath.Dir(f), m[1]); ok {
					g.addEdge(from, relDir(root, target))
				}
			}
		}
	}
}

// resolveJSSpecifier resolves a relative specifier the way bundlers do: the
// path itself, the path plus a source extension, or an index file inside it.
func resolveJSSpecifier(fromDir, spec string) (string, bool) {
	p := filepath.Join(fromDir, filepath.FromSlash(spec))
	if fileExists(p) {
		return p, true
	}
	for _, e := range jsExts {
		if fileExists(p + e) {
			return p + e, true
		}
	}
	// "./lib.js" written for a lib.ts source (TypeScript's ESM convention).
	if ext := filepath.Ext(p); ext != "" {
		stem := strings.TrimSuffix(p, ext)
		for _, e := range jsExts {
			if fileExists(stem + e) {
				return stem + e, true
			}
		}
	}
	if dirExists(p) {
		for _, e := range jsExts {
			idx := filepath.Join(p, "index"+e)
			if fileExists(idx) {
				return idx, true
			}
		}
	}
	return "", false
}

// ---------------------------------------------------------------------------
// Rust
// ---------------------------------------------------------------------------

var (
	cargoPackage  = regexp.MustCompile(`(?m)^\s*\[package\]`)
	cargoPathDeps = regexp.MustCompile(`path\s*=\s*"([^"]+)"`)
)

// scanRustCrates adds one node per crate (a Cargo.toml with [package]) and an
// edge per path dependency on another crate in the workspace.
func scanRustCrates(root string, manifests []string, g *workspaceGraph) {
	crates := map[string]string{} // manifest dir (abs) -> node dir
	for _, m := range manifests {
		data, err := os.ReadFile(m)
		if err != nil || !cargoPackage.Match(data) {
			continue
		}
		dir := relDir(root, m)
		crates[filepath.Dir(m)] = dir
		g.addNode(dir, "rust")
	}
	for _, m := range manifests {
		from, ok := crates[filepath.Dir(m)]
		if !ok {
			continue
		}
		data, err := os.ReadFile(m)
		if err != nil {
			continue
		}
		for _, dep := range cargoPathDeps.FindAllSubmatch(data, -1) {
			target := filepath.Clean(filepath.Join(filepath.Dir(m), filepath.FromSlash(string(dep[1]))))
			if to, ok := crates[target]; ok {
				g.addEdge(from, to)
			}
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
