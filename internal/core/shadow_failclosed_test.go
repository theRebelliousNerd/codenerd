package core

import (
	"errors"
	"strings"
	"testing"
)

// failingQuerier answers every query with an error, standing in for a shadow
// kernel that cannot evaluate — a stratification failure, a parse error, or a
// cancelled fixpoint.
type failingQuerier struct{ err error }

func (f *failingQuerier) Query(string) ([]Fact, error) { return nil, f.err }

var errQueryUnavailable = errors.New("kernel evaluation unavailable")

// TestQueryOrBlock_FailsClosed pins the fix for a fail-open safety gate.
// checkViolations dropped every query error with `_`, so a shadow kernel that
// could not evaluate returned zero violations and the caller read the action as
// safe. The constitution is default-deny; a check that cannot run must block.
func TestQueryOrBlock_FailsClosed(t *testing.T) {
	facts, violation := queryOrBlock(&failingQuerier{err: errQueryUnavailable}, "act-1", "block_commit")
	if facts != nil {
		t.Errorf("a failed query must return no facts, got %d", len(facts))
	}
	if violation == nil {
		t.Fatal("a shadow kernel that cannot answer must produce a violation, not silence")
	}
	if !violation.Blocking {
		t.Error("a failed safety check must block")
	}
	if violation.ViolationType != "safety_check_failed" {
		t.Errorf("violation type = %q, want safety_check_failed", violation.ViolationType)
	}
	if violation.ActionID != "act-1" {
		t.Errorf("violation lost its action id: %q", violation.ActionID)
	}
	if !strings.Contains(violation.Description, "block_commit") {
		t.Errorf("description does not name the failed predicate: %q", violation.Description)
	}
	if !strings.Contains(violation.Description, errQueryUnavailable.Error()) {
		t.Errorf("description drops the underlying error: %q", violation.Description)
	}
}

// TestQueryOrBlock_NilQuerierBlocks covers the other way the check can fail to
// run: no shadow kernel at all. Silence there would be the same inversion.
func TestQueryOrBlock_NilQuerierBlocks(t *testing.T) {
	_, violation := queryOrBlock(nil, "act-1", "deny_edit")
	if violation == nil || !violation.Blocking {
		t.Fatal("a nil shadow kernel must block, not pass")
	}
}

// TestQueryOrBlock_HealthyKernelIsSilent is the other half of the contract: an
// empty result must still mean "the kernel answered and found nothing", or the
// fail-closed change would refuse every simulation.
func TestQueryOrBlock_HealthyKernelIsSilent(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	facts, violation := queryOrBlock(kernel, "act-1", "block_commit")
	if violation != nil {
		t.Fatalf("a healthy kernel must not produce a safety_check_failed violation: %+v", violation)
	}
	if len(facts) != 0 {
		t.Fatalf("a fresh kernel derives no block_commit facts, got %d", len(facts))
	}
}

// TestCheckViolations_HealthyKernelIsSilent exercises the real call path so a
// refactor that stops threading the querier through is caught here.
func TestCheckViolations_HealthyKernelIsSilent(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	sm := &ShadowMode{shadowKernel: kernel}
	if got := sm.checkViolations("act-1"); len(got) != 0 {
		t.Fatalf("a healthy kernel with no violations must report none, got %d: %+v", len(got), got)
	}
}

// TestCheckViolations_NilKernelBlocksEverySafetyQuery proves the wiring: with no
// shadow kernel, all four safety predicates must report a blocking failure
// rather than an empty, reassuring result.
func TestCheckViolations_NilKernelBlocksEverySafetyQuery(t *testing.T) {
	sm := &ShadowMode{}
	violations := sm.checkViolations("act-1")
	if len(violations) != 4 {
		t.Fatalf("expected one blocking violation per safety query, got %d: %+v", len(violations), violations)
	}
	for _, v := range violations {
		if !v.Blocking || v.ViolationType != "safety_check_failed" {
			t.Fatalf("unexpected violation: %+v", v)
		}
	}
}
