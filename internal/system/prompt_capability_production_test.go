package system

import (
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/prompt"
)

// Exercise the actual factory, embedded corpus, kernel selector and cache.
// The five-tool envelope is a legitimate restricted executor, not a new prompt
// mode: guidance must follow its capabilities without losing acceptance rules.
func TestProductionPromptRespectsRestrictedCapabilities(t *testing.T) {
	cortex := bootKernelForShardModeTest(t, true)
	t.Cleanup(func() { _ = cortex.Close() })
	if cortex.JITCompiler == nil {
		t.Fatal("factory omitted prompt compiler")
	}
	cc := prompt.NewCompilationContext().WithShard("coder", "coder", "").WithTokenBudget(65536, 2048)
	cc.IntentVerb = "/fix"
	cc.OperationalMode = "/active"
	cc.AvailableTools = []string{"read_file", "list_files", "write_file", "edit_file", "run_tests"}
	compile := func() map[string]bool {
		t.Helper()
		result, err := cortex.JITCompiler.Compile(t.Context(), cc)
		if err != nil {
			t.Fatal(err)
		}
		ids := make(map[string]bool)
		for _, atom := range result.IncludedAtoms {
			ids[atom.ID] = true
		}
		for _, required := range []string{"safety/constitutional/core", "safety/constitutional/permissions", "safety/honesty/no_unverified_claims"} {
			if !ids[required] {
				t.Errorf("capability filtering lost mandatory safety/evidence atom %s", required)
			}
		}
		return ids
	}
	assertRestricted := func(ids map[string]bool) {
		t.Helper()
		for _, unsupported := range []string{"identity/coder/codedom_premier", "capability/codedom_first", "capability/codedom_core", "capability/codedom_tools", "capability/codedom_selection", "capability/codedom_impact"} {
			if ids[unsupported] {
				t.Errorf("actual factory selected unavailable tool instructions: %s", unsupported)
			}
		}
	}
	assertRestricted(compile())
	cc.AvailableTools = append(cc.AvailableTools, "get_elements", "get_element", "edit_lines", "insert_lines", "delete_lines")
	if !compile()["capability/codedom_first"] {
		t.Error("available semantic editing guidance was suppressed")
	}
	// Same compiler after a broader envelope: neither cache reuse nor lingering
	// compilation facts may carry the now-unavailable guidance forward.
	cc.AvailableTools = []string{}
	assertRestricted(compile())
}

// Bypass Go pre-filtering: the executive itself must reject unsupported
// guidance and its dependent, while admitting the same bundle when available.
func TestProductionPolicyEnforcesCapabilityDependencies(t *testing.T) {
	_, adapter := newPromptScopeTestKernel(t)
	for _, available := range []bool{false, true} {
		t.Run(fmt.Sprint(available), func(t *testing.T) {
			scope, err := adapter.NewCompilationScope()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = scope.Close() })
			var facts []any
			for _, id := range []string{"tool-leaf", "dependent", "agnostic"} {
				facts = append(facts, fmt.Sprintf(`atom(%q)`, id), fmt.Sprintf(`is_mandatory(%q)`, id), fmt.Sprintf(`atom_priority(%q, 90)`, id))
			}
			facts = append(facts, `atom_requires_tool("tool-leaf", "read_file")`, `atom_requires("dependent", "tool-leaf")`)
			if available {
				facts = append(facts, `available_tool("read_file")`)
			}
			if err := scope.AssertBatch(facts); err != nil {
				t.Fatal(err)
			}
			selected, err := scope.Query("selected_result(Atom, Priority, Source)")
			if err != nil {
				t.Fatal(err)
			}
			ids := make(map[string]bool)
			for _, fact := range selected {
				if len(fact.Args) > 0 {
					ids[fmt.Sprint(fact.Args[0])] = true
				}
			}
			if !ids["agnostic"] {
				t.Fatal("tool-agnostic rule failed to select; test would be vacuous")
			}
			for _, id := range []string{"tool-leaf", "dependent"} {
				if ids[id] != available {
					t.Errorf("Mangle selected %s=%v with tool available=%v", id, ids[id], available)
				}
			}
		})
	}
}

func TestProductionPromptPreservesPersistedCapabilityDependencies(t *testing.T) {
	const atoms = `- id: "capabilityexpert/base"
  category: "identity"
  priority: 100
  is_mandatory: true
  content: "PERSISTED_BASE_GUIDANCE"
- id: "capabilityexpert/read"
  category: "capability"
  priority: 90
  is_mandatory: true
  requires_tools: [read_file]
  content: "PERSISTED_READ_GUIDANCE"
- id: "capabilityexpert/dependent"
  category: "methodology"
  priority: 80
  is_mandatory: true
  depends_on: [capabilityexpert/read]
  content: "PERSISTED_DEPENDENT_GUIDANCE"
`
	dbPath := writeAndSyncUserAgent(t, t.TempDir(), "capabilityexpert", atoms)
	cortex := bootKernelForShardModeTest(t, true)
	t.Cleanup(func() { _ = cortex.Close() })
	if err := prompt.RegisterAgentDBWithJIT(cortex.JITCompiler, "capabilityexpert", dbPath); err != nil {
		t.Fatal(err)
	}
	cc := prompt.NewCompilationContext().WithShard("capabilityexpert", "capabilityexpert", "").WithTokenBudget(65536, 2048)
	cc.OperationalMode = "/active"
	for _, tools := range [][]string{{}, {"read_file"}, {}} {
		cc.AvailableTools = tools
		result, err := cortex.JITCompiler.Compile(t.Context(), cc)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result.Prompt, "PERSISTED_BASE_GUIDANCE") {
			t.Fatal("registered expert identity did not reach production compilation")
		}
		for _, marker := range []string{"PERSISTED_READ_GUIDANCE", "PERSISTED_DEPENDENT_GUIDANCE"} {
			if got, want := strings.Contains(result.Prompt, marker), len(tools) != 0; got != want {
				t.Errorf("tools=%v: persisted marker %s present=%v, want %v", tools, marker, got, want)
			}
		}
	}
}
