package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// Source-level guard: runCampaignResume must boot through the same Cortex
// factory as runCampaignStart (GetOrBootCortex +
// buildCampaignOrchestratorConfig + SetPromptProvider) instead of
// hand-assembling a parallel stack with nil stores and no ToolPregenerator.
// No Cortex boot here: these tests only parse cmd_campaign.go.

func parseCmdCampaign(t *testing.T) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "cmd_campaign.go", nil, 0)
	if err != nil {
		t.Fatalf("ParseFile cmd_campaign.go: %v", err)
	}
	return f
}

func findFuncDecl(f *ast.File, name string) *ast.FuncDecl {
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

func classifyCall(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		return fun.Sel.Name
	case *ast.Ident:
		return fun.Name
	}
	return ""
}

func collectResumeCalls(body *ast.BlockStmt) map[string]bool {
	found := map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if name := classifyCall(call); name != "" {
			found[name] = true
		}
		return true
	})
	return found
}

func fileUsesIdent(f *ast.File, name string) bool {
	used := false
	ast.Inspect(f, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == name {
			used = true
			return false
		}
		return true
	})
	return used
}

func resumeCallSet(t *testing.T) map[string]bool {
	t.Helper()
	resume := findFuncDecl(parseCmdCampaign(t), "runCampaignResume")
	if resume == nil {
		t.Fatalf("runCampaignResume not found in cmd_campaign.go")
	}
	return collectResumeCalls(resume.Body)
}

func TestRunCampaignResumeBootsCortex(t *testing.T) {
	got := resumeCallSet(t)
	want := []string{"GetOrBootCortex", "buildCampaignOrchestratorConfig", "SetPromptProvider"}
	for _, name := range want {
		if !got[name] {
			t.Errorf("runCampaignResume must call %s (same boot as runCampaignStart)", name)
		}
	}
}

func TestRunCampaignResumeHasNoParallelStack(t *testing.T) {
	got := resumeCallSet(t)
	banned := []string{"NewRealKernel", "NewIntelligenceGatherer"}
	for _, name := range banned {
		if got[name] {
			t.Errorf("runCampaignResume must not call %s (boot through the Cortex factory)", name)
		}
	}
}

func TestCampaignResumeHelpersRemoved(t *testing.T) {
	f := parseCmdCampaign(t)
	banned := []string{"newCampaignLLMClients", "campaignLLMClients"}
	for _, name := range banned {
		if fileUsesIdent(f, name) {
			t.Errorf("identifier %q must no longer exist in cmd_campaign.go", name)
		}
	}
}
