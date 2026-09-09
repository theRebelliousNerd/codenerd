package codedom

import (
	"testing"
)

// factKernel answers Query from a fixed table.
type factKernel struct {
	facts map[string][]FactData
	err   error
}

func (k *factKernel) Query(predicate string) ([]FactData, error) {
	if k.err != nil {
		return nil, k.err
	}
	return k.facts[predicate], nil
}

// TestEditedRefsFromKernel_PrefersWhatIsActuallyProduced pins the fix for a
// shape mismatch that made both impacted-test tools return nothing on every
// real invocation.
//
// The no-argument path read plan_edit alone. plan_edit is declared as
// plan_edit(Ref) and joined against code_element's ref, but its only producer
// — TransactionManager.ToFacts — wrote FileEdit.FilePath into it. A file path
// never matches a ref, so the dependency graph matched nothing. element_modified
// is emitted by every CodeDOM edit handler and carries a real ref.
func TestEditedRefsFromKernel_PrefersWhatIsActuallyProduced(t *testing.T) {
	k := &factKernel{facts: map[string][]FactData{
		"element_modified": {
			{Predicate: "element_modified", Args: []any{"fn:calc.Add", "sess", int64(1)}},
			{Predicate: "element_modified", Args: []any{"fn:calc.Sub", "sess", int64(2)}},
		},
	}}
	got := editedRefsFromKernel(k)
	if len(got) != 2 || got[0] != "fn:calc.Add" || got[1] != "fn:calc.Sub" {
		t.Fatalf("got %v, want both element_modified refs in order", got)
	}
}

// TestEditedRefsFromKernel_StillReadsPlanEdit keeps the declared predicate in
// play for a future producer that emits the right shape.
func TestEditedRefsFromKernel_StillReadsPlanEdit(t *testing.T) {
	k := &factKernel{facts: map[string][]FactData{
		"plan_edit": {{Predicate: "plan_edit", Args: []any{"fn:calc.Mul"}}},
	}}
	if got := editedRefsFromKernel(k); len(got) != 1 || got[0] != "fn:calc.Mul" {
		t.Fatalf("got %v, want [fn:calc.Mul]", got)
	}
}

// TestEditedRefsFromKernel_Deduplicates: an element edited twice, or named by
// both predicates, must be analysed once.
func TestEditedRefsFromKernel_Deduplicates(t *testing.T) {
	k := &factKernel{facts: map[string][]FactData{
		"element_modified": {
			{Predicate: "element_modified", Args: []any{"fn:calc.Add", "s", int64(1)}},
			{Predicate: "element_modified", Args: []any{"fn:calc.Add", "s", int64(2)}},
		},
		"plan_edit": {{Predicate: "plan_edit", Args: []any{"fn:calc.Add"}}},
	}}
	if got := editedRefsFromKernel(k); len(got) != 1 {
		t.Fatalf("got %v, want one deduplicated ref", got)
	}
}

// TestEditedRefsFromKernel_Degrades covers the shapes a kernel can hand back
// that are not usable refs.
func TestEditedRefsFromKernel_Degrades(t *testing.T) {
	if got := editedRefsFromKernel(nil); got != nil {
		t.Errorf("nil kernel returned %v", got)
	}
	if got := editedRefsFromKernel(&factKernel{err: errQuery("kernel down")}); got != nil {
		t.Errorf("a failing query returned %v, want nothing", got)
	}
	malformed := &factKernel{facts: map[string][]FactData{
		"element_modified": {
			{Predicate: "element_modified", Args: nil},
			{Predicate: "element_modified", Args: []any{""}},
			{Predicate: "element_modified", Args: []any{42}},
		},
	}}
	if got := editedRefsFromKernel(malformed); got != nil {
		t.Errorf("malformed facts produced %v, want nothing", got)
	}
}

type errQuery string

func (e errQuery) Error() string { return string(e) }
