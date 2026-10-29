package core

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The 2026-10-28 dogfood log refused a candidate rule with "anonymous
// variables in negated atoms are unbound" for
// !executive_processed_intent(/current_intent): the regex took the underscore
// inside the name constant for a wildcard. Name constants, strings and named
// variables with underscores are bound; only the wildcard is not.
func TestCheckUnsafeNegation_UnderscoresInsideNamesAreNotWildcards(t *testing.T) {
	for _, rule := range []string{
		`next_action(/x) :- current_intent(/current_intent), !executive_processed_intent(/current_intent).`,
		`a(X) :- b(X), !c(X, "snake_case").`,
		`a(Shard_Type) :- b(Shard_Type), !c(Shard_Type).`,
	} {
		require.NoError(t, checkUnsafeNegation(rule), rule)
	}
}

func TestCheckUnsafeNegation_WildcardInNegationIsRefused(t *testing.T) {
	err := checkUnsafeNegation(`idle(T) :- shard_type(T), !active_shard(_, T).`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "active_shard(_")
}

// HotLoadRule is the entry point the feedback loop calls; the false refusal
// happened there, before the sandbox ever saw the rule.
func TestHotLoadRule_AcceptsGroundNegationWithUnderscoredName(t *testing.T) {
	t.Parallel()
	k := admissionKernel(t, `Decl probe_intent(I) bound [/name].
Decl probe_processed(I) bound [/name].
Decl probe_pending(I) bound [/name].`)
	require.NoError(t, k.HotLoadRule(`probe_pending(/current_intent) :- probe_intent(/current_intent), !probe_processed(/current_intent).`))
}
