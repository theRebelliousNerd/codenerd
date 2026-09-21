package session

import (
	"context"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
)

// stubNamedTool puts a stub under a real tool name for one test, whatever ran
// before it, and puts the original back afterwards.
func stubNamedTool(t *testing.T, name string, effect tools.Effect, reply func() string) {
	t.Helper()
	original := tools.Global().Get(name)
	if original != nil {
		tools.Global().Unregister(name)
	}
	if err := tools.Global().Register(&tools.Tool{
		Name: name, Effect: effect, Category: tools.CategoryGeneral, Description: "stub",
		Execute: func(context.Context, map[string]any) (string, error) { return reply(), nil },
	}); err != nil {
		t.Fatalf("register stub %q: %v", name, err)
	}
	t.Cleanup(func() {
		tools.Global().Unregister(name)
		if original != nil {
			_ = tools.Global().Register(original)
		}
	})
}

func runStructuralLoop(t *testing.T, asked string, rounds int, allowed []string) *roundScriptProvider {
	t.Helper()
	client := &roundScriptProvider{MockLLMClient: &MockLLMClient{}, toolName: asked, rounds: rounds}
	e := newWorkingLoopExecutor(t, client)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/explain"}}
	if _, _, err := e.runToolLoop(context.Background(), "system", "where is it",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: allowed},
		&prompt.CompilationContext{ShardID: "probe"}, result); err != nil {
		t.Fatalf("runToolLoop: %v", err)
	}
	return client
}

// The raw search tools are withheld until the structural queries have been
// given a real trial: four that ran, per working_structural_trials.
func TestStructuralFirst_SearchOpensAfterTheStructuralTrial(t *testing.T) {
	stubNamedTool(t, "find_symbol", tools.EffectRead, func() string { return "a.go:1-2  function  p.F\n-- 1 rows, complete\n" })
	stubNamedTool(t, "grep", tools.EffectRead, func() string { return "grep ran" })

	client := runStructuralLoop(t, "find_symbol", 6, []string{"find_symbol", "grep"})
	for i, names := range client.catalogs {
		offered := slices.Contains(names, "grep")
		if i < 4 && offered {
			t.Fatalf("request %d offers grep before four structural queries ran (%v)", i+1, names)
		}
		if i >= 4 && !offered {
			t.Fatalf("request %d still withholds grep after four structural queries (%v)", i+1, names)
		}
		if !slices.Contains(names, "find_symbol") {
			t.Fatalf("request %d dropped find_symbol (%v)", i+1, names)
		}
	}
	if !anyContains(toolResultContents(client.histories), "Raw search (grep, glob, list_files, search_code) is now offered") {
		t.Fatal("the model was never told that raw search opened")
	}
}

// Two structural queries with no answer are evidence the index cannot help
// (a workspace it cannot parse, a question about prose): search opens early.
func TestStructuralFirst_SearchOpensEarlyWhenTheIndexHasNoAnswer(t *testing.T) {
	stubNamedTool(t, "find_symbol", tools.EffectRead, func() string { return "none.\n" + tools.StructuralNoRows + "\n" })
	stubNamedTool(t, "grep", tools.EffectRead, func() string { return "grep ran" })

	client := runStructuralLoop(t, "find_symbol", 4, []string{"find_symbol", "grep"})
	for i, names := range client.catalogs {
		offered := slices.Contains(names, "grep")
		if i < 2 && offered {
			t.Fatalf("request %d offers grep before two misses (%v)", i+1, names)
		}
		if i >= 2 && !offered {
			t.Fatalf("request %d still withholds grep after two structural misses (%v)", i+1, names)
		}
	}
}

// A model that asks for a withheld search tool anyway is answered with the
// structural tools, the search does not run, and asking again opens nothing.
func TestStructuralFirst_WithheldSearchIsAnsweredNotRun(t *testing.T) {
	stubNamedTool(t, "find_symbol", tools.EffectRead, func() string { return "-- 1 rows, complete\n" })
	ran := 0
	stubNamedTool(t, "grep", tools.EffectRead, func() string { ran++; return "grep ran" })

	client := runStructuralLoop(t, "grep", 3, []string{"find_symbol", "grep"})
	if ran != 0 {
		t.Fatalf("grep ran %d time(s) while withheld", ran)
	}
	results := toolResultContents(client.histories)
	if !anyContains(results, "find_symbol locates a declaration by name") {
		t.Fatalf("a withheld search must be answered with the structural tools; results: %q", results)
	}
	for i, names := range client.catalogs {
		if slices.Contains(names, "grep") {
			t.Fatalf("request %d offers grep though no structural query ever ran (%v)", i+1, names)
		}
	}
}

// An agent that was never given the structural queries keeps its search
// tools: withholding grep with nothing in its place is a brick.
func TestStructuralFirst_NoStructuralToolsMeansNoGate(t *testing.T) {
	ran := 0
	stubNamedTool(t, "grep", tools.EffectRead, func() string { ran++; return "grep ran" })

	client := runStructuralLoop(t, "grep", 2, []string{"grep"})
	if ran != 2 {
		t.Fatalf("grep ran %d time(s), want 2", ran)
	}
	for i, names := range client.catalogs {
		if !slices.Contains(names, "grep") {
			t.Fatalf("request %d withholds grep from an agent with no structural tool (%v)", i+1, names)
		}
	}
	if anyContains(toolResultContents(client.histories), strings.TrimSpace("Raw search (grep")) {
		t.Fatal("an ungated agent was handed structural-first steering")
	}
}
