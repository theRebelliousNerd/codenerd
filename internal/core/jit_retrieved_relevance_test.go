package core

import "testing"

// A retrieved knowledge row Jev reads as not applying to the request is
// refused; one it reads as applying, one it is unsure of and one it never
// judged stay admitted -- a missing judgment is never a "no". Live 2026-11-12
// every lexically retrieved row entered a Go step's prompt at full score,
// about 100k of its 147k chars.
func TestJITRetrievedContext_ARowJudgedNotToApplyIsRefused(t *testing.T) {
	t.Parallel()
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	rows := []string{"knowledge/applies", "knowledge/unsure", "knowledge/unjudged", "knowledge/offtopic"}
	var facts []string
	for _, id := range rows {
		facts = append(facts,
			`atom("`+id+`")`,
			`prompt_atom("`+id+`", /knowledge, 20, /false)`,
			`retrieved_context("`+id+`")`)
	}
	facts = append(facts,
		`atom_relevance("knowledge/applies", 90)`,
		`atom_relevance("knowledge/unsure", 50)`,
		`atom_relevance("knowledge/offtopic", 5)`,
		`atom_relevance_threshold(/yes, 70)`,
		`atom_relevance_threshold(/no, 30)`)
	assertPinFixture(t, k, facts...)
	got, err := k.Query("selected_result")
	if err != nil {
		t.Fatal(err)
	}
	selected := map[string]bool{}
	for _, f := range got {
		if len(f.Args) > 0 {
			if id, ok := f.Args[0].(string); ok {
				selected[id] = true
			}
		}
	}
	for _, id := range []string{"knowledge/applies", "knowledge/unsure", "knowledge/unjudged"} {
		if !selected[id] {
			t.Errorf("%s was refused; only a row judged not to apply may be (selected %v)", id, selected)
		}
	}
	if selected["knowledge/offtopic"] {
		t.Errorf("a retrieved row Jev read as not applying was admitted: %v", selected)
	}
	refused, err := k.Query("flesh_refused")
	if err != nil {
		t.Fatal(err)
	}
	if len(refused) != 1 {
		t.Errorf("flesh_refused = %v, want the off-topic row named once", refused)
	}
}

// The share governs the rows measured against it. A row the compile gave no
// tokens or rank cannot be placed in the order the share is spent in; it is
// admitted, not dropped without a name.
func TestJITRetrievedContext_AnUnmeasuredRowIsAdmittedBesideTheShare(t *testing.T) {
	t.Parallel()
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	facts := []string{
		`compile_budget(1000)`,
		`retrieved_knowledge_share_config(10, 8000)`, // a share of 100 tokens
	}
	for _, id := range []string{"knowledge/fits", "knowledge/over", "knowledge/unmeasured"} {
		facts = append(facts,
			`atom("`+id+`")`,
			`prompt_atom("`+id+`", /knowledge, 20, /false)`,
			`retrieved_context("`+id+`")`)
	}
	facts = append(facts,
		`retrieved_context_tokens("knowledge/fits", 60)`,
		`retrieved_context_rank("knowledge/fits", 0)`,
		`retrieved_context_tokens("knowledge/over", 60)`,
		`retrieved_context_rank("knowledge/over", 1)`)
	assertPinFixture(t, k, facts...)
	got, err := k.Query("selected_result")
	if err != nil {
		t.Fatal(err)
	}
	selected := map[string]bool{}
	for _, f := range got {
		if id, ok := f.Args[0].(string); ok {
			selected[id] = true
		}
	}
	if !selected["knowledge/fits"] || !selected["knowledge/unmeasured"] || selected["knowledge/over"] {
		t.Errorf("selected %v, want the row that fits and the unmeasured row, not the one past the share", selected)
	}
	withheld, err := k.Query("retrieved_withheld")
	if err != nil {
		t.Fatal(err)
	}
	if len(withheld) != 1 || withheld[0].Args[0] != "knowledge/over" {
		t.Errorf("retrieved_withheld = %v, want only the row past the share", withheld)
	}
}
