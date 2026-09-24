package session

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	toolscodedom "codenerd/internal/tools/codedom"
	toolscore "codenerd/internal/tools/core"
	toolsmcp "codenerd/internal/tools/mcpctl"
	toolsshell "codenerd/internal/tools/shell"
	"codenerd/internal/types"
)

// registerProductionTools puts the persona catalogs' tools in the global
// registry buildToolDefinitions reads, as VirtualStore hydration does at boot,
// and takes back out at cleanup every one it added: other tests in this
// package register their own tools under the same names.
func registerProductionTools(t *testing.T) {
	t.Helper()
	before := make(map[string]bool)
	for _, name := range tools.Global().Names() {
		before[name] = true
	}
	t.Cleanup(func() {
		for _, name := range tools.Global().Names() {
			if !before[name] {
				tools.Global().Unregister(name)
			}
		}
	})
	for _, register := range []func(*tools.Registry) error{
		toolscore.RegisterAll, toolscodedom.RegisterAll, toolsshell.RegisterAll, toolsmcp.RegisterAll,
	} {
		if err := register(tools.Global()); err != nil {
			t.Fatalf("register tools: %v", err)
		}
	}
}

// The kernel withholds from a turn what its target and its circumstances make
// useless (policy/jit_tools.mg); the executor only measures and asks. A
// Markdown target loses the build and impacted-test tools, a Go target keeps
// them; with no MCP server registered the five MCP verbs go, with one they
// stay; subagent_expand rides only a turn whose input carries a
// subagent-return handle.
func TestTurnWithheldToolsFollowTheTurn(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	e := NewExecutor(k, nil, nil, nil, nil, nil)
	build := []string{"run_build", "get_impacted_tests", "run_impacted_tests"}
	mcp := []string{"mcp_map", "mcp_probe", "mcp_call", "mcp_expand", "mcp_context"}

	md := e.turnWithheldTools("/markdown", "write the gap analysis")
	goTurn := e.turnWithheldTools("/go", "fix the race")
	for _, tool := range build {
		if !slices.Contains(md, tool) {
			t.Errorf("a Markdown turn keeps %s (withheld: %v)", tool, md)
		}
		if slices.Contains(goTurn, tool) {
			t.Errorf("a Go turn loses %s", tool)
		}
	}
	if slices.Contains(md, "run_tests") || slices.Contains(md, "read_file") || slices.Contains(md, "edit_lines") {
		t.Errorf("a Markdown turn loses a tool it can use: %v", md)
	}
	for _, tool := range append(slices.Clone(mcp), "subagent_expand") {
		if !slices.Contains(goTurn, tool) {
			t.Errorf("with no MCP server and no subagent handle the turn keeps %s (withheld: %v)", tool, goTurn)
		}
	}

	if slices.Contains(e.turnWithheldTools("/go", "the reviewer returned obs:sa:0123abcd; read it"), "subagent_expand") {
		t.Error("a turn whose input carries a subagent-return handle lost subagent_expand")
	}
	if err := k.Assert(core.Fact{Predicate: "mcp_server_registered", Args: []any{"srv1", "http://127.0.0.1:1", core.MangleAtom("/http"), int64(1)}}); err != nil {
		t.Fatalf("assert mcp_server_registered: %v", err)
	}
	withServer := e.turnWithheldTools("/go", "fix the race")
	for _, tool := range mcp {
		if slices.Contains(withServer, tool) {
			t.Errorf("with an MCP server registered the turn lost %s", tool)
		}
	}
	if got := e.turnWithheldTools("not an atom", "x"); slices.Contains(got, "run_build") {
		t.Errorf("a malformed language reached the kernel query: %v", got)
	}
}

// Narrowing never widens and never mutates the config it was given: a
// precompiled config is shared across turns.
func TestConfigWithoutToolsCopies(t *testing.T) {
	shared := &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{"read_file", "run_build", "edit_lines"}}
	narrowed := configWithoutTools(shared, []string{"run_build", "not_in_envelope"})
	if !slices.Equal(narrowed.AllowedTools, []string{"read_file", "edit_lines"}) {
		t.Errorf("narrowed = %v", narrowed.AllowedTools)
	}
	if !slices.Equal(shared.AllowedTools, []string{"read_file", "run_build", "edit_lines"}) {
		t.Errorf("the shared config was mutated: %v", shared.AllowedTools)
	}
	if got := withoutTools(nil, []string{"run_build"}); len(got) != 0 {
		t.Errorf("withoutTools widened an empty envelope: %v", got)
	}
}

// Through the turn: a coder turn aimed at a Markdown document compiles with,
// is offered, and may call only the narrowed catalog. The model is never shown
// run_build, the compile's requires_tools gating sees the same catalog, and a
// call to the withheld tool anyway is refused, not run.
func TestProseTurnIsOfferedTheNarrowedCatalog(t *testing.T) {
	registerProductionTools(t)
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	var mu sync.Mutex
	var compiledTools, offered []string
	jit := &MockJITCompiler{CompileFunc: func(_ context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
		mu.Lock()
		compiledTools = slices.Clone(cc.AvailableTools)
		mu.Unlock()
		return &prompt.CompilationResult{Prompt: "system"}, nil
	}}
	llm := &MockLLMClient{CompleteWithToolsFunc: func(_ context.Context, _, _ string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
		mu.Lock()
		for _, d := range tools {
			offered = append(offered, d.Name)
		}
		mu.Unlock()
		return &types.LLMToolResponse{Text: "done"}, nil
	}}
	e := NewExecutor(k, &MockVirtualStore{}, llm, jit, prompt.NewDefaultConfigFactory(), &MockTransducer{})
	preset := &perception.Intent{Verb: "/create", Category: "/mutation", Target: "Docs/architecture/features/03-GAP-ANALYSIS.md", Confidence: 1}
	_, _ = e.ProcessWithIntent(t.Context(), "Create the gap analysis document", preset)

	mu.Lock()
	defer mu.Unlock()
	if len(offered) == 0 {
		t.Fatal("the turn offered the model no tools; the test proves nothing")
	}
	for _, list := range [][]string{compiledTools, offered} {
		if slices.Contains(list, "run_build") || slices.Contains(list, "run_impacted_tests") {
			t.Errorf("a Markdown turn carries a build tool: %v", list)
		}
		if !slices.Contains(list, "read_file") || !slices.Contains(list, "write_file") {
			t.Errorf("a Markdown turn lost the tools it writes with: %v", list)
		}
	}

	cfg := configWithoutTools(&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{"read_file", "run_build"}},
		e.turnWithheldTools("/markdown", "Create the gap analysis document"))
	if _, err := e.executeToolCall(t.Context(), ToolCall{Name: "run_build", Args: map[string]any{}}, cfg); err == nil ||
		!strings.Contains(err.Error(), "not allowed") {
		t.Errorf("a call to a withheld tool was not refused: %v", err)
	}
}

// TestTurnCatalogSize is the MEASUREMENT: the coder envelope's schema bytes
// for a Go turn and a Markdown turn, with no MCP server and no subagent handle.
func TestTurnCatalogSize(t *testing.T) {
	registerProductionTools(t)
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	e := NewExecutor(k, nil, nil, nil, nil, nil)
	envelope, err := prompt.NewDefaultConfigFactory().ResolveAllowedTools(t.Context(), "/create")
	if err != nil {
		t.Fatalf("ResolveAllowedTools: %v", err)
	}
	size := func(tools []string) int {
		defs := e.buildToolDefinitions(&config.EffectiveAgentRuntimeConfig{AllowedTools: tools})
		b, _ := json.Marshal(defs)
		return len(b)
	}
	for _, tc := range []struct{ name, language string }{{"go", "/go"}, {"markdown", "/markdown"}} {
		turn := withoutTools(envelope, e.turnWithheldTools(tc.language, "a task"))
		t.Logf("%s turn: %d of %d tools, %d of %d schema bytes", tc.name, len(turn), len(envelope), size(turn), size(envelope))
	}
}
