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

// Every OrchestratorConfig literal in the repository must set NorthstarObserver.
//
// This is a source-level test on purpose. The defect it guards is a field
// MISSING from a composite literal, and a field that is not written has no
// runtime behaviour to observe from inside this package — the literals live in
// package chat and package system, inside Bubble Tea closures and a supervision
// loop that cannot be constructed in a unit test.
//
// The defect: NorthstarObserver was set at 2 of 5 construction sites.
// risk_scoring.go refuses to start any campaign whose targets touch a protected
// root (protectedCampaignRiskRoots) when configuredNorthstarObserver is nil, so
// every in-chat campaign against internal/core, internal/mangle,
// internal/campaign, internal/perception or internal/articulation was
// permanently blocked. Measured live 2026-08-08: one campaign was refused 850
// times in a day, retried every 5 seconds.
//
// northstar_wiring_test.go already covered wireIntelligenceComponents and
// passed throughout, because the wiring function was never the broken part. The
// call sites were. That is why this test reads the call sites.
func TestOrchestratorConfigLiterals_AllSetNorthstarObserver(t *testing.T) {
	root := repoRoot(t)

	var checked int
	var missing []string

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Worktrees hold full copies of the repo; scanning them would
			// report another branch's state as this one's.
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
			// A file this package cannot parse is not this test's business.
			return nil
		}

		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok || !isOrchestratorConfigType(lit.Type) {
				return true
			}
			// An empty literal is a zero value used as a return placeholder
			// (e.g. `return OrchestratorConfig{}, err`), not a construction.
			if len(lit.Elts) == 0 {
				return true
			}
			checked++
			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if id, ok := kv.Key.(*ast.Ident); ok && id.Name == "NorthstarObserver" {
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

	// If the literals move or get renamed, silently checking nothing would let
	// this test "pass" forever.
	// Four since 2026-09-04: nerd campaign start and resume build their config
	// through one function (cmd/nerd/cmd_campaign.go buildCampaignOrchestratorConfig).
	if checked < 4 {
		t.Fatalf("found only %d OrchestratorConfig literals; expected at least 4. "+
			"If construction moved, update this test rather than deleting it", checked)
	}

	if len(missing) > 0 {
		t.Errorf("OrchestratorConfig built without NorthstarObserver at %d site(s):\n  %s\n"+
			"Campaigns from these sites are refused outright when their targets touch a "+
			"protected root (see protectedCampaignRiskRoots in risk_scoring.go). Use "+
			"northstar.BuildCampaignObserver(workspace, llmClient, kernel).",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// isOrchestratorConfigType matches both `OrchestratorConfig{...}` (inside this
// package) and `campaign.OrchestratorConfig{...}` (outside it).
func isOrchestratorConfigType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name == "OrchestratorConfig"
	case *ast.SelectorExpr:
		return t.Sel.Name == "OrchestratorConfig"
	}
	return false
}

// repoRoot walks up from this package until it finds the go.mod.
func repoRoot(t *testing.T) string {
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
