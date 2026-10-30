package prompt

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Project lexical fallback kept only the expanded query's first eight terms,
// and the expansion leads with generated synonyms: for /fix on
// database/checkpoint.go with task "wal checkpoint stalls" those were
// "fix debug error issue resolve bug database db", so the one relevant row,
// "wal checkpoint", never reached SQL (audit
// project-knowledge-lexical-term-budget). The task's own terms come first in
// the lexical plan; the semantic query is unchanged.
func TestProjectKnowledge_LexicalFallbackKeepsTheTaskTerms(t *testing.T) {
	project := createProjectKnowledgeStore(t, "wal checkpoint", "Passive checkpoints can stall behind long-lived readers.")
	t.Cleanup(func() { project.Close() })
	compiler := bridgeCompiler(t, project, nil)

	cc := NewCompilationContext()
	cc.IntentVerb = "/fix"
	cc.IntentTarget = "database/checkpoint.go"
	cc.ShardID = "coder"
	cc.SemanticQuery = "wal checkpoint stalls"

	got := compiler.collectKnowledgeAtoms(context.Background(), cc)
	require.True(t, containsKnowledgeMarker(got, "long-lived readers"),
		"the project row matching the task's own terms was not retrieved")
}

// The plan: target and task terms first, deduplicated, stopwords dropped,
// then the expansion; the expanded (semantic) query keeps its order.
func TestBuildLexicalQuery_TaskTermsLead(t *testing.T) {
	cc := NewCompilationContext()
	cc.IntentVerb = "/fix"
	cc.IntentTarget = "database/checkpoint.go"
	cc.ShardID = "coder"
	cc.SemanticQuery = "the wal checkpoint stalls"
	lex := strings.Fields(buildLexicalQuery(cc))
	require.GreaterOrEqual(t, len(lex), 8)
	require.Equal(t, []string{"database", "checkpoint", "go", "wal", "stalls"}, lex[:5])
	require.True(t, strings.HasPrefix(buildExpandedQuery(cc), "fix "), "the expanded query's order changed")
}
