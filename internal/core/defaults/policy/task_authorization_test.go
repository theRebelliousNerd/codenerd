package policy

import (
	"testing"

	"codenerd/internal/mangle"

	"go.uber.org/goleak"
)

// Task authorization (constitution.mg, section 7E), at the level that decides
// it. Go asserts the turn's authorization and one gated shell call's measured
// shape; the policy derives whether the call is authorized. These pin that
// "commit" covers staging and committing the files the turn wrote and nothing
// beside it, and that a restriction or a standing "confirm first" on the same
// turn always wins.

func fact(pred string, args ...any) approvalFact { return approvalFact{pred, args} }

func shellCallAuthorized(t *testing.T, eng *mangle.Engine, id string) bool {
	t.Helper()
	facts, err := eng.GetFacts("shell_call_authorized")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range facts {
		if got, _ := f.Args[0].(string); got == id {
			return true
		}
	}
	return false
}

func TestTaskAuthorization_CommitCoversStagingAndCommittingTheWriteSetOnly(t *testing.T) {
	defer goleak.VerifyNone(t)
	for _, tc := range []struct {
		name  string
		facts []approvalFact
		want  bool
	}{
		{"add of written files", []approvalFact{
			fact("turn_authorization", "/t1", "/commit"), fact("shell_call", "c1", "/t1", "/git_add"), fact("shell_call_in_write_set", "c1")}, true},
		{"commit of written files", []approvalFact{
			fact("turn_authorization", "/t1", "/commit"), fact("shell_call", "c1", "/t1", "/git_commit"), fact("shell_call_in_write_set", "c1")}, true},
		{"no authorization for the turn", []approvalFact{
			fact("shell_call", "c1", "/t1", "/git_commit"), fact("shell_call_in_write_set", "c1")}, false},
		{"another turn's authorization", []approvalFact{
			fact("turn_authorization", "/t2", "/commit"), fact("shell_call", "c1", "/t1", "/git_commit"), fact("shell_call_in_write_set", "c1")}, false},
		{"files outside the write set", []approvalFact{
			fact("turn_authorization", "/t1", "/commit"), fact("shell_call", "c1", "/t1", "/git_add")}, false},
		// No kind covers a push: the Go side never even measures one, and the
		// policy would not admit it if it did.
		{"a push is covered by no kind", []approvalFact{
			fact("turn_authorization", "/t1", "/commit"), fact("shell_call", "c1", "/t1", "/git_push"), fact("shell_call_in_write_set", "c1")}, false},
		{"a reset is covered by no kind", []approvalFact{
			fact("turn_authorization", "/t1", "/commit"), fact("shell_call", "c1", "/t1", "/git_reset"), fact("shell_call_in_write_set", "c1")}, false},
		{"an unknown kind covers nothing", []approvalFact{
			fact("turn_authorization", "/t1", "/everything"), fact("shell_call", "c1", "/t1", "/git_commit"), fact("shell_call_in_write_set", "c1")}, false},
		// The user's limits win over the user's authorization.
		{"read_only turn", []approvalFact{
			fact("turn_authorization", "/t1", "/commit"), fact("turn_restriction", "/t1", "/read_only"),
			fact("shell_call", "c1", "/t1", "/git_commit"), fact("shell_call_in_write_set", "c1")}, false},
		{"no_changes turn", []approvalFact{
			fact("turn_authorization", "/t1", "/commit"), fact("turn_restriction", "/t1", "/no_changes"),
			fact("shell_call", "c1", "/t1", "/git_add"), fact("shell_call_in_write_set", "c1")}, false},
		{"dry_run turn", []approvalFact{
			fact("turn_authorization", "/t1", "/commit"), fact("turn_restriction", "/t1", "/dry_run"),
			fact("shell_call", "c1", "/t1", "/git_add"), fact("shell_call_in_write_set", "c1")}, false},
		{"hypothetical turn", []approvalFact{
			fact("turn_authorization", "/t1", "/commit"), fact("turn_restriction", "/t1", "/hypothetical"),
			fact("shell_call", "c1", "/t1", "/git_add"), fact("shell_call_in_write_set", "c1")}, false},
		{"confirm_first turn", []approvalFact{
			fact("turn_authorization", "/t1", "/commit"), fact("turn_restriction", "/t1", "/confirm_first"),
			fact("shell_call", "c1", "/t1", "/git_commit"), fact("shell_call_in_write_set", "c1")}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			eng := approvalEngine(t, tc.facts...)
			if got := shellCallAuthorized(t, eng, "c1"); got != tc.want {
				t.Fatalf("shell_call_authorized(c1) = %v, want %v", got, tc.want)
			}
		})
	}
}

// Two calls in one turn are decided apart: the write-set measurement is the
// call's, not the turn's.
func TestTaskAuthorization_EachCallIsDecidedOnItsOwnMeasurement(t *testing.T) {
	defer goleak.VerifyNone(t)
	eng := approvalEngine(t,
		fact("turn_authorization", "/t1", "/commit"),
		fact("shell_call", "good", "/t1", "/git_add"), fact("shell_call_in_write_set", "good"),
		fact("shell_call", "bad", "/t1", "/git_add"),
	)
	if !shellCallAuthorized(t, eng, "good") || shellCallAuthorized(t, eng, "bad") {
		t.Fatal("a call borrowed its sibling's write-set measurement")
	}
}
