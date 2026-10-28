package system

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/articulation"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// Every autopoiesis model call that is not tool code generation sends a
// system prompt compiled from its role atoms (internal/prompt/atoms/
// autopoiesis/), under the request autopoiesis and prompt_evolution make
// (role, intent /judge). Each compile carries its role's contract, no
// Piggyback contract (Go parses every reply), and no worker intent atoms.
// Until 2026-10-28 each was Go text: fmt.Sprintf prompts and the constants
// atomGenerationSystemPrompt and strategyRefinementSystemPrompt.
func TestRolePrompts_AutopoiesisRolesCompileFromTheirAtoms(t *testing.T) {
	roles := newBootRolePrompts(t)

	for role, contract := range map[string][]string{
		"complexity_analyzer": {
			"Analyze the task the message gives for complexity.",
			`"complexity": "simple|moderate|complex|epic"`,
		},
		"persistence_analyzer": {
			"Analyze if the user request the message gives requires a PERSISTENT agent.",
			`"needs_persistent": true/false`,
		},
		"agent_prompt_writer": {
			"Generate a system prompt for a persistent coding agent",
			"Generate a professional system prompt (no code blocks, just the prompt text).",
		},
		"tool_need_refiner": {
			"codeNERD's kernel has found a capability gap",
			`"tool_name": "snake_case_name"`,
		},
		"tool_profiler": {
			"Generate a quality profile for the tool the message gives",
			"3. **Output Expectations**:",
			`"analysis_summary": "1-2 sentence rationale for tool type and quality expectations"`,
		},
		"tool_quality_assessor": {
			"Analyze the tool execution the message gives and assess its quality.",
			`"completeness": 0.0-1.0`,
		},
		"tool_test_writer": {
			"Generate Go test code for the tool whose code the message gives.",
			`Generate a complete test file in package "tools".`,
		},
		"panic_maker": {
			"You are the PanicMaker.",
			"Output ONLY a JSON array of attack vectors.",
		},
		"prompt_atom_generator": {
			"You are an expert at creating prompt atoms for an AI coding agent.",
			"Output ONLY valid YAML, no other text.",
		},
		"strategy_refiner": {
			"You are an expert at improving problem-solving strategies for an AI coding agent.",
			"Output the refined strategy content in a markdown code block.",
		},
	} {
		t.Run(role, func(t *testing.T) {
			got, err := roles.CompileRolePrompt(context.Background(), types.RolePromptRequest{Role: role, Intent: "judge"})
			if err != nil {
				t.Fatalf("CompileRolePrompt(%s): %v", role, err)
			}
			// The contract lines appear, in order: a role split across sibling
			// atoms (tool_profiler) reads as the one prompt it was.
			at := 0
			for _, want := range contract {
				i := strings.Index(got[at:], want)
				if i < 0 {
					t.Errorf("the %s prompt lacks its atom's %q (in order)", role, want)
					continue
				}
				at += i + len(want)
			}
			for _, leaked := range []string{
				"OUTPUT PROTOCOL: PIGGYBACK ENVELOPE", // protocol/piggyback/envelope
				"CONTROL-PACKET FIELD ORDER",          // protocol/piggyback/control_first
				"DECISION SUMMARY",                    // protocol/decision_summary/requirements
			} {
				if strings.Contains(got, leaked) {
					t.Errorf("the %s prompt carries %q, a contract its Go-parsed reply cannot keep", role, leaked)
				}
			}
			if est := prompt.EstimateTokens(got); est > 4000 {
				t.Errorf("the %s prompt is %d tokens; a role compile is ~2k -- worker atoms leaked in", role, est)
			}
		})
	}
}

// The tool refiner's /refinement stage prompt, assembled the way autopoiesis
// asks for it, carries the optimizer priorities that were the Go constant
// refinementSystemPrompt until 2026-10-28.
func TestRolePrompts_ToolRefinementStageCarriesOptimizerPriorities(t *testing.T) {
	compiler, kernel := newBootJIT(t)
	assembler, err := articulation.NewPromptAssembler(kernel)
	if err != nil {
		t.Fatalf("NewPromptAssembler: %v", err)
	}
	assembler.SetJITCompiler(compiler)
	jit := config.DefaultJITConfig()
	assembler.SetJITBudgets(jit.TokenBudget, jit.ReservedTokens, jit.SemanticTopK, jit.ReservedTokensFallbackRatio)
	assembler.EnableJIT(true)

	got, err := assembler.AssembleSystemPrompt(context.Background(), map[string]any{
		"shard_id":        "tool_refiner_count_lines",
		"shard_type":      "tool_generator",
		"stage":           "/refinement",
		"ouroboros_stage": "/refinement",
		"tool_name":       "count_lines",
	})
	if err != nil {
		t.Fatalf("AssembleSystemPrompt: %v", err)
	}
	for _, want := range []string{
		"You are an elite Go code optimizer.",
		"Do not add network retries/pagination unless the code clearly performs network I/O.",
		"1. NIL SAFETY - Check pointers/interfaces before dereference.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the /refinement stage prompt lacks %q", want)
		}
	}
}

// systemCaptureLLM records the system prompt of every CompleteWithSystem call.
type systemCaptureLLM struct {
	MockLLMClient
	mu      sync.Mutex
	systems []string
}

func (m *systemCaptureLLM) CompleteWithSystem(_ context.Context, system, _ string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.systems = append(m.systems, system)
	return "{}", nil
}

// A booted cortex hands its role prompt compiler to autopoiesis
// (initIntelligenceLayer -> Orchestrator.SetRolePrompts): the boot
// orchestrator's tool profiler sends the prompt compiled from its atoms, not
// an ErrRolePromptUnavailable.
func TestBootCortex_AutopoiesisCompilesRolePromptsFromAtoms(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	llm := &systemCaptureLLM{}
	// A real kernel: a role compile runs in an isolated kernel snapshot.
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	userCfg := config.DefaultUserConfig()
	userCfg.Embedding = &config.EmbeddingConfig{Provider: "none"}
	cortex, err := BootCortexWithConfig(context.Background(), BootConfig{
		Workspace: workspace,
		APIKey:    "test-key",
		DisableSystemShards: []string{
			"constitution_gate", "perception_firewall", "executive_policy", "world_model_ingestor",
			"session_planner", "tactile_router", "campaign_runner", "mangle_repair", "legislator",
		},
		UserConfigOverride: userCfg,
		LLMClientOverride:  llm,
		KernelOverride:     kernel,
	})
	if err != nil {
		t.Fatalf("BootCortexWithConfig: %v", err)
	}
	defer cortex.Close()
	if cortex.Orchestrator == nil || cortex.RolePrompts == nil {
		t.Fatalf("boot built no orchestrator (%v) or role compiler (%v)", cortex.Orchestrator, cortex.RolePrompts)
	}

	if _, err := cortex.Orchestrator.GenerateToolProfile(context.Background(), "csv_tool", "converts CSV", "package tools"); err != nil {
		t.Fatalf("GenerateToolProfile on the booted orchestrator: %v", err)
	}
	llm.mu.Lock()
	defer llm.mu.Unlock()
	if len(llm.systems) != 1 || !strings.Contains(llm.systems[0], "Generate a quality profile for the tool the message gives") {
		t.Errorf("the booted tool profiler sent %d system prompts; want its compiled tool_profiler atoms: %q", len(llm.systems), llm.systems)
	}
}
