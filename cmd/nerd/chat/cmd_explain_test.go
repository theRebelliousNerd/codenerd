package chat

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/provenance"
)

// TestRenderExplainProofs_EmptyProofs verifies the user-friendly
// "no proofs" path so a /explain on a fact that isn't derivable doesn't
// emit a blank message.
func TestRenderExplainProofs_EmptyProofs(t *testing.T) {
	out := renderExplainProofs("next_action(/do_thing)", nil)
	require.Contains(t, out, "## /explain next_action(/do_thing)",
		"header should echo the goal")
	require.Contains(t, out, "No proofs found",
		"empty proofs must render the documented fallback message")
}

// TestRenderExplainProofs_AllKinds builds a synthetic proof DAG that
// exercises every ProofNode.Kind branch in renderProofNode plus a
// Partial node. We're testing the renderer in isolation — the proof
// tree need not be semantically valid as long as the shape exercises
// every label-emitting branch.
func TestRenderExplainProofs_AllKinds(t *testing.T) {
	// EDB leaf: stored fact, no rule.
	edb := &provenance.ProofNode{
		ID:   "/proof/edb01",
		Fact: mustParseGroundAtom(t, "thing", nameConst(t, "/alice")),
		Kind: provenance.KindEDB,
	}

	// Absence leaf: negated premise with no body.
	absence := &provenance.ProofNode{
		ID:   "/proof/abs01",
		Fact: mustParseGroundAtom(t, "forbidden", nameConst(t, "/bob")),
		Kind: provenance.KindAbsence,
	}

	// LetRow with a single derived premise underneath.
	innerDerived := &provenance.ProofNode{
		ID:   "/proof/d01",
		Fact: mustParseGroundAtom(t, "row_fact", ast.Number(7)),
		Kind: provenance.KindDerived,
	}
	letRow := &provenance.ProofNode{
		ID:            "/proof/let01",
		Fact:          mustParseGroundAtom(t, "let_output", ast.Number(7)),
		Kind:          provenance.KindLetRow,
		TransformText: "let Y = fn:plus(X, 1)",
		Premises:      []*provenance.ProofNode{innerDerived},
		Partial:       true, // exercise the partial branch on a non-root node too
	}

	// DoAggregate with two premises and a group key.
	groupName, err := ast.Name("/groupA")
	require.NoError(t, err)
	aggPremise1 := &provenance.ProofNode{
		ID:   "/proof/p01",
		Fact: mustParseGroundAtom(t, "data", ast.Number(1)),
		Kind: provenance.KindEDB,
	}
	aggPremise2 := &provenance.ProofNode{
		ID:   "/proof/p02",
		Fact: mustParseGroundAtom(t, "data", ast.Number(2)),
		Kind: provenance.KindEDB,
	}
	doAgg := &provenance.ProofNode{
		ID:            "/proof/agg01",
		Fact:          mustParseGroundAtom(t, "sum_data", ast.Number(3)),
		Kind:          provenance.KindDoAggregate,
		GroupKey:      []ast.Constant{groupName},
		TransformText: "do fn:group_by(G), let S = fn:sum(N)",
		Premises:      []*provenance.ProofNode{aggPremise1, aggPremise2},
	}

	// Top-level derived node with bindings, holding the EDB + absence +
	// letRow + doAgg as its premises so every Kind appears in the tree.
	xVar := ast.Variable{Symbol: "X"}
	yVar := ast.Variable{Symbol: "Y"}
	xVal, _ := ast.Name("/alice")
	yVal := ast.Number(42)
	derivedRoot := &provenance.ProofNode{
		ID:   "/proof/root01",
		Fact: mustParseGroundAtom(t, "explained_goal", nameConst(t, "/alice")),
		Kind: provenance.KindDerived,
		Bindings: []provenance.Binding{
			{Var: xVar, Value: xVal},
			{Var: yVar, Value: yVal},
		},
		Premises: []*provenance.ProofNode{edb, absence, letRow, doAgg},
		// Rule is nil here: cmd_explain.go only prints the rule line when
		// it's non-nil AND Kind != KindEDB; nil-Rule keeps the test
		// independent of internal ast.Clause construction quirks while
		// still exercising the rest of the renderer.
	}

	// A "Partial — max depth" sibling proof so the multi-proof "---"
	// separator path is exercised.
	partialRoot := &provenance.ProofNode{
		ID:      "/proof/partial01",
		Fact:    mustParseGroundAtom(t, "explained_goal", nameConst(t, "/carol")),
		Kind:    provenance.KindDerived,
		Partial: true,
	}

	proofs := []*provenance.ProofNode{derivedRoot, partialRoot}
	out := renderExplainProofs("explained_goal(/alice)", proofs)

	// Header + per-proof labels.
	require.Contains(t, out, "## /explain explained_goal(/alice)", "global header")
	require.Contains(t, out, "### Proof 1", "first proof header")
	require.Contains(t, out, "### Proof 2", "second proof header")
	require.Contains(t, out, "---", "multi-proof separator")

	// Every Kind label must appear at least once (verifies kindToLabel
	// is wired into the renderer for each enum value).
	for _, want := range []string{
		"**derived**",
		"**fact**",
		"**absence (negation)**",
		"**let-row**",
		"**do-aggregate**",
	} {
		require.Contains(t, out, want, "missing kind label %q", want)
	}

	// Bindings line: must show "X = /alice" and "Y = 42".
	require.Contains(t, out, "bindings:", "bindings prefix")
	require.Contains(t, out, "`X = /alice`", "X binding")
	require.Contains(t, out, "`Y = 42`", "Y binding")

	// Partial markers on both the let-row (nested) and the partialRoot.
	require.GreaterOrEqual(t, strings.Count(out, "partial — max depth or transform skipped"), 2,
		"partial annotation should appear on every partial node")

	// Indentation: the EDB premise sits one level below derivedRoot,
	// so a "  - **fact**" line (two-space indent) must be present.
	require.Contains(t, out, "  - **fact**",
		"EDB premise must be rendered at depth 1 (two-space indent)")

	// And the EDB-under-do-aggregate premise sits at depth 2.
	require.Contains(t, out, "    - **fact**",
		"do-aggregate's EDB premises must be rendered at depth 2 (four-space indent)")
}

// TestExplainCommandReply_ErrorPath verifies the error-formatting branch
// renders the error message instead of trying to walk the (nil) proof
// slice — important because in production /explain calls into the
// kernel and that call can fail before any proofs exist.
func TestExplainCommandReply_ErrorPath(t *testing.T) {
	msg := explainCommandReply("permitted(/edit, \"main.go\")", nil, errSentinel{})
	require.Equal(t, "assistant", msg.Role, "explain output is an assistant message")
	require.Contains(t, msg.Content, "## /explain permitted(/edit, \"main.go\")")
	require.Contains(t, msg.Content, "_error: sentinel explain failure_",
		"error message must surface to the user")
}

// nameConst constructs a name-constant safely or fails the test —
// used for atom args where we want "/alice"-style ground terms.
func nameConst(t *testing.T, s string) ast.Constant {
	t.Helper()
	c, err := ast.Name(s)
	require.NoError(t, err, "ast.Name(%q)", s)
	return c
}

// mustParseGroundAtom constructs an Atom from a predicate symbol and
// constant args. We bypass the parser to keep the test independent of
// surface-syntax tweaks in the Mangle fork.
func mustParseGroundAtom(t *testing.T, pred string, args ...ast.Constant) ast.Atom {
	t.Helper()
	terms := make([]ast.BaseTerm, len(args))
	for i, a := range args {
		terms[i] = a
	}
	return ast.NewAtom(pred, terms...)
}

// errSentinel is a tiny error type for the explain-error test. Defined
// as a struct so we don't pull in errors.New just for one message.
type errSentinel struct{}

func (errSentinel) Error() string { return "sentinel explain failure" }
