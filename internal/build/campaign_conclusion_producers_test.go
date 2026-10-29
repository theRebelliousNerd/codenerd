package build

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

// The campaign scheduler's conclusions are derived, never stored by a side
// producer. current_campaign, current_phase and next_campaign_task are rules
// over the campaign owner's canonical facts (policy/campaign_core.mg,
// campaign_phases.mg). The chat used to assert them on every turn as stored
// facts that never retired, and the VirtualStore's campaign workflow handlers
// asserted current_phase("/verify") and the like; a stale current_phase made
// getCurrentPhase return a completed phase, so the scheduler ran it again and
// counted its completion twice (audit
// campaign-context-seed-scheduler-ownership). This guard fails on any
// production Go file that builds a fact with one of those predicates.
func TestCampaignSchedulerConclusionsHaveNoGoProducer(t *testing.T) {
	derived := map[string]bool{"current_campaign": true, "current_phase": true, "next_campaign_task": true}
	root := repoRoot(t)
	var offenders []string
	for _, top := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if name := d.Name(); name == "testdata" || strings.HasPrefix(name, ".") {
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
				kv, ok := n.(*ast.KeyValueExpr)
				if !ok {
					return true
				}
				key, ok := kv.Key.(*ast.Ident)
				lit, isLit := kv.Value.(*ast.BasicLit)
				if !ok || key.Name != "Predicate" || !isLit || lit.Kind != token.STRING {
					return true
				}
				if name, err := strconv.Unquote(lit.Value); err == nil && derived[name] {
					rel, _ := filepath.Rel(root, path)
					offenders = append(offenders, filepath.ToSlash(rel)+":"+strconv.Itoa(fset.Position(lit.Pos()).Line)+" "+name)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("production Go stores derived campaign conclusions (the campaign owner's canonical facts and policy produce them):\n  %s", strings.Join(offenders, "\n  "))
	}
}
