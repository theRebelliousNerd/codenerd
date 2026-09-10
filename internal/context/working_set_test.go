package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorkingSetEvictionRecallAndRevision(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package a"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "b.go"), []byte("package b"), 0600))
	w, err := NewWorkingSet(nil, root, "task")
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	for i := 0; i < 120; i++ {
		entity := "b.go"
		if i == 0 {
			entity = "a.go"
		}
		require.NoError(t, w.Save(t.Context(), WorkingRecord{ID: fmt.Sprint(i), Entity: entity, Revision: w.Revision(entity), Kind: fmt.Sprint(i), Step: int64(i), Body: strings.Repeat("payload ", 80) + fmt.Sprintf(" fact-%d", i)}))
	}
	selected, err := w.Select(t.Context(), "b.go", nil, 1800)
	require.NoError(t, err)
	require.NotContains(t, selected.Text, "fact-0")
	require.LessOrEqual(t, len(selected.Text), 1800)
	recalled, err := w.Select(t.Context(), "a.go", nil, 1800)
	require.NoError(t, err)
	require.Contains(t, recalled.Text, "fact-0", "early fact must return after many intervening observations")
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package changed"), 0600))
	stale, err := w.Select(t.Context(), "a.go", nil, 1800)
	require.NoError(t, err)
	require.NotContains(t, stale.Text, "fact-0", "changed source invalidates old evidence")
	page, err := w.Recall(t.Context(), "0", 0, 1000)
	require.NoError(t, err)
	require.Contains(t, page, "fact-0", "eviction is not deletion")
	other, err := NewWorkingSet(nil, root, "sibling")
	require.NoError(t, err)
	defer other.Close()
	_, err = other.Recall(t.Context(), "0", 0, 1000)
	require.Error(t, err, "sibling scopes cannot recover each other's records")
}
