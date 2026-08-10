package campaign

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every OrchestratorConfig literal in the repository must set TaskExecutor.
//
// This is a source-level test on purpose. The defect it guards is a field
// MISSING from a composite literal, and a field that is not written has no
// runtime behaviour to observe from inside this package — the literals live in
// package chat, package system and internal/campaign tests, inside Bubble Tea
// closures and a supervision loop that cannot be constructed in a unit test.
//
// The defect: TaskExecutor was not set at several construction sites.
// With TaskExecutor nil, runShardValidationCheckpoint and
// runNemesisGauntletCheckpoint in checkpoint.go cannot run, and before
// commit 20a90c79 they returned PASS for verifications that never ran —
// the single most dangerous answer a verification gate can give, because
// "we did not check" became indistinguishable from "we checked and it was fine".
//
// Every other site must set TaskExecutor.
func TestOrchestratorTaskExecutorCallsite(t *testing.T) {
	root := repoRoot(t)

	var checked int
	var missing []string

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".nerd", "node_modules", "worktrees":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil
		}

		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok || !isOrchestratorConfigType(lit.Type) {
				return true
			}
			if len(lit.Elts) == 0 {
				return true
			}
			checked++
			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if id, ok := kv.Key.(*ast.Ident); ok && id.Name == "TaskExecutor" {
					return true
				}
			}
			rel, _ := filepath.Rel(root, path)
			missing = append(missing, filepath.ToSlash(rel)+":"+
				fset.Position(lit.Pos()).String()[len(fset.Position(lit.Pos()).Filename)+1:])
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking repo: %v", err)
	}

	if checked < 5 {
		t.Fatalf("found only %d OrchestratorConfig literals; expected at least 5. "+
			"If construction moved, update this test rather than deleting it", checked)
	}

	if len(missing) > 0 {
		t.Errorf("OrchestratorConfig built without TaskExecutor at %d site(s):\n  %s\n"+
			"With TaskExecutor nil, runShardValidationCheckpoint and runNemesisGauntletCheckpoint in checkpoint.go cannot run, "+
			"and before commit 20a90c79 they returned PASS for verifications that never ran. "+
			"Construct the orchestrator with OrchestratorConfig.TaskExecutor.",
			len(missing), strings.Join(missing, "\n  "))
	}
}
