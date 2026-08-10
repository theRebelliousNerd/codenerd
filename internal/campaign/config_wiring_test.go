package campaign

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// wiringRequirement describes a field that must be set at every non-empty
// composite literal of a given type. Adding a new collaborator that is
// settable and required is one table row, not a new test.
type wiringRequirement struct {
	TypeName  string
	FieldName string
	MinSites  int
	Why       string
}

// TestConfigWiring replaces the two bespoke OrchestratorConfig call-site tests
// with a single table-driven walk. Every non-empty keyed literal of the
// required type must set the required field, or campaigns silently break.
func TestConfigWiring(t *testing.T) {
	requirements := []wiringRequirement{
		{
			TypeName:  "OrchestratorConfig",
			FieldName: "NorthstarObserver",
			MinSites:  5,
			Why:       "campaigns on protected roots refused when NorthstarObserver is nil (protectedCampaignRiskRoots in risk_scoring.go) - one campaign was refused 850 times in a day",
		},
		{
			TypeName:  "OrchestratorConfig",
			FieldName: "TaskExecutor",
			MinSites:  5,
			Why:       "checkpoints reporting PASS without verifying when TaskExecutor is nil (runShardValidationCheckpoint and runNemesisGauntletCheckpoint in checkpoint.go cannot run and returned PASS for verifications that never ran)",
		},
	}

	root := wiringRepoRoot(t)

	for _, req := range requirements {
		req := req
		t.Run(req.TypeName+"/"+req.FieldName, func(t *testing.T) {
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
					if !ok || !isWiringType(lit.Type, req.TypeName) {
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
						if id, ok := kv.Key.(*ast.Ident); ok && id.Name == req.FieldName {
							return true
						}
					}
					rel, _ := filepath.Rel(root, path)
					pos := fset.Position(lit.Pos())
					loc := filepath.ToSlash(rel) + ":" + strconv.Itoa(pos.Line)
					missing = append(missing, loc)
					return true
				})
				return nil
			})
			if err != nil {
				t.Fatalf("walking repo: %v", err)
			}

			if checked < req.MinSites {
				t.Fatalf("found only %d %s literals; expected at least %d. If construction moved, update this test rather than deleting it (Why: %s)", checked, req.TypeName, req.MinSites, req.Why)
			}

			if len(missing) > 0 {
				t.Errorf("%s built without %s at %d site(s):\n  %s\nWhy: %s\n%s.%s must be set at every non-empty %s literal; see Why above.",
					req.TypeName, req.FieldName, len(missing), strings.Join(missing, "\n  "), req.Why, req.TypeName, req.FieldName, req.TypeName)
			}
		})
	}
}

// isWiringType matches both `TypeName{...}` and `pkg.TypeName{...}` so the
// walk works from inside and outside the defining package.
func isWiringType(expr ast.Expr, typeName string) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name == typeName
	case *ast.SelectorExpr:
		return t.Sel.Name == typeName
	}
	return false
}

// wiringRepoRoot walks up from the test's working directory until it finds go.mod.
func wiringRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate go.mod above the test's working directory")
		}
		dir = parent
	}
}
