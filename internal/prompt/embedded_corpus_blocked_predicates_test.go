package prompt

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/core"
)

// S4's Design F, item 3: "protocol/piggyback/mangle_updates loses the
// test_state(/passing). line it taught on every turn." This is the regression
// net for that edit and for every predicate the hard block will ever grow to
// cover: nothing the model is handed as protocol or exemplar may teach it to
// assert a predicate core.predicateAllowed refuses. A model that writes
// build_state or turn_gate is a model witnessing its own completion, and the
// verdict reads those.
//
// The corpus asked is the one the runtime serves: the go:embedded YAML under
// internal/prompt/atoms/ (LoadEmbeddedCorpus), which wins duplicate IDs in the
// compiler's merge and to which boot reconciles the runtime DB. The wave-1
// reviewer wrote this against the shipped prompt_corpus.db seed instead, which
// is a first-boot seed and 42 atoms stale; that version is not kept.
//
// Scoped to the protocol and exemplar categories: those are the atoms that tell
// the model what it may put in a piggyback control packet. The language/mangle
// reference atoms also contain `test_state(/failing).` — as examples of Mangle
// SYNTAX for a shard that writes rules, not as facts to emit — and folding the
// two together would make this assertion mean nothing.
func TestEmbeddedCorpus_ProtocolAtomsTeachNoHardBlockedPredicate(t *testing.T) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("load the embedded corpus: %v", err)
	}

	// The blocked set is probed from the filter rather than restated here, so
	// the two cannot drift: a predicate added to the hard block is forbidden in
	// the corpus from the same commit.
	blocked := hardBlockedPredicates(t)
	if len(blocked) == 0 {
		t.Fatal("no hard-blocked predicate was discovered; the probe below measures nothing")
	}

	var offences []string
	checked := 0
	for _, a := range corpus.All() {
		if a == nil || (a.Category != CategoryProtocol && string(a.Category) != "exemplar") {
			continue
		}
		checked++
		for _, pred := range blocked {
			// A fact the model is taught to WRITE: the predicate at the head of
			// a line with an argument list, ending in a period. Prose that
			// merely names the predicate stays legal — the replacement
			// paragraph in piggyback.yaml says build_state and test_state "are
			// refused here", and that sentence must not trip this.
			taught := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(pred) + `\s*\([^)\n]*\)\s*\.`)
			if taught.MatchString(a.Content) {
				where := "optional"
				if a.IsMandatory {
					where = "MANDATORY"
				}
				offences = append(offences, fmt.Sprintf("atom %q (%s) teaches %s(...).", a.ID, where, pred))
			}
		}
	}
	if checked == 0 {
		t.Fatal("no protocol or exemplar atom was found in the embedded corpus")
	}
	sort.Strings(offences)
	if len(offences) > 0 {
		t.Errorf("%d of %d protocol/exemplar atoms in the embedded corpus teach a "+
			"hard-blocked assertion; the filter drops it and the model is never told:\n  %s",
			len(offences), checked, strings.Join(offences, "\n  "))
	}
}

// hardBlockedPredicates discovers the hard block by probing the filter with the
// most permissive policy there is. A predicate the filter refuses under a
// policy that allows everything is refused by the hard block and nothing else.
func hardBlockedPredicates(t *testing.T) []string {
	t.Helper()
	// The candidate set bounds what is probed; the probe decides what is
	// blocked, so this list can never claim something the filter does not.
	candidates := []string{
		"build_state", "test_state", "turn_gate",
		"turn_build_green", "turn_build_red", "turn_tests_green", "turn_tests_red",
		"turn_evidence", "turn_acceptance", "turn_executed", "turn_done", "turn_cost",
		"turn_verified", "turn_unverified", "turn_wrote", "turn_build_failed",
		"turn_missing_evidence", "turn_created_source", "has_turn_acceptance",
	}
	permissive := core.MangleUpdatePolicy{AllowedPrefixes: []string{""}}

	var blocked []string
	for _, pred := range candidates {
		kept, _ := core.FilterMangleUpdates(nil, []string{pred + "(/passing)."}, permissive)
		if len(kept) == 0 {
			blocked = append(blocked, pred)
		}
	}
	return blocked
}
