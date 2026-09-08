package store

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLearningContent_QualificationRedactionAndBounds(t *testing.T) {
	ls, err := NewLearningStore(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = ls.Close() })
	body := strings.Repeat("measurement ", 70) + " qualification; api_key=confidentialvalue"
	require.NoError(t, ls.Save("expert", "learned_pattern", []any{"zephyrquark", body}, "campaign"))
	hits, err := ls.RecallLearningsLexicalContext(t.Context(), "zephyrquark", 1)
	require.NoError(t, err)
	require.Len(t, hits, 1)
	require.NotContains(t, hits[0].Summary, "qualification", "negative control: handle is short")
	content, err := ls.RecallLearningContentContext(t.Context(), hits[0])
	require.NoError(t, err)
	require.Contains(t, content, "qualification")
	require.Contains(t, content, "[redacted]")
	require.NotContains(t, content, "confidentialvalue")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = ls.RecallLearningContentContext(ctx, hits[0])
	require.ErrorIs(t, err, context.Canceled)

	require.NoError(t, ls.Save("expert", "learned_pattern", []any{"oversizedquux", strings.Repeat("x", 17000)}, "campaign"))
	hits, err = ls.RecallLearningsLexicalContext(t.Context(), "oversizedquux", 1)
	require.NoError(t, err)
	require.Len(t, hits, 1)
	_, err = ls.RecallLearningContentContext(t.Context(), hits[0])
	require.ErrorContains(t, err, "exceeds recall content limit")
}
