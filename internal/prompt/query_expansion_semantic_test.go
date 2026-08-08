package prompt

import (
	"strings"
	"testing"
)

// The defect these guard (F-OURO-4, observed live): Ouroboros was asked to
// "count mangle predicates given a directory path" and generated a counter
// that strips // and /* */ comments — C syntax — in a repo whose 244 .mg files
// use #. It counted every commented-out "# Decl ..." line as a real
// declaration and over-reported the corpus by 2.5x (168 vs 67).
//
// The tool compiled, passed the safety check, survived the Thunderdome, and
// registered. Nothing caught it, because the generated tests carried the same
// misconception as the generated code.
//
// Root cause was retrieval starvation, not model error. buildExpandedQuery
// feeds the knowledge-base search over 29,174 embedded entries, and it read
// only IntentVerb / IntentTarget / ShardID / Language / Frameworks. Ouroboros
// sets none of those. The single word that would have retrieved Mangle syntax
// — "mangle" — was present, buried inside an underscore-joined shard ID that
// was added to the query as one 60-character term.

func TestBuildExpandedQuery_TokenizesShardID(t *testing.T) {
	cc := &CompilationContext{
		ShardID: "tool_generator_count_mangle_predicates_given_a_directory_path_s",
	}

	query := buildExpandedQuery(cc)

	if !strings.Contains(query, "mangle") {
		t.Errorf("query does not contain the word that would retrieve Mangle syntax: %q", query)
	}
	if strings.Contains(query, "tool_generator_count_mangle") {
		t.Errorf("shard ID was embedded whole rather than tokenized: %q", query)
	}
}

// Simple IDs must survive tokenization unchanged, or every existing caller's
// retrieval shifts as a side effect of this fix.
func TestBuildExpandedQuery_SimpleShardIDIsUnchanged(t *testing.T) {
	cc := &CompilationContext{ShardID: "coder"}

	if got := buildExpandedQuery(cc); got != "coder" {
		t.Errorf("buildExpandedQuery for a simple shard ID = %q, want %q", got, "coder")
	}
}

// SemanticQuery was consumed by atom selection but never reached the
// knowledge-base search, so a caller describing its task in prose had that
// prose silently dropped.
func TestBuildExpandedQuery_IncludesSemanticQuery(t *testing.T) {
	cc := &CompilationContext{
		ShardID:       "tool_generator",
		SemanticQuery: "count mangle predicates given a directory path",
	}

	query := buildExpandedQuery(cc)

	for _, want := range []string{"count", "mangle", "predicates", "directory"} {
		if !strings.Contains(query, want) {
			t.Errorf("query %q is missing %q from the caller's prose description", query, want)
		}
	}
}

// Stop words must still be filtered out of the prose, or the query degrades
// into common-word noise that matches everything.
func TestBuildExpandedQuery_SemanticQueryStillDropsStopWords(t *testing.T) {
	cc := &CompilationContext{
		ShardID:       "tool_generator",
		SemanticQuery: "count the predicates in a directory",
	}

	query := buildExpandedQuery(cc)
	for _, term := range strings.Fields(query) {
		if _, isStop := stopWords[term]; isStop {
			t.Errorf("stop word %q survived into the query: %q", term, query)
		}
	}
}

// An empty context must still produce an empty query — collectKnowledgeAtoms
// skips the search entirely on "", and searching on noise is worse than not
// searching.
func TestBuildExpandedQuery_EmptyContextStaysEmpty(t *testing.T) {
	if got := buildExpandedQuery(&CompilationContext{}); got != "" {
		t.Errorf("empty context produced query %q, want empty", got)
	}
}
