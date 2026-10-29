package prompt

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func (querier *realKernelQuerier) ContentFingerprint() (string, error) {
	return querier.k.ContentFingerprint()
}

func TestDynamicKernelFingerprint_CanonicalContent(test *testing.T) {
	contextRows := []Fact{
		{Predicate: "injectable_context", Args: []any{"coder", "context B"}},
		{Predicate: "injectable_context", Args: []any{"coder", "context A"}},
	}
	knowledgeRows := []Fact{
		{Predicate: "specialist_knowledge", Args: []any{"coder", "topic B", "knowledge B"}},
		{Predicate: "specialist_knowledge", Args: []any{"coder", "topic A", "knowledge A"}},
	}
	kernel := &snapshotPredicateKernel{&predicateKernel{facts: map[string][]Fact{"injectable_context": contextRows, "specialist_knowledge": knowledgeRows}}}
	compiler, err := NewJITPromptCompiler(WithKernel(kernel))
	require.NoError(test, err)
	test.Cleanup(func() { require.NoError(test, compiler.Close()) })
	caller := NewCompilationContext().WithShard("/coder", "coder", "")
	firstAtoms, firstHash, err := compiler.collectDynamicKernelSnapshot(kernel, caller)
	require.NoError(test, err)
	kernel.facts["injectable_context"] = []Fact{contextRows[1], contextRows[0], contextRows[1]}
	kernel.facts["specialist_knowledge"] = []Fact{knowledgeRows[1], knowledgeRows[0], knowledgeRows[1]}
	secondAtoms, secondHash, err := compiler.collectDynamicKernelSnapshot(kernel, caller)
	require.NoError(test, err)
	require.Equal(test, firstHash, secondHash)
	// NewPromptAtom timestamps each materialization. Creation time is metadata,
	// not part of canonical injected content or the snapshot fingerprint, so
	// normalize only that field and compare every other atom field structurally.
	normalizeCreationTime := func(atoms []*PromptAtom) []*PromptAtom {
		normalized := make([]*PromptAtom, len(atoms))
		for i, atom := range atoms {
			copy := *atom
			copy.CreatedAt = time.Time{}
			normalized[i] = &copy
		}
		return normalized
	}
	require.Equal(test, normalizeCreationTime(firstAtoms), normalizeCreationTime(secondAtoms))
	kernel.facts["specialist_knowledge"] = []Fact{{Predicate: "specialist_knowledge", Args: []any{"coder", "topic A", "knowledge changed"}}}
	_, changedHash, err := compiler.collectDynamicKernelSnapshot(kernel, caller)
	require.NoError(test, err)
	require.NotEqual(test, firstHash, changedHash)
}

type snapshotPredicateKernel struct {
	*predicateKernel
}

// ContentFingerprint names the mock's rows as a set: the same rows in any
// order and with repeats name the same content, as a kernel's would.
func (kernel *snapshotPredicateKernel) ContentFingerprint() (string, error) {
	var rows []string
	for predicate, facts := range kernel.facts {
		for _, fact := range facts {
			rows = append(rows, fmt.Sprintf("%s %q %v", predicate, fact.Predicate, fact.Args))
		}
	}
	return HashContent(strings.Join(canonicalStringSet(rows), "\n")), nil
}

// predicateKernel is a minimal kernel mock that returns facts by predicate.
type predicateKernel struct {
	facts map[string][]Fact
}

func (k *predicateKernel) Query(predicate string) ([]Fact, error) {
	return k.facts[predicate], nil
}

func (k *predicateKernel) AssertBatch(facts []any) error {
	// No-op for this test.
	return nil
}

func TestCollectKernelInjectedAtoms(t *testing.T) {
	kernel := &predicateKernel{
		facts: map[string][]Fact{
			"injectable_context": {
				{Predicate: "injectable_context", Args: []any{"coder-123", "Ctx A"}},
				{Predicate: "injectable_context", Args: []any{"/coder", "Ctx B"}},
				{Predicate: "injectable_context", Args: []any{"*", "Global Ctx"}},
			},
			"specialist_knowledge": {
				{Predicate: "specialist_knowledge", Args: []any{"coder", "Auth", "Use JWT refresh tokens."}},
			},
		},
	}

	compiler, err := NewJITPromptCompiler(WithKernel(kernel))
	require.NoError(t, err)

	// Provide minimal mandatory skeleton so Compile() can run.
	corpus := NewEmbeddedCorpus([]*PromptAtom{
		func() *PromptAtom {
			a := NewPromptAtom("identity/test/mission", CategoryIdentity, "test identity")
			a.IsMandatory = true
			a.Importance = 100
			return a
		}(),
	})
	compiler.embeddedCorpus = corpus

	cc := NewCompilationContext().
		WithShard("/coder", "coder", "").
		WithOperationalMode("/active")
	cc.ShardInstanceID = "coder-123"

	atoms, err := compiler.collectKernelInjectedAtoms(cc)
	require.NoError(t, err)
	require.Len(t, atoms, 2)

	// Context atom
	require.Equal(t, CategoryContext, atoms[0].Category)
	require.True(t, atoms[0].IsMandatory)
	require.Contains(t, atoms[0].Content, "Ctx A")
	require.Contains(t, atoms[0].Content, "Global Ctx")

	// Knowledge atom
	require.Equal(t, CategoryKnowledge, atoms[1].Category)
	require.True(t, atoms[1].IsMandatory)
	require.Contains(t, atoms[1].Content, "Auth")
	require.Contains(t, atoms[1].Content, "JWT refresh tokens")

	// Ensure compilation still works with these atoms present.
	_, err = compiler.Compile(context.Background(), cc)
	require.NoError(t, err)
}
