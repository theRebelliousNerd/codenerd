package prompt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// predicateKernel is a minimal kernel mock that returns facts by predicate.
type predicateKernel struct {
	facts map[string][]Fact
}

func (k *predicateKernel) Query(predicate string) ([]Fact, error) {
	return k.facts[predicate], nil
}

func (k *predicateKernel) AssertBatch(facts []interface{}) error {
	// No-op for this test.
	return nil
}

func TestCollectKernelInjectedAtoms(t *testing.T) {
	kernel := &predicateKernel{
		facts: map[string][]Fact{
			"injectable_context": {
				{Predicate: "injectable_context", Args: []interface{}{"coder-123", "Ctx A"}},
				{Predicate: "injectable_context", Args: []interface{}{"/coder", "Ctx B"}},
				{Predicate: "injectable_context", Args: []interface{}{"*", "Global Ctx"}},
			},
			"specialist_knowledge": {
				{Predicate: "specialist_knowledge", Args: []interface{}{"coder", "Auth", "Use JWT refresh tokens."}},
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
