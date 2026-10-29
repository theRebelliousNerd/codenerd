package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

// 2026-10-29: a turn asked to keep go vet clean had no vet tool in view and
// said so in its control packet, missing_tool_for(/lint, /go_vet). The shipped
// policy derived delegate_task(/tool_generator, /go_vet, /pending) from that
// fact alone, and autopoiesis spent minutes generating a vet tool. run_lint is
// that tool: a capability it provides is not generated, a capability nothing
// provides still is, and every persona that writes or judges Go is offered it.
func TestPolicy_TheStaticGatesAreRunLintNotAGeneratedTool(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("the shipped corpus must load: %v", err)
	}
	// The model's control packet, through the filter the executor applies.
	facts, blocked := FilterMangleUpdates(k, []string{
		"missing_tool_for(/lint, /go_vet).",
		"missing_tool_for(/lint, /staticcheck).",
		"missing_tool_for(/build, /frobnicate_widgets).",
	}, ModelObservationPolicy())
	if len(blocked) != 0 || len(facts) != 3 {
		t.Fatalf("the control packet's missing_tool_for was not admitted: facts=%v blocked=%v", facts, blocked)
	}
	for _, f := range facts {
		if err := k.Assert(f); err != nil {
			t.Fatalf("assert %v: %v", f, err)
		}
	}

	delegated := map[string]bool{}
	rows, err := k.Query("delegate_task")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if len(row.Args) == 3 && row.Args[0] == "/tool_generator" {
			if capability, ok := row.Args[1].(string); ok {
				delegated[capability] = true
			}
		}
	}
	for _, capability := range []string{"/go_vet", "/staticcheck"} {
		if delegated[capability] {
			t.Errorf("delegate_task(/tool_generator, %s, /pending) derived: run_lint provides it", capability)
		}
	}
	if !delegated["/frobnicate_widgets"] {
		t.Errorf("a capability no tool provides must still go to the tool generator; delegated = %v", delegated)
	}

	for _, verb := range []string{"/lint", "/review", "/fix", "/test", "/verify"} {
		allowed, err := k.Query("turn_tool_allowed(" + verb + ", /run_lint)")
		if err != nil {
			t.Fatal(err)
		}
		if len(allowed) == 0 {
			t.Errorf("a %s turn is not offered run_lint", verb)
		}
	}
	permitted, err := k.Query("safe_action(/run_lint)")
	if err != nil || len(permitted) == 0 {
		t.Errorf("run_lint is not a safe_action (err %v)", err)
	}
}

// The production installer puts run_lint in the store's registry, and the
// registered tool is the real one: go vet over a module with a planted finding
// returns it typed.
func TestHydrateModularTools_InstallsRunLint(t *testing.T) {
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	if err := vs.HydrateModularTools(hydrateCatalogSearcher{}); err != nil {
		t.Fatalf("HydrateModularTools: %v", err)
	}
	tool := vs.GetModularTools().Get("run_lint")
	if tool == nil {
		t.Fatal("the production registry has no run_lint")
	}
	root := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":     "module lintwiring\n\ngo 1.22\n",
		"bad/bad.go": "package bad\n\nimport \"fmt\"\n\n// Say prints n.\nfunc Say(n string) { fmt.Printf(\"%d\\n\", n) }\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err := tool.Execute(tools.WithWorkspaceRoot(context.Background(), root), map[string]any{"gate": "vet", "packages": []any{"./bad"}})
	if err != nil {
		t.Fatalf("run_lint: %v", err)
	}
	if !strings.Contains(out, `"status":"findings"`) || !strings.Contains(out, `"file":"bad/bad.go"`) || !strings.Contains(out, `"check":"printf"`) {
		t.Fatalf("run_lint result does not carry the planted vet finding: %s", out)
	}
}
