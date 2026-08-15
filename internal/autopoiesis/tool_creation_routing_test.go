package autopoiesis

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This file is the enforcement half of two TODO items that would otherwise
// decay the moment someone adds a new caller:
//
//   P0 "Route all production tool creation through ExecuteOuroborosLoop"
//   P1 "Campaign pregen always uses same safety depth as chat Ouroboros helpers"
//
// Autopoiesis writes Go, compiles it, and executes the binary in the user's
// workspace. The only thing standing between an LLM completion and that binary
// is the Ouroboros pipeline: go_safety.mg audit → Thunderdome → Mangle
// transition simulation → compile → register. Any code path that reaches
// ToolGenerator directly skips all five, and until now three did (chat
// generate_tool, ExecuteAction, GenerateToolWithTracing).
//
// Prose in a corpus document cannot notice a fourth appearing, so the rule is
// a test: the low-level generator is callable only from the pipeline itself,
// and every consumer of an Ouroboros loop calls a method that carries the
// safety stages.

// unauditedGeneratorMethods are ToolGenerator methods that produce or persist
// tool code without any safety audit.
var unauditedGeneratorMethods = map[string]bool{
	"GenerateTool":           true,
	"RegenerateWithFeedback": true,
	"WriteTool":              true,
	"RegisterTool":           true,
}

// generatorCallExemptions maps a repo-relative file to the reason it may call
// ToolGenerator directly.
var generatorCallExemptions = map[string]string{
	"internal/autopoiesis/ouroboros.go": "this IS the audited pipeline: generation feeds straight into SafetyChecker.Check and the Thunderdome",
	"internal/autopoiesis/autopoiesis_tools.go": "WriteAndRegisterTool is the documented unaudited test/diagnostic seam; " +
		"it has no production callers and GenerateTool above it routes through ExecuteOuroborosLoop",
}

// safetyBearingOuroborosMethods are the entry points that run the full loop,
// plus the read-only/registry surface that creates nothing.
var safetyBearingOuroborosMethods = map[string]bool{
	// Creation paths — each runs SafetyChecker.Check before commitTool.
	"Execute":              true,
	"ExecuteWithConfig":    true,
	"GenerateToolFromCode": true,
	// Read-only / non-creating surface.
	"CheckToolSafety":     true,
	"ExecuteTool":         true,
	"GetStats":            true,
	"GetRuntimeTool":      true,
	"GetTool":             true,
	"ListRuntimeTools":    true,
	"ListTools":           true,
	"SetLearningsContext": true,
	"SetOnToolRegistered": true,
}

type callSite struct {
	file     string // repo-relative, slash separated
	line     int
	receiver string
	method   string
}

func TestToolCreation_WhenCallingToolGeneratorDirectly_ShouldBeInsidePipelineOrExempt(t *testing.T) {
	root := autopoiesisRepoRoot(t)

	var offenders []string
	filesWithCalls := map[string]bool{}

	for _, site := range scanSelectorCalls(t, root) {
		if !unauditedGeneratorMethods[site.method] {
			continue
		}
		if !strings.Contains(strings.ToLower(site.receiver), "toolgen") {
			continue
		}
		filesWithCalls[site.file] = true
		if _, ok := generatorCallExemptions[site.file]; ok {
			t.Logf("%-45s %s.%s:%d  [%s]", site.file, site.receiver, site.method, site.line, generatorCallExemptions[site.file])
			continue
		}
		offenders = append(offenders, site.file+":"+strconv.Itoa(site.line)+" ("+site.receiver+"."+site.method+")")
	}
	sort.Strings(offenders)

	if len(filesWithCalls) == 0 {
		t.Fatal("scanner found no ToolGenerator calls at all — the scanner is broken, not the repo")
	}

	if len(offenders) > 0 {
		t.Errorf("unaudited tool creation (bypasses go_safety.mg, Thunderdome, simulation and compile):\n  %s\n\n"+
			"Route the call through Orchestrator.ExecuteOuroborosLoop / GenerateTool, or add the file to "+
			"generatorCallExemptions with a real reason.",
			strings.Join(offenders, "\n  "))
	}

	for file := range generatorCallExemptions {
		if !filesWithCalls[file] {
			t.Errorf("stale exemption for %q: it no longer calls ToolGenerator directly. Remove the entry.", file)
		}
	}
}

func TestOuroborosConsumers_WhenCreatingTools_ShouldUseSafetyBearingEntryPoint(t *testing.T) {
	root := autopoiesisRepoRoot(t)

	var offenders []string
	seen := 0

	for _, site := range scanSelectorCalls(t, root) {
		recv := strings.ToLower(site.receiver)
		// Field/variable holding an Ouroboros loop or the ToolSynthesizer.
		if !strings.HasSuffix(recv, "ouroboros") && !strings.HasSuffix(recv, "ouroborosloop") {
			continue
		}
		seen++
		if safetyBearingOuroborosMethods[site.method] {
			continue
		}
		offenders = append(offenders, site.file+":"+strconv.Itoa(site.line)+" ("+site.receiver+"."+site.method+")")
	}
	sort.Strings(offenders)

	if seen == 0 {
		t.Fatal("scanner found no Ouroboros call sites at all — the scanner is broken")
	}
	t.Logf("inspected %d Ouroboros call sites across the repo", seen)

	if len(offenders) > 0 {
		t.Errorf("Ouroboros consumers calling a method outside the safety-bearing surface:\n  %s\n\n"+
			"Campaign pregeneration and the chat helpers must run the same stages. Either use Execute/"+
			"ExecuteWithConfig/GenerateToolFromCode, or add the method to safetyBearingOuroborosMethods "+
			"once you have confirmed it cannot register a tool.",
			strings.Join(offenders, "\n  "))
	}
}

// TestCampaignPregen_WhenGeneratingTools_ShouldRunFullOuroborosLoop pins the
// specific cross-package wire the TODO names, so a refactor that swaps
// campaign's loop for a shallower helper fails here by name rather than by a
// generic inventory message.
func TestCampaignPregen_WhenGeneratingTools_ShouldRunFullOuroborosLoop(t *testing.T) {
	root := autopoiesisRepoRoot(t)
	pregen := filepath.Join(root, "internal", "campaign", "tool_pregenerator.go")

	src, err := os.ReadFile(pregen)
	if err != nil {
		t.Skipf("campaign pregenerator not present: %v", err)
	}
	if !strings.Contains(string(src), "ouroboros.Execute(ctx, need)") {
		t.Error("internal/campaign/tool_pregenerator.go no longer calls ouroboros.Execute; " +
			"campaign-pregenerated tools would run at a different safety depth than chat-generated ones")
	}
}

// scanSelectorCalls returns every `receiver.Method(...)` call in non-test .go
// files under root.
func scanSelectorCalls(t *testing.T, root string) []callSite {
	t.Helper()

	skipDir := map[string]bool{
		".git": true, "node_modules": true, "vendor": true, "testdata": true,
		"Docs": true, ".nerd": true, "dist": true, "build": true,
	}

	var sites []callSite
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDir[d.Name()] || strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			sites = append(sites, callSite{
				file:     relSlash,
				line:     fset.Position(sel.Sel.Pos()).Line,
				receiver: exprText(fset, sel.X),
				method:   sel.Sel.Name,
			})
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	slices.SortFunc(sites, func(a, b callSite) int {
		if a.file != b.file {
			return strings.Compare(a.file, b.file)
		}
		return a.line - b.line
	})
	return sites
}

func exprText(fset *token.FileSet, expr ast.Expr) string {
	var sb strings.Builder
	if err := printer.Fprint(&sb, fset, expr); err != nil {
		return ""
	}
	return sb.String()
}

// autopoiesisRepoRoot walks up from the test's working directory to the module root.
func autopoiesisRepoRoot(t *testing.T) string {
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
			t.Fatal("could not locate module root (no go.mod found walking up from the test directory)")
		}
		dir = parent
	}
}
