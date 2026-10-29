package campaign

import (
	"strings"
	"testing"
)

func TestActionPriority(t *testing.T) {
	d := &EdgeCaseDetector{}
	cases := map[FileAction]int{
		ActionRefactorFirst: 4,
		ActionModularize:    3,
		ActionCreate:        2,
		ActionExtend:        1,
		ActionSkip:          0,
	}
	for action, want := range cases {
		if got := d.actionPriority(action); got != want {
			t.Errorf("actionPriority(%v)=%d, want %d", action, got, want)
		}
	}
}

func TestEdgeCaseAnalysisBlockingAndPrework(t *testing.T) {
	// Modularize files block.
	a := &EdgeCaseAnalysis{ModularizeFiles: []string{"big.go"}}
	if !a.HasBlockingIssues() {
		t.Error("files needing modularization should block")
	}
	// More than 3 refactor files block.
	if !(&EdgeCaseAnalysis{RefactorFiles: []string{"a", "b", "c", "d"}}).HasBlockingIssues() {
		t.Error(">3 refactor files should block")
	}
	// A couple of refactor files with no modularization does not block.
	if (&EdgeCaseAnalysis{RefactorFiles: []string{"a", "b"}}).HasBlockingIssues() {
		t.Error("<=3 refactor files (no modularization) should not block")
	}
	if (&EdgeCaseAnalysis{}).HasBlockingIssues() {
		t.Error("empty analysis should not block")
	}

	tasks := (&EdgeCaseAnalysis{ModularizeFiles: []string{"big.go"}, RefactorFiles: []string{"messy.go"}}).GetPreworkTasks()
	if len(tasks) != 2 {
		t.Fatalf("expected 2 prework tasks, got %d: %v", len(tasks), tasks)
	}
	joined := strings.Join(tasks, "\n")
	if !strings.Contains(joined, "Modularize big.go") || !strings.Contains(joined, "Refactor messy.go") {
		t.Errorf("prework tasks missing expected entries: %v", tasks)
	}
}
