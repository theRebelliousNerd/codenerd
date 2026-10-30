package core

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/store"
	"codenerd/internal/tactile"
	"codenerd/internal/types"

	"github.com/stretchr/testify/require"
)

func TestHydratedLanguagePreferencePreservesToolActivationIdentity(t *testing.T) {
	db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	k, err := NewRealKernel()
	require.NoError(t, err)
	require.NoError(t, k.AssertBatch([]Fact{
		{Predicate: "tool_capabilities", Args: []any{MangleAtom("/write_file"), MangleAtom("/code_generation")}},
		{Predicate: "tool_language", Args: []any{MangleAtom("/write_file"), MangleAtom("/go")}},
		{Predicate: "knowledge_link", Args: []any{types.MangleString("/workspace/file.go"), MangleAtom("/related_to"), "related.go"}},
	}))
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	vs.SetLocalDB(db)
	vs.SetKernel(k)

	// An unrelated stored preference must not activate the language rule.
	require.NoError(t, db.StoreFact("prefer_indent", []any{"tabs"}, "preference", 10))
	_, err = vs.HydrateLearnings(context.Background())
	require.NoError(t, err)
	rows, err := k.Query(`activation(/write_file, 85)`)
	require.NoError(t, err)
	require.Empty(t, rows)

	require.NoError(t, db.StoreFact("prefer_language", []any{"go"}, "preference", 10))
	require.NoError(t, db.LogActivation("/workspace/file.go", 0.9))
	_, err = vs.HydrateLearnings(context.Background())
	require.NoError(t, err)
	for _, query := range []string{
		`learned_preference("prefer_language", "go")`,
		`activation(/write_file, 85)`,
		`activation("/workspace/file.go", 90)`,
		`activation("related.go", 60)`,
		`context_atom("/write_file")`,
		`context_priority("/write_file", 80)`,
		`should_include_context("/write_file", /p60)`,
	} {
		rows, err := k.Query(query)
		require.NoError(t, err, query)
		require.Len(t, rows, 1, query)
	}
	rows, err = k.Query(`activation("/write_file", 85)`)
	require.NoError(t, err)
	require.Empty(t, rows, "rendering context must not change the tool's activation identity")
}

func TestActivationAdmitsNamesAndStringsAndRendersContext(t *testing.T) {
	k, err := NewRealKernel()
	require.NoError(t, err)
	require.NoError(t, k.AssertBatch([]Fact{
		{Predicate: "activation", Args: []any{MangleAtom("/review_tool"), int64(85)}},
		{Predicate: "activation", Args: []any{types.MangleString("/workspace/file.go"), int64(90)}},
		{Predicate: "activation", Args: []any{"cold.go", int64(30)}},
		{Predicate: "active_goal", Args: []any{"review goal"}},
		{Predicate: "goal_requires", Args: []any{"review goal", MangleAtom("/code_generation")}},
		{Predicate: "tool_capabilities", Args: []any{MangleAtom("/write_file"), MangleAtom("/code_generation")}},
	}))
	for _, query := range []string{
		`activation(/review_tool, 85)`,
		`activation("/workspace/file.go", 90)`,
		`activation(/write_file, 80)`,
		`context_atom("/review_tool")`,
		`context_atom("/workspace/file.go")`,
		`context_priority("/review_tool", 80)`,
		`context_priority("/workspace/file.go", 80)`,
	} {
		rows, err := k.Query(query)
		require.NoError(t, err, query)
		require.Len(t, rows, 1, query)
	}
	rows, err := k.Query(`context_atom("cold.go")`)
	require.NoError(t, err)
	require.Empty(t, rows)
	for _, args := range [][]any{
		{int64(7), int64(85)},
		{types.MangleString("/workspace/file.go"), "85"},
		{MangleAtom("/review_tool"), "85"},
	} {
		require.Error(t, k.Assert(Fact{Predicate: "activation", Args: args}),
			"each bound alternative must enforce both identity and numeric score: %v", args)
	}
}

func TestReviewKnowledgeAndLearningProduceDeclaredStrings(t *testing.T) {
	k, err := NewRealKernel()
	require.NoError(t, err)
	require.NoError(t, k.AssertBatch([]Fact{
		{Predicate: "user_intent", Args: []any{MangleAtom("/current_intent"), MangleAtom("/query"), MangleAtom("/review"), "parser", ""}},
		{Predicate: "learned_fact", Args: []any{"preference", "use tabs"}},
		{Predicate: "relevant_to_intent", Args: []any{"preference", MangleAtom("/review")}},
		{Predicate: "campaign_phase", Args: []any{"phase-review", "campaign-review", "Verify", int64(1), MangleAtom("/completed"), "profile"}},
		{Predicate: "phase_objective", Args: []any{"phase-review", MangleAtom("/test"), "verify", MangleAtom("/automated")}},
		{Predicate: "rejection", Args: []any{"task-review", MangleAtom("/error"), "check the error"}},
		{Predicate: "coder_rejection_count", Args: []any{MangleAtom("/error"), "check the error", int64(2)}},
		{Predicate: "successful_edit", Args: []any{"ref-a", MangleAtom("/replace")}},
		{Predicate: "successful_edit", Args: []any{"ref-b", MangleAtom("/replace")}},
		{Predicate: "successful_edit", Args: []any{"ref-c", MangleAtom("/replace")}},
		{Predicate: "missing_tool_for", Args: []any{MangleAtom("/review_intent"), MangleAtom("/review_capability")}},
	}))
	for _, query := range []string{
		`context_atom("preference: use tabs")`,
		`coder_error_pattern("check the error")`,
		`promote_to_long_term(/phase_pattern, "/test")`,
		`promote_to_long_term(/edit_pattern, "/replace")`,
		`tool_lifecycle("/review_capability", /detected)`,
		`delegate_task(/tool_generator, "/review_capability", /pending)`,
	} {
		rows, err := k.Query(query)
		require.NoError(t, err, query)
		require.Len(t, rows, 1, query)
	}
	rows, err := k.Query(`context_atom("preference: unrelated")`)
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestBuildDiagnosticsEnterTheKernelAtDeclaredArityAndTypes(t *testing.T) {
	k, err := NewRealKernel()
	require.NoError(t, err)
	vs := NewVirtualStore(&mockActionsExecutor{executeFunc: func(context.Context, tactile.Command) (*tactile.ExecutionResult, error) {
		return &tactile.ExecutionResult{ExitCode: 1, Stderr: "a.go:12:3: undefined: symbol\nb.go:9:1: warning: unused value\na.go:bad:3: error: malformed location"}, nil
	}})
	result, err := vs.handleBuildProject(context.Background(), ActionRequest{})
	require.NoError(t, err)
	require.False(t, result.Success)
	require.Len(t, result.FactsToAdd, 3) // build result and two valid diagnostics
	require.NoError(t, k.AssertBatch(result.FactsToAdd))
	for _, query := range []string{
		`diagnostic(/error, "a.go", 12, "", "undefined: symbol")`,
		`diagnostic(/warning, "b.go", 9, "", "warning: unused value")`,
	} {
		rows, err := k.Query(query)
		require.NoError(t, err)
		require.Len(t, rows, 1, query)
	}
}

func TestWorkflowHandlersEmitDeclaredFactsWithoutTruncatingSummary(t *testing.T) {
	k, err := NewRealKernel()
	require.NoError(t, err)
	vs := NewVirtualStore(nil)
	clarification, err := vs.handleCampaignClarify(context.Background(), ActionRequest{
		Target: "Which branch?", Payload: map[string]any{"campaign_id": "campaign-review"},
	})
	require.NoError(t, err)
	require.NoError(t, k.AssertBatch(clarification.FactsToAdd))
	rows, err := k.Query(`campaign_awaiting_clarification("campaign-review")`)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Which branch?", clarification.Output)

	summary := "/" + strings.Repeat("completed with evidence ", 50)
	complete, err := vs.handleComplete(context.Background(), ActionRequest{
		Target: "task-review", Payload: map[string]any{"summary": summary},
	})
	require.NoError(t, err)
	require.NoError(t, k.AssertBatch(complete.FactsToAdd))
	rows, err = k.Query(`observation("/task_summary", Summary)`)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, summary, types.ExtractString(rows[0].Args[1]))
}

func TestAgendaUsesNumericPriorityWithoutChangingCampaignPriorityVocabulary(t *testing.T) {
	k, err := NewRealKernel()
	require.NoError(t, err)
	require.NoError(t, k.AssertBatch([]Fact{
		{Predicate: "agenda_item", Args: []any{"agenda-low", "later", int64(10), MangleAtom("/pending"), int64(1)}},
		{Predicate: "agenda_item", Args: []any{"agenda-high", "first", int64(90), MangleAtom("/pending"), int64(1)}},
	}))
	rows, err := k.Query(`next_agenda_item("agenda-high")`)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	rows, err = k.Query(`next_agenda_item("agenda-low")`)
	require.NoError(t, err)
	require.Empty(t, rows)
	rows, err = k.Query(`priority_higher(/critical, /low)`)
	require.NoError(t, err)
	require.Len(t, rows, 1)
}
