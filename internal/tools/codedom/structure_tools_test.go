package codedom_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools"
	"codenerd/internal/tools/codedom"
	"codenerd/internal/world"
)

// structWorkspace writes files under a temp root, registers the real
// structure index over it, and returns a registry holding the codedom tools
// with that root as its workspace.
func structWorkspace(t *testing.T, files map[string]string) (*tools.Registry, string) {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	codedom.RegisterStructureProvider(world.NewStructureIndex(root).Provider())
	t.Cleanup(func() { codedom.RegisterStructureProvider(nil) })
	reg := tools.NewRegistry()
	if err := codedom.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	reg.SetWorkspaceRoot(root)
	return reg, root
}

func run(t *testing.T, reg *tools.Registry, name string, args map[string]any) string {
	t.Helper()
	res, err := reg.Execute(context.Background(), name, args)
	if err != nil {
		t.Fatalf("%s %v: %v", name, args, err)
	}
	return res.Result
}

var queryFixture = map[string]string{
	"go.mod": "module fixture\n\ngo 1.24\n",
	"internal/store/local_vector.go": `package store

import "log"

// Recall looks things up.
func Recall(q string) error {
	if q == "" {
		log.Printf("Vector recall query failed: %s", q)
	}
	return nil
}
`,
	"internal/features/features.go":  "package features\n\n// On reports the flag.\nfunc On() bool { return true }\n\nfunc TestLike() {}\n",
	"cmd/nerd/cmd_features.go":       "package main\n\nimport \"fixture/internal/features\"\n\nfunc run() bool { return features.On() }\n",
	"internal/core/policy/impact.mg": "Decl modified_function(F, File) bound [/string, /string].\n\nimpact(C) :- modified_function(F, _), calls(C, F).\n",
}

// The audit's failing tests for G5: the three importer greps and the quoted
// log message had no structural verb.
func TestImportersOf_ListsTheImportingFiles(t *testing.T) {
	reg, _ := structWorkspace(t, queryFixture)
	out := run(t, reg, "importers_of", map[string]any{"package": "internal/features"})
	if !strings.Contains(out, `cmd/nerd/cmd_features.go:3  imports "fixture/internal/features" as features`) {
		t.Fatalf("importers_of:\n%s", out)
	}
}

func TestFindText_AnswersWithTheElementThatLogsIt(t *testing.T) {
	reg, _ := structWorkspace(t, queryFixture)
	out := run(t, reg, "find_text", map[string]any{"text": "Vector recall query failed"})
	if !strings.Contains(out, "internal/store/local_vector.go:8  internal/store.Recall  in string") {
		t.Fatalf("find_text:\n%s", out)
	}
	none := run(t, reg, "find_text", map[string]any{"text": "not anywhere"})
	if !tools.StructuralMissed(none) {
		t.Fatalf("an empty answer must carry the miss marker:\n%s", none)
	}
}

func TestFindSymbol_SeveralNamesPatternKindAndPath(t *testing.T) {
	reg, _ := structWorkspace(t, queryFixture)
	out := run(t, reg, "find_symbol", map[string]any{"name": "On|Recall"})
	if !strings.Contains(out, "internal/features.On") || !strings.Contains(out, "internal/store.Recall") {
		t.Fatalf("several names:\n%s", out)
	}
	out = run(t, reg, "find_symbol", map[string]any{"pattern": "^(On|TestLike)$", "path": "internal/features", "kind": "function"})
	if !strings.Contains(out, "-- 2 rows, complete") {
		t.Fatalf("pattern within a path:\n%s", out)
	}
}

func TestPredicateOutline_DeclaresDerivesReads(t *testing.T) {
	reg, _ := structWorkspace(t, queryFixture)
	out := run(t, reg, "predicate_outline", map[string]any{"predicate": "modified_function/2"})
	if !strings.Contains(out, "declares  internal/core/policy/impact.mg:1-1  decl  internal/core/policy/impact.mg:decl:modified_function/2") ||
		!strings.Contains(out, "reads  internal/core/policy/impact.mg:3-3  rule") {
		t.Fatalf("predicate_outline:\n%s", out)
	}
}
