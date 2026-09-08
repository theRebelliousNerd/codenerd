package prompt

import (
	"strings"
	"testing"
)

// Capability gating proves the tool envelope agrees with the compiled prompt
// BEFORE selection: tool-gated atoms are omitted when any required tool is
// absent, while safety/evidence and tool-agnostic guidance are retained.
// Never widen execution authority to satisfy this; omit instead.

func TestCapabilityGatingToolSatisfied(t *testing.T) {
	codedom := &PromptAtom{
		ID:            "capability/codedom_first",
		Category:      CategoryCapability,
		RequiresTools: []string{"get_elements", "get_element", "edit_lines", "insert_lines", "delete_lines"},
	}
	tests := []struct {
		name      string
		atom      *PromptAtom
		available map[string]struct{}
		want      bool
	}{
		{
			name:      "unavailable catalog blocks codedom bundle",
			atom:      codedom,
			available: availableToolSet(&CompilationContext{AvailableTools: []string{"read_file", "list_files", "write_file", "edit_file", "run_tests"}}),
			want:      false,
		},
		{
			name:      "available catalog admits codedom bundle",
			atom:      codedom,
			available: availableToolSet(&CompilationContext{AvailableTools: []string{"read_file", "get_elements", "get_element", "edit_lines", "insert_lines", "delete_lines"}}),
			want:      true,
		},
		{
			name:      "empty catalog blocks every gated atom",
			atom:      codedom,
			available: availableToolSet(&CompilationContext{}),
			want:      false,
		},
		{
			name:      "nil context blocks every gated atom",
			atom:      codedom,
			available: availableToolSet(nil),
			want:      false,
		},
		{
			name:      "tool-agnostic safety retained on empty catalog",
			atom:      &PromptAtom{ID: "safety/x", Category: CategorySafety},
			available: availableToolSet(&CompilationContext{}),
			want:      true,
		},
		{
			name:      "tool-agnostic methodology retained on empty catalog",
			atom:      &PromptAtom{ID: "methodology/y", Category: CategoryMethodology},
			available: availableToolSet(&CompilationContext{}),
			want:      true,
		},
		{
			name: "explicit requirement gates skeleton too",
			atom: &PromptAtom{
				ID:            "identity/coder/codedom_premier",
				Category:      CategoryIdentity,
				RequiresTools: []string{"get_elements"},
			},
			available: availableToolSet(&CompilationContext{AvailableTools: []string{"read_file"}}),
			want:      false,
		},
		{
			name:      "partial bundle blocked (one tool missing)",
			atom:      codedom,
			available: availableToolSet(&CompilationContext{AvailableTools: []string{"get_elements", "get_element", "edit_lines", "insert_lines"}}),
			want:      false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := atomToolSatisfied(tc.atom, tc.available); got != tc.want {
				t.Fatalf("atomToolSatisfied(%s) = %v, want %v", tc.atom.ID, got, tc.want)
			}
		})
	}
}

func TestCapabilityGatingDependencyClosure(t *testing.T) {
	// If a tool-gated parent is omitted, its dependents must be omitted too
	// rather than rendered dangling.
	parent := &ScoredAtom{Atom: &PromptAtom{
		ID:            "capability/codedom_tools",
		Category:      CategoryCapability,
		RequiresTools: []string{"get_elements"},
	}, Combined: 1.0}
	child := &ScoredAtom{Atom: &PromptAtom{
		ID:        "capability/codedom_test_tools",
		Category:  CategoryCapability,
		DependsOn: []string{"capability/codedom_tools"},
	}, Combined: 0.9}
	grandchild := &ScoredAtom{Atom: &PromptAtom{
		ID:        "capability/codedom_query_tools",
		Category:  CategoryCapability,
		DependsOn: []string{"capability/codedom_test_tools"},
	}, Combined: 0.8}

	r := NewDependencyResolver()
	// Simulate post-gating selection where the parent was already omitted:
	// only child + grandchild remain, both dangling.
	ordered, err := r.Resolve([]*ScoredAtom{child, grandchild})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(ordered) != 0 {
		t.Fatalf("dangling dependents not pruned, got %d atoms", len(ordered))
	}

	// Intact chain resolves in dependency order.
	ordered, err = r.Resolve([]*ScoredAtom{grandchild, child, parent})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(ordered) != 3 {
		t.Fatalf("intact chain got %d atoms, want 3", len(ordered))
	}
	if ordered[0].Atom.ID != parent.Atom.ID {
		t.Fatalf("parent should order first, got %s", ordered[0].Atom.ID)
	}

	// allowMissingDeps preserves legacy lenient behavior when explicitly set.
	r.SetAllowMissingDeps(true)
	ordered, err = r.Resolve([]*ScoredAtom{child})
	if err != nil {
		t.Fatalf("lenient Resolve failed: %v", err)
	}
	if len(ordered) != 1 {
		t.Fatalf("lenient mode should keep dangling atom, got %d", len(ordered))
	}
	_ = parent
}

func TestCapabilityGatingCacheIdentity(t *testing.T) {
	// Stale cache identity would serve a prompt compiled for a different tool
	// envelope. The hash must distinguish catalogs.
	base := NewCompilationContext()
	withWrite := base.Clone()
	withWrite.AvailableTools = []string{"read_file", "write_file"}
	withoutWrite := base.Clone()
	withoutWrite.AvailableTools = []string{"read_file"}
	if withWrite.Hash() == withoutWrite.Hash() {
		t.Fatal("cache identity ignores AvailableTools: stale catalog could be served")
	}
	// Same catalog, different order, must share identity (set semantics).
	a := base.Clone()
	a.AvailableTools = []string{"write_file", "read_file"}
	b := base.Clone()
	b.AvailableTools = []string{"read_file", "write_file"}
	if a.Hash() != b.Hash() {
		t.Fatal("cache identity should be order-insensitive for tool sets")
	}
	// Tools must not mutate the caller's context through the clone.
	if len(base.AvailableTools) != 0 {
		t.Fatalf("Clone leaked AvailableTools into base: %v", base.AvailableTools)
	}
}

func TestCapabilityGatingMangleFacts(t *testing.T) {
	// The Go selector must arm the Mangle veto: atom_requires_tool per gated
	// atom plus available_tool per catalog entry.
	s := NewAtomSelector()
	cc := NewCompilationContext()
	cc.AvailableTools = []string{"read_file"}
	atoms := []*PromptAtom{
		{ID: "capability/codedom_first", Category: CategoryCapability, RequiresTools: []string{"get_elements", "edit_lines"}},
		{ID: "safety/core", Category: CategorySafety, IsMandatory: true},
	}
	facts, err := s.buildContextFacts(cc, atoms, nil)
	if err != nil {
		t.Fatalf("buildContextFacts failed: %v", err)
	}
	joined := strings.Join(toStringSlice(facts), "\n")
	for _, want := range []string{
		`atom_requires_tool("capability/codedom_first", "get_elements")`,
		`atom_requires_tool("capability/codedom_first", "edit_lines")`,
		`available_tool("read_file")`,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing Mangle fact %s in:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, `atom_requires_tool("safety/core"`) {
		t.Fatalf("safety atom must not emit requires_tool facts:\n%s", joined)
	}
}

func toStringSlice(facts []any) []string {
	out := make([]string, 0, len(facts))
	for _, f := range facts {
		if s, ok := f.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func TestCapabilityGatingRequiresToolsRoundTrip(t *testing.T) {
	// Persisted atom round trip: YAML strict parse preserves requires_tools,
	// and invalid values reject the document rather than silently widening.
	data := []byte(`
id: test/capability-gated
category: capability
priority: 88
is_mandatory: true
requires_tools: [get_elements, edit_lines]
content: body
`)
	parsed, _, err := ParsePromptAtomYAML(data, "test.yaml", nil)
	if err != nil {
		t.Fatalf("valid requires_tools rejected: %v", err)
	}
	got := parsed[0].Atom.RequiresTools
	if len(got) != 2 || got[0] != "get_elements" || got[1] != "edit_lines" {
		t.Fatalf("round trip got %v", got)
	}
	// Clone must preserve the requirement (regression: field was dropped).
	cloned := parsed[0].Atom.Clone()
	if len(cloned.RequiresTools) != 2 {
		t.Fatalf("Clone dropped RequiresTools: %v", cloned.RequiresTools)
	}

	invalid := []struct {
		name string
		yaml string
	}{
		{"empty value", "requires_tools: ['']"},
		{"slash-prefixed", "requires_tools: [/get_elements]"},
		{"uppercase", "requires_tools: [Get_Elements]"},
		{"whitespace", "requires_tools: [' get_elements']"},
		{"duplicate", "requires_tools: [get_elements, get_elements]"},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			doc := []byte("id: test/bad\ncategory: capability\npriority: 50\nis_mandatory: false\ncontent: body\n" + tc.yaml + "\n")
			if _, _, err := ParsePromptAtomYAML(doc, "test.yaml", nil); err == nil {
				t.Fatalf("invalid requires_tools accepted: %s", tc.yaml)
			}
		})
	}
}

func TestCapabilityGatingSafetyEvidenceRetained(t *testing.T) {
	// Uniform requirements: an explicit requires_tools entry gates in every
	// selection path. Tool-agnostic safety/evidence and general identity
	// atoms (empty RequiresTools) are retained on every catalog, including
	// empty; gated atoms (skeleton or flesh) are blocked when any required
	// tool is absent.
	skeleton := &PromptAtom{ID: "safety/core", Category: CategorySafety, IsMandatory: true}
	gatedSkeleton := &PromptAtom{ID: "identity/coder/codedom_premier", Category: CategoryIdentity, IsMandatory: true, RequiresTools: []string{"get_elements"}}
	flesh := &PromptAtom{
		ID:            "capability/codedom_first",
		Category:      CategoryCapability,
		IsMandatory:   true,
		RequiresTools: []string{"get_elements"},
	}
	cc := NewCompilationContext() // empty catalog
	available := availableToolSet(cc)
	if !atomToolSatisfied(skeleton, available) {
		t.Fatal("tool-agnostic safety must be retained on empty catalog")
	}
	if atomToolSatisfied(gatedSkeleton, available) {
		t.Fatal("gated skeleton must be blocked on empty catalog")
	}
	if atomToolSatisfied(flesh, available) {
		t.Fatal("gated flesh must be blocked on empty catalog")
	}
	// Category sanity: safety stays skeleton, capability stays flesh.
	if !isSkeletonCategory(skeleton.Category) {
		t.Fatal("safety must be a skeleton category")
	}
	if isSkeletonCategory(flesh.Category) {
		t.Fatal("capability must be flesh, never skeleton")
	}
}

func TestCapabilityGatingRepeatedCatalogs(t *testing.T) {
	// Repeated empty/available/empty catalogs must gate identically each
	// time: no cache reuse or lingering state may carry unavailable guidance
	// forward, and restoring the catalog must restore the guidance.
	gated := &PromptAtom{ID: "capability/codedom_core", Category: CategoryCapability, RequiresTools: []string{"get_elements", "edit_lines"}}
	empty := availableToolSet(NewCompilationContext())
	full := availableToolSet(&CompilationContext{AvailableTools: []string{"get_elements", "edit_lines"}})
	if atomToolSatisfied(gated, empty) {
		t.Fatal("empty catalog must block gated atom")
	}
	if !atomToolSatisfied(gated, full) {
		t.Fatal("available catalog must admit gated atom")
	}
	if atomToolSatisfied(gated, empty) {
		t.Fatal("return to empty catalog must block gated atom again")
	}
}
