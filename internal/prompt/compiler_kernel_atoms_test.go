package prompt

import (
	"context"
	"testing"

	"codenerd/internal/core"

	"github.com/stretchr/testify/require"
)

func (querier *realKernelQuerier) QueryAll() (map[string][]Fact, error) {
	return promptSnapshotFacts(querier.k)
}

func promptSnapshotFacts(kernel *core.RealKernel) (map[string][]Fact, error) {
	rows, err := kernel.QueryAll()
	if err != nil {
		return nil, err
	}
	snapshot := make(map[string][]Fact, len(rows))
	for predicate, facts := range rows {
		snapshot[predicate] = make([]Fact, 0, len(facts))
		for _, fact := range facts {
			snapshot[predicate] = append(snapshot[predicate], Fact{Predicate: fact.Predicate, Args: fact.Args})
		}
	}
	return snapshot, nil
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
	require.Equal(test, firstAtoms, secondAtoms)
	kernel.facts["specialist_knowledge"] = []Fact{{Predicate: "specialist_knowledge", Args: []any{"coder", "topic A", "knowledge changed"}}}
	_, changedHash, err := compiler.collectDynamicKernelSnapshot(kernel, caller)
	require.NoError(test, err)
	require.NotEqual(test, firstHash, changedHash)
}

type snapshotPredicateKernel struct {
	*predicateKernel
}

func (kernel *snapshotPredicateKernel) QueryAll() (map[string][]Fact, error) {
	return kernel.facts, nil
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
			a.Priority = 100
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
