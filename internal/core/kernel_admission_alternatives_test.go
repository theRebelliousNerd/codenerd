package core

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func lenientNoted(pred string, arity, arg int) bool {
	_, seen := lenientNameWarnings.Load(lenientKey(pred, arity, arg))
	return seen
}

// compile_context(Dimension, Value) is declared [/name, /name] or
// [/name, /string]. A string Value is exactly what the second alternative
// declares; the first alternative used to accept it leniently, so every prompt
// compile reported "compile_context/2 arg 1 ... bare Go string" for a fact the
// Decl declares. An exact alternative must win over a tolerated one.
func TestAdmission_ExactAlternativeBeatsToleratedOne(t *testing.T) {
	t.Parallel()
	k := admissionKernel(t, `Decl ctx_dim_probe(Dim, Value) bound [/name, /name] bound [/name, /string].`)

	require.NoError(t, k.Assert(Fact{Predicate: "ctx_dim_probe", Args: []any{MangleAtom("/language"), "go"}}))
	require.False(t, lenientNoted("ctx_dim_probe", 2, 1),
		"a string the second alternative declares exactly must not be reported as a bare-string /name defect")

	rows, err := k.Query("ctx_dim_probe")
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

// When only the tolerance admits a fact, every tolerated argument is
// reported. The old check noted only the last one, so a producer passing two
// bare strings was fixed one argument per dogfood session.
func TestAdmission_EveryToleratedArgumentIsReported(t *testing.T) {
	t.Parallel()
	k := admissionKernel(t, `Decl two_names_probe(A, B) bound [/name, /name].`)

	require.NoError(t, k.Assert(Fact{Predicate: "two_names_probe", Args: []any{"left", "right"}}))
	require.True(t, lenientNoted("two_names_probe", 2, 0), "arg 0 relied on the tolerance")
	require.True(t, lenientNoted("two_names_probe", 2, 1), "arg 1 relied on the tolerance")
}
