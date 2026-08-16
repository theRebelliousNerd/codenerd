package logging

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// categoryInventory is every logging Category that may exist, with the
// subsystem that owns it. Adding a category means adding a row here, which
// is the point: the taxonomy grows by decision rather than by drift.
//
// Q7 asked to cap proliferation. This is the cap. It deliberately does NOT
// answer the other half of Q7 - whether the taxonomy should become
// hierarchical (shard.coder) instead of flat - which is a redesign, not a
// guard.
var categoryInventory = map[string]string{
	"api":            "LLM API calls",
	"articulation":   "Atoms -> NL transduction (Piggyback)",
	"autopoiesis":    "Self-learning, Ouroboros",
	"boot":           "Boot/initialization",
	"browser":        "Browser automation, DOM events",
	"build":          "Build environment and compilation",
	"campaign":       "Campaign orchestration",
	"coder":          "Coder shard activity",
	"context":        "Context compression",
	"dream":          "Dream state / what-if simulations",
	"embedding":      "Embedding engine",
	"jit":            "JIT Prompt Compiler operations",
	"kernel":         "Mangle kernel operations",
	"northstar":      "Northstar vision guardian",
	"perception":     "NL -> atoms transduction",
	"performance":    "Performance metrics, slow operations",
	"persist":        "Fact snapshot write/read",
	"regression":     "Regression battery runs",
	"researcher":     "Researcher shard activity",
	"reviewer":       "Reviewer shard activity",
	"routing":        "Action routing decisions",
	"session":        "Session management, persistence",
	"shards":         "Shard spawning and lifecycle",
	"store":          "Store operations (RAM, Vector, Graph, Cold)",
	"system_shards":  "System shards (legislator, etc.)",
	"tactile":        "Tactile executor, command execution",
	"tester":         "Tester shard activity",
	"tools":          "Tool execution",
	"virtual_store":  "Virtual store operations",
	"world":          "World scanner (filesystem, AST)",
}

// categoryCap is the ceiling for the number of logging Categories that may
// exist at the time of writing (Q7). Raising it is allowed but must be a
// deliberate edit to this constant with a reason in the commit message; the
// alternative to raising it is folding the new subsystem into an existing
// category rather than widening the taxonomy.
const categoryCap = 30

func TestCategoryInventory_WhenCategoryAdded_ShouldBeDeclared(t *testing.T) {
	discovered := discoveredCategories(t)

	var missing []string
	for cat := range discovered {
		if _, ok := categoryInventory[cat]; !ok {
			missing = append(missing, strconv.Quote(cat))
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("new logging Category constant(s) discovered in internal/logging/logger.go with no row in categoryInventory: %s\nAdd a row to categoryInventory in internal/logging/category_inventory_test.go with the owning subsystem/purpose (this inventory is the cap from Q7 — taxonomy grows by decision, not drift).", strings.Join(missing, ", "))
	}

	var stale []string
	for cat, reason := range categoryInventory {
		if _, ok := discovered[cat]; !ok {
			stale = append(stale, strconv.Quote(cat)+" ("+reason+")")
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("stale categoryInventory row(s) with no matching Category constant in internal/logging/logger.go: %s\nRemove the stale row(s) from categoryInventory in internal/logging/category_inventory_test.go (a deleted Category must clean its inventory entry).", strings.Join(stale, ", "))
	}
}

func TestCategoryInventory_ShouldNotExceedTheCap(t *testing.T) {
	discovered := discoveredCategories(t)
	count := len(discovered)
	if count > categoryCap {
		t.Errorf("logging Category count %d exceeds cap %d (discovered from internal/logging/logger.go)\nEither raise categoryCap in internal/logging/category_inventory_test.go with a reason in the commit message, or fold the new subsystem into an existing category (Q7 cap: prefer reuse over proliferation).", count, categoryCap)
	}
}

func discoveredCategories(t *testing.T) map[string]bool {
	t.Helper()
	root := categoryInventoryRepoRoot(t)
	path := filepath.Join(root, "internal", "logging", "logger.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	out := map[string]bool{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			if vs.Type == nil {
				continue
			}
			ident, ok := vs.Type.(*ast.Ident)
			if !ok || ident.Name != "Category" {
				continue
			}
			for _, v := range vs.Values {
				lit, ok := v.(*ast.BasicLit)
				if !ok {
					continue
				}
				if lit.Kind != token.STRING {
					continue
				}
				val, _ := strconv.Unquote(lit.Value)
				out[val] = true
			}
		}
	}
	if len(out) == 0 {
		t.Fatal("discovered no Category constants in internal/logging/logger.go — the scanner has drifted from the source")
	}
	return out
}

func categoryInventoryRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate the module root from the test working directory")
		}
		dir = parent
	}
}
