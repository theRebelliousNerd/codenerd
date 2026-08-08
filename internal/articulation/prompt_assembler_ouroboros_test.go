package articulation

import "testing"

// Companion to internal/prompt's F-OURO-4 tests. Ouroboros hands the assembler
// a map whose only prose is tool_purpose; everything else is an identifier or a
// type name. That prose landed in SessionCtx.ExtraContext — a tag bucket read
// for gating, not for retrieval — so both semantic channels ran blind.

func TestMapToPromptContext_ToolPurposeBecomesSemanticQuery(t *testing.T) {
	pa := &PromptAssembler{}

	pc, err := pa.mapToPromptContext(map[string]any{
		"shard_id":     "tool_generator_count_mangle_predicates",
		"shard_type":   "tool_generator",
		"tool_name":    "count_mangle_predicates",
		"tool_purpose": "count mangle predicates given a directory path",
	})
	if err != nil {
		t.Fatalf("mapToPromptContext: %v", err)
	}

	if pc.SemanticQuery != "count mangle predicates given a directory path" {
		t.Errorf("SemanticQuery = %q; the tool's purpose is the only prose in the request and must drive retrieval", pc.SemanticQuery)
	}
}

// An explicit semantic_query is the caller being specific on purpose, and must
// win over the derived default.
func TestMapToPromptContext_ExplicitSemanticQueryWins(t *testing.T) {
	pa := &PromptAssembler{}

	pc, err := pa.mapToPromptContext(map[string]any{
		"shard_id":       "tool_generator_x",
		"shard_type":     "tool_generator",
		"tool_purpose":   "count mangle predicates",
		"semantic_query": "datalog stratification",
	})
	if err != nil {
		t.Fatalf("mapToPromptContext: %v", err)
	}

	if pc.SemanticQuery != "datalog stratification" {
		t.Errorf("SemanticQuery = %q, want the caller's explicit query", pc.SemanticQuery)
	}
}

// With no purpose, the tool name is the last remaining signal — better than an
// empty query, which skips the knowledge search entirely.
func TestMapToPromptContext_FallsBackToToolName(t *testing.T) {
	pa := &PromptAssembler{}

	pc, err := pa.mapToPromptContext(map[string]any{
		"shard_id":   "tool_generator_y",
		"shard_type": "tool_generator",
		"tool_name":  "count_mangle_predicates",
	})
	if err != nil {
		t.Fatalf("mapToPromptContext: %v", err)
	}

	if pc.SemanticQuery != "count_mangle_predicates" {
		t.Errorf("SemanticQuery = %q, want the tool name as the fallback signal", pc.SemanticQuery)
	}
}

// Non-Ouroboros callers pass none of these keys and must be unaffected.
func TestMapToPromptContext_NoToolKeysLeavesQueryEmpty(t *testing.T) {
	pa := &PromptAssembler{}

	pc, err := pa.mapToPromptContext(map[string]any{
		"shard_id":   "coder",
		"shard_type": "coder",
	})
	if err != nil {
		t.Fatalf("mapToPromptContext: %v", err)
	}

	if pc.SemanticQuery != "" {
		t.Errorf("SemanticQuery = %q for a non-Ouroboros caller, want empty", pc.SemanticQuery)
	}
}
