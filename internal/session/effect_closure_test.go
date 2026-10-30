package session

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/observation"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

func parentUncertainty(t *testing.T, result *ExecutionResult) []string {
	t.Helper()
	ret := observedReturn("coder", "fix a.json", result)
	return observation.ProjectReturn(ret, observation.ReturnLimits{}).Uncertainty
}

// A rejected write a later verified write repaired: the kernel retired its
// /repair, but the receipt's cached recovery crossed the handoff and the parent
// read "tool effect not settled ... owed recovery /repair" (audit
// effect-recovery-closure). The closure is read at the turn's close and the
// parent receives that; the rejection stays in the receipt history.
func TestEffectClosure_ARepairedRejectionIsNotOwedToTheParent(t *testing.T) {
	k := realKernel(t)
	gate := &scriptedGate{answers: []types.ToolValidation{
		{State: types.ToolValidationRejected, Reason: "invalid JSON"},
		{State: types.ToolValidationVerified},
	}}
	e, cfg, _ := effectFixture(t, k, gate, nil)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}
	e.executeToolBatch(context.Background(), []types.ToolCall{writeCall("c1", "a.json", `{"a":`)}, cfg, result)
	e.executeToolBatch(context.Background(), []types.ToolCall{writeCall("c2", "a.json", `{"a":1}`)}, cfg, result)
	// The production closure: settlement runs checkHollowSuccess, which asserts
	// the turn evidence, reads the closure, and then clears the turn facts.
	_ = e.checkHollowSuccess(result)

	for _, u := range parentUncertainty(t, result) {
		if strings.Contains(u, "owed recovery") || strings.Contains(u, "(c1#") {
			t.Errorf("the parent was told a retired recovery is owed: %q", u)
		}
	}
	ret := observedReturn("coder", "fix a.json", result)
	if len(ret.Effects) != 2 || ret.Effects[0].Recovery != "/repair" || ret.Effects[0].Validation != "rejected" {
		t.Fatalf("the rejection left the receipt history: %+v", ret.Effects)
	}
	if c := ret.Effects[0].Closure; c == nil || c.State != observation.ClosureSuperseded {
		t.Fatalf("c1 closure = %+v, want superseded", c)
	}
	if c := ret.Effects[1].Closure; c == nil || c.State != observation.ClosureSettled || len(c.Current) != 1 || c.Current[0] != "a.json" {
		t.Fatalf("c2 closure = %+v, want settled and current on a.json", c)
	}
}

// Control: a rejection nobody repaired still owes /repair at the close.
func TestEffectClosure_AnUnrepairedRejectionStaysOwed(t *testing.T) {
	k := realKernel(t)
	gate := &scriptedGate{answers: []types.ToolValidation{{State: types.ToolValidationRejected, Reason: "invalid JSON"}}}
	e, cfg, _ := effectFixture(t, k, gate, nil)
	result := &ExecutionResult{}
	turn := result.turnAtom()
	e.executeToolBatch(context.Background(), []types.ToolCall{writeCall("c1", "a.json", `{"a":`)}, cfg, result)
	e.assertTurnEvidence(turn, "/fix", result)

	var owed bool
	for _, u := range parentUncertainty(t, result) {
		if strings.Contains(u, "(c1#") && strings.Contains(u, "changed a.json;") && strings.Contains(u, "owed recovery /repair") {
			owed = true
		}
	}
	if !owed {
		t.Fatalf("an unrepaired rejection's /repair did not reach the parent: %v", parentUncertainty(t, result))
	}
}

// A call that changed A and B was rejected; a later verified call repaired A
// only. B still owes the repair, and A is not named as current debt.
func TestEffectClosure_APartialRepairOwesOnlyTheRest(t *testing.T) {
	k := realKernel(t)
	e, _, _ := effectFixture(t, k, &scriptedGate{}, nil)
	result := &ExecutionResult{}
	turn := result.turnAtom()
	r1 := ToolEffectReceipt{CallID: "c1", Tool: "write_files", Seq: 1, Validation: types.ToolValidation{State: types.ToolValidationRejected},
		Effects: []ResourceEffect{{Path: "a.go", State: EffectChanged}, {Path: "b.go", State: EffectChanged}}}
	r2 := ToolEffectReceipt{CallID: "c2", Tool: "write_file", Seq: 2, Validation: types.ToolValidation{State: types.ToolValidationVerified},
		Effects: []ResourceEffect{{Path: "a.go", State: EffectChanged}}}
	e.assertToolReceipt(result, r1)
	e.assertToolReceipt(result, r2)
	result.ToolReceipts = []ToolEffectReceipt{r1, r2}

	e.recordEffectClosure(turn, result)
	c := result.ToolReceipts[0].Closure
	if c == nil || c.State != observation.ClosureOwed || c.Recovery != "/repair" || len(c.Current) != 1 || c.Current[0] != "b.go" {
		t.Fatalf("c1 closure = %+v, want /repair owed on b.go only", c)
	}
	u := projectReceipts(result.ToolReceipts)[0].Unsettled()
	if !strings.Contains(u, "changed b.go;") || strings.Contains(u, "a.go") {
		t.Fatalf("the parent's sentence = %q, want b.go only", u)
	}
}

// A closure that could not be read is said, never taken for settled.
func TestEffectClosure_AnUnknownClosureIsNotSettled(t *testing.T) {
	r := observation.EffectReceipt{Call: "c1", Tool: "write_file", Validation: "rejected", Recovery: "/repair",
		Effects: []observation.ResourceEffect{{Path: "a.json", State: "changed"}}, Closure: &observation.EffectClosure{State: observation.ClosureUnknown}}
	if u := r.Unsettled(); !strings.Contains(u, "could not be read") {
		t.Fatalf("an unknown closure read as %q", u)
	}
}
