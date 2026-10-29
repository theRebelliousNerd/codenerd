package core

var defaultIntentSchemaFiles = []string{
	"schema/intent_qualifiers.mg",
	"schema/intent_queries.mg",
	"schema/intent_mutations.mg",
	"schema/intent_instructions.mg",
	"schema/intent_campaign.mg",
	"schema/intent_system.mg",
	"schema/intent_stats.mg",
	"schema/intent_conversational.mg",
	"schema/intent_code_review.mg",
	"schema/intent_code_mutations.mg",
	"schema/intent_testing.mg",
	"schema/intent_operations.mg",
	"schema/intent_multi_step.mg",
	"schema/intent_routing.mg",
}

// DefaultIntentSchemaFiles returns the ordered list of embedded intent modules.
func DefaultIntentSchemaFiles() []string {
	out := make([]string, len(defaultIntentSchemaFiles))
	copy(out, defaultIntentSchemaFiles)
	return out
}

func defaultIntentFactPredicates() map[string]struct{} {
	return map[string]struct{}{
		"intent_definition":          {},
		"intent_category":            {},
		"valid_semantic_type":        {},
		"valid_action_type":          {},
		"valid_domain":               {},
		"valid_scope_level":          {},
		"valid_mode":                 {},
		"valid_urgency":              {},
		"mode_from_semantic":         {},
		"mode_from_action":           {},
		"mode_from_domain":           {},
		"mode_from_signal":           {},
		"context_affinity_semantic":  {},
		"context_affinity_action":    {},
		"context_affinity_domain":    {},
		"shard_affinity_action":      {},
		"shard_affinity_domain":      {},
		"tool_affinity_action":       {},
		"tool_affinity_domain":       {},
		"constraint_type":            {},
		"constraint_forces_mode":     {},
		"constraint_blocks_tool":     {},
		"comparative_marker":         {},
		"copular_verb":               {},
		"existence_pattern":          {},
		"interrogative_state_signal": {},
		"interrogative_type":         {},
		"modal_type":                 {},
		"modal_verb_signal":          {},
		"negation_marker":            {},
		"best_mode":                  {},
		"best_shard":                 {},
		"context_category_priority":  {},
		"state_adjective":            {},
		"tool_priority":              {},
	}
}
