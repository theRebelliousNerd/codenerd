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

// IntentCorpusPredicates are the predicates the embedded intent corpus
// (schema/intent_*.mg) loads as boot facts. Every kernel shard loads them at
// init (loadEmbeddedIntentFacts), so they exist in every shard exactly as
// program facts do: the derivation analysis takes them as program predicates.
// The corpus's other clauses are not kernel rules: only fact clauses of these
// predicates are read from the files, and the rest (the multistep_* tables and
// the pattern_* rules over them) runs in the perception taxonomy engine, a
// single unsharded store.
func IntentCorpusPredicates() map[string]struct{} {
	return defaultIntentFactPredicates()
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
		"state_adjective":            {},
	}
}
