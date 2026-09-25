package retrieval

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func relAll(t *testing.T, root string, paths []string) []string {
	t.Helper()
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		rel, err := filepath.Rel(root, p)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, filepath.ToSlash(rel))
	}
	slices.Sort(out)
	return out
}

// Tier 3 follows TypeScript/JavaScript relative specifiers the way tsc and a
// bundler resolve them, and never a package from node_modules or a path
// outside the workspace.
func TestImportNeighbors_TypeScriptRelativeSpecifiers(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"src/app.ts": `import { render } from "./view";
import type { Model } from '../lib/model';
export * from "./widgets";
import "./polyfill.js";
const legacy = require("./legacy");
import React from "react";
import secret from "../../outside";
`,
		"src/view.tsx":          "export const render = 1\n",
		"lib/model.ts":          "export type Model = {}\n",
		"src/widgets/index.ts":  "export {}\n",
		"src/polyfill.ts":       "export {}\n",
		"src/legacy.js":         "module.exports = {}\n",
		"node_modules/react.js": "module.exports = {}\n",
		"src/unrelated.ts":      "export {}\n",
	})
	b := NewTieredContextBuilder(&TieredContextConfig{WorkDir: root})
	got := relAll(t, root, b.importNeighbors(filepath.Join(root, "src/app.ts")))
	want := []string{"lib/model.ts", "src/legacy.js", "src/polyfill.ts", "src/view.tsx", "src/widgets/index.ts"}
	if !slices.Equal(got, want) {
		t.Fatalf("TS import neighbours = %v, want %v", got, want)
	}
}

// Tier 3 follows Rust `mod` declarations and `use crate::` / `use super::`
// paths to the module files that define them, and no external crate.
func TestImportNeighbors_RustModulesAndUsePaths(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"src/lib.rs": `pub mod billing;
mod util;
use crate::billing::refund::process;
use serde::Serialize;
`,
		"src/billing.rs":        "pub mod refund;\nuse super::util::round;\n",
		"src/billing/refund.rs": "pub fn process() {}\n",
		"src/util/mod.rs":       "pub fn round() {}\n",
		"src/unrelated.rs":      "\n",
	})
	b := NewTieredContextBuilder(&TieredContextConfig{WorkDir: root})

	got := relAll(t, root, b.importNeighbors(filepath.Join(root, "src/lib.rs")))
	want := []string{"src/billing.rs", "src/billing/refund.rs", "src/util/mod.rs"}
	if !slices.Equal(got, want) {
		t.Fatalf("lib.rs import neighbours = %v, want %v", got, want)
	}
	got = relAll(t, root, b.importNeighbors(filepath.Join(root, "src/billing.rs")))
	want = []string{"src/billing/refund.rs", "src/util/mod.rs"}
	if !slices.Equal(got, want) {
		t.Fatalf("billing.rs import neighbours = %v, want %v", got, want)
	}
}

// End to end: an issue naming a TS file gets its import neighbours as Tier 3.
// Before, Tier 3 resolved Go and Python only and was empty here.
func TestBuildContext_TypeScriptIssueFillsTheImportTier(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"src/checkout.ts": "import { total } from \"./pricing\";\nexport const checkout = () => total();\n",
		"src/pricing.ts":  "export const total = () => 0;\n",
	})
	b := NewTieredContextBuilder(&TieredContextConfig{WorkDir: root})
	tc, err := b.BuildContext(context.Background(), "checkout.ts computes the wrong total")
	if err != nil {
		t.Fatal(err)
	}
	var tier3 []string
	for _, f := range tc.Files {
		if f.Tier == 3 {
			tier3 = append(tier3, f.FilePath)
		}
	}
	if got := relAll(t, root, tier3); !slices.Equal(got, []string{"src/pricing.ts"}) {
		t.Fatalf("Tier 3 = %v, want the file checkout.ts imports", got)
	}
}
