package core

import (
	"strings"
	"testing"

	"codeberg.org/TauCeti/mangle-go/factstore"
)

// The defect this guards (F-LEG-1): every candidate rule the Legislator
// generated and the sandbox rejected produced two ERROR lines —
//
//	[kernel] rebuildProgram: parse failed: 18178:6 mismatched input 'Mode' ...
//	[kernel] HotLoadRule: rule rejected by sandbox compiler: ...
//
// indistinguishable from the production corpus failing to parse. They were the
// loudest entries in the kernel log once `nerd logs` started working, and cost
// a full investigation before the Legislator's own log showed, seconds later:
// "Rule auto-repaired by feedback loop sanitizer ... Rule ratified and
// hot-loaded successfully".
//
// The rejection is the gate working. A false ERROR in an unattended run is
// worse than no log line, because it spends triage attention that the real
// failures need.

func TestValidateRuleSandbox_RejectsMalformedRule(t *testing.T) {
	// The sandbox must still REJECT — quieting the log must not quiet the gate.
	err := validateRuleSandbox("this is not( valid mangle", "", "", "")
	if err == nil {
		t.Fatal("a malformed rule passed sandbox validation")
	}
	if !strings.Contains(err.Error(), "parse") && !strings.Contains(err.Error(), "analyze") {
		t.Errorf("rejection reason is not a compile failure: %v", err)
	}
}

func TestValidateRuleSandbox_AcceptsAWellFormedRule(t *testing.T) {
	schemas := `
Decl projected_action(ActionID, ActionType, Target) bound [/string, /string, /string].
Decl dream_block(ActionID, Reason) bound [/string, /name].
`
	// The shape the Legislator actually produced and successfully ratified.
	rule := `dream_block(ActionID, /consultation_describe_only) :- projected_action(ActionID, ActionType, Target).`

	if err := validateRuleSandbox(rule, schemas, "", ""); err != nil {
		t.Fatalf("a well-formed rule was rejected: %v", err)
	}
}

// The marker must be set on the throwaway kernel and nowhere else: its zero
// value is what keeps every production kernel logging these at ERROR.
func TestSandboxMarker_DefaultsOffForOrdinaryKernels(t *testing.T) {
	k := &RealKernel{
		store:             factstore.NewSimpleInMemoryStore(),
		loadedPolicyFiles: make(map[string]struct{}),
		policyDirty:       true,
	}
	if k.sandbox {
		t.Error("an ordinary kernel is marked as a sandbox; production parse failures would be demoted to debug")
	}

	real, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	if real.sandbox {
		t.Error("NewRealKernel produced a sandbox-marked kernel")
	}
}

// A sandbox rejection must not churn debug_program_ERROR.mg. That dump exists
// to capture the real corpus failing analysis; writing it for every rejected
// candidate would bury the case it was built for.
func TestSandboxAnalysisFailure_DoesNotWriteDebugDump(t *testing.T) {
	sandbox := &RealKernel{
		store:             factstore.NewSimpleInMemoryStore(),
		loadedPolicyFiles: make(map[string]struct{}),
		policyDirty:       true,
		sandbox:           true,
	}
	// Undeclared predicate in the head: parses, fails analysis.
	sandbox.learned = `undeclared_head(X) :- also_undeclared(X).`

	if err := sandbox.rebuildProgram(); err == nil {
		t.Fatal("expected the sandbox to reject an unanalyzable program")
	}
}
