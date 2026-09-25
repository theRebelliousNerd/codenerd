package campaign

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// writeTree writes files (slash paths relative to root) with the given contents.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// sweepOrder derives the workspace DAG, orders it, and returns the node IDs.
func sweepOrder(t *testing.T, root string) []string {
	t.Helper()
	nodes, err := DeriveWorkspaceDAG(context.Background(), root)
	if err != nil {
		t.Fatalf("DeriveWorkspaceDAG: %v", err)
	}
	ordered, err := TopoOrder(nodes)
	if err != nil {
		t.Fatalf("TopoOrder: %v", err)
	}
	ids := make([]string, len(ordered))
	for i, n := range ordered {
		ids[i] = n.ID
	}
	return ids
}

// before reports whether a precedes b in ids.
func before(ids []string, a, b string) bool {
	ia, ib := slices.Index(ids, a), slices.Index(ids, b)
	return ia >= 0 && ib >= 0 && ia < ib
}

func assertClose(t *testing.T, ids []string) {
	t.Helper()
	n := len(ids)
	if n < 3 || ids[n-3] != RecurseWiringNodeID || ids[n-2] != RecurseReviewNodeID || ids[n-1] != RecurseBenchNodeID {
		t.Fatalf("a pass must close with wiring, review, bench: %v", ids)
	}
}

// The order is the code's own: a Go package sweeps after every package it
// imports, and the entry point last. This is what recurse did not know when
// its DAG was a table of codeNERD's packages.
func TestDeriveWorkspaceDAG_GoModuleSweepsLeavesFirst(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain")
	}
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"go.mod":              "module example.com/m\n\ngo 1.21\n",
		"store/store.go":      "package store\n\nfunc Get() int { return 1 }\n",
		"service/service.go":  "package service\n\nimport \"example.com/m/store\"\n\nfunc Run() int { return store.Get() }\n",
		"cmd/app/main.go":     "package main\n\nimport \"example.com/m/service\"\n\nfunc main() { _ = service.Run() }\n",
		"internal/util/u.go":  "package util\n\nfunc U() {}\n",
		"testdata/skip/x.go":  "package skip\n",
		"node_modules/n/a.js": "module.exports = 1\n",
	})
	ids := sweepOrder(t, root)
	if !before(ids, "store", "service") || !before(ids, "service", "cmd/app") {
		t.Fatalf("store must sweep before service, service before cmd/app: %v", ids)
	}
	if !slices.Contains(ids, "internal/util") {
		t.Fatalf("an unimported package is still a node: %v", ids)
	}
	for _, id := range ids {
		if id == "testdata/skip" || id == "node_modules/n" {
			t.Fatalf("fixtures and dependencies are not the workspace's order: %v", ids)
		}
	}
	assertClose(t, ids)
}

// Python: absolute imports (from the root and from src/) and relative ones
// resolve to the workspace's own package directories; third-party imports do
// not add edges.
func TestDeriveWorkspaceDAG_PythonImportsOrderThePackages(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"src/shop/__init__.py":        "",
		"src/shop/models/__init__.py": "",
		"src/shop/models/item.py":     "import requests\n\nclass Item: pass\n",
		"src/shop/api/views.py":       "from shop.models.item import Item\nfrom ..models import item\n",
		"src/shop/cli.py":             "from .api import views\nimport os, sys\n",
	})
	ids := sweepOrder(t, root)
	if !before(ids, "src/shop/models", "src/shop/api") {
		t.Fatalf("models must sweep before the api that imports it: %v", ids)
	}
	if !before(ids, "src/shop/api", "src/shop") {
		t.Fatalf("api must sweep before the cli module that imports it: %v", ids)
	}
	assertClose(t, ids)
}

// JavaScript/TypeScript: relative specifiers resolve to files, extensionless
// paths, index files and the .js-for-.ts ESM convention; bare specifiers are
// dependencies.
func TestDeriveWorkspaceDAG_TypeScriptRelativeImportsOrderTheDirectories(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"src/lib/util.ts":        "export const x = 1\n",
		"src/lib/fmt/index.ts":   "export const f = 1\n",
		"src/core/engine.ts":     "import { x } from '../lib/util'\nimport { f } from '../lib/fmt'\nimport React from 'react'\n",
		"src/app/main.tsx":       "import { run } from '../core/engine.js'\nconst y = require('../lib/util')\n",
		"src/types/global.d.ts":  "declare const z: number\n",
		"dist/bundle/output.js":  "import '../../src/app/main'\n",
		"src/app/main.test.tsx":  "import '../app/main'\n",
		"src/lib/fmt/helpers.ts": "export {}\n",
	})
	ids := sweepOrder(t, root)
	if !before(ids, "src/lib", "src/core") || !before(ids, "src/lib/fmt", "src/core") || !before(ids, "src/core", "src/app") {
		t.Fatalf("lib and lib/fmt before core, core before app: %v", ids)
	}
	for _, id := range ids {
		if id == "dist/bundle" || id == "src/types" {
			t.Fatalf("build output and declaration-only dirs are not nodes: %v", ids)
		}
	}
	assertClose(t, ids)
}

// Rust: one node per crate, ordered by path dependencies.
func TestDeriveWorkspaceDAG_RustCratesOrderByPathDependencies(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"Cargo.toml":              "[workspace]\nmembers = [\"crates/*\"]\n",
		"crates/core/Cargo.toml":  "[package]\nname = \"core\"\n",
		"crates/core/src/lib.rs":  "pub fn f() {}\n",
		"crates/cli/Cargo.toml":   "[package]\nname = \"cli\"\n\n[dependencies]\ncore = { path = \"../core\" }\nserde = \"1\"\n",
		"crates/cli/src/main.rs":  "fn main() {}\n",
		"target/debug/Cargo.toml": "[package]\nname = \"junk\"\n",
	})
	ids := sweepOrder(t, root)
	if !before(ids, "crates/core", "crates/cli") {
		t.Fatalf("core must sweep before the cli crate that depends on it: %v", ids)
	}
	if slices.Contains(ids, "target/debug") || slices.Contains(ids, ".") {
		t.Fatalf("build output and a virtual workspace manifest are not crates: %v", ids)
	}
	assertClose(t, ids)
}

// A cycle (legal in Python) collapses into one node rather than failing the
// order or dropping an edge.
func TestDeriveWorkspaceDAG_AnImportCycleBecomesOneNode(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"a/__init__.py": "from b import thing\n",
		"b/__init__.py": "from a import other\n",
		"c/__init__.py": "import a\n",
	})
	ids := sweepOrder(t, root)
	if !slices.Contains(ids, "a+b") {
		t.Fatalf("a and b import each other and must be one node: %v", ids)
	}
	if !before(ids, "a+b", "c") {
		t.Fatalf("c imports the cycle and must sweep after it: %v", ids)
	}
}

// A workspace with nothing the scanners read is swept as one node, not
// refused: recurse still has somewhere to start.
func TestDeriveWorkspaceDAG_AnUnreadableWorkspaceIsOneNode(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"README.md": "# hi\n"})
	ids := sweepOrder(t, root)
	if ids[0] != "." {
		t.Fatalf("want the whole workspace as one node first, got %v", ids)
	}
	assertClose(t, ids)
}

// The sweep is the workspace's own: a Python workspace's order names its
// packages, not codeNERD's.
func TestRecurseSweepOrder_IsTheWorkspacesOwn(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"pkg/core/__init__.py": "",
		"pkg/web/app.py":       "from pkg.core import x\n",
	})
	nodes, err := RecurseSweepOrder(context.Background(), root, nil)
	if err != nil {
		t.Fatalf("RecurseSweepOrder: %v", err)
	}
	var ids []string
	for _, n := range nodes {
		ids = append(ids, n.ID)
	}
	if !before(ids, "pkg/core", "pkg/web") {
		t.Fatalf("pkg/core sweeps before pkg/web, which imports it: %v", ids)
	}
	if slices.ContainsFunc(ids, func(id string) bool { return strings.HasPrefix(id, "internal/") }) {
		t.Fatalf("a Python workspace's sweep must not name codeNERD's packages: %v", ids)
	}
}
