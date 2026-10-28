package observation

import (
	"strings"
	"testing"
)

func TestPartialState_NamesEveryChangeAndCheck(t *testing.T) {
	r := Return{
		Changed: []string{"a.go", "b.go", "c.go"},
		Build:   &Verification{Kind: "build", Ran: true, OK: false, Outcome: "failed", Source: SourceObserved},
		Tests:   &Verification{Kind: "tests", Ran: false, Source: SourceObserved},
		Missing: []string{"/tests_not_green"},
		Notes:   []string{"tool call failed: disk on fire"},
	}
	got := r.PartialState()
	for _, want := range []string{
		"changed 3 file(s): a.go, b.go, c.go",
		"Build FAILED",
		"Tests did not run",
		"Not established: /tests_not_green",
		"tool call failed: disk on fire",
		"nothing here is verified complete",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("partial state is missing %q:\n%s", want, got)
		}
	}
}

// A return that measured nothing cannot say nothing happened.
func TestPartialState_UnmeasuredIsSilent(t *testing.T) {
	if got := (Return{Output: "I made some changes", Failure: "boom"}).PartialState(); got != "" {
		t.Errorf("read a state out of prose: %q", got)
	}
}
