package prompt

import (
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/core"

	"github.com/stretchr/testify/require"
)

func TestExpandTemplate_InsertedValuesAreDataNotTemplates(t *testing.T) {
	lookup := func(name string) (string, bool) {
		switch name {
		case "roster":
			return "agent says {{secret}}", true
		case "secret":
			return "LEAKED", true
		}
		return "", false
	}
	got := expandTemplate("before {{roster}} after {{unknown}} {{secret}}", lookup)
	require.Equal(t, "before agent says {{secret}} after {{unknown}} LEAKED", got,
		"a placeholder arriving inside an inserted value must stay literal; source placeholders expand once")

	require.Equal(t, "{{ LEAKED", expandTemplate("{{ {{secret}}", lookup))
	require.Equal(t, "open {{never closed", expandTemplate("open {{never closed", lookup))
}

func TestRenderSession_RendersEachFragmentOnceAcrossReassembly(t *testing.T) {
	assembler := NewFinalAssembler()
	var calls atomic.Int64
	assembler.templateEngine.RegisterFunction("costly", func(*CompilationContext, ...string) string {
		calls.Add(1)
		return strings.Repeat("expanded ", 50)
	})
	atom := &PromptAtom{ID: "context/costly", Category: CategoryContext, Content: "x {{costly}} y {{costly}}", TokenCount: 2}
	other := &PromptAtom{ID: "context/plain", Category: CategoryContext, Content: "plain", TokenCount: 2}
	ordered := []*OrderedAtom{{Atom: atom, Order: 0}, {Atom: other, Order: 1}}

	session := assembler.NewRenderSession(NewCompilationContext())
	for range 3 {
		_, err := session.Assemble(ordered)
		require.NoError(t, err)
		_, err = session.Assemble(ordered[:1])
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, calls.Load(), "one value per placeholder per session, however often it is reassembled")
}

func TestRenderSession_FitChargesTheRenderedFragment(t *testing.T) {
	assembler := NewFinalAssembler()
	assembler.templateEngine.RegisterFunction("roster", func(*CompilationContext, ...string) string {
		return strings.Repeat("specialist line\n", 400)
	})
	atom := &PromptAtom{ID: "context/roster", Category: CategoryContext, Content: "{{roster}}", TokenCount: 3}
	session := assembler.NewRenderSession(NewCompilationContext())

	declared := tokenCountForMode(atom, "standard")
	charged := session.Tokens(atom, "standard")
	require.Greater(t, charged, declared*100, "the fitter must see the expanded roster, not the placeholder")
	require.Equal(t, EstimateTokens(session.Fragment(atom, "standard")), charged)

	// The declared count stays the floor for atoms the estimator undercounts.
	padded := &PromptAtom{ID: "context/declared", Category: CategoryContext, Content: "tiny", TokenCount: 500}
	require.Equal(t, 500, session.Tokens(padded, "standard"))
}

// Typed candidate facts must be exactly what parsing their old Mangle text
// produced, for every atom the product ships.
func TestCandidateFacts_TypedFactsEqualTheirParsedText(t *testing.T) {
	corpus, err := getEmbeddedCorpusCached()
	require.NoError(t, err)
	selector := NewAtomSelector()
	forced := map[string]struct{}{}
	atoms := corpus.All()
	if len(atoms) > 0 {
		forced[atoms[0].ID] = struct{}{}
	}
	facts := selector.appendCandidateFacts(nil, atoms, forced)
	require.NotEmpty(t, facts)
	for _, raw := range facts {
		typed, ok := raw.(Fact)
		require.True(t, ok, "candidate fact %v is not typed", raw)
		parsed, err := core.ParseFactString(typed.String())
		require.NoError(t, err, "fact text %q does not parse", typed.String())
		// Compare what the kernel stores: the Mangle atom each converts to.
		want, err := parsed.ToAtom()
		require.NoError(t, err)
		got, err := core.Fact{Predicate: typed.Predicate, Args: typed.Args}.ToAtom()
		require.NoError(t, err, "typed fact %s does not convert", typed.String())
		require.True(t, want.Equals(got), "typed %s converts to %v, its parse to %v", typed.String(), got, want)
	}

	// The cache returns the same static facts on the next compile.
	again := selector.appendCandidateFacts(nil, atoms, forced)
	require.Equal(t, facts, again)
}
