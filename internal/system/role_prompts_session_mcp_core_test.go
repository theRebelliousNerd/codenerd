package system

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// The build critic, step planner and final-report auditor (session), the MCP
// tool analyzer (mcp), the kernel's rule generator (core) and `nerd whatif`'s
// analyst (cmd/nerd) each send a system prompt compiled from their role
// atoms, under the requests those packages pin in their own tests. Each
// compile carries the role's atom, no Piggyback contract (Go parses the
// reply), and no worker intent atoms (a role compile is ~1.5k-2k tokens; one
// without the judge intent took ~10k, measured 2026-10-28).
func TestRolePrompts_SessionMCPCoreRolesCompileFromTheirAtoms(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	embedded, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	compiler, err := prompt.NewJITPromptCompiler(
		prompt.WithKernel(NewKernelAdapter(kernel)),
		prompt.WithEmbeddedCorpus(embedded),
	)
	if err != nil {
		t.Fatalf("NewJITPromptCompiler: %v", err)
	}
	t.Cleanup(func() { _ = compiler.Close() })
	roles, err := prompt.NewRoleCompiler(compiler, config.DefaultJITConfig())
	if err != nil {
		t.Fatalf("NewRoleCompiler: %v", err)
	}

	for role, contract := range map[string][]string{
		"build_critic": {
			"You are a rigorous, adversarial code reviewer.",
			"Output findings in the exact line format 'FINDING file.go:123 severity: claim text' one per line.",
			"output the single line 'NO FINDINGS' and nothing else",
		},
		"mcp_tool_analyzer": {
			"You analyze one MCP tool and extract structured metadata for tool selection.",
			`"shard_affinities": {"coder": 0-100, "tester": 0-100, "reviewer": 0-100, "researcher": 0-100}`,
			"Respond with ONLY the JSON object, no other text.",
		},
		"work_step_planner": {
			"You divide one code-change task into the edit steps an executive will run one at a time",
			"STEP <workspace-relative file path> :: <the change to make in that file, with its location>",
			"Output only STEP lines, or NO STEPS, and nothing else.",
		},
		"final_report_auditor": {
			"answer with exactly one word: INCOMPLETE or COMPLETE",
			"Does the report itself say that part of the requested work was not done",
		},
		"what_if_analyst": {
			"Analyze the implications of the hypothetical change the message names, in 6-10 bullet points.",
		},
		"mangle_rule_generator": {
			"You are an expert Mangle/Datalog programmer for codeNERD",
			"## Critical Mangle Syntax Rules:",
			"Output ONLY the rule, no explanation. The rule must compile.",
		},
	} {
		t.Run(role, func(t *testing.T) {
			got, err := roles.CompileRolePrompt(context.Background(), types.RolePromptRequest{Role: role, Intent: "judge"})
			if err != nil {
				t.Fatalf("CompileRolePrompt(%s): %v", role, err)
			}
			for _, want := range contract {
				if !strings.Contains(got, want) {
					t.Errorf("the %s prompt lacks its atom's %q", role, want)
				}
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
