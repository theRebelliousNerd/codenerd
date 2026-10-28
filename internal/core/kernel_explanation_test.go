package core

import (
	"strings"
	"testing"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/provenance"
	"github.com/stretchr/testify/require"
)

func explanationKernel(t *testing.T) *RealKernel {
	t.Helper()
	k := setupMockKernel(t)
	k.AppendPolicy(`Decl edge(A, B) bound [/name, /name].
Decl reach(A, B) bound [/name, /name].
reach(A, B) :- edge(A, B).
reach(A, C) :- edge(A, B), reach(B, C).`)
	require.NoError(t, k.Evaluate())
	return k
}

func TestExplainEnvelope_ArmedButUnrecordedIsNotNoProof(t *testing.T) {
	k := explanationKernel(t)
	require.NoError(t, k.Assert(Fact{Predicate: "edge", Args: []any{MangleAtom("/a"), MangleAtom("/b")}}))
	_, err := k.Query("reach")
	require.NoError(t, err)

	k.EnableProvenance()
	env, err := k.ExplainEnvelope("reach(/a, /b)", ExplainOptions{})
	require.NoError(t, err)
	require.False(t, env.RecordedThisEpoch, "no evaluation has run since recording was armed")
	require.Empty(t, env.Proofs)
	require.NotEmpty(t, env.ProgramDigest)

	require.NoError(t, k.Evaluate())
	env, err = k.ExplainEnvelope("reach(/a, /b)", ExplainOptions{})
	require.NoError(t, err)
	require.True(t, env.RecordedThisEpoch)
	require.NotEmpty(t, env.Proofs)
	gen := env.RecorderGeneration

	// A new evaluation is a new epoch.
	require.NoError(t, k.Assert(Fact{Predicate: "edge", Args: []any{MangleAtom("/b"), MangleAtom("/c")}}))
	env, err = k.ExplainEnvelope("reach(/a, /c)", ExplainOptions{})
	require.NoError(t, err)
	require.Greater(t, env.RecorderGeneration, gen)
	require.NotEmpty(t, env.Proofs)

	// The proof facts carry the epoch row and EmitFacts' schema, in their own store.
	var meta []ast.Atom
	require.NoError(t, env.ProofFacts.GetFacts(ast.NewQuery(ast.PredicateSym{Symbol: "explanation", Arity: 3}), func(a ast.Atom) error {
		meta = append(meta, a)
		return nil
	}))
	require.Len(t, meta, 1)
	var proves int
	require.NoError(t, env.ProofFacts.GetFacts(ast.NewQuery(ast.PredicateSym{Symbol: "proves", Arity: 2}), func(ast.Atom) error {
		proves++
		return nil
	}))
	require.Positive(t, proves)
	rows, err := k.Query("proves")
	require.NoError(t, err)
	require.Empty(t, rows, "proof facts must never enter the execution kernel")
}

func TestExplainEnvelope_BudgetsTruncateAndMarkPartial(t *testing.T) {
	k := explanationKernel(t)
	k.EnableProvenance()
	chain := []string{"/a", "/b", "/c", "/d", "/e", "/f"}
	for i := 0; i+1 < len(chain); i++ {
		require.NoError(t, k.Assert(Fact{Predicate: "edge", Args: []any{MangleAtom(chain[i]), MangleAtom(chain[i+1])}}))
	}
	full, err := k.ExplainEnvelope("reach(/a, /f)", ExplainOptions{})
	require.NoError(t, err)
	require.False(t, full.Truncated)

	small, err := k.ExplainEnvelope("reach(/a, /f)", ExplainOptions{MaxNodes: 3})
	require.NoError(t, err)
	require.True(t, small.Truncated)
	require.LessOrEqual(t, len(small.Nodes), 3)
	cut := false
	for _, st := range small.Nodes {
		if st.Reason == PartialHostTruncated {
			cut = true
		}
	}
	require.True(t, cut, "a node whose premises were cut is marked partial with the host's reason")
	var partial int
	require.NoError(t, small.ProofFacts.GetFacts(ast.NewQuery(ast.PredicateSym{Symbol: "proof_partial", Arity: 2}), func(ast.Atom) error {
		partial++
		return nil
	}))
	require.Positive(t, partial, "Partial survives into the proof facts EmitFacts alone drops it from")

	tiny, err := k.ExplainEnvelope("reach(/a, /f)", ExplainOptions{MaxBytes: full.ProofFactBytes / 3})
	require.NoError(t, err)
	require.True(t, tiny.Truncated)
	require.Less(t, tiny.ProofFactBytes, full.ProofFactBytes)
	require.True(t, strings.Contains(strings.Join(tiny.TruncationReasons, ";"), "byte budget"))
}

func TestPartialReason(t *testing.T) {
	require.Equal(t, "", partialReason(&provenance.ProofNode{}))
	require.Equal(t, PartialDepthLimit, partialReason(&provenance.ProofNode{ID: "/proof/partial/x", Partial: true}))
	child := &provenance.ProofNode{ID: "/proof/c", Partial: true}
	require.Equal(t, PartialPremise, partialReason(&provenance.ProofNode{ID: "/proof/p", Partial: true, Premises: []*provenance.ProofNode{child}}))
	require.Equal(t, PartialUnrepresented, partialReason(&provenance.ProofNode{ID: "/proof/u", Partial: true}))
}

func TestCapabilityManifest_TeachesOnlyWhatTheHostPreserves(t *testing.T) {
	m := realKernelManifest(qualifiedMangleVersion, "digest", false)
	g := m.GeneratedRuleGuidance()
	require.Contains(t, g, "Decl bounds are enforced on every fact")
	require.Contains(t, g, "Do not use temporal syntax")
	require.NotContains(t, g, "unverified on")
	byName := map[string]MangleFeature{}
	for _, f := range m.Features {
		byName[f.Name] = f
	}
	require.False(t, byName["temporal_evaluation"].Teach)
	require.Equal(t, FeatureNotWired, byName["temporal_evaluation"].State)
	require.Equal(t, FeatureImplementedOptIn, byName["derivation_recording"].State)

	// On another dependency version nothing qualified here is taught.
	other := realKernelManifest("v0.8.0", "digest", false)
	for _, f := range other.Features {
		require.False(t, f.Teach, f.Name)
	}
	require.Contains(t, other.GeneratedRuleGuidance(), "unverified on v0.8.0")
}

func TestCapabilityManifest_StampsTheLoadedProgram(t *testing.T) {
	k := explanationKernel(t)
	m := k.CapabilityManifest()
	require.Equal(t, "RealKernel", m.HostPath)
	require.Equal(t, mangleModulePath, m.Dependency)
	require.Len(t, m.ProgramDigest, 64)
	// The linked dependency is the version the feature states were qualified
	// on; a go.mod bump without requalification shows up here.
	if v := mangleDependencyVersion(); v != "unknown" {
		require.Equal(t, qualifiedMangleVersion, v)
	}
	k.AppendPolicy(`Decl extra(A) bound [/name].`)
	require.NoError(t, k.Evaluate())
	require.NotEqual(t, m.ProgramDigest, k.CapabilityManifest().ProgramDigest, "a new program is a new digest")
}
