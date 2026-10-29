package kbfill

import "codenerd/internal/knowledge"

// What each knowledge database must carry. A column not named here is
// required in every row of its table; a table not named here must hold rows
// unless the spec says why it may not (OtherTables).

// sharedStoreSchema is why the LocalStore tables a database does not use may
// be empty: every database opened through store.NewLocalStore gets the whole
// store schema (sessions, traces, world facts, ...), and a KB or corpus uses a
// few of its tables.
const sharedStoreSchema = "the database is opened through store.NewLocalStore, which creates the whole store schema; this database does not use that table"

// selectorMirror is why a prompt atom's per-dimension selector columns may be
// empty: the authored selectors are rows of atom_context_tags, which the
// compiler reads (internal/prompt/compiler_db.go); an atom with no selector in
// a dimension is unconstrained in it, and most atoms constrain few dimensions.
const selectorMirror = "a selector dimension: absent means the atom is unconstrained in it; authored selectors are atom_context_tags rows"

// promptAtomColumns is the column spec prompt_atoms shares between the shipped
// prompt corpus and every shard agent's KB.
func promptAtomColumns() map[string]Column {
	cols := map[string]Column{
		"importance":     {Optional: "an unscored atom has no importance (schemas_prompts.mg atom_importance)"},
		"is_exclusive":   {Optional: "an exclusion group is declared only by atoms that have one"},
		"depends_on":     {Optional: "a dependency is declared only by atoms that have one (atom_context_tags depends_on rows)"},
		"conflicts_with": {Optional: "a conflict is declared only by atoms that have one (atom_context_tags conflicts_with rows)"},
	}
	for _, dim := range []string{"operational_modes", "campaign_phases", "build_layers", "init_phases",
		"northstar_phases", "ouroboros_stages", "intent_verbs", "shard_types", "languages", "frameworks", "world_states"} {
		cols[dim] = Column{Optional: selectorMirror}
	}
	return cols
}

// PromptCorpus is internal/core/defaults/prompt_corpus.db, the shipped atom
// corpus (cmd/tools/prompt_builder).
var PromptCorpus = Spec{
	Name: "prompt_corpus",
	Tables: map[string]Table{
		"prompt_atoms": {Columns: promptAtomColumns()},
	},
	OtherTables: sharedStoreSchema,
}

// PredicateCorpus is internal/core/defaults/predicate_corpus.db, the schema
// reference the repair and JIT layers read (cmd/tools/predicate_corpus_builder).
var PredicateCorpus = Spec{
	Name: "predicate_corpus",
	Tables: map[string]Table{
		"predicates":         {},
		"predicate_args":     {},
		"predicate_domains":  {},
		"error_patterns":     {},
		"predicate_examples": {},
	},
}

// IntentCorpus is internal/core/defaults/intent_corpus.db, the perception
// corpus's embedded facts. Its verb, target and category columns carry the
// argument of that name, which only some fact kinds have.
var IntentCorpus = Spec{
	Name: "intent_corpus",
	Tables: map[string]Table{
		"corpus_embeddings": {Columns: map[string]Column{
			"verb":     {When: "predicate LIKE 'verb_%'"},
			"target":   {When: "predicate = 'verb_composition'"},
			"category": {When: "predicate IN ('verb_def', 'build_phase_type', 'completion_marker', 'constraint_marker', 'iterative_marker', 'layer_priority', 'priority_marker', 'pronoun_ref', 'step_connector', 'verification_marker')"},
		}},
	},
	OtherTables: sharedStoreSchema,
}

// ShardAgentKB is a shard agent's knowledge base,
// .nerd/shards/<name>_knowledge.db: its prompts.yaml atoms (prompt sync), its
// knowledge (internal/knowledge's sources, ingest) and every source's last
// attempt on every subject. Knowledge that is only the removed seed's
// templated placeholders is no knowledge.
var ShardAgentKB = Spec{
	Name: "shard_agent_kb",
	Tables: map[string]Table{
		"prompt_atoms":    {Columns: promptAtomColumns()},
		"knowledge_atoms": {Filler: knowledge.PlaceholderSQL},
		"vectors":         {},
		"sync_metadata":   {},
		"knowledge_acquisitions": {Columns: map[string]Column{
			"detail": {When: "status IN ('failed', 'unavailable')"},
		}},
	},
	OtherTables: sharedStoreSchema,
}
