package context

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools"
	"codenerd/internal/world/codemodel"

	"github.com/stretchr/testify/require"
)

// referenceRevision is the one-entity-at-a-time revision the batch replaced:
// a whole read (and, for an element, a whole parse) per entity.
func referenceRevision(root, entity string) string {
	if entity == "" {
		return workingRevisionNonFile
	}
	file, key, element := strings.Cut(entity, elementSeparator)
	path, err := tools.ResolveWorkspacePath(context.Background(), root, file)
	if err != nil {
		return workingRevisionUnavailable
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return workingRevisionUnavailable
	}
	if !element {
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:])
	}
	model, ok := codemodel.Parse(file, string(data))
	if !ok {
		return workingRevisionUnavailable
	}
	for i := range model.Elements {
		if model.Elements[i].Key == key {
			return model.Elements[i].Revision
		}
	}
	return workingRevisionAbsent
}

const revisionFixtureGo = `package a

// One is documented.
func One() int { return 1 }

func Two() int { return 2 }

type Three struct{ X int }
`

// A file whose tail does not parse: the elements before the break are still
// retained, and must keep their revisions.
const revisionFixtureBroken = `package b

func Kept() int { return 1 }

func Broken( {
`

func elementKeys(t *testing.T, file, src string) []string {
	t.Helper()
	model, ok := codemodel.Parse(file, src)
	require.True(t, ok)
	var keys []string
	for _, e := range model.Elements {
		keys = append(keys, e.Key)
	}
	require.NotEmpty(t, keys)
	return keys
}

// The batch gives every entity exactly what reading it alone gave: digests,
// element revisions, absent keys, a partly parsed file's retained elements,
// unsupported languages, missing files, escapes and the nonfile entity.
func TestWorkingRevisions_BatchEqualsOneAtATime(t *testing.T) {
	t.Parallel()
	w, root := ledgerSet(t, 4096, 1)
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte(revisionFixtureGo), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "b.go"), []byte(revisionFixtureBroken), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "notes.txt"), []byte("plain text"), 0600))

	entities := []string{"", "a.go", "b.go", "notes.txt", "missing.go", "../outside.go",
		ElementEntity("a.go", "no-such-key"), ElementEntity("notes.txt", "k"), ElementEntity("missing.go", "k"),
		ElementEntity("b.go", "no-such-key")}
	for _, key := range elementKeys(t, "a.go", revisionFixtureGo) {
		entities = append(entities, ElementEntity("a.go", key))
	}
	brokenKeys := elementKeys(t, "b.go", revisionFixtureBroken)
	for _, key := range brokenKeys {
		entities = append(entities, ElementEntity("b.go", key))
	}

	got := w.revisions(t.Context(), entities)
	require.Len(t, got, len(entities))
	for _, entity := range entities {
		require.Equal(t, referenceRevision(root, entity), got[entity], "entity %q", entity)
	}
	require.Equal(t, workingRevisionNonFile, got[""])
	require.Equal(t, workingRevisionUnavailable, got["missing.go"])
	require.Equal(t, workingRevisionUnavailable, got[ElementEntity("notes.txt", "k")], "an unsupported language has no element revisions")
	require.Equal(t, workingRevisionAbsent, got[ElementEntity("a.go", "no-such-key")])
	retained := 0
	for _, key := range brokenKeys {
		if r := got[ElementEntity("b.go", key)]; r != workingRevisionUnavailable && r != workingRevisionAbsent {
			retained++
		}
	}
	require.Positive(t, retained, "a syntax error does not make the elements it left standing unavailable")
}

// An edit elsewhere in the file leaves an element's revision alone; removing
// the element makes it absent -- observed in one batch, as one at a time.
func TestWorkingRevisions_ElementIdentitySurvivesUnrelatedEdits(t *testing.T) {
	t.Parallel()
	w, root := ledgerSet(t, 4096, 1)
	path := filepath.Join(root, "a.go")
	require.NoError(t, os.WriteFile(path, []byte(revisionFixtureGo), 0600))
	keys := elementKeys(t, "a.go", revisionFixtureGo)
	var entities []string
	for _, key := range keys {
		entities = append(entities, ElementEntity("a.go", key))
	}
	before := w.revisions(t.Context(), append(entities, "a.go"))

	edited := strings.Replace(revisionFixtureGo, "return 2", "return 22", 1)
	require.NoError(t, os.WriteFile(path, []byte(edited), 0600))
	after := w.revisions(t.Context(), append(entities, "a.go"))
	require.NotEqual(t, before["a.go"], after["a.go"], "the file digest moves with any edit")
	changed := 0
	for _, entity := range entities {
		if before[entity] != after[entity] {
			changed++
			require.Contains(t, entity, "Two")
		}
	}
	require.Equal(t, 1, changed, "only the edited element's revision moves")

	removed := strings.Replace(revisionFixtureGo, "func Two() int { return 2 }\n", "", 1)
	require.NoError(t, os.WriteFile(path, []byte(removed), 0600))
	gone := w.revisions(t.Context(), entities)
	for _, entity := range entities {
		if strings.Contains(entity, "Two") {
			require.Equal(t, workingRevisionAbsent, gone[entity])
		}
	}
}

// A canceled epoch claims no revision it did not read.
func TestWorkingRevisions_CanceledEpochIsUnavailable(t *testing.T) {
	t.Parallel()
	w, root := ledgerSet(t, 4096, 1)
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte(revisionFixtureGo), 0600))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got := w.revisions(ctx, []string{"a.go", ElementEntity("a.go", "x")})
	for entity, revision := range got {
		require.Equal(t, workingRevisionUnavailable, revision, entity)
	}
}

// The ledger decides on one epoch and hands it back: the revisions the
// policy judged staleness against, and the extra entities the harness asked
// to observe beside them (the focus).
func TestWorkingLedger_ReturnsTheEpochItDecidedOn(t *testing.T) {
	t.Parallel()
	w, root := ledgerSet(t, 1<<20, 1, "a.go", "focus.go")
	keys := elementKeys(t, "a.go", revisionFixtureGo)
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte(revisionFixtureGo), 0600))
	var entries []LedgerEntry
	for i, key := range keys {
		entries = append(entries, observe(t, w, fmt.Sprintf("e%d", i), ElementEntity("a.go", key), "get_element/"+key, 1, 10, 0, 0))
	}
	decision, err := w.Ledger(t.Context(), LedgerRound{Room: unboundedRoom, Entries: entries, Round: 1, Observe: []string{"focus.go"}})
	require.NoError(t, err)
	for _, key := range keys {
		entity := ElementEntity("a.go", key)
		require.Equal(t, referenceRevision(root, entity), decision.Revisions[entity])
	}
	require.Equal(t, referenceRevision(root, "focus.go"), decision.Revisions["focus.go"])
}

func BenchmarkWorkingRevisions_ManyElementsOneFile(b *testing.B) {
	root := b.TempDir()
	var src strings.Builder
	src.WriteString("package big\n")
	for i := range 200 {
		fmt.Fprintf(&src, "\nfunc F%d() int { return %d }\n", i, i)
	}
	require.NoError(b, os.WriteFile(filepath.Join(root, "big.go"), []byte(src.String()), 0600))
	w := &WorkingSet{root: root}
	resolved, err := tools.CanonicalWorkspaceRoot(root)
	require.NoError(b, err)
	w.root = resolved
	var entities []string
	for i := range 200 {
		entities = append(entities, ElementEntity("big.go", fmt.Sprintf("F%d", i)))
	}
	b.ResetTimer()
	for range b.N {
		_ = w.revisions(context.Background(), entities)
	}
}

// The pre-batch algorithm over the same 200 entities, so the pair reports the
// preparation-cost difference (LEDGER-PERF-04, A against B) on one fixture.
func BenchmarkWorkingRevisions_ManyElementsOneFileReference(b *testing.B) {
	root := b.TempDir()
	var src strings.Builder
	src.WriteString("package big\n")
	for i := range 200 {
		fmt.Fprintf(&src, "\nfunc F%d() int { return %d }\n", i, i)
	}
	require.NoError(b, os.WriteFile(filepath.Join(root, "big.go"), []byte(src.String()), 0600))
	resolved, err := tools.CanonicalWorkspaceRoot(root)
	require.NoError(b, err)
	var entities []string
	for i := range 200 {
		entities = append(entities, ElementEntity("big.go", fmt.Sprintf("F%d", i)))
	}
	b.ResetTimer()
	for range b.N {
		for _, entity := range entities {
			_ = referenceRevision(resolved, entity)
		}
	}
}
