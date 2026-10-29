package core

import (
	"sort"
	"testing"
)

// The flesh admission table in jit_compiler.mg (section 3): Jev's per-turn
// relevance (atom_relevance) is read in bands against the turn's thresholds,
// and a missing judgment leaves the vector channel's slots in charge.
func relevanceFixture() []string {
	facts := []string{
		`atom_relevance_threshold(/yes, 70)`,
		`atom_relevance_threshold(/no, 30)`,
	}
	for _, id := range []string{"slot/applies", "slot/refused", "slot/unsettled", "slot/unjudged", "pool/applies", "pool/unsettled", "pool/refused"} {
		facts = append(facts, `atom("`+id+`")`, `prompt_atom("`+id+`", /exemplar, 10, /false)`, `vector_hit("`+id+`", 60)`)
	}
	for _, id := range []string{"slot/applies", "slot/refused", "slot/unsettled", "slot/unjudged"} {
		facts = append(facts, `vector_slot("`+id+`")`)
	}
	facts = append(facts,
		`atom_relevance("slot/applies", 91)`,
		`atom_relevance("slot/refused", 12)`,
		`atom_relevance("slot/unsettled", 50)`,
		`atom_relevance("pool/applies", 70)`,
		`atom_relevance("pool/unsettled", 69)`,
		`atom_relevance("pool/refused", 30)`,
	)
	return facts
}

func relevanceIDs(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestAtomRelevance_BandsDecideFleshAdmission(t *testing.T) {
	t.Parallel()
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	supersessionFixture(t, k, relevanceFixture()...)
	got := selectedAtoms(t, k)
	want := map[string]bool{
		"slot/applies":   true, // applies, and a slot
		"slot/unsettled": true, // unsettled: the slot decides
		"slot/unjudged":  true, // no judgment is never a no
		"pool/applies":   true, // applies at exactly the yes edge, from beyond the slots
	}
	if len(got) != len(want) {
		t.Fatalf("selected %v, want %v", relevanceIDs(got), relevanceIDs(want))
	}
	for id := range want {
		if !got[id] {
			t.Fatalf("selected %v, want %v", relevanceIDs(got), relevanceIDs(want))
		}
	}

	rows, err := k.Query("flesh_refused")
	if err != nil {
		t.Fatal(err)
	}
	refused := map[string]bool{}
	for _, r := range rows {
		refused[r.Args[0].(string)] = true
	}
	if !refused["slot/refused"] || !refused["pool/refused"] || len(refused) != 2 {
		t.Fatalf("flesh_refused = %v, want slot/refused and pool/refused (no at exactly the no edge)", relevanceIDs(refused))
	}
}

// Without any atom_relevance (Jev off, failed or late) the admission is the
// vector channel's: every slot, nothing beyond.
func TestAtomRelevance_NoJudgmentIsTheVectorChannel(t *testing.T) {
	t.Parallel()
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	supersessionFixture(t, k,
		`atom("a")`, `prompt_atom("a", /exemplar, 10, /false)`, `vector_hit("a", 60)`, `vector_slot("a")`,
		`atom("b")`, `prompt_atom("b", /exemplar, 10, /false)`, `vector_hit("b", 55)`,
	)
	got := selectedAtoms(t, k)
	if !got["a"] || got["b"] || len(got) != 1 {
		t.Fatalf("selected %v, want only the vector slot a", relevanceIDs(got))
	}
}

// Build-time importance arbitrates a conflict between two of a shard's
// correctness-level candidates (jit_selection.mg conflict_loser) and needs
// both atoms scored: an unscored atom loses nothing on importance.
func TestAtomImportance_ConflictLoserNeedsBothScores(t *testing.T) {
	t.Parallel()
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	supersessionFixture(t, k,
		`compile_shard("s1", /coder)`,
		`atom("hi")`, `prompt_atom("hi", /exemplar, 10, /false)`, `atom_importance("hi", 5)`, `atom_tag("hi", /shard, /coder)`,
		`atom("lo")`, `prompt_atom("lo", /exemplar, 10, /false)`, `atom_importance("lo", 4)`, `atom_tag("lo", /shard, /coder)`,
		`atom_conflicts("hi", "lo")`,
		`atom("un")`, `prompt_atom("un", /exemplar, 10, /false)`, `atom_tag("un", /shard, /coder)`,
		`atom_conflicts("hi", "un")`,
	)
	rows, err := k.Query("conflict_loser")
	if err != nil {
		t.Fatal(err)
	}
	losers := map[string]bool{}
	for _, r := range rows {
		losers[r.Args[0].(string)] = true
	}
	if !losers["lo"] || losers["hi"] || losers["un"] {
		t.Fatalf("conflict_loser = %v, want only lo", relevanceIDs(losers))
	}
}

// A carried atom (the task's previous judged compile admitted it) stays
// through an unsettled reading and even from outside the vector slots; only a
// does-not-apply reading drops it.
func TestAtomRelevance_CarriedHoldsUntilJudgedNotToApply(t *testing.T) {
	t.Parallel()
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	facts := []string{`atom_relevance_threshold(/yes, 70)`, `atom_relevance_threshold(/no, 30)`}
	for id, pct := range map[string]string{"kept/unsettled": "45", "kept/dropped": "20", "new/unsettled": "45"} {
		facts = append(facts, `atom("`+id+`")`, `prompt_atom("`+id+`", /exemplar, 10, /false)`, `vector_hit("`+id+`", 40)`, `atom_relevance("`+id+`", `+pct+`)`)
	}
	facts = append(facts, `flesh_carried("kept/unsettled")`, `flesh_carried("kept/dropped")`)
	supersessionFixture(t, k, facts...)
	got := selectedAtoms(t, k)
	if !got["kept/unsettled"] || got["kept/dropped"] || got["new/unsettled"] {
		t.Fatalf("selected %v, want only kept/unsettled", relevanceIDs(got))
	}
}
