package system_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/system"
	"codenerd/internal/tactile"
	"codenerd/internal/types"
	"codenerd/internal/world"
)

// TestImpactChain_EndToEndThroughVirtualStore is the whole-system proof for the
// defect this branch exists to fix.
//
// The chain — a CodeDOM edit produces modified_function, impact.mg derives
// impact_caller then impact_graph then context_priority_file, and the
// holographic provider renders impact-ranked callers into the prompt — was
// built end to end and executed zero times in production. Nothing produced
// modified_function: the kernel, the executive in a design whose premise is
// that logic determines reality, was waiting for the model to describe an edit
// the kernel had just performed itself.
//
// This test drives a real VirtualStore edit against a real kernel and asserts
// every link, so any one of them breaking again fails here rather than in a
// user's session.
func TestImpactChain_EndToEndThroughVirtualStore(t *testing.T) {
	ws := t.TempDir()
	writeImpactModule(t, ws)
	t.Chdir(ws)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	kernel.SetWorkspace(ws)

	vsCfg := core.DefaultVirtualStoreConfig()
	vsCfg.WorkingDir = ws
	vs := core.NewVirtualStoreWithConfig(tactile.NewDirectExecutor(), vsCfg)
	vs.SetKernel(kernel)
	vs.DisableBootGuard()
	vs.SetCodeScope(system.NewHolographicCodeScope(ws, kernel, nil, 0))

	editor := tactile.NewFileEditor()
	editor.SetWorkingDir(ws)
	vs.SetFileEditor(core.NewTactileFileEditorAdapter(editor))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	target := filepath.Join(ws, "lib.go")
	if _, err := routePermittedAction(t, ctx, vs, kernel,
		core.Fact{Predicate: "next_action", Args: []any{"open", "/open_file", target}}); err != nil {
		t.Fatalf("open_file: %v", err)
	}

	// The scope must have found the caller, or the impact walk has nothing to
	// walk and the rest of the test would pass vacuously.
	if n := countFactsFor(t, kernel, "code_calls"); n == 0 {
		t.Fatal("no code_calls facts after open_file; the deep scan did not run")
	}

	// Edit the function every caller depends on.
	ref := "fn:impactdemo.Target"
	if _, err := routePermittedAction(t, ctx, vs, kernel, core.Fact{
		Predicate: "next_action",
		Args: []any{"edit", "/edit_element", ref, map[string]any{
			"content": "func Target() int {\n\treturn 42\n}",
		}},
	}); err != nil {
		t.Fatalf("edit_element: %v", err)
	}

	// Link 1: the edit produced the fact the whole chain hangs on.
	modified := factsFor(t, kernel, "modified_function")
	if len(modified) == 0 {
		t.Fatal("edit_element produced no modified_function fact; the impact chain has no input")
	}
	var sawTarget bool
	for _, f := range modified {
		if len(f.Args) >= 1 && types.ExtractString(f.Args[0]) == "impactdemo.Target" {
			sawTarget = true
		}
	}
	if !sawTarget {
		t.Fatalf("modified_function does not name the edited symbol in the shape code_calls uses (want impactdemo.Target): %+v", modified)
	}

	// Link 2: the kernel derived the impact walk from it.
	if len(factsFor(t, kernel, "impact_caller")) == 0 {
		t.Fatal("impact.mg derived no impact_caller from modified_function + code_calls")
	}
	if len(factsFor(t, kernel, "impact_graph")) == 0 {
		t.Fatal("impact.mg derived no impact_graph")
	}

	// Link 3: the context-priority relation the holographic provider reads.
	if len(factsFor(t, kernel, "context_priority_file")) == 0 {
		t.Fatal("impact.mg derived no context_priority_file; the prompt renderer has nothing to rank")
	}

	// Link 4: it reaches the prompt as a ranked section, not an unordered list.
	section := world.NewHolographicProvider(kernel, ws).PromptSection(ctx, target)
	if section == "" {
		t.Fatal("holographic PromptSection is empty for an edited file")
	}
	if !contains(section, "Callers (impact-prioritized)") {
		t.Fatalf("prompt fell through to the unranked callers branch:\n%s", section)
	}
}

func writeImpactModule(t *testing.T, ws string) {
	t.Helper()
	files := map[string]string{
		"go.mod": "module example.com/impactdemo\n\ngo 1.22\n",
		"lib.go": "package impactdemo\n\n" +
			"// Target is the function under edit.\n" +
			"func Target() int {\n\treturn 1\n}\n",
		"caller.go": "package impactdemo\n\n" +
			"// DirectCaller calls Target.\n" +
			"func DirectCaller() int {\n\treturn Target()\n}\n\n" +
			"// GrandCaller calls DirectCaller.\n" +
			"func GrandCaller() int {\n\treturn DirectCaller()\n}\n",
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(src), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

func factsFor(t *testing.T, k *core.RealKernel, predicate string) []core.Fact {
	t.Helper()
	facts, err := k.Query(predicate)
	if err != nil {
		t.Fatalf("query %s: %v", predicate, err)
	}
	return facts
}

func countFactsFor(t *testing.T, k *core.RealKernel, predicate string) int {
	t.Helper()
	return len(factsFor(t, k, predicate))
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
