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

// Every door a campaign starts through builds its orchestrator from the same
// policy: the campaign section of the user's config. Before the section
// existed, chat /assault and /recurse hard-coded one task at a time with
// auto-replan and checkpoint-on-fail, while `nerd campaign start` and chat
// /campaign set none of it (live log: "Orchestrator config: maxParallel=0,
// checkpointOnFail=false, autoReplan=false"), so the same campaign behaved
// differently depending on who started it.
//
// OrchestratorConfig now carries the policy in one field, Campaign. This scans
// every production construction site and requires that field to come from
// UserConfig.GetCampaignConfig: a literal that omits it runs on defaults the
// user never wrote, and one that fills it any other way runs on a policy the
// config file cannot reach.
func TestEveryEntryPointTakesThePolicyFromTheUserConfig(t *testing.T) {
	root := filepath.Join("..", "..")
	var sites []string
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" || d.Name() == ".nerd" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !strings.Contains(string(src), "OrchestratorConfig") {
				return nil
			}
			f, err := parser.ParseFile(token.NewFileSet(), path, src, 0)
			if err != nil {
				return err
			}
			for _, fn := range f.Decls {
				fd, ok := fn.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				sites = append(sites, checkPolicySites(t, path, fd)...)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
	// start/resume/recurse (CLI builder), chat campaign/assault/recurse, and
	// the campaign runner shard.
	if len(sites) < 5 {
		t.Fatalf("found %d orchestrator construction sites (%v); the scan no longer sees them", len(sites), sites)
	}
}

// checkPolicySites reports each OrchestratorConfig built in fd and fails the
// test for one whose Campaign field is not UserConfig.GetCampaignConfig().
func checkPolicySites(t *testing.T, path string, fd *ast.FuncDecl) []string {
	t.Helper()
	isOrchConfig := func(e ast.Expr) bool {
		switch x := e.(type) {
		case *ast.SelectorExpr:
			return x.Sel.Name == "OrchestratorConfig"
		case *ast.Ident:
			return x.Name == "OrchestratorConfig"
		}
		return false
	}
	fromUserConfig := func(e ast.Expr) bool {
		call, ok := e.(*ast.CallExpr)
		if !ok {
			return false
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		return ok && sel.Sel.Name == "GetCampaignConfig"
	}
	var sites []string
	declaredVar := false
	assignedFromConfig := false
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CompositeLit:
			if !isOrchConfig(x.Type) {
				return true
			}
			site := path + ":" + fd.Name.Name
			sites = append(sites, site)
			ok := false
			for _, el := range x.Elts {
				kv, isKV := el.(*ast.KeyValueExpr)
				if !isKV {
					continue
				}
				if key, isIdent := kv.Key.(*ast.Ident); isIdent && key.Name == "Campaign" && fromUserConfig(kv.Value) {
					ok = true
				}
			}
			if !ok {
				t.Errorf("%s builds an OrchestratorConfig whose Campaign is not UserConfig.GetCampaignConfig()", site)
			}
		case *ast.ValueSpec:
			if x.Type != nil && isOrchConfig(x.Type) {
				declaredVar = true
			}
		case *ast.AssignStmt:
			for i, lhs := range x.Lhs {
				sel, ok := lhs.(*ast.SelectorExpr)
				if ok && sel.Sel.Name == "Campaign" && i < len(x.Rhs) && fromUserConfig(x.Rhs[i]) {
					assignedFromConfig = true
				}
			}
		}
		return true
	})
	if declaredVar {
		site := path + ":" + fd.Name.Name
		sites = append(sites, site)
		if !assignedFromConfig {
			t.Errorf("%s declares an OrchestratorConfig and never sets its Campaign from UserConfig.GetCampaignConfig()", site)
		}
	}
	return sites
}
