package prompt

import (
	"strings"
	"testing"

	"codenerd/internal/store"
	"github.com/stretchr/testify/require"
)

func TestLearningEvidence_FreshFactKeepsQualificationThroughBudget(t *testing.T) {
	dir := t.TempDir()
	ls, err := store.NewLearningStore(dir)
	require.NoError(t, err)
	body := strings.Repeat("bounded measurement detail; ", 40) + "QUALIFICATION: supervised development; no scale claim."
	require.NoError(t, ls.Save("expert", "learned_pattern", []any{"zephyrquark", body}, "campaign"))
	require.NoError(t, ls.Close())
	ls, err = store.NewLearningStore(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ls.Close() })
	c := bridgeCompiler(t, nil, nil)
	c.SetLearningStore(ls)
	cc := bridgeContext("expert", "zephyrquark")
	cc.IntentVerb = "/fix" // expanded synonyms must not consume lexical keywords
	cc.IntentTarget = "zephyrquark"
	atoms := c.collectLearningAtoms(t.Context(), cc)
	require.Len(t, atoms, 1)
	require.Contains(t, atoms[0].Content, body, "search descriptors must not replace saved evidence")
	require.Contains(t, atoms[0].Content, `source_campaign="campaign"`)

	mgr := NewTokenBudgetManager()
	mgr.SetCategoryBudget(CategoryBudget{Category: CategoryKnowledge, MinTokens: 20, MaxTokens: 20, Priority: PriorityMedium})
	fitted, err := mgr.Fit([]*OrderedAtom{{Atom: atoms[0], Score: 1}}, 4000)
	require.NoError(t, err)
	require.Len(t, fitted, 1, "whole evidence must use available second-pass space")
	require.Equal(t, atoms[0].Content, fitted[0].Atom.Content)

	tooSmall, err := mgr.Fit([]*OrderedAtom{{Atom: atoms[0], Score: 1}}, 550)
	require.NoError(t, err)
	require.Empty(t, tooSmall, "omit evidence when its qualification cannot fit")
	require.Contains(t, atoms[0].Content, body, "fitting must not mutate persisted content")
	cc.IntentTarget, cc.SemanticQuery = "absentquux", "absentquux"
	require.Empty(t, c.collectLearningAtoms(t.Context(), cc), "missing records stay missing")
}

func TestLearningEvidence_HybridIdentityAndLimit(t *testing.T) {
	fresh := store.LearningRecallHit{ShardType: "expert", LearningID: 1}
	old := store.LearningRecallHit{ShardType: "sibling", LearningID: 1}
	other := store.LearningRecallHit{ShardType: "expert", LearningID: 2}
	hits := mergeLearningRecallHits([]store.LearningRecallHit{fresh, other}, []store.LearningRecallHit{old, fresh}, 3)
	require.Equal(t, []store.LearningRecallHit{fresh, old, other}, hits)
	require.Len(t, mergeLearningRecallHits([]store.LearningRecallHit{fresh, other}, []store.LearningRecallHit{old}, 2), 2)
}
